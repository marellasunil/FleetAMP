package main

import (
	"context"
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/marellasunil/FleetAMP/internal/agents"
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
	store := memory.NewAgentStore()
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
				if err := store.Upsert(ctx, event.Agent); err != nil && ctx.Err() == nil {
					log.Printf("store agent %s: %v", event.Agent.InstanceUID, err)
				}
				log.Printf("agent event=%s id=%s name=%s connected=%t healthy=%t",
					event.Type, event.Agent.InstanceUID, event.Agent.Name,
					event.Agent.Connected, event.Agent.Healthy)
			}
		}
	}()

	mux := http.NewServeMux()
	registerHealthRoutes(mux)
	registerAgentRoutes(mux, store)

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

func registerAgentRoutes(mux *http.ServeMux, store interface {
	List(context.Context) ([]*agents.ManagedAgent, error)
}) {
	mux.HandleFunc("/api/v1/agents", func(w http.ResponseWriter, r *http.Request) {
		agentsList, err := store.List(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(agentsList)
	})

	mux.HandleFunc("/agents", func(w http.ResponseWriter, r *http.Request) {
		agentsList, err := store.List(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := agentsPage.Execute(w, agentsList); err != nil {
			log.Printf("render agents page: %v", err)
		}
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/agents", http.StatusTemporaryRedirect)
	})
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
code{color:#c4b5fd}
</style></head><body>
<h1>FleetAMP — Managed Agents</h1><div class="sub">Live in-memory inventory from management adapters.</div>
{{if .}}<table><thead><tr><th>Name</th><th>Type</th><th>Runtime</th><th>Cluster</th><th>Version</th><th>Connected</th><th>Healthy</th><th>Last Seen</th></tr></thead><tbody>
{{range .}}<tr><td>{{.Name}}<br><code>{{.InstanceUID}}</code></td><td>{{.Type}}</td><td>{{.Deployment.Runtime}}</td><td>{{.Deployment.Cluster}}</td><td>{{.Version}}</td><td class="{{if .Connected}}ok{{else}}bad{{end}}">{{.Connected}}</td><td class="{{if .Healthy}}ok{{else}}bad{{end}}">{{.Healthy}}</td><td>{{.LastSeen}}</td></tr>{{end}}
</tbody></table>{{else}}<div class="empty">No managed agents connected yet.</div>{{end}}
</body></html>`))
