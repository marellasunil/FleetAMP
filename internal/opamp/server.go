package opamp

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/marellasunil/FleetAMP/internal/agents"
	"github.com/marellasunil/FleetAMP/internal/management"
	"github.com/open-telemetry/opamp-go/protobufs"
	"github.com/open-telemetry/opamp-go/server"
	servertypes "github.com/open-telemetry/opamp-go/server/types"
)

type Adapter struct {
	listenEndpoint string
	events         chan management.Event
	server         server.OpAMPServer
	mu             sync.Mutex
	byConn         map[servertypes.Connection]*agents.ManagedAgent
}

func NewAdapter(listenEndpoint string) *Adapter {
	return &Adapter{
		listenEndpoint: listenEndpoint,
		events:         make(chan management.Event, 128),
		byConn:         make(map[servertypes.Connection]*agents.ManagedAgent),
	}
}

func (a *Adapter) Name() string                    { return "opamp" }
func (a *Adapter) Events() <-chan management.Event { return a.events }

func (a *Adapter) Start(ctx context.Context) error {
	callbacks := servertypes.Callbacks{
		OnConnecting: func(_ *http.Request) servertypes.ConnectionResponse {
			return servertypes.ConnectionResponse{
				Accept: true,
				ConnectionCallbacks: servertypes.ConnectionCallbacks{
					OnConnected:       a.onConnected,
					OnMessage:         a.onMessage,
					OnConnectionClose: a.onConnectionClose,
				},
			}
		},
	}

	a.server = server.New(stdLogger{})
	if err := a.server.Start(server.StartSettings{
		Settings:       server.Settings{Callbacks: callbacks},
		ListenEndpoint: a.listenEndpoint,
		ListenPath:     "/v1/opamp",
	}); err != nil {
		return err
	}
	log.Printf("FleetAMP OpAMP server listening on %s/v1/opamp", a.listenEndpoint)
	<-ctx.Done()
	stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return a.server.Stop(stopCtx)
}

func (a *Adapter) onConnected(_ context.Context, _ servertypes.Connection) {
	log.Printf("OpAMP transport connected")
}

func (a *Adapter) onMessage(_ context.Context, conn servertypes.Connection, msg *protobufs.AgentToServer) *protobufs.ServerToAgent {
	agent := managedAgentFromMessage(msg)
	if agent.InstanceUID == "" {
		log.Printf("OpAMP message ignored: missing/invalid instance UID")
		return nil
	}

	a.mu.Lock()
	previous, existed := a.byConn[conn]
	if existed {
		mergeAgent(agent, previous)
	}
	a.byConn[conn] = cloneManagedAgent(agent)
	a.mu.Unlock()

	if existed && msg.GetHealth() == nil {
		agent.Healthy = previous.Healthy
	}
	eventType := management.EventUpdated
	if !existed {
		eventType = management.EventConnected
	}
	a.events <- management.Event{Type: eventType, Agent: agent}

	if !existed {
		return &protobufs.ServerToAgent{
			InstanceUid: msg.GetInstanceUid(),
			Flags:       uint64(protobufs.ServerToAgentFlags_ServerToAgentFlags_ReportFullState),
			Capabilities: uint64(protobufs.ServerCapabilities_ServerCapabilities_AcceptsStatus) |
				uint64(protobufs.ServerCapabilities_ServerCapabilities_AcceptsEffectiveConfig),
		}
	}
	return nil
}

func (a *Adapter) onConnectionClose(conn servertypes.Connection) {
	a.mu.Lock()
	agent, ok := a.byConn[conn]
	if ok {
		delete(a.byConn, conn)
	}
	a.mu.Unlock()
	if !ok {
		return
	}
	agent = cloneManagedAgent(agent)
	agent.Connected = false
	agent.Touch()
	a.events <- management.Event{Type: management.EventDisconnected, Agent: agent}
	log.Printf("OpAMP agent disconnected: %s", agent.InstanceUID)
}

func managedAgentFromMessage(msg *protobufs.AgentToServer) *agents.ManagedAgent {
	attrs := map[string]string{}
	if d := msg.GetAgentDescription(); d != nil {
		for _, kv := range append(d.GetIdentifyingAttributes(), d.GetNonIdentifyingAttributes()...) {
			if kv == nil || kv.GetValue() == nil {
				continue
			}
			attrs[kv.GetKey()] = anyValueString(kv.GetValue())
		}
	}

	uid := formatInstanceUID(msg.GetInstanceUid())
	name := firstNonEmpty(attrs["service.instance.id"], attrs["host.name"], attrs["service.name"])
	if name == "" && uid != "" {
		name = "otel-collector-" + shortUID(uid)
	}

	healthy := false
	if msg.GetHealth() != nil {
		healthy = msg.GetHealth().GetHealthy()
	}

	agent := &agents.ManagedAgent{
		InstanceUID:  uid,
		Type:         agents.AgentTypeOTelCollector,
		Name:         name,
		Hostname:     attrs["host.name"],
		Version:      attrs["service.version"],
		Connected:    true,
		Healthy:      healthy,
		Attributes:   attrs,
		Capabilities: capabilityNames(msg.GetCapabilities()),
	}
	agent.Deployment = deploymentFromAttributes(attrs)
	agent.Touch()
	return agent
}

