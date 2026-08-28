// OpenTelemetry OpAMP management adapter for FleetAMP.
//
// Purpose:
//   Accepts OpAMP WebSocket connections, normalizes agent state into
//   ManagedAgent events, caches effective configuration, and delivers remote
//   configuration only when the agent advertises AcceptsRemoteConfig.
//
// Runtime flow:
//   OpAMP Agent/Supervisor <-> opamp-go server -> Adapter -> FleetAMP events.
//   FleetAMP Configuration -> Adapter -> OpAMP RemoteConfig -> status report.
//
// Main dependencies:
//   github.com/open-telemetry/opamp-go plus FleetAMP agents/configs/management.
//
// Design constraints:
//   Missing partial-state fields must not erase previously known health/state.
//   Protocol capability checks are enforced before remote-control operations.

package opamp

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/marellasunil/FleetAMP/internal/agents"
	"github.com/marellasunil/FleetAMP/internal/configs"
	"github.com/marellasunil/FleetAMP/internal/management"
	"github.com/open-telemetry/opamp-go/protobufs"
	"github.com/open-telemetry/opamp-go/server"
	servertypes "github.com/open-telemetry/opamp-go/server/types"
)

type Adapter struct {
	listenEndpoint string
	events         chan management.Event
	configEvents   chan configs.StatusReport
	server         server.OpAMPServer
	mu             sync.Mutex
	byConn         map[servertypes.Connection]*agents.ManagedAgent
	byUID          map[string]servertypes.Connection
	effective      map[string]string
}

// NewAdapter creates an OpAMP adapter bound to the configured WebSocket listener.
func NewAdapter(listenEndpoint string) *Adapter {
	return &Adapter{
		listenEndpoint: listenEndpoint,
		events:         make(chan management.Event, 128),
		configEvents:   make(chan configs.StatusReport, 128),
		byConn:         make(map[servertypes.Connection]*agents.ManagedAgent),
		byUID:          make(map[string]servertypes.Connection),
		effective:      make(map[string]string),
	}
}

func (a *Adapter) Name() string                              { return "opamp" }
func (a *Adapter) Events() <-chan management.Event           { return a.events }
func (a *Adapter) ConfigEvents() <-chan configs.StatusReport { return a.configEvents }

// Start runs the opamp-go server until context cancellation or a listener error.
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

// onMessage converts incremental OpAMP messages into a complete normalized agent
// view by merging omitted fields with the last state seen on this connection.
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
		if msg.GetHealth() == nil {
			agent.Healthy = previous.Healthy
		}
	}
	a.byConn[conn] = cloneManagedAgent(agent)
	a.byUID[agent.InstanceUID] = conn
	if effective := effectiveConfigString(msg.GetEffectiveConfig()); effective != "" {
		a.effective[agent.InstanceUID] = effective
	}
	a.mu.Unlock()
	eventType := management.EventUpdated
	if !existed {
		eventType = management.EventConnected
	}
	a.events <- management.Event{Type: eventType, Agent: agent}

	if status := msg.GetRemoteConfigStatus(); status != nil && len(status.GetLastRemoteConfigHash()) > 0 {
		a.configEvents <- configs.StatusReport{
			AgentInstanceUID:  agent.InstanceUID,
			ConfigurationHash: hex.EncodeToString(status.GetLastRemoteConfigHash()),
			Status:            remoteConfigStatus(status.GetStatus()),
			Error:             status.GetErrorMessage(),
			UpdatedAt:         time.Now().UTC(),
		}
	}

	if !existed {
		return &protobufs.ServerToAgent{
			InstanceUid: msg.GetInstanceUid(),
			Flags:       uint64(protobufs.ServerToAgentFlags_ServerToAgentFlags_ReportFullState),
			Capabilities: uint64(protobufs.ServerCapabilities_ServerCapabilities_AcceptsStatus) |
				uint64(protobufs.ServerCapabilities_ServerCapabilities_AcceptsEffectiveConfig) |
				uint64(protobufs.ServerCapabilities_ServerCapabilities_OffersRemoteConfig),
		}
	}
	return nil
}

// EffectiveConfig returns the latest effective configuration text reported by an
// agent. An empty string means no effective configuration body has been received.
func (a *Adapter) EffectiveConfig(instanceUID string) string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.effective[instanceUID]
}

