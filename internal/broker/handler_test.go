package broker_test

import (
	"github.com/seletz/passierschein/internal/broker"
	"github.com/seletz/passierschein/internal/daemon"
)

// The broker is the handler of both sockets.
var (
	_ daemon.AgentHandler = (*broker.Broker)(nil)
	_ daemon.AdminHandler = (*broker.Broker)(nil)
)
