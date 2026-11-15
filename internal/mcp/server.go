package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/seletz/passierschein/internal/client"
	"github.com/seletz/passierschein/internal/protocol"
)

// Names of the tools (D61). The skill text names the same tools.
const (
	// ToolRequestCredentials is the tool that asks for credentials.
	ToolRequestCredentials = "request_credentials"
	// ToolGrantStatus is the tool that lists the grants of the session.
	ToolGrantStatus = "grant_status"
)

// connectTimeout limits the connection and the hello message. A request for
// credentials has no limit here: the human takes the time that he needs.
const connectTimeout = 10 * time.Second

// Dialer connects to the agent socket of the daemon.
type Dialer func(ctx context.Context) (*client.Conn, error)

// Options configures a [Server].
type Options struct {
	// Dial connects to the agent socket. Required.
	Dial Dialer
	// SessionName is the name of the session in the agent tool. Empty means
	// that no name is set. The daemon checks the name.
	SessionName string
	// Version is the version that the server reports to the MCP client.
	Version string
	// Logger receives the log. Nil discards it. The log never holds a
	// secret value.
	Logger *slog.Logger
	// WorkDir is the working directory of "a38 mcp": the default of cwd
	// of run_with_credentials. Empty means the working directory of the
	// process.
	WorkDir string
	// ProjectFor returns the name of the project of a directory without
	// symbolic links (D49). run_with_credentials accepts only a cwd in
	// the project of WorkDir (D64). Nil skips this check; the daemon
	// checks again in each case.
	ProjectFor func(dir string) (project string, ok bool)
	// Environ returns the environment of "a38 mcp": the base of the
	// environment of a command and the PATH for the search of the
	// program. Nil means os.Environ.
	Environ func() []string
}

// Server is the MCP server. Create it with [New].
type Server struct {
	opts Options
	log  *slog.Logger
	mcp  *mcp.Server

	mu   sync.Mutex
	conn *client.Conn
}

// New returns a server with the three tools.
func New(opts Options) *Server {
	s := &Server{opts: opts, log: opts.Logger}
	if s.log == nil {
		s.log = slog.New(slog.DiscardHandler)
	}
	s.mcp = mcp.NewServer(&mcp.Implementation{Name: "a38", Version: opts.Version}, &mcp.ServerOptions{
		Instructions: "a38 asks a human before it gives credentials. Use the skill of a38.",
		Logger:       s.log,
	})
	s.mcp.AddTool(&mcp.Tool{
		Name: ToolRequestCredentials,
		Description: "Ask the human for credentials. Send all references of the task in one request. " +
			"Returns for each reference, in the same order, a value or an error code. " +
			"On an error, stop and tell the human the code.",
		InputSchema: requestSchema,
	}, s.requestCredentials)
	s.mcp.AddTool(&mcp.Tool{
		Name:        ToolGrantStatus,
		Description: "List the grants of this session and how long they last. Never returns a value.",
		InputSchema: json.RawMessage(`{"type":"object","additionalProperties":false}`),
	}, s.grantStatus)
	s.mcp.AddTool(&mcp.Tool{
		Name: ToolRunWithCredentials,
		Description: "Run one command with credentials in its environment. a38 asks the human if necessary. " +
			"Returns exit_code, signal, timed_out, stdout and stderr, never a value. " +
			"An exact value in the output becomes [masked:NAME]; an encoded value is not masked. " +
			"A command that ran is never an error, also with an exit code other than 0. " +
			"On an error, stop and tell the human the code.",
		InputSchema: runSchema,
	}, s.runWithCredentials)
	return s
}

// MCP returns the MCP server of the SDK, for example for a test.
func (s *Server) MCP() *mcp.Server { return s.mcp }

// Run serves MCP on the transport until the client ends the session or ctx
// ends. It closes the connection to the daemon when it returns.
func (s *Server) Run(ctx context.Context, t mcp.Transport) error {
	defer s.Close()
	return s.mcp.Run(ctx, t)
}

// Close closes the connection to the daemon, if there is one.
func (s *Server) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.drop()
}