func effectiveConfigString(e *protobufs.EffectiveConfig) string {
	if e == nil || e.GetConfigMap() == nil {
		return ""
	}
	var b strings.Builder
	for name, f := range e.GetConfigMap().GetConfigMap() {
		if f == nil {
			continue
		}
		if name != "" {
			fmt.Fprintf(&b, "# %s\n", name)
		}
		b.Write(f.GetBody())
		if !strings.HasSuffix(b.String(), "\n") {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func (a *Adapter) onConnectionClose(conn servertypes.Connection) {
	a.mu.Lock()
	agent, ok := a.byConn[conn]
	if ok {
		delete(a.byConn, conn)
		if current, exists := a.byUID[agent.InstanceUID]; exists && current == conn {
			delete(a.byUID, agent.InstanceUID)
		}
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
	name := firstNonEmpty(attrs["host.name"], attrs["service.name"], attrs["service.instance.id"])
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

// mergeAgent preserves previously known values when an OpAMP incremental message
// omits fields such as health, metadata, or capabilities.
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

// deploymentFromAttributes maps protocol-reported metadata into FleetAMP deployment
// context without requiring Kubernetes/cloud SDK dependencies.
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

var ErrRemoteConfigUnsupported = errors.New("agent does not advertise accepts_remote_config")
var ErrAgentNotConnected = errors.New("agent is not connected")

// SendRemoteConfig offers one immutable FleetAMP configuration to a connected agent.
func (a *Adapter) SendRemoteConfig(ctx context.Context, instanceUID string, config *configs.Configuration) error {
	a.mu.Lock()
	conn, ok := a.byUID[instanceUID]
	agent := a.byConn[conn]
	a.mu.Unlock()
	if !ok || agent == nil {
		return ErrAgentNotConnected
	}
	if !hasCapability(agent.Capabilities, "accepts_remote_config") {
		return ErrRemoteConfigUnsupported
	}
	uid, err := parseInstanceUID(instanceUID)
	if err != nil {
		return err
	}
	hash, err := hex.DecodeString(config.Hash)
	if err != nil {
		return fmt.Errorf("decode configuration hash: %w", err)
	}
	name := config.Name
	if name == "" {
		name = "fleetamp.yaml"
	}
	message := &protobufs.ServerToAgent{
		InstanceUid:  uid,
		Capabilities: uint64(protobufs.ServerCapabilities_ServerCapabilities_OffersRemoteConfig),
		RemoteConfig: &protobufs.AgentRemoteConfig{
			Config: &protobufs.AgentConfigMap{ConfigMap: map[string]*protobufs.AgentConfigFile{
				name: {Body: []byte(config.Content), ContentType: config.ContentType},
			}},
			ConfigHash: hash,
		},
	}
	return conn.Send(ctx, message)
}

func remoteConfigStatus(status protobufs.RemoteConfigStatuses) configs.DeliveryStatus {
	switch status {
	case protobufs.RemoteConfigStatuses_RemoteConfigStatuses_APPLIED:
		return configs.DeliveryApplied
	case protobufs.RemoteConfigStatuses_RemoteConfigStatuses_APPLYING:
		return configs.DeliveryApplying
	case protobufs.RemoteConfigStatuses_RemoteConfigStatuses_FAILED:
		return configs.DeliveryFailed
	default:
		return configs.DeliverySent
	}
}

func hasCapability(capabilities []string, wanted string) bool {
	for _, capability := range capabilities {
		if capability == wanted {
			return true
		}
	}
	return false
}

func parseInstanceUID(value string) ([]byte, error) {
	clean := strings.ReplaceAll(value, "-", "")
	decoded, err := hex.DecodeString(clean)
	if err != nil || len(decoded) != 16 {
		return nil, fmt.Errorf("invalid instance UID %q", value)
	}
	return decoded, nil
}

type stdLogger struct{}

func (stdLogger) Debugf(_ context.Context, format string, v ...interface{}) {
	log.Printf("opamp: "+format, v...)
}

func (stdLogger) Errorf(_ context.Context, format string, v ...interface{}) {
	log.Printf("opamp error: "+format, v...)
}

var _ management.Adapter = (*Adapter)(nil)
