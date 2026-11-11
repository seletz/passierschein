// Package protocol defines the wire protocol of the a38 daemon.
//
// The daemon has two Unix sockets: the agent socket and the admin socket.
// Each message on a socket is one JSON object on one line. Each message has
// a version number and a type. The set of message types is fixed. There is
// no generic RPC.
//
// The package rejects an unknown type, an unknown field, a wrong version and
// a message above the size limit. It has no dependency on other packages of
// a38, so each component can import it.
//
// The package also defines the shared vocabulary of all components: the
// stable error codes ([Code]), the credential reference ([Reference]) and
// the container for a secret value ([Secret]).
package protocol
