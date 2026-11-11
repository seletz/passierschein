package daemon

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/seletz/passierschein/internal/client"
	"github.com/seletz/passierschein/internal/config"
	"github.com/seletz/passierschein/internal/protocol"
)

// rawConn is a connection that sends lines without the client package, so
// that a test can send invalid messages.
type rawConn struct {
	c net.Conn
	r *bufio.Reader
}

// rawDial connects to a socket.
func rawDial(t *testing.T, path string) *rawConn {
	t.Helper()
	c, err := net.Dial("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	_ = c.SetDeadline(time.Now().Add(testTimeout))
	return &rawConn{c: c, r: bufio.NewReader(c)}
}

// rawAgent connects to the agent socket and completes the hello.
func rawAgent(t *testing.T, r *running, name string) *rawConn {
	t.Helper()
	conn := rawDial(t, r.srv.AgentSocket())
	hello := fmt.Sprintf(`{"version":1,"type":"hello","session_name":%q}`, name)
	if _, ok := conn.roundTrip(t, hello).(*protocol.Welcome); !ok {
		t.Fatal("no welcome")
	}
	return conn
}

// send writes raw bytes.
func (c *rawConn) send(t *testing.T, data string) {
	t.Helper()
	// The daemon can close the connection before it has read all data,
	// for example after an oversized message. The response shows the
	// result; the write error does not.
	_, _ = io.WriteString(c.c, data)
}

// read reads one response. It returns nil at the end of the connection.
func (c *rawConn) read(t *testing.T, set protocol.Set) protocol.Message {
	t.Helper()
	line, err := protocol.ReadLine(c.r)
	if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
		return nil
	}
	if err != nil {
		t.Fatalf("read error = %v", err)
	}
	msg, err := set.Decode(line)
	if err != nil {
		t.Fatalf("the daemon sent an invalid response: %v", err)
	}
	return msg
}

// roundTrip sends one line on the agent socket and reads the response.
func (c *rawConn) roundTrip(t *testing.T, line string) protocol.Message {
	t.Helper()
	return c.roundTripSet(t, line, protocol.AgentResponses)
}

// roundTripSet sends one line and reads the response of the set.
func (c *rawConn) roundTripSet(t *testing.T, line string, set protocol.Set) protocol.Message {
	t.Helper()
	c.send(t, line+"\n")
	return c.read(t, set)
}

// wantErrorResponse checks that the message is an error with the code.
func wantErrorResponse(t *testing.T, m protocol.Message, want protocol.Code) *protocol.ErrorResponse {
	t.Helper()
	e, ok := m.(*protocol.ErrorResponse)
	if !ok {
		t.Fatalf("response = %#v, want an error with the code %s", m, want)
	}
	if e.Error.Code != want {
		t.Fatalf("error code = %s, want %s", e.Error.Code, want)
	}
	return e
}