func mergeAgent(current, previous *agents.ManagedAgent) {
	if current.Name == "" {
		current.Name = previous.Name
	}
	if current.Hostname == "" {
		current.Hostname = previous.Hostname
	}
	if current.Version == "" {
		current.Version = previous.Version
	}
	if len(current.Attributes) == 0 {
		current.Attributes = previous.Attributes
	}
	if len(current.Capabilities) == 0 {
		current.Capabilities = previous.Capabilities
	}
	if current.Deployment.Runtime == agents.RuntimeUnknown {
		current.Deployment = previous.Deployment
	}
	if current.Labels == nil {
		current.Labels = previous.Labels
	}
}

func deploymentFromAttributes(attrs map[string]string) agents.DeploymentContext {
	d := agents.DeploymentContext{Runtime: agents.RuntimeUnknown}
	if attrs["k8s.cluster.name"] != "" || attrs["k8s.namespace.name"] != "" || attrs["cluster.name"] != "" {
		d.Runtime = agents.RuntimeKubernetes
		d.Cluster = firstNonEmpty(attrs["k8s.cluster.name"], attrs["cluster.name"])
		d.Namespace = attrs["k8s.namespace.name"]
		d.Node = attrs["k8s.node.name"]
	}
	if p := attrs["cloud.provider"]; p != "" {
		d.Provider = p
	}
	return d
}

func capabilityNames(bits uint64) []string {
	caps := []struct {
		bit  uint64
		name string
	}{
		{1, "reports_status"},
		{2, "accepts_remote_config"},
		{4, "reports_effective_config"},
		{8, "accepts_packages"},
		{16, "reports_package_statuses"},
		{32, "reports_own_traces"},
		{64, "reports_own_metrics"},
		{128, "reports_own_logs"},
		{256, "accepts_opamp_connection_settings"},
		{512, "accepts_other_connection_settings"},
		{1024, "accepts_restart_command"},
		{2048, "reports_health"},
		{4096, "reports_remote_config"},
		{8192, "reports_heartbeat"},
		{16384, "reports_available_components"},
		{32768, "reports_connection_settings_status"},
	}
	result := make([]string, 0, len(caps))
	for _, c := range caps {
		if bits&c.bit != 0 {
			result = append(result, c.name)
		}
	}
	return result
}

func anyValueString(v *protobufs.AnyValue) string {
	switch x := v.GetValue().(type) {
	case *protobufs.AnyValue_StringValue:
		return x.StringValue
	case *protobufs.AnyValue_BoolValue:
		return fmt.Sprintf("%t", x.BoolValue)
	case *protobufs.AnyValue_IntValue:
		return fmt.Sprintf("%d", x.IntValue)
	case *protobufs.AnyValue_DoubleValue:
		return fmt.Sprintf("%g", x.DoubleValue)
	case *protobufs.AnyValue_BytesValue:
		return hex.EncodeToString(x.BytesValue)
	default:
		return ""
	}
}

func formatInstanceUID(uid []byte) string {
	if len(uid) != 16 {
		return ""
	}
	h := hex.EncodeToString(uid)
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:32]
}

func shortUID(uid string) string {
	clean := strings.ReplaceAll(uid, "-", "")
	if len(clean) > 8 {
		return clean[:8]
	}
	return clean
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func cloneManagedAgent(agent *agents.ManagedAgent) *agents.ManagedAgent {
	if agent == nil {
		return nil
	}
	clone := *agent
	if agent.Attributes != nil {
		clone.Attributes = make(map[string]string, len(agent.Attributes))
		for k, v := range agent.Attributes {
			clone.Attributes[k] = v
		}
	}
	if agent.Labels != nil {
		clone.Labels = make(map[string]string, len(agent.Labels))
		for k, v := range agent.Labels {
			clone.Labels[k] = v
		}
	}
	clone.Capabilities = append([]string(nil), agent.Capabilities...)
	return &clone
}

type stdLogger struct{}

func (stdLogger) Debugf(_ context.Context, format string, v ...interface{}) {
	log.Printf("opamp: "+format, v...)
}

func (stdLogger) Errorf(_ context.Context, format string, v ...interface{}) {
	log.Printf("opamp error: "+format, v...)
}

var _ management.Adapter = (*Adapter)(nil)
