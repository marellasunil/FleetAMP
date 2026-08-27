package main

import (
	"context"
	"encoding/json"
	"errors"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/marellasunil/FleetAMP/internal/agents"
	"github.com/marellasunil/FleetAMP/internal/configs"
	fleetopamp "github.com/marellasunil/FleetAMP/internal/opamp"
	"github.com/marellasunil/FleetAMP/internal/storage/memory"
)

type healthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
	Time    string `json:"time"`
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	httpAddr := envOrDefault("FLEETAMP_HTTP_ADDR", ":8080")
	opampAddr := envOrDefault("FLEETAMP_OPAMP_ADDR", ":4320")
	agentStore := memory.NewAgentStore()
	configStore := memory.NewConfigStore()
	assignmentStore := memory.NewAssignmentStore()
	adapter := fleetopamp.NewAdapter(opampAddr)

	go func() {
		if err := adapter.Start(ctx); err != nil && ctx.Err() == nil {
			log.Printf("OpAMP server stopped with error: %v", err)
			stop()
		}
	}()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-adapter.Events():
				if event.Agent == nil {
					continue
				}
				if err := agentStore.Upsert(ctx, event.Agent); err != nil && ctx.Err() == nil {
					log.Printf("store agent %s: %v", event.Agent.InstanceUID, err)
				}
				log.Printf("agent event=%s id=%s name=%s connected=%t healthy=%t",
					event.Type, event.Agent.InstanceUID, event.Agent.Name,
					event.Agent.Connected, event.Agent.Healthy)
			}
		}
	}()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case report := <-adapter.ConfigEvents():
				if err := assignmentStore.UpdateByAgentHash(ctx, report.AgentInstanceUID, report.ConfigurationHash, report.Status, report.Error); err != nil && ctx.Err() == nil {
					log.Printf("update config status agent=%s hash=%s: %v", report.AgentInstanceUID, report.ConfigurationHash, err)
				}
			}
		}
	}()

	mux := http.NewServeMux()
	registerHealthRoutes(mux)
	registerAgentRoutes(mux, agentStore, configStore, assignmentStore, adapter)
	registerConfigRoutes(mux, configStore, assignmentStore, agentStore, adapter)

	httpServer := &http.Server{Addr: httpAddr, Handler: mux}
	go func() {
		log.Printf("FleetAMP HTTP server listening on %s", httpAddr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP server stopped with error: %v", err)
			stop()
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(shutdownCtx)
}

func registerHealthRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(healthResponse{
			Status: "ok", Service: "fleetamp", Time: time.Now().UTC().Format(time.RFC3339),
		})
	})
	mux.HandleFunc("/ready", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ready"}`))
	})
}

func registerAgentRoutes(mux *http.ServeMux, agentStore *memory.AgentStore, configStore *memory.ConfigStore, assignmentStore *memory.AssignmentStore, adapter *fleetopamp.Adapter) {
	mux.HandleFunc("/api/v1/agents", func(w http.ResponseWriter, r *http.Request) {
		agentsList, err := agentStore.List(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, agentsList)
	})

	mux.HandleFunc("/agents", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/agents" {
			http.NotFound(w, r)
			return
		}
		agentsList, err := agentStore.List(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := agentsPage.Execute(w, agentsList); err != nil {
			log.Printf("render agents page: %v", err)
		}
	})

	mux.HandleFunc("/agents/", func(w http.ResponseWriter, r *http.Request) {
		uid := strings.Trim(strings.TrimPrefix(r.URL.Path, "/agents/"), "/")
		if uid == "" || strings.Contains(uid, "/") {
			http.NotFound(w, r)
			return
		}
		agent, err := agentStore.Get(r.Context(), uid)
		if err != nil {
			http.NotFound(w, r)
			return
		}

		view := agentDetailView{Agent: agent, EffectiveConfig: adapter.EffectiveConfig(uid), RemoteConfigSupported: hasCapability(agent.Capabilities, "accepts_remote_config")}
		assignments, _ := assignmentStore.List(r.Context())
		for _, a := range assignments {
			if a.AgentInstanceUID != uid {
				continue
			}
			if view.Assignment == nil || a.UpdatedAt.After(view.Assignment.UpdatedAt) {
				view.Assignment = a
			}
		}
		if view.Assignment != nil {
			view.DesiredConfig, _ = configStore.Get(r.Context(), view.Assignment.ConfigurationID)
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := agentDetailPage.Execute(w, view); err != nil {
			log.Printf("render agent detail page: %v", err)
		}
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/agents", http.StatusTemporaryRedirect)
	})
}

