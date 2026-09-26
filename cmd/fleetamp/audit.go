// Persistent control-plane audit middleware and Admin-only audit log UI.
package main

import (
	"context"
	"html/template"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/marellasunil/FleetAMP/internal/audit"
	"github.com/marellasunil/FleetAMP/internal/storage"
)

type auditPageData struct {
	Page, Actor, Action, Outcome, Category, From, To string
	Events                                           []*audit.Event
}

var auditPage = template.Must(template.New("audit").Funcs(template.FuncMap{
	"auditTime": func(value time.Time) string { return value.Local().Format("2006-01-02 15:04:05") },
}).Parse(`<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>FleetAMP Audit log</title><style>` +
	controlPlaneCSS + detailCSS + `</style></head><body><div class="shell">` + sideNav +
	`<main class="main"><header class="top"><div><div class="crumb">FleetAMP / Audit</div><div class="pagetitle">Audit log</div><div class="subtitle">Append-only control-plane activity and security history</div></div></header><div class="content">` +
	`<section class="card"><div class="cardbody"><div class="detailactions" style="margin-bottom:14px"><a class="btn" href="/audit-log">All activity</a><a class="btn" href="/audit-log?category=configuration">Configuration</a><a class="btn" href="/audit-log?category=deployment">Deployments</a><a class="btn" href="/audit-log?category=drift">Drift</a><a class="btn" href="/audit-log?category=security">Security & users</a></div><form class="detailform" method="get" action="/audit-log">{{if .Category}}<input type="hidden" name="category" value="{{.Category}}">{{end}}<label>Actor<input class="input" name="actor" value="{{.Actor}}" placeholder="username"></label><label>Action<input class="input" name="action" value="{{.Action}}" placeholder="deployment.approve"></label><label>Outcome<select class="select" name="outcome"><option value="">All</option><option value="success" {{if eq .Outcome "success"}}selected{{end}}>Success</option><option value="failed" {{if eq .Outcome "failed"}}selected{{end}}>Failed</option><option value="denied" {{if eq .Outcome "denied"}}selected{{end}}>Denied</option></select></label><label>From<input class="input" type="date" name="from" value="{{.From}}"></label><label>To<input class="input" type="date" name="to" value="{{.To}}"></label><button class="btn primary" type="submit">Filter</button><a class="btn" href="/audit-log">Clear</a></form></div></section>` +
	`<section class="card" style="margin-top:14px"><div class="cardhead"><div><div class="cardtitle">Recorded events</div><div class="cardsub">{{len .Events}} most recent matching event(s)</div></div><span class="badge ok">Append-only</span></div>{{if .Events}}<div style="overflow:auto"><table><thead><tr><th>Time</th><th>Actor</th><th>Action</th><th>Resource</th><th>Outcome</th><th>Request</th></tr></thead><tbody>{{range .Events}}<tr><td>{{auditTime .Timestamp}}</td><td>{{.Actor}}</td><td><span class="code">{{.Action}}</span></td><td>{{.ResourceType}}{{if .ResourceID}}<div class="tiny code">{{.ResourceID}}</div>{{end}}</td><td>{{if eq .Outcome "success"}}<span class="badge ok">Success</span>{{else if eq .Outcome "denied"}}<span class="badge warn">Denied</span>{{else}}<span class="badge off">Failed</span>{{end}}</td><td><span class="code">{{.HTTPMethod}} {{.Path}}</span><div class="tiny">HTTP {{.StatusCode}}</div></td></tr>{{end}}</tbody></table></div>{{else}}<div class="empty">No audit events match these filters.</div>{{end}}</section></div></main></div></body></html>`))

func registerAuditRoutes(mux *http.ServeMux, store storage.AuditStore) {
	mux.HandleFunc("/audit-log", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/audit-log" {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		filter := audit.Filter{
			Actor: r.URL.Query().Get("actor"), Action: r.URL.Query().Get("action"),
			Outcome: r.URL.Query().Get("outcome"), Limit: 200,
		}
		var err error
		filter.Since, err = parseAuditDate(r.URL.Query().Get("from"), false)
		if err != nil {
			http.Error(w, "invalid from date", http.StatusBadRequest)
			return
		}
		filter.Until, err = parseAuditDate(r.URL.Query().Get("to"), true)
		if err != nil {
			http.Error(w, "invalid to date", http.StatusBadRequest)
			return
		}
		events, err := store.List(r.Context(), filter)
		if err != nil {
			internalServerError(w, err)
			return
		}
		category := strings.TrimSpace(r.URL.Query().Get("category"))
		if category != "" {
			filtered := make([]*audit.Event, 0, len(events))
			for _, event := range events {
				if auditCategoryMatches(category, event.Action) {
					filtered = append(filtered, event)
				}
			}
			events = filtered
		}
		view := auditPageData{
			Page: "audit", Actor: filter.Actor, Action: filter.Action, Category: category,
			Outcome: filter.Outcome, From: r.URL.Query().Get("from"),
			To: r.URL.Query().Get("to"), Events: events,
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := auditPage.Execute(w, view); err != nil {
			slog.Error("render audit log", "component", "http", "error", err)
		}
	})
}