// requestSchema is the input schema of request_credentials.
var requestSchema = json.RawMessage(`{
  "type": "object",
  "properties": {
    "references": {
      "type": "array",
      "items": {"type": "string"},
      "minItems": 1,
      "description": "One reference for each credential, in the form op://vault/item/field."
    },
    "task": {"type": "string", "description": "What you are doing. One short sentence."},
    "reason": {"type": "string", "description": "Why you need these credentials for the task. One short sentence."}
  },
  "required": ["references"],
  "additionalProperties": false
}`)

// requestArgs are the arguments of request_credentials.
type requestArgs struct {
	References []string `json:"references"`
	Task       string   `json:"task"`
	Reason     string   `json:"reason"`
}

// requestCredentials handles the tool request_credentials.
func (s *Server) requestCredentials(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args requestArgs
	if err := decodeArgs(req.Params.Arguments, &args); err != nil {
		return toolError(protocol.Errorf(protocol.CodeInvalidRequest, "%v", err)), nil
	}
	// Reject early. The daemon stays the authority and checks again.
	if len(args.References) == 0 {
		return toolError(protocol.Errorf(protocol.CodeInvalidRequest, "references must have at least one entry")), nil
	}
	if len(args.References) > protocol.MaxReferences {
		return toolError(protocol.Errorf(protocol.CodeInvalidRequest, "references has more than %d entries", protocol.MaxReferences)), nil
	}
	wire := &protocol.RequestCredentials{Task: args.Task, Reason: args.Reason}
	for i, text := range args.References {
		ref, err := protocol.ParseReference(text)
		if err != nil {
			var perr *protocol.Error
			msg := "the reference is not valid"
			if errors.As(err, &perr) {
				msg = perr.Message
			}
			return toolError(protocol.Errorf(protocol.CodeInvalidRequest, "references[%d]: %s", i, msg)), nil
		}
		wire.References = append(wire.References, ref)
	}
	conn, perr := s.connect(ctx, req.Session)
	if perr != nil {
		return toolError(perr), nil
	}
	creds, err := conn.RequestCredentials(ctx, wire)
	if err != nil {
		return s.failed(conn, err), nil
	}
	return credentialsResult(creds), nil
}

// grantStatus handles the tool grant_status.
func (s *Server) grantStatus(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var none struct{}
	if err := decodeArgs(req.Params.Arguments, &none); err != nil {
		return toolError(protocol.Errorf(protocol.CodeInvalidRequest, "%v", err)), nil
	}
	conn, perr := s.connect(ctx, req.Session)
	if perr != nil {
		return toolError(perr), nil
	}
	grants, err := conn.GrantStatus(ctx)
	if err != nil {
		return s.failed(conn, err), nil
	}
	return grantsResult(grants), nil
}