type agentDetailView struct {
	Agent                 *agents.ManagedAgent
	Assignment            *configs.Assignment
	DesiredConfig         *configs.Configuration
	EffectiveConfig       string
	RemoteConfigSupported bool
}

func hasCapability(capabilities []string, wanted string) bool {
	for _, capability := range capabilities {
		if capability == wanted {
			return true
		}
	}
	return false
}

type createConfigurationRequest struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Content     string `json:"content"`
	ContentType string `json:"content_type"`
}

func registerConfigRoutes(mux *http.ServeMux, configStore *memory.ConfigStore, assignmentStore *memory.AssignmentStore, agentStore *memory.AgentStore, adapter *fleetopamp.Adapter) {
	mux.HandleFunc("/api/v1/configurations", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			items, err := configStore.List(r.Context())
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			writeJSON(w, http.StatusOK, items)
		case http.MethodPost:
			var request createConfigurationRequest
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				http.Error(w, "invalid JSON body", http.StatusBadRequest)
				return
			}
			if strings.TrimSpace(request.Name) == "" || strings.TrimSpace(request.Version) == "" || strings.TrimSpace(request.Content) == "" {
				http.Error(w, "name, version and content are required", http.StatusBadRequest)
				return
			}
			configuration := configs.NewConfiguration(request.Name, request.Version, request.Content, request.ContentType)
			if err := configStore.Put(r.Context(), configuration); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			writeJSON(w, http.StatusCreated, configuration)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/v1/assignments", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		items, err := assignmentStore.List(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, items)
	})

	mux.HandleFunc("/api/v1/agents/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/agents/"), "/"), "/")
		if len(parts) != 3 || parts[1] != "configurations" {
			http.NotFound(w, r)
			return
		}
		agentUID, configID := parts[0], parts[2]
		if _, err := agentStore.Get(r.Context(), agentUID); err != nil {
			http.Error(w, "managed agent not found", http.StatusNotFound)
			return
		}
		configuration, err := configStore.Get(r.Context(), configID)
		if err != nil {
			http.Error(w, "configuration not found", http.StatusNotFound)
			return
		}
		assignment := &configs.Assignment{AgentInstanceUID: agentUID, ConfigurationID: configID, ConfigurationHash: configuration.Hash, Status: configs.DeliveryPending, UpdatedAt: time.Now().UTC()}
		_ = assignmentStore.Upsert(r.Context(), assignment)
		err = adapter.SendRemoteConfig(r.Context(), agentUID, configuration)
		if err != nil {
			assignment.Error = err.Error()
			assignment.UpdatedAt = time.Now().UTC()
			if errors.Is(err, fleetopamp.ErrRemoteConfigUnsupported) {
				assignment.Status = configs.DeliveryUnsupported
			} else {
				assignment.Status = configs.DeliveryFailed
			}
			_ = assignmentStore.Upsert(r.Context(), assignment)
			writeJSON(w, http.StatusConflict, assignment)
			return
		}
		assignment.Status = configs.DeliverySent
		assignment.UpdatedAt = time.Now().UTC()
		_ = assignmentStore.Upsert(r.Context(), assignment)
		writeJSON(w, http.StatusAccepted, assignment)
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

var agentsPage = template.Must(template.New("agents").Parse(`<!doctype html>
<html><head><meta charset="utf-8"><title>FleetAMP Agents</title>
<style>
body{font-family:system-ui,sans-serif;background:#0b1220;color:#e5e7eb;margin:0;padding:32px}
h1{margin-bottom:8px}.sub{color:#94a3b8;margin-bottom:24px}
table{width:100%;border-collapse:collapse;background:#111827;border-radius:12px;overflow:hidden}
th,td{padding:14px 16px;text-align:left;border-bottom:1px solid #1f2937}th{color:#93c5fd}
.ok{color:#86efac}.bad{color:#fca5a5}.empty{padding:24px;background:#111827;border-radius:12px;color:#94a3b8}
code{color:#c4b5fd}a{color:#93c5fd;text-decoration:none}a:hover{text-decoration:underline}
</style></head><body>
<h1>FleetAMP — Managed Agents</h1><div class="sub">Live in-memory inventory from management adapters.</div>
{{if .}}<table><thead><tr><th>Name</th><th>Type</th><th>Runtime</th><th>Cluster</th><th>Version</th><th>Connected</th><th>Healthy</th><th>Last Seen</th></tr></thead><tbody>
{{range .}}<tr><td><a href="/agents/{{.InstanceUID}}">{{.Name}}</a><br><code>{{.InstanceUID}}</code></td><td>{{.Type}}</td><td>{{.Deployment.Runtime}}</td><td>{{.Deployment.Cluster}}</td><td>{{.Version}}</td><td class="{{if .Connected}}ok{{else}}bad{{end}}">{{.Connected}}</td><td class="{{if .Healthy}}ok{{else}}bad{{end}}">{{.Healthy}}</td><td>{{.LastSeen}}</td></tr>{{end}}
</tbody></table>{{else}}<div class="empty">No managed agents connected yet.</div>{{end}}
</body></html>`))