func auditCategoryMatches(category, action string) bool {
	switch category {
	case "configuration":
		return strings.HasPrefix(action, "configuration.")
	case "deployment":
		return strings.HasPrefix(action, "deployment.")
	case "drift":
		return strings.HasPrefix(action, "drift.") || action == "configuration.reconcile"
	case "security":
		return strings.HasPrefix(action, "authentication.") || strings.HasPrefix(action, "user.") || strings.HasPrefix(action, "policy.")
	default:
		return true
	}
}

func parseAuditDate(value string, exclusiveEnd bool) (time.Time, error) {
	if strings.TrimSpace(value) == "" {
		return time.Time{}, nil
	}
	parsed, err := time.ParseInLocation("2006-01-02", value, time.Local)
	if err != nil {
		return time.Time{}, err
	}
	if exclusiveEnd {
		parsed = parsed.AddDate(0, 0, 1)
	}
	return parsed, nil
}

type auditResponseWriter struct {
	http.ResponseWriter
	status int
}

func (w *auditResponseWriter) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *auditResponseWriter) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(body)
}

func auditMiddleware(auth *authManager, store storage.AuditStore, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !auditMethod(r.Method) {
			next.ServeHTTP(w, r)
			return
		}
		actor, _ := auth.sessionUsername(r)
		if actor == "" && (r.URL.Path == "/login" || r.URL.Path == "/setup") {
			_ = r.ParseForm()
			actor = strings.TrimSpace(r.FormValue("username"))
		}
		if actor == "" {
			actor = "anonymous"
		}
		recorder := &auditResponseWriter{ResponseWriter: w}
		next.ServeHTTP(recorder, r)
		if recorder.status == 0 {
			recorder.status = http.StatusOK
		}
		action, resourceType, resourceID := describeAuditAction(r)
		event := &audit.Event{
			Timestamp: time.Now().UTC(), Actor: actor, Action: action,
			ResourceType: resourceType, ResourceID: resourceID,
			Outcome: auditRequestOutcome(r, recorder.status), HTTPMethod: r.Method,
			Path: r.URL.Path, StatusCode: recorder.status,
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := store.Append(ctx, event); err != nil {
			slog.Error("append audit event", "component", "audit", "error", err)
		}
	})
}

func auditMethod(method string) bool {
	return method == http.MethodPost || method == http.MethodPut ||
		method == http.MethodPatch || method == http.MethodDelete
}

func auditRequestOutcome(r *http.Request, status int) string {
	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		return "denied"
	}
	if status >= 400 {
		return "failed"
	}
	if (r.URL.Path == "/login" || r.URL.Path == "/setup") &&
		(status < 300 || status >= 400) {
		return "failed"
	}
	return "success"
}
func describeAuditAction(r *http.Request) (string, string, string) {
	path := strings.Trim(r.URL.Path, "/")
	parts := strings.Split(path, "/")
	resourceType, resourceID := "control_plane", ""
	if len(parts) > 0 && parts[0] != "" {
		resourceType = parts[0]
	}
	if len(parts) > 1 {
		resourceID = parts[1]
	}
	switch r.URL.Path {
	case "/login":
		return "authentication.login", "session", ""
	case "/logout":
		return "authentication.logout", "session", ""
	case "/setup":
		return "authentication.setup", "administrator", ""
	case "/settings/configuration-drift":
		return "policy.drift_update", "configuration_policy", "drift"
	case "/settings/configuration-sections":
		return "policy.section_update", "configuration_policy", r.FormValue("section")
	}
	action := strings.TrimSpace(r.FormValue("action"))
	if strings.HasPrefix(r.URL.Path, "/settings/users") {
		if action == "" {
			action = "create"
		}
		return "user." + action, "user", firstNonEmpty(r.FormValue("username"), resourceID)
	}
	if strings.HasPrefix(r.URL.Path, "/groups/") && action != "" {
		if action == "request_deployment" || action == "approve_deployment" || action == "reject_deployment" {
			return "deployment." + action, "group", resourceID
		}
		if action == "create_configuration" {
			return "configuration.version_create", "group", resourceID
		}
		return "group." + action, "group", resourceID
	}
	if r.URL.Path == "/groups" {
		return "group.create", "group", ""
	}
	if strings.HasSuffix(r.URL.Path, "/configurations") {
		return "configuration.version_create", "agent", resourceID
	}
	if strings.HasSuffix(r.URL.Path, "/rollback") {
		return "configuration.rollback", "agent", resourceID
	}
	if strings.HasSuffix(r.URL.Path, "/config") {
		return "configuration.deploy", "agent", resourceID
	}
	if strings.HasSuffix(r.URL.Path, "/labels") || strings.HasSuffix(r.URL.Path, "/label") {
		return "agent.labels_update", "agent", resourceID
	}
	if strings.HasSuffix(r.URL.Path, "/group") {
		return "agent.group_assign", "agent", resourceID
	}
	return strings.ToLower(r.Method) + "." + strings.ReplaceAll(path, "/", "."), resourceType, resourceID
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}
