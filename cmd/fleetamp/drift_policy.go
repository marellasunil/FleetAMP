// Admin configuration drift policy UI and reconciliation worker.
package main

import (
	"context"
	"html/template"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/marellasunil/FleetAMP/internal/configs"
	fleetopamp "github.com/marellasunil/FleetAMP/internal/opamp"
	"github.com/marellasunil/FleetAMP/internal/storage"
)

type driftPolicyPageData struct {
	Page    string
	Policy  configs.DriftPolicy
	Message string
}

var driftPolicyPage = template.Must(template.New("drift-policy").Parse(
	`<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>FleetAMP Drift policy</title><style>` +
		controlPlaneCSS + detailCSS + `</style></head><body><div class="shell">` + sideNav +
		`<main class="main"><header class="top"><div><div class="crumb">FleetAMP / Settings</div><div class="pagetitle">Configuration drift</div><div class="subtitle">Control FleetAMP response to manual Collector configuration changes</div></div></header><div class="content">` +
		`{{if .Message}}<div class="notice">{{.Message}}</div>{{end}}<section class="card"><div class="cardhead"><div><div class="cardtitle">Drift policy</div><div class="cardsub">Applies globally to managed Collectors with an approved desired configuration</div></div></div><div class="cardbody">` +
		`<form method="post" action="/settings/configuration-drift" class="detailform"><label>When drift is detected<select class="select" name="policy"><option value="report_only" {{if eq .Policy "report_only"}}selected{{end}}>Report only</option><option value="enforce" {{if eq .Policy "enforce"}}selected{{end}}>Enforce approved configuration</option></select></label><button class="btn primary" type="submit">Save policy</button></form>` +
		`<div class="notice" style="margin-top:16px"><strong>Report only</strong> records and displays drift without changing the Collector. <strong>Enforce</strong> sends the latest approved desired configuration when a changed effective configuration is reported. Every enforcement is recorded as a reconcile deployment.</div>` +
		`</div></section></div></main></div></body></html>`))

func registerDriftPolicyRoutes(mux *http.ServeMux, store storage.DriftPolicyStore, auth *authManager) {
	mux.HandleFunc("/settings/configuration-drift", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/settings/configuration-drift" {
			http.NotFound(w, r)
			return
		}
		if r.Method == http.MethodPost {
			if err := r.ParseForm(); err != nil {
				http.Error(w, "invalid drift policy request", http.StatusBadRequest)
				return
			}
			policy, err := configs.ParseDriftPolicy(r.FormValue("policy"))
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if err := store.Set(r.Context(), policy); err != nil {
				internalServerError(w, err)
				return
			}
			actor, _ := auth.sessionUsername(r)
			slog.Info("configuration drift policy updated", "component", "config",
				"event", "drift_policy_updated", "actor", actor, "policy", policy)
			message := url.QueryEscape("Configuration drift policy updated.")
			http.Redirect(w, r, "/settings/configuration-drift?message="+message, http.StatusSeeOther)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		policy, err := store.Get(r.Context())
		if err != nil {
			internalServerError(w, err)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := driftPolicyPage.Execute(w, driftPolicyPageData{
			Page: "settings", Policy: policy, Message: r.URL.Query().Get("message"),
		}); err != nil {
			slog.Error("render drift policy page", "component", "http", "error", err)
		}
	})
}
func runDriftReconciler(
	ctx context.Context,
	policyStore storage.DriftPolicyStore,
	assignmentStore storage.AssignmentStore,
	configStore storage.ConfigurationStore,
	deploymentStore storage.DeploymentStore,
	adapter *fleetopamp.Adapter,
) {
	for {
		select {
		case <-ctx.Done():
			return
		case report := <-adapter.EffectiveConfigEvents():
			policy, err := policyStore.Get(ctx)
			if err != nil {
				slog.Error("read drift policy", "component", "config", "error", err)
				continue
			}
			if policy != configs.DriftPolicyEnforce {
				continue
			}
			assignment, err := latestAssignmentForAgent(ctx, assignmentStore, report.AgentInstanceUID)
			if err != nil {
				if err != storage.ErrAssignmentNotFound {
					slog.Error("read desired assignment for drift", "component", "config", "error", err)
				}
				continue
			}
			configuration, err := configStore.Get(ctx, assignment.ConfigurationID)
			if err != nil {
				slog.Error("read desired configuration for drift", "component", "config", "error", err)
				continue
			}
			if configs.CompareDesiredEffective(configuration.Content, report.Content).InSync {
				continue
			}
			_, deployment, err := deliverConfiguration(ctx, report.AgentInstanceUID, configuration,
				configs.DeploymentActionReconcile, assignmentStore, deploymentStore, adapter)
			if err != nil {
				slog.Warn("configuration drift reconciliation failed", "component", "config",
					"event", "drift_reconcile_failed", "agent_uid", report.AgentInstanceUID, "error", err)
				continue
			}
			slog.Info("configuration drift reconciliation sent", "component", "config",
				"event", "drift_reconcile_sent", "agent_uid", report.AgentInstanceUID,
				"configuration_id", configuration.ID, "deployment_id", deployment.ID)
		}
	}
}