// decodeArgs reads the arguments strictly: no unknown field.
func decodeArgs(raw json.RawMessage, into any) error {
	if len(raw) == 0 {
		raw = json.RawMessage(`{}`)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(into); err != nil {
		return errors.New("the arguments of the tool are not valid")
	}
	return nil
}

// connect returns the connection to the daemon. It connects and sends hello
// if there is no connection. The error is a *protocol.Error for the agent.
func (s *Server) connect(ctx context.Context, ss *mcp.ServerSession) (*client.Conn, *protocol.Error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conn != nil {
		return s.conn, nil
	}
	cctx, cancel := context.WithTimeout(ctx, connectTimeout)
	defer cancel()
	conn, err := s.opts.Dial(cctx)
	if err != nil {
		s.log.Warn("cannot connect to the daemon", "error", err)
		return nil, s.unavailable("cannot connect to the daemon; the human must start \"a38 serve\"")
	}
	if _, err := conn.Hello(cctx, s.opts.SessionName, agentName(ss)); err != nil {
		_ = conn.Close()
		var perr *protocol.Error
		if errors.As(err, &perr) {
			s.log.Warn("the daemon rejected hello", "code", perr.Code)
			return nil, protocol.Errorf(perr.Code, "the daemon rejected the session: %s", perr.Message)
		}
		s.log.Warn("hello failed", "error", err)
		return nil, s.unavailable("the daemon did not accept the connection")
	}
	s.conn = conn
	return conn, nil
}

// agentName returns the name of the MCP client from the initialize request.
// It is empty if the client sent none.
func agentName(ss *mcp.ServerSession) string {
	if ss == nil {
		return ""
	}
	if p := ss.InitializeParams(); p != nil && p.ClientInfo != nil {
		return p.ClientInfo.Name
	}
	return ""
}

// unavailable returns daemon_unavailable. The text says that a new
// connection is a new session.
func (s *Server) unavailable(what string) *protocol.Error {
	return protocol.Errorf(protocol.CodeDaemonUnavailable,
		"%s. A new connection is a new session: earlier grants are gone", what)
}

// drop closes the connection. Call it with s.mu held.
func (s *Server) drop() {
	if s.conn != nil {
		_ = s.conn.Close()
		s.conn = nil
	}
}

// failed maps the error of a request to a tool error. An error of the
// transport drops the connection, so that the next call connects again.
func (s *Server) failed(conn *client.Conn, err error) *mcp.CallToolResult {
	var perr *protocol.Error
	if errors.As(err, &perr) {
		return toolError(perr)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conn == conn {
		s.drop()
	}
	s.log.Warn("the connection to the daemon failed", "error", err)
	return toolError(s.unavailable("the connection to the daemon ended"))
}

// wireError is the structured form of an error.
type wireError struct {
	Code    protocol.Code `json:"code"`
	Message string        `json:"message"`
}

// toolError returns a tool result with isError. The structured content is
// {"error": {"code", "message"}}; the text content is "code: message".
func toolError(e *protocol.Error) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		IsError:           true,
		StructuredContent: map[string]any{"error": wireError{e.Code, e.Message}},
		Content:           []mcp.Content{&mcp.TextContent{Text: e.Error()}},
	}
}

// entryResult is the structured form of one entry of the answer.
type entryResult struct {
	Reference string     `json:"reference"`
	Value     *string    `json:"value,omitempty"`
	Error     *wireError `json:"error,omitempty"`
}

// credentialsResult builds the result. The secret values are in the result
// of the approved entries only: this is the only place that reveals them.
func credentialsResult(c *protocol.Credentials) *mcp.CallToolResult {
	entries := make([]entryResult, 0, len(c.Entries))
	for _, e := range c.Entries {
		r := entryResult{Reference: "op://" + e.Reference.Vault + "/" + e.Reference.Item + "/" + e.Reference.Field}
		switch {
		case e.Value != nil:
			v := e.Value.Reveal()
			r.Value = &v
		case e.Error != nil:
			r.Error = &wireError{e.Error.Code, e.Error.Message}
		default:
			r.Error = &wireError{protocol.CodeDaemonFaulted, "the daemon sent neither a value nor an error"}
		}
		entries = append(entries, r)
	}
	return structured(map[string]any{"entries": entries})
}

// grantResult is the structured form of one grant. It has no value.
type grantResult struct {
	GrantID          string    `json:"grant_id"`
	References       []string  `json:"references"`
	GrantedAt        time.Time `json:"granted_at"`
	ExpiresAt        time.Time `json:"expires_at"`
	RemainingSeconds int64     `json:"remaining_seconds"`
}

// grantsResult builds the result of grant_status.
func grantsResult(g *protocol.Grants) *mcp.CallToolResult {
	out := make([]grantResult, 0, len(g.Grants))
	for _, gr := range g.Grants {
		refs := make([]string, 0, len(gr.References))
		for _, r := range gr.References {
			refs = append(refs, "op://"+r.Vault+"/"+r.Item+"/"+r.Field)
		}
		out = append(out, grantResult{gr.GrantID, refs, gr.GrantedAt, gr.ExpiresAt, gr.RemainingSeconds})
	}
	return structured(map[string]any{"grants": out})
}

// structured returns a successful result: the structured content and the
// same JSON as text, for a client that reads only the text.
func structured(v map[string]any) *mcp.CallToolResult {
	text, err := json.Marshal(v)
	if err != nil {
		return toolError(protocol.Errorf(protocol.CodeDaemonFaulted, "cannot encode the result"))
	}
	return &mcp.CallToolResult{
		StructuredContent: v,
		Content:           []mcp.Content{&mcp.TextContent{Text: string(text)}},
	}
}