func TestInvalidMessages(t *testing.T) {
	// T10: each invalid message ends with invalid_request or with the end
	// of the connection. The daemon serves other sessions after each case.
	const valid = `{"version":1,"type":"request_credentials","id":"r-9","references":["op://beispiel.app.k8s-dev/datenbank-dev/password"]}`
	tests := []struct {
		name string
		// before says whether the case runs before or after hello.
		beforeHello bool
		data        string
		wantID      string
		// wantClose says that the connection ends after the error.
		wantClose bool
		// noResponse says that the connection ends without an answer.
		noResponse bool
	}{
		{name: "unknown type", data: `{"version":1,"type":"execute","id":"r-1"}` + "\n", wantID: "r-1"},
		{name: "unknown field", data: `{"version":1,"type":"grant_status","id":"r-2","command":"x"}` + "\n", wantID: "r-2"},
		{name: "wrong version", data: `{"version":2,"type":"grant_status","id":"r-3"}` + "\n", wantID: "r-3"},
		{name: "version as text", data: `{"version":"1","type":"grant_status","id":"r-4"}` + "\n", wantID: "r-4"},
		{name: "no version", data: `{"type":"grant_status","id":"r-5"}` + "\n", wantID: "r-5"},
		{name: "not JSON", data: "beispiel\n"},
		{name: "JSON array", data: "[1,2]\n"},
		{name: "empty line", data: "\n"},
		{name: "upper-case key", data: `{"version":1,"type":"grant_status","ID":"r-6"}` + "\n"},
		{name: "key twice", data: `{"version":1,"type":"grant_status","id":"r-7","id":"r-8"}` + "\n", wantID: "r-8"},
		{name: "invalid reference", data: `{"version":1,"type":"request_credentials","id":"r-9","references":["op://beispiel/x"]}` + "\n", wantID: "r-9"},
		{name: "invalid correlation ID", data: `{"version":1,"type":"grant_status","id":"r 1"}` + "\n"},
		{name: "invalid UTF-8", data: "{\"version\":1,\"type\":\"grant_status\",\"id\":\"\xff\"}\n"},
		{name: "oversized message", data: `{"version":1,"type":"grant_status","id":"` + strings.Repeat("a", protocol.MaxMessageSize) + "\"}\n", wantClose: true},
		{name: "oversized message without line end", data: strings.Repeat("a", 2*protocol.MaxMessageSize), wantClose: true},
		{name: "closed in the middle of a message", data: `{"version":1,"type":"grant_st`, noResponse: true},
		{name: "unknown type before hello", beforeHello: true, data: `{"version":1,"type":"execute"}` + "\n", wantClose: true},
		{name: "request before hello", beforeHello: true, data: valid + "\n", wantID: "r-9", wantClose: true},
		{name: "not JSON before hello", beforeHello: true, data: "beispiel\n", wantClose: true},
		{name: "closed in the middle of hello", beforeHello: true, data: `{"version":1,"type":"hel`, noResponse: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			broker := newFakeBroker()
			r := startServer(t, testConfig(t, nil), broker)
			// A second session runs during the case.
			other, _ := r.agent(t, "beispiel-koordinator")

			var conn *rawConn
			if tt.beforeHello {
				conn = rawDial(t, r.srv.AgentSocket())
			} else {
				conn = rawAgent(t, r, "beispiel-test")
			}
			conn.send(t, tt.data)
			if tt.noResponse {
				_ = conn.c.(*net.UnixConn).CloseWrite()
				if m := conn.read(t, protocol.AgentResponses); m != nil {
					t.Fatalf("response = %#v, want the end of the connection", m)
				}
			} else {
				e := wantErrorResponse(t, conn.read(t, protocol.AgentResponses), protocol.CodeInvalidRequest)
				if e.ID != tt.wantID {
					t.Errorf("error ID = %q, want %q", e.ID, tt.wantID)
				}
				if strings.Contains(e.Error.Message, "beispiel") {
					t.Errorf("the error text repeats the content of the line: %q", e.Error.Message)
				}
				if tt.wantClose {
					if m := conn.read(t, protocol.AgentResponses); m != nil {
						t.Fatalf("response = %#v, want the end of the connection", m)
					}
				} else {
					// The session continues.
					if _, ok := conn.roundTrip(t, valid).(*protocol.Credentials); !ok {
						t.Fatal("the session does not continue after the error")
					}
				}
			}
			if _, err := request(t, other, "datenbank-dev"); err != nil {
				t.Errorf("the other session does not work: %v", err)
			}
			r.wantWorks(t)
		})
	}
}

// memConn is a connection in memory for the fuzz test. The reader gives the
// input; the writer collects the responses.
type memConn struct {
	io.Reader
	mu  sync.Mutex
	out bytes.Buffer
}

func (c *memConn) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.out.Write(p)
}

