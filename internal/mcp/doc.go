// Package mcp is the MCP server of "a38 mcp". It runs in the zone of the
// agent. It offers three tools (D61): request_credentials, grant_status and
// run_with_credentials. It translates them into messages for the agent
// socket of the daemon.
//
// run_with_credentials is the exec way. The server gets the values from
// the daemon, starts the command itself with the values in its
// environment, masks the values in the output and gives the agent only the
// result (D61, D64). The daemon never starts a program.
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
