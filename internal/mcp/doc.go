// Package mcp is the MCP server of "a38 mcp". It runs in the zone of the
// agent. It offers two tools (D41), request_credentials and grant_status,
// and translates them into messages for the agent socket of the daemon.
//
// One [Server] has at most one connection to the daemon at a time. The
// connection is the session (D25). The server connects on the first tool
// call, so that the name of the MCP client from the initialize request is
// available for the hello message (G24). If the daemon is not reachable,
// each tool call ends with the code daemon_unavailable, and the next call
// tries again. A new connection is a new session; the grants of the old
// session are gone.
//
// The package must not import the broker, a backend or the audit log: the
// agent zone has no code for them (D2, T13). A test checks the rule.
package mcp