func FuzzAgentConn(f *testing.F) {
	// T10: the connection loop does not crash for any input, each response
	// is a valid message, and SessionEnded is called once for each
	// session.
	hello := `{"version":1,"type":"hello","session_name":"beispiel-koordinator","agent":"coding-agent"}` + "\n"
	seeds := []string{
		hello,
		hello + `{"version":1,"type":"request_credentials","id":"r-1","references":["op://beispiel.app.k8s-dev/datenbank-dev/password"]}` + "\n",
		hello + `{"version":1,"type":"grant_status","id":"r-2"}` + "\n" + hello,
		hello + `{"version":1,"type":"grants_list","id":"r-3"}` + "\n",
		hello + "beispiel\n\n[]\n{}\n",
		`{"version":1,"type":"hello","session_name":"beispiel\u001b"}` + "\n",
		`{"version":1,"type":"grant_status","id":"r-4"}` + "\n",
		hello + `{"version":1,"type":"request_credentials","id":"r-5","references":[]}`,
		"\xff\xfe",
		"",
	}
	for _, s := range seeds {
		f.Add([]byte(s))
	}
	cfg := testConfigNoDir(f)
	f.Fuzz(func(t *testing.T, input []byte) {
		broker := newFakeBroker()
		srv, err := New(Options{Config: cfg, Agent: broker, Admin: broker})
		if err != nil {
			t.Fatal(err)
		}
		conn := &memConn{Reader: bytes.NewReader(input)}
		srv.serveAgent(context.Background(), conn, Peer{PID: 1, UID: os.Getuid()})

		welcomes := 0
		sc := bufio.NewScanner(bytes.NewReader(conn.out.Bytes()))
		sc.Buffer(nil, protocol.MaxMessageSize+1)
		for sc.Scan() {
			msg, err := protocol.AgentResponses.Decode(sc.Bytes())
			if err != nil {
				t.Fatalf("the daemon sent an invalid response: %v", err)
			}
			if _, ok := msg.(*protocol.Welcome); ok {
				welcomes++
			}
		}
		if welcomes > 1 {
			t.Fatalf("welcomes = %d, want at most 1", welcomes)
		}
		broker.mu.Lock()
		defer broker.mu.Unlock()
		ended := 0
		for _, n := range broker.ended {
			ended += n
		}
		if ended != welcomes {
			t.Fatalf("SessionEnded calls = %d, want %d", ended, welcomes)
		}
		if srv.reg.count() != 0 {
			t.Fatal("the registry keeps a session after the end of the connection")
		}
	})
}

// testConfigNoDir returns a configuration for a test that uses no socket.
func testConfigNoDir(tb testing.TB) *config.Config {
	tb.Helper()
	cfg, err := config.Parse([]byte("version: 1\npaths:\n  socket_dir: /tmp/a38-fuzz\n  audit_file: /tmp/a38-fuzz/audit.db\n"))
	if err != nil {
		tb.Fatal(err)
	}
	return cfg
}

// helperResult is the output of the helper process.
type helperResult struct {
	pid     int
	project string
}

// runHelper runs the test binary as a helper process in dir. The helper
// connects to the agent socket, claims a project in its session name and
// sends one request.
func runHelper(t *testing.T, dir, socket, claim string) helperResult {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, exe, "-test.run=^TestHelperProcess$")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "A38_TEST_HELPER_SOCKET="+socket, "A38_TEST_HELPER_CLAIM="+claim)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("helper process error = %v, output:\n%s", err, out)
	}
	for _, line := range strings.Split(string(out), "\n") {
		if rest, ok := strings.CutPrefix(line, "helper "); ok {
			fields := strings.SplitN(rest, " ", 2)
			pid, _ := strconv.Atoi(fields[0])
			project := ""
			if len(fields) == 2 {
				project = fields[1]
			}
			if pid != cmd.Process.Pid {
				t.Fatalf("helper PID = %d, want %d", pid, cmd.Process.Pid)
			}
			return helperResult{pid: pid, project: project}
		}
	}
	t.Fatalf("no result from the helper process:\n%s", out)
	return helperResult{}
}

// TestHelperProcess is the helper process of runHelper. It does nothing in
// a normal test run.
func TestHelperProcess(t *testing.T) {
	socket := os.Getenv("A38_TEST_HELPER_SOCKET")
	if socket == "" {
		t.Skip("helper process only")
	}
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	c, err := client.DialAgent(ctx, socket)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()
	w, err := c.Hello(ctx, os.Getenv("A38_TEST_HELPER_CLAIM"), "coding-agent")
	if err != nil {
		t.Fatal(err)
	}
	ref, _ := protocol.ParseReference("op://beispiel.app.k8s-dev/datenbank-dev/password")
	claim := "project " + os.Getenv("A38_TEST_HELPER_CLAIM")
	if _, err := c.RequestCredentials(ctx, &protocol.RequestCredentials{References: []protocol.Reference{ref}, Task: claim, Reason: claim}); err != nil {
		t.Fatal(err)
	}
	fmt.Printf("helper %d %s\n", os.Getpid(), w.Project)
}
