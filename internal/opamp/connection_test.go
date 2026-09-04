package opamp

import (
	"context"
	"net"
	"testing"

	"github.com/marellasunil/FleetAMP/internal/agents"
	"github.com/marellasunil/FleetAMP/internal/management"
	"github.com/open-telemetry/opamp-go/protobufs"
)

type testConnection struct{ id string }

func (c *testConnection) Connection() net.Conn { return nil }
func (c *testConnection) Send(context.Context, *protobufs.ServerToAgent) error {
	return nil
}
func (c *testConnection) Disconnect() error { return nil }

func TestSupersededConnectionCloseDoesNotDisconnectReplacement(t *testing.T) {
	adapter := NewAdapter("", "", nil)
	oldConn := &testConnection{id: "old"}
	newConn := &testConnection{id: "new"}
	agent := &agents.ManagedAgent{
		InstanceUID: "agent-1",
		Connected:   true,
	}
	adapter.byConn[oldConn] = agent
	adapter.byConn[newConn] = agent
	adapter.byUID[agent.InstanceUID] = newConn

	adapter.onConnectionClose(oldConn)

	select {
	case event := <-adapter.Events():
		t.Fatalf("unexpected event: %#v", event)
	default:
	}
	if adapter.byUID[agent.InstanceUID] != newConn {
		t.Fatal("replacement connection index was removed")
	}
}

func TestCurrentConnectionCloseEmitsDisconnected(t *testing.T) {
	adapter := NewAdapter("", "", nil)
	conn := &testConnection{id: "current"}
	agent := &agents.ManagedAgent{
		InstanceUID: "agent-1",
		Connected:   true,
	}
	adapter.byConn[conn] = agent
	adapter.byUID[agent.InstanceUID] = conn

	adapter.onConnectionClose(conn)

	select {
	case event := <-adapter.Events():
		if event.Type != management.EventDisconnected {
			t.Fatalf("event type=%q", event.Type)
		}
		if event.Agent.Connected {
			t.Fatal("disconnect event agent is still connected")
		}
	default:
		t.Fatal("missing disconnect event")
	}
	if _, ok := adapter.byUID[agent.InstanceUID]; ok {
		t.Fatal("current connection index was not removed")
	}
}

func TestLateMessageCannotReclaimReplacementConnection(t *testing.T) {
	adapter := NewAdapter("", "", nil)
	oldConn := &testConnection{id: "old"}
	newConn := &testConnection{id: "new"}
	msg := &protobufs.AgentToServer{
		InstanceUid: []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
	}

	adapter.onMessage(context.Background(), oldConn, msg)
	adapter.onMessage(context.Background(), newConn, msg)
	<-adapter.Events()
	<-adapter.Events()

	adapter.onMessage(context.Background(), oldConn, msg)
	adapter.onConnectionClose(oldConn)

	select {
	case event := <-adapter.Events():
		t.Fatalf("superseded connection emitted event: %#v", event)
	default:
	}
	for _, owner := range adapter.byUID {
		if owner != newConn {
			t.Fatal("late message reclaimed replacement connection")
		}
	}
}