var agentDetailPage = template.Must(template.New("agent-detail").Parse(`<!doctype html>
<html><head><meta charset="utf-8"><title>FleetAMP Agent</title><style>
body{font-family:system-ui,sans-serif;background:#0b1220;color:#e5e7eb;margin:0;padding:32px;max-width:1400px}a{color:#93c5fd;text-decoration:none}.back{display:inline-block;margin-bottom:22px}.head{display:flex;justify-content:space-between;gap:24px;align-items:flex-start}.muted{color:#94a3b8}.chips{display:flex;gap:8px;flex-wrap:wrap}.chip{background:#1f2937;border-radius:999px;padding:5px 10px;font-size:13px}.ok{color:#86efac}.bad{color:#fca5a5}.warn{color:#fde68a}.grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(260px,1fr));gap:16px;margin:24px 0}.card{background:#111827;border:1px solid #1f2937;border-radius:12px;padding:18px}.card h2{margin-top:0;font-size:18px}.kv{display:grid;grid-template-columns:130px 1fr;gap:8px 12px}.kv span:nth-child(odd){color:#94a3b8}pre{white-space:pre-wrap;overflow:auto;background:#060b14;border:1px solid #1f2937;border-radius:10px;padding:16px;max-height:520px}code{color:#c4b5fd}.configgrid{display:grid;grid-template-columns:1fr 1fr;gap:16px}@media(max-width:900px){.configgrid{grid-template-columns:1fr}}
</style></head><body>
<a class="back" href="/agents">← Back to agents</a>
<div class="head"><div><h1>{{.Agent.Name}}</h1><div class="muted"><code>{{.Agent.InstanceUID}}</code></div></div><div class="chips"><span class="chip {{if .Agent.Connected}}ok{{else}}bad{{end}}">Connected: {{.Agent.Connected}}</span><span class="chip {{if .Agent.Healthy}}ok{{else}}bad{{end}}">Healthy: {{.Agent.Healthy}}</span></div></div>
<div class="grid"><section class="card"><h2>Overview</h2><div class="kv"><span>Type</span><span>{{.Agent.Type}}</span><span>Version</span><span>{{.Agent.Version}}</span><span>Hostname</span><span>{{.Agent.Hostname}}</span><span>Runtime</span><span>{{.Agent.Deployment.Runtime}}</span><span>Cluster</span><span>{{.Agent.Deployment.Cluster}}</span><span>Last seen</span><span>{{.Agent.LastSeen}}</span></div></section>
<section class="card"><h2>Capabilities</h2><div class="chips">{{range .Agent.Capabilities}}<span class="chip">{{.}}</span>{{end}}</div><p class="{{if .RemoteConfigSupported}}ok{{else}}warn{{end}}">Remote config: {{if .RemoteConfigSupported}}supported{{else}}not advertised by this agent{{end}}</p></section>
<section class="card"><h2>Latest assignment</h2>{{if .Assignment}}<div class="kv"><span>Status</span><span>{{.Assignment.Status}}</span><span>Config ID</span><span><code>{{.Assignment.ConfigurationID}}</code></span><span>Hash</span><span><code>{{.Assignment.ConfigurationHash}}</code></span><span>Updated</span><span>{{.Assignment.UpdatedAt}}</span>{{if .Assignment.Error}}<span>Error</span><span class="bad">{{.Assignment.Error}}</span>{{end}}</div>{{else}}<p class="muted">No FleetAMP configuration has been assigned.</p>{{end}}</section></div>
<div class="configgrid"><section class="card"><h2>Desired configuration</h2>{{if .DesiredConfig}}<p class="muted">{{.DesiredConfig.Name}} · version {{.DesiredConfig.Version}}</p><pre>{{.DesiredConfig.Content}}</pre>{{else}}<p class="muted">No desired FleetAMP configuration.</p>{{end}}</section>
<section class="card"><h2>Effective configuration</h2><p class="muted">Reported by the managed agent through OpAMP.</p>{{if .EffectiveConfig}}<pre>{{.EffectiveConfig}}</pre>{{else}}<p class="muted">No effective configuration has been reported yet.</p>{{end}}</section></div>
</body></html>`))
