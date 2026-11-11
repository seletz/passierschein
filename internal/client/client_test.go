package client

import (
	"bufio"
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/seletz/passierschein/internal/protocol"
)

// fakeDaemon accepts one connection and runs serve on it.
func fakeDaemon(t *testing.T, serve func(r *bufio.Reader, c net.Conn)) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "a38-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	path := filepath.Join(dir, "agent.sock")
	l, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Close() })
	go func() {
		c, err := l.Accept()
		if err != nil {
			return
		}
		defer func() { _ = c.Close() }()
		serve(bufio.NewReader(c), c)
	}()
	return path
}

func ctx(t *testing.T) context.Context {
	t.Helper()
	c, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	return c
}

func TestHelloAndOutOfOrderResponses(t *testing.T) {
	// The client matches each response by its correlation ID, also if the
	// daemon answers in a different sequence.
	path := fakeDaemon(t, func(r *bufio.Reader, c net.Conn) {
		line, _ := protocol.ReadLine(r)
		if m, err := protocol.AgentRequests.Decode(line); err != nil || m.MessageType() != protocol.TypeHello {
			return
		}
		_ = protocol.Write(c, &protocol.Welcome{SessionID: "s-1", Project: "beispielprojekt"})
		var ids []string
		for range 2 {
			line, _ := protocol.ReadLine(r)
			m, err := protocol.AgentRequests.Decode(line)
			if err != nil {
				return
			}
			ids = append(ids, m.(*protocol.GrantStatus).ID)
		}
		_ = protocol.Write(c, &protocol.ErrorResponse{ID: ids[1], Error: *protocol.Errorf(protocol.CodeSessionNameConflict, "conflict")})
		_ = protocol.Write(c, &protocol.Grants{ID: ids[0], Grants: []protocol.Grant{}})
	})
	c, err := DialAgent(ctx(t), path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()
	w, err := c.Hello(ctx(t), "beispiel-koordinator", "coding-agent")
	if err != nil || w.SessionID != "s-1" || w.Project != "beispielprojekt" {
		t.Fatalf("Hello() = %+v, %v", w, err)
	}
	first := make(chan error, 1)
	go func() {
		_, err := c.GrantStatus(ctx(t))
		first <- err
	}()
	// Make sure that the first request is sent first.
	time.Sleep(50 * time.Millisecond)
	_, err = c.GrantStatus(ctx(t))
	if protocol.CodeOf(err) != protocol.CodeSessionNameConflict {
		t.Errorf("second request error = %v, want session_name_conflict", err)
	}
	if err := <-first; err != nil {
		t.Errorf("first request error = %v", err)
	}
}

func TestHelloRejected(t *testing.T) {
	path := fakeDaemon(t, func(r *bufio.Reader, c net.Conn) {
		_, _ = protocol.ReadLine(r)
		_ = protocol.Write(c, &protocol.ErrorResponse{Error: *protocol.Errorf(protocol.CodeInvalidRequest, "the session name is not valid")})
	})
	c, err := DialAgent(ctx(t), path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()
	if _, err := c.Hello(ctx(t), "beispiel-koordinator", ""); protocol.CodeOf(err) != protocol.CodeInvalidRequest {
		t.Errorf("Hello() error = %v, want invalid_request", err)
	}
}

func TestConnectionEnds(t *testing.T) {
	// An open request ends with ErrClosed when the daemon closes the
	// connection.
	path := fakeDaemon(t, func(r *bufio.Reader, _ net.Conn) {
		_, _ = protocol.ReadLine(r)
	})
	c, err := DialAdmin(ctx(t), path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()
	if _, err := c.Status(ctx(t)); !errors.Is(err, ErrClosed) {
		t.Errorf("Status() error = %v, want ErrClosed", err)
	}
	if _, err := c.GrantsList(ctx(t)); !errors.Is(err, ErrClosed) {
		t.Errorf("GrantsList() after the end error = %v, want ErrClosed", err)
	}
}

func TestContextEnds(t *testing.T) {
	path := fakeDaemon(t, func(r *bufio.Reader, _ net.Conn) {
		_, _ = protocol.ReadLine(r)
		_, _ = protocol.ReadLine(r)
	})
	c, err := DialAdmin(ctx(t), path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()
	short, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := c.FaultClear(short); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("FaultClear() error = %v, want the deadline", err)
	}
}

func TestDialFails(t *testing.T) {
	if _, err := DialAgent(ctx(t), "/tmp/a38-missing/agent.sock"); err == nil {
		t.Error("DialAgent() of a missing socket error = nil")
	}
}
