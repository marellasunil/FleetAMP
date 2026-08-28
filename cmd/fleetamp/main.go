// FleetAMP application entry point.
//
// Purpose:
//   Wires the management protocol adapter, stores, lifecycle/event handling,
//   REST API, and lightweight web UI into one FleetAMP server process.
//
// Runtime flow:
//   OpAMP client -> internal/opamp.Adapter -> ManagedAgent/config events
//   -> stores -> lifecycle/history -> REST API and /agents UI.
//
// Main dependencies:
//   internal/opamp, internal/agents, internal/configs, internal/events,
//   internal/storage/memory, internal/storage/file, and internal/storage/sqlite.
//
// Configuration:
//   FLEETAMP_HTTP_ADDR, FLEETAMP_OPAMP_ADDR, FLEETAMP_DATA_DIR,
//   FLEETAMP_RETIRE_AFTER, FLEETAMP_DATABASE_PATH, and FLEETAMP_OTELCOL_BINARY
//   control listeners, persistence, lifecycle retirement, and validation.
//
// Design note:
//   Protocol-specific OpAMP types stay behind the adapter boundary; core
//   FleetAMP code works with protocol-independent domain models.

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/marellasunil/FleetAMP/internal/agents"
	"github.com/marellasunil/FleetAMP/internal/configs"
	"github.com/marellasunil/FleetAMP/internal/events"
	"github.com/marellasunil/FleetAMP/internal/groups"
	fleetopamp "github.com/marellasunil/FleetAMP/internal/opamp"
	"github.com/marellasunil/FleetAMP/internal/storage"
	filestore "github.com/marellasunil/FleetAMP/internal/storage/file"
	"github.com/marellasunil/FleetAMP/internal/storage/memory"
	sqlitestore "github.com/marellasunil/FleetAMP/internal/storage/sqlite"
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
	dataDir := envOrDefault("FLEETAMP_DATA_DIR", "./data")
	eventStore, err := filestore.NewEventStore(dataDir)
	if err != nil {
		log.Fatalf("initialize event store: %v", err)
	}
	if err := loadAgentSnapshot(ctx, agentStore, dataDir); err != nil {
		log.Fatalf("load agent snapshot: %v", err)
	}
	retireAfter := durationEnvOrDefault("FLEETAMP_RETIRE_AFTER", 24*time.Hour)
	databasePath := envOrDefault("FLEETAMP_DATABASE_PATH", dataDir+"/fleetamp.db")
	database, err := sqlitestore.Open(ctx, databasePath)
	if err != nil {
		log.Fatalf("initialize sqlite database: %v", err)
	}
	defer database.Close()
	configStore := database.Configurations()
	assignmentStore := database.Assignments()
	deploymentStore := database.Deployments()
	groupStore := database.Groups()
	configValidator := configs.NewValidator(os.Getenv("FLEETAMP_OTELCOL_BINARY"))
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
				now := time.Now().UTC()
				previous, _ := agentStore.Get(ctx, event.Agent.InstanceUID)
				switch event.Type {
				case "connected", "updated":
					event.Agent.Status = agents.LifecycleConnected
					event.Agent.DisconnectedAt = nil
					event.Agent.RetiredAt = nil
				case "disconnected":
					event.Agent.Status = agents.LifecycleDisconnected
					event.Agent.Healthy = false
					event.Agent.DisconnectedAt = &now
				}
				if previous != nil {
					event.Agent.Labels = copyStringMap(previous.Labels)
					event.Agent.GroupFields = copyStringMap(previous.GroupFields)
				}
				if previous != nil && !previous.FirstSeen.IsZero() {
					event.Agent.FirstSeen = previous.FirstSeen
				} else if event.Agent.FirstSeen.IsZero() {
					event.Agent.FirstSeen = now
				}
				if err := agentStore.Upsert(ctx, event.Agent); err != nil && ctx.Err() == nil {
					log.Printf("store agent %s: %v", event.Agent.InstanceUID, err)
				} else if err := saveAgentSnapshot(ctx, agentStore, dataDir); err != nil && ctx.Err() == nil {
					log.Printf("save agent snapshot: %v", err)
				}
				if event.Type == "connected" || event.Type == "disconnected" {
					_ = eventStore.Append(ctx, &events.AgentEvent{AgentInstanceUID: event.Agent.InstanceUID, AgentName: event.Agent.Name, Type: events.Type(event.Type), Timestamp: now})
				}
				if previous != nil && previous.Healthy != event.Agent.Healthy {
					_ = eventStore.Append(ctx, &events.AgentEvent{AgentInstanceUID: event.Agent.InstanceUID, AgentName: event.Agent.Name, Type: events.HealthChanged, Timestamp: now, Metadata: map[string]string{"healthy": fmt.Sprintf("%t", event.Agent.Healthy)}})
				}
				log.Printf("agent event=%s id=%s name=%s connected=%t healthy=%t status=%s",
					event.Type, event.Agent.InstanceUID, event.Agent.Name,
					event.Agent.Connected, event.Agent.Healthy, event.Agent.Status)
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
				if err := deploymentStore.UpdateLatestByAgentHash(ctx, report.AgentInstanceUID, report.ConfigurationHash, report.Status, report.Error); err != nil && !errors.Is(err, storage.ErrDeploymentNotFound) && ctx.Err() == nil {
					log.Printf("update deployment status agent=%s hash=%s: %v", report.AgentInstanceUID, report.ConfigurationHash, err)
				}
			}
		}
	}()

	go runRetirementLoop(ctx, agentStore, eventStore, retireAfter, dataDir)

	mux := http.NewServeMux()
	registerHealthRoutes(mux)
	registerAgentRoutes(mux, agentStore, configStore, assignmentStore, deploymentStore, groupStore, eventStore, adapter)
	registerConfigRoutes(mux, configStore, assignmentStore, deploymentStore, agentStore, configValidator, adapter)
	registerGroupRoutes(mux, groupStore, agentStore, dataDir)

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

func registerAgentRoutes(mux *http.ServeMux, agentStore *memory.AgentStore, configStore storage.ConfigurationStore, assignmentStore storage.AssignmentStore, deploymentStore storage.DeploymentStore, groupStore storage.GroupStore, eventStore interface {
	ListSince(context.Context, time.Time) ([]*events.AgentEvent, error)
}, adapter *fleetopamp.Adapter) {
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
		rangeKey := r.URL.Query().Get("range")
		if rangeKey == "" {
			rangeKey = "active"
		}
		agentsList, err := selectAgentsForRange(r.Context(), agentStore, eventStore, rangeKey)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		selectedGroup := r.URL.Query().Get("group")
		allGroups, _ := groupStore.List(r.Context())
		var selected *groups.Group
		if selectedGroup != "" {
			for _, group := range allGroups {
				if group.ID == selectedGroup {
					selected = group
					break
				}
			}
		}
		view := agentListView{Range: rangeKey, Groups: allGroups, SelectedGroup: selectedGroup, Items: make([]agentListItem, 0, len(agentsList))}
		for _, agent := range agentsList {
			if selected != nil && !groups.Matches(selected, agent) {
				continue
			}
			item := agentListItem{Agent: agent}
			for _, group := range allGroups {
				if groups.Matches(group, agent) {
					item.Groups = append(item.Groups, group)
				}
			}
			if deployments, depErr := deploymentStore.ListByAgent(r.Context(), agent.InstanceUID, 1); depErr == nil && len(deployments) > 0 {
				item.LastDeployment = deployments[0]
			}
			view.Items = append(view.Items, item)
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := agentsPage.Execute(w, view); err != nil {
			log.Printf("render agents page: %v", err)
		}
	})

	mux.HandleFunc("/api/v1/agent-events", func(w http.ResponseWriter, r *http.Request) {
		since := timeRangeStart(r.URL.Query().Get("range"))
		items, err := eventStore.ListSince(r.Context(), since)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, items)
	})

	mux.HandleFunc("/agents/", func(w http.ResponseWriter, r *http.Request) {
		uid := strings.Trim(strings.TrimPrefix(r.URL.Path, "/agents/"), "/")
		if uid == "" {
			http.Redirect(w, r, "/agents", http.StatusPermanentRedirect)
			return
		}
		if strings.Contains(uid, "/") {
			http.NotFound(w, r)
			return
		}
		agent, err := agentStore.Get(r.Context(), uid)
		if err != nil {
			http.NotFound(w, r)
			return
		}

		view := agentDetailView{Agent: agent, EffectiveConfig: adapter.EffectiveConfig(uid), RemoteConfigSupported: hasCapability(agent.Capabilities, "accepts_remote_config"), TargetingMetadata: groups.TargetingMetadata(agent), GroupIdentity: groups.GroupIdentity(agent), EffectiveLabels: groups.EffectiveLabels(agent), UnknownGroupFields: agent.UnknownGroupFields, Error: r.URL.Query().Get("error")}
		view.Deployments, _ = deploymentStore.ListByAgent(r.Context(), uid, 10)
		if allGroups, groupErr := groupStore.List(r.Context()); groupErr == nil {
			view.AllGroups = allGroups
			for _, group := range allGroups {
				if groups.Matches(group, agent) {
					view.Groups = append(view.Groups, group)
				}
			}
		}
		view.DeploymentSummary = summarizeDeployments(view.Deployments)
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
		if view.DesiredConfig != nil {
			view.Drift = configs.CompareDesiredEffective(view.DesiredConfig.Content, view.EffectiveConfig)
			allConfigs, _ := configStore.List(r.Context())
			for _, candidate := range allConfigs {
				if candidate.Name == view.DesiredConfig.Name {
					view.ConfigurationHistory = append(view.ConfigurationHistory, candidate)
				}
			}
		} else {
			view.Drift = configs.CompareDesiredEffective("", view.EffectiveConfig)
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
	Drift                 configs.DriftResult
	ConfigurationHistory  []*configs.Configuration
	Deployments           []*configs.Deployment
	DeploymentSummary     deploymentSummary
	Groups                []*groups.Group
	AllGroups             []*groups.Group
	TargetingMetadata     map[string]string
	GroupIdentity         map[string]string
	EffectiveLabels       map[string]string
	UnknownGroupFields    map[string]string
	Error                 string
}

func hasCapability(capabilities []string, wanted string) bool {
	for _, capability := range capabilities {
		if capability == wanted {
			return true
		}
	}
	return false
}

type deploymentSummary struct {
	CurrentDeployedVersion string              `json:"current_deployed_version,omitempty"`
	LastDeployment         *configs.Deployment `json:"last_deployment,omitempty"`
	LastDeploymentDuration string              `json:"last_deployment_duration,omitempty"`
	LastSuccessful         *configs.Deployment `json:"last_successful_deployment,omitempty"`
}

func summarizeDeployments(items []*configs.Deployment) deploymentSummary {
	var summary deploymentSummary
	if len(items) == 0 {
		return summary
	}
	summary.LastDeployment = items[0]
	summary.LastDeploymentDuration = deploymentDuration(items[0])
	for _, d := range items {
		if d.Status == configs.DeliveryApplied {
			if summary.LastSuccessful == nil {
				summary.LastSuccessful = d
			}
			if summary.CurrentDeployedVersion == "" {
				summary.CurrentDeployedVersion = d.ConfigurationVersion
			}
		}
	}
	return summary
}

func deploymentDuration(d *configs.Deployment) string {
	if d == nil || d.SentAt == nil {
		return ""
	}
	var end *time.Time
	if d.AppliedAt != nil {
		end = d.AppliedAt
	} else if d.FailedAt != nil {
		end = d.FailedAt
	}
	if end == nil || end.Before(*d.SentAt) {
		return ""
	}
	return end.Sub(*d.SentAt).Round(time.Millisecond).String()
}

type createConfigurationRequest struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Content     string `json:"content"`
	ContentType string `json:"content_type"`
}

type rollbackResponse struct {
	Action              string                 `json:"action"`
	FromConfigurationID string                 `json:"from_configuration_id"`
	TargetConfiguration *configs.Configuration `json:"target_configuration"`
	Assignment          *configs.Assignment    `json:"assignment"`
	Deployment          *configs.Deployment    `json:"deployment,omitempty"`
}

func latestAssignmentForAgent(ctx context.Context, store storage.AssignmentStore, agentUID string) (*configs.Assignment, error) {
	assignments, err := store.List(ctx)
	if err != nil {
		return nil, err
	}
	var latest *configs.Assignment
	for _, assignment := range assignments {
		if assignment.AgentInstanceUID == agentUID && (latest == nil || assignment.UpdatedAt.After(latest.UpdatedAt)) {
			latest = assignment
		}
	}
	if latest == nil {
		return nil, storage.ErrAssignmentNotFound
	}
	return latest, nil
}

func deliverConfiguration(ctx context.Context, agentUID string, configuration *configs.Configuration, action configs.DeploymentAction, assignmentStore storage.AssignmentStore, deploymentStore storage.DeploymentStore, adapter *fleetopamp.Adapter) (*configs.Assignment, *configs.Deployment, error) {
	if recent, err := deploymentStore.ListByAgent(ctx, agentUID, 1); err != nil {
		return nil, nil, err
	} else if len(recent) > 0 && (recent[0].Status == configs.DeliveryPending || recent[0].Status == configs.DeliverySent || recent[0].Status == configs.DeliveryApplying) {
		return nil, nil, configs.ErrDeploymentInProgress
	}
	deployment, err := configs.NewDeployment(agentUID, configuration, action)
	if err != nil {
		return nil, nil, err
	}
	if err := deploymentStore.Create(ctx, deployment); err != nil {
		return nil, nil, err
	}
	assignment := &configs.Assignment{
		AgentInstanceUID: agentUID, ConfigurationID: configuration.ID, ConfigurationHash: configuration.Hash,
		Status: configs.DeliveryPending, UpdatedAt: time.Now().UTC(),
	}
	if err := assignmentStore.Upsert(ctx, assignment); err != nil {
		_ = deploymentStore.UpdateLatestByAgentHash(ctx, agentUID, configuration.Hash, configs.DeliveryFailed, err.Error())
		return nil, deployment, err
	}
	if err := adapter.SendRemoteConfig(ctx, agentUID, configuration); err != nil {
		assignment.Error = err.Error()
		assignment.UpdatedAt = time.Now().UTC()
		if errors.Is(err, fleetopamp.ErrRemoteConfigUnsupported) {
			assignment.Status = configs.DeliveryUnsupported
		} else {
			assignment.Status = configs.DeliveryFailed
		}
		if storeErr := assignmentStore.Upsert(ctx, assignment); storeErr != nil {
			return assignment, deployment, fmt.Errorf("persist failed delivery state: %w", storeErr)
		}
		_ = deploymentStore.UpdateLatestByAgentHash(ctx, agentUID, configuration.Hash, assignment.Status, assignment.Error)
		return assignment, deployment, err
	}
	assignment.Status = configs.DeliverySent
	assignment.Error = ""
	assignment.UpdatedAt = time.Now().UTC()
	if err := assignmentStore.Upsert(ctx, assignment); err != nil {
		return assignment, deployment, err
	}
	_ = deploymentStore.UpdateLatestByAgentHash(ctx, agentUID, configuration.Hash, configs.DeliverySent, "")
	updated, _ := deploymentStore.Get(ctx, deployment.ID)
	if updated != nil {
		deployment = updated
	}
	return assignment, deployment, nil
}

// registerConfigRoutes exposes configuration artifact, validation, and
// assignment APIs. Validation occurs both before artifact creation and again
// immediately before delivery to protect against unsafe desired state.
func registerConfigRoutes(mux *http.ServeMux, configStore storage.ConfigurationStore, assignmentStore storage.AssignmentStore, deploymentStore storage.DeploymentStore, agentStore *memory.AgentStore, validator *configs.Validator, adapter *fleetopamp.Adapter) {
	mux.HandleFunc("/api/v1/configurations/validate", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var request createConfigurationRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, "invalid JSON body", http.StatusBadRequest)
			return
		}
		result := validator.Validate(r.Context(), request.Content)
		status := http.StatusOK
		if !result.Valid {
			status = http.StatusUnprocessableEntity
		}
		writeJSON(w, status, result)
	})

	mux.HandleFunc("/api/v1/configurations", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			items, err := configStore.List(r.Context())
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			name := strings.TrimSpace(r.URL.Query().Get("name"))
			if name != "" {
				filtered := make([]*configs.Configuration, 0)
				for _, item := range items {
					if item.Name == name {
						filtered = append(filtered, item)
					}
				}
				items = filtered
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
			validation := validator.Validate(r.Context(), request.Content)
			if !validation.Valid {
				writeJSON(w, http.StatusUnprocessableEntity, validation)
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
		parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/agents/"), "/"), "/")
		if r.Method == http.MethodGet && len(parts) == 2 && parts[1] == "deployment-summary" {
			agentUID := parts[0]
			if _, err := agentStore.Get(r.Context(), agentUID); err != nil {
				http.Error(w, "managed agent not found", http.StatusNotFound)
				return
			}
			items, err := deploymentStore.ListByAgent(r.Context(), agentUID, 100)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			writeJSON(w, http.StatusOK, summarizeDeployments(items))
			return
		}

		if r.Method == http.MethodGet && len(parts) == 2 && parts[1] == "deployments" {
			agentUID := parts[0]
			if _, err := agentStore.Get(r.Context(), agentUID); err != nil {
				http.Error(w, "managed agent not found", http.StatusNotFound)
				return
			}
			limit := 10
			if raw := r.URL.Query().Get("limit"); raw != "" {
				if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
					limit = parsed
				}
			}
			items, err := deploymentStore.ListByAgent(r.Context(), agentUID, limit)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			writeJSON(w, http.StatusOK, items)
			return
		}
		if r.Method == http.MethodGet && len(parts) == 2 && parts[1] == "drift" {
			agentUID := parts[0]
			if _, err := agentStore.Get(r.Context(), agentUID); err != nil {
				http.Error(w, "managed agent not found", http.StatusNotFound)
				return
			}
			latest, err := latestAssignmentForAgent(r.Context(), assignmentStore, agentUID)
			effective := adapter.EffectiveConfig(agentUID)
			if errors.Is(err, storage.ErrAssignmentNotFound) {
				writeJSON(w, http.StatusOK, configs.CompareDesiredEffective("", effective))
				return
			}
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			desired, err := configStore.Get(r.Context(), latest.ConfigurationID)
			if err != nil {
				http.Error(w, "desired configuration not found", http.StatusInternalServerError)
				return
			}
			writeJSON(w, http.StatusOK, configs.CompareDesiredEffective(desired.Content, effective))
			return
		}

		if r.Method == http.MethodPost && len(parts) == 3 && parts[1] == "rollback" {
			agentUID, targetConfigID := parts[0], parts[2]
			if _, err := agentStore.Get(r.Context(), agentUID); err != nil {
				http.Error(w, "managed agent not found", http.StatusNotFound)
				return
			}
			latest, err := latestAssignmentForAgent(r.Context(), assignmentStore, agentUID)
			if errors.Is(err, storage.ErrAssignmentNotFound) {
				http.Error(w, "agent has no desired configuration to roll back from", http.StatusConflict)
				return
			}
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			current, err := configStore.Get(r.Context(), latest.ConfigurationID)
			if err != nil {
				http.Error(w, "current desired configuration not found", http.StatusInternalServerError)
				return
			}
			target, err := configStore.Get(r.Context(), targetConfigID)
			if err != nil {
				http.Error(w, "rollback configuration not found", http.StatusNotFound)
				return
			}
			if err := configs.ValidateRollbackTarget(current, target); err != nil {
				http.Error(w, err.Error(), http.StatusConflict)
				return
			}
			validation := validator.Validate(r.Context(), target.Content)
			if !validation.Valid {
				writeJSON(w, http.StatusUnprocessableEntity, validation)
				return
			}
			assignment, deployment, deliveryErr := deliverConfiguration(r.Context(), agentUID, target, configs.DeploymentActionRollback, assignmentStore, deploymentStore, adapter)
			response := rollbackResponse{Action: "rollback", FromConfigurationID: current.ID, TargetConfiguration: target, Assignment: assignment, Deployment: deployment}
			if deliveryErr != nil {
				status := http.StatusInternalServerError
				if errors.Is(deliveryErr, fleetopamp.ErrRemoteConfigUnsupported) || errors.Is(deliveryErr, fleetopamp.ErrAgentNotConnected) || errors.Is(deliveryErr, configs.ErrDeploymentInProgress) {
					status = http.StatusConflict
				}
				writeJSON(w, status, response)
				return
			}
			writeJSON(w, http.StatusAccepted, response)
			return
		}

		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
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
		validation := validator.Validate(r.Context(), configuration.Content)
		if !validation.Valid {
			writeJSON(w, http.StatusUnprocessableEntity, validation)
			return
		}
		assignment, _, deliveryErr := deliverConfiguration(r.Context(), agentUID, configuration, configs.DeploymentActionDeploy, assignmentStore, deploymentStore, adapter)
		if deliveryErr != nil {
			status := http.StatusInternalServerError
			if errors.Is(deliveryErr, fleetopamp.ErrRemoteConfigUnsupported) || errors.Is(deliveryErr, fleetopamp.ErrAgentNotConnected) {
				status = http.StatusConflict
			}
			writeJSON(w, status, assignment)
			return
		}
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

func durationEnvOrDefault(key string, fallback time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if parsed, err := time.ParseDuration(value); err == nil {
			return parsed
		}
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
code{color:#c4b5fd}a{color:#93c5fd;text-decoration:none}a:hover{text-decoration:none}.topbar{text-align:center;margin-bottom:24px}.brand{font-size:30px;font-weight:800;color:#f8fafc}.tagline{margin-top:6px;color:#94a3b8;font-size:14px}.tabs{display:flex;gap:6px;border-bottom:1px solid #1f2937;margin-bottom:28px}.tab{padding:11px 18px;color:#94a3b8;border-bottom:2px solid transparent}.tab.active{color:#e5e7eb;border-bottom-color:#60a5fa;background:#111827;border-radius:8px 8px 0 0}.pagehead{display:flex;align-items:flex-end;justify-content:space-between;gap:20px;margin-bottom:22px}.pagehead h1{margin:0}.pagehead .sub{margin:6px 0 0}
</style></head><body>
<div class="topbar"><div class="brand">FleetAMP</div><div class="tagline">Manage OpenTelemetry Collectors, groups, configuration deployments and fleet health.</div></div><nav class="tabs"><a class="tab active" href="/agents">Managed Agents</a><a class="tab" href="/groups">Groups</a></nav><div class="pagehead"><div><h1>Managed Agents</h1><div class="sub">Fleet inventory, health, groups and deployment state.</div></div></div>
<form method="get" action="/agents" style="margin-bottom:18px;display:flex;gap:18px;align-items:center;flex-wrap:wrap"><label for="range">Time range: <select id="range" name="range" onchange="this.form.submit()"><option value="active" {{if eq .Range "active"}}selected{{end}}>Active / recent</option><option value="15m" {{if eq .Range "15m"}}selected{{end}}>Last 15 minutes</option><option value="1h" {{if eq .Range "1h"}}selected{{end}}>Last 1 hour</option><option value="24h" {{if eq .Range "24h"}}selected{{end}}>Last 24 hours</option><option value="7d" {{if eq .Range "7d"}}selected{{end}}>Last 7 days</option><option value="30d" {{if eq .Range "30d"}}selected{{end}}>Last 30 days</option><option value="all" {{if eq .Range "all"}}selected{{end}}>All known</option></select></label><label for="group">Group: <select id="group" name="group" onchange="this.form.submit()"><option value="">All groups</option>{{range .Groups}}<option value="{{.ID}}" {{if eq $.SelectedGroup .ID}}selected{{end}}>{{.Name}}</option>{{end}}</select></label></form>
{{if .Items}}<table><thead><tr><th>Name</th><th>Group</th><th>Collector Type</th><th>Version</th><th>OS / Arch</th><th>Runtime</th><th>Status</th><th>Health</th><th>Last Deployment Status</th><th>Last Deployment Time</th><th>Last Seen</th></tr></thead><tbody>
{{range .Items}}<tr><td><a href="/agents/{{.Agent.InstanceUID}}">{{.Agent.Name}}</a><br><code>{{.Agent.InstanceUID}}</code></td><td>{{if .Groups}}{{range .Groups}}<a href="/groups/{{.ID}}">{{.Name}}</a><br>{{end}}{{else}}—{{end}}</td><td>{{if eq .Agent.Type "otel_collector"}}OTel Collector{{else if eq .Agent.Type "grafana_alloy"}}Grafana Alloy{{else}}{{.Agent.Type}}{{end}}</td><td>{{.Agent.Version}}</td><td>{{index .Agent.Attributes "os.type"}} / {{index .Agent.Attributes "host.arch"}}</td><td>{{.Agent.Deployment.Runtime}}</td><td><span class="{{if .Agent.Connected}}ok{{else}}bad{{end}}">{{.Agent.Status}}</span></td><td><span class="{{if .Agent.Healthy}}ok{{else}}bad{{end}}">{{if .Agent.Healthy}}Healthy{{else}}Unhealthy{{end}}</span></td><td>{{if .LastDeployment}}<span class="{{if eq .LastDeployment.Status "applied"}}ok{{else if eq .LastDeployment.Status "failed"}}bad{{else}}warn{{end}}">{{.LastDeployment.Status}}</span><br><span class="sub" style="margin:0">{{.LastDeployment.ConfigurationName}} v{{.LastDeployment.ConfigurationVersion}}</span>{{else}}<span class="sub" style="margin:0">No deployment</span>{{end}}</td><td>{{if .LastDeployment}}{{.LastDeployment.UpdatedAt}}{{else}}—{{end}}</td><td>{{.Agent.LastSeen}}</td></tr>{{end}}
</tbody></table>{{else}}<div class="empty">No managed agents found for this time range.</div>{{end}}
</body></html>`))

var agentDetailPage = template.Must(template.New("agent-detail").Parse(`<!doctype html>
<html><head><meta charset="utf-8"><title>FleetAMP Agent</title><style>
body{font-family:system-ui,sans-serif;background:#0b1220;color:#e5e7eb;margin:0;padding:32px;max-width:1400px}a{color:#93c5fd;text-decoration:none}.back{display:inline-block;margin-bottom:22px}.head{display:flex;justify-content:space-between;gap:24px;align-items:flex-start}.muted{color:#94a3b8}.chips{display:flex;gap:8px;flex-wrap:wrap}.chip{background:#1f2937;border-radius:999px;padding:5px 10px;font-size:13px}.ok{color:#86efac}.bad{color:#fca5a5}.warn{color:#fde68a}.grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(260px,1fr));gap:16px;margin:24px 0}.card{background:#111827;border:1px solid #1f2937;border-radius:12px;padding:18px}.card h2{margin-top:0;font-size:18px}.kv{display:grid;grid-template-columns:130px 1fr;gap:8px 12px}.kv span:nth-child(odd){color:#94a3b8}pre{white-space:pre-wrap;overflow:auto;background:#060b14;border:1px solid #1f2937;border-radius:10px;padding:16px;max-height:520px}code{color:#c4b5fd}.configgrid{display:grid;grid-template-columns:1fr 1fr;gap:16px}@media(max-width:900px){.configgrid{grid-template-columns:1fr}}
</style></head><body>
<div style="display:flex;gap:16px;margin-bottom:22px"><a href="/agents">← Agents</a><a href="/groups">Groups</a></div>
<div class="head"><div><h1>{{.Agent.Name}}</h1><div class="muted"><code>{{.Agent.InstanceUID}}</code></div></div><div class="chips"><span class="chip {{if .Agent.Connected}}ok{{else}}bad{{end}}">Connected: {{.Agent.Connected}}</span><span class="chip {{if .Agent.Healthy}}ok{{else}}bad{{end}}">Healthy: {{.Agent.Healthy}}</span></div></div>
{{if .Error}}<div class="card bad" style="margin-top:16px"><strong>{{.Error}}</strong></div>{{end}}
<div class="grid"><section class="card"><h2>Overview</h2><div class="kv"><span>Type</span><span>{{.Agent.Type}}</span><span>OTel Version</span><span>{{.Agent.Version}}</span><span>Hostname</span><span>{{.Agent.Hostname}}</span><span>OS</span><span>{{index .Agent.Attributes "os.type"}}</span><span>OS Details</span><span>{{index .Agent.Attributes "os.description"}}</span><span>Architecture</span><span>{{index .Agent.Attributes "host.arch"}}</span><span>Runtime</span><span>{{.Agent.Deployment.Runtime}}</span><span>Cluster</span><span>{{.Agent.Deployment.Cluster}}</span><span>Last seen</span><span>{{.Agent.LastSeen}}</span></div></section>
<section class="card"><h2>Capabilities</h2><div class="chips">{{range .Agent.Capabilities}}<span class="chip">{{.}}</span>{{end}}</div><p class="{{if .RemoteConfigSupported}}ok{{else}}warn{{end}}">Remote config: {{if .RemoteConfigSupported}}supported{{else}}not advertised by this agent{{end}}</p></section>
<section class="card"><h2>Group identity</h2><p class="muted">Controlled fields used for group membership: application, environment and place. FleetAMP-managed values override Collector-reported values.</p>{{if .GroupIdentity}}<div class="chips">{{range $k,$v := .GroupIdentity}}<span class="chip"><code>{{$k}}={{$v}}</code></span>{{end}}</div>{{else}}<p class="muted">No valid group identity reported or assigned.</p>{{end}}{{if .UnknownGroupFields}}<div class="bad" style="margin-top:12px"><strong>Unknown group field(s):</strong> {{range $k,$v := .UnknownGroupFields}}<code>{{$k}}={{$v}}</code> {{end}}<br><span class="muted">Allowed fields: application, environment, place.</span></div>{{end}}</section>
<section class="card"><h2>Labels</h2><p class="muted">Optional metadata. Maximum 5 FleetAMP-managed labels per agent. Collector-reported labels and FleetAMP-managed labels are combined; FleetAMP values override duplicate keys.</p>{{if .EffectiveLabels}}<div class="chips">{{range $k,$v := .EffectiveLabels}}<span class="chip"><code>{{$k}}={{$v}}</code></span>{{end}}</div>{{else}}<p class="muted">No labels available.</p>{{end}}<form method="post" action="/agents/{{.Agent.InstanceUID}}/label" style="margin-top:14px;display:flex;gap:8px;flex-wrap:wrap"><input name="key" placeholder="label key (team)" required style="padding:8px;background:#0b1220;color:#e5e7eb;border:1px solid #334155;border-radius:7px"><input name="value" placeholder="value (payments)" required style="padding:8px;background:#0b1220;color:#e5e7eb;border:1px solid #334155;border-radius:7px"><button type="submit" style="padding:8px 12px">+ Add label</button></form></section>
<section class="card"><h2>Reported OTel attributes</h2><p class="muted">Raw AgentDescription metadata reported by the Collector/OpAMP.</p>{{if .Agent.Attributes}}<div class="chips">{{range $k,$v := .Agent.Attributes}}<span class="chip"><code>{{$k}}={{$v}}</code></span>{{end}}</div>{{else}}<p class="muted">No reported attributes available.</p>{{end}}</section>
<section class="card"><h2>Groups</h2>{{if .Groups}}<div class="chips">{{range .Groups}}<a class="chip" href="/groups/{{.ID}}">{{.Name}}</a>{{end}}</div>{{else}}<p class="muted">This agent does not match any group.</p>{{end}}{{if .AllGroups}}<form method="post" action="/agents/{{.Agent.InstanceUID}}/group" style="margin-top:14px"><label>Add to group: <select name="group_id" required style="padding:8px;background:#0b1220;color:#e5e7eb;border:1px solid #334155;border-radius:7px"><option value="">Select group</option>{{range .AllGroups}}<option value="{{.ID}}">{{.Name}}</option>{{end}}</select></label><button type="submit" style="margin-left:8px;padding:8px 12px">Assign group</button></form><p class="muted">This assigns the controlled Application, Environment and Place values to this Collector in FleetAMP.</p>{{end}}</section>
<section class="card"><h2>Deployment status</h2>{{if .DeploymentSummary.LastDeployment}}<div class="kv"><span>Current deployed</span><span>{{if .DeploymentSummary.CurrentDeployedVersion}}v{{.DeploymentSummary.CurrentDeployedVersion}}{{else}}Unknown{{end}}</span><span>Last deployment</span><span>{{.DeploymentSummary.LastDeployment.ConfigurationName}} v{{.DeploymentSummary.LastDeployment.ConfigurationVersion}}</span><span>Status</span><span class="{{if eq .DeploymentSummary.LastDeployment.Status "applied"}}ok{{else if eq .DeploymentSummary.LastDeployment.Status "failed"}}bad{{else}}warn{{end}}">{{.DeploymentSummary.LastDeployment.Status}}</span><span>Duration</span><span>{{if .DeploymentSummary.LastDeploymentDuration}}{{.DeploymentSummary.LastDeploymentDuration}}{{else}}—{{end}}</span>{{if .DeploymentSummary.LastSuccessful}}<span>Last successful</span><span>{{.DeploymentSummary.LastSuccessful.ConfigurationName}} v{{.DeploymentSummary.LastSuccessful.ConfigurationVersion}} · {{.DeploymentSummary.LastSuccessful.AppliedAt}}</span>{{end}}</div>{{else}}<p class="muted">No FleetAMP deployment history has been recorded yet.</p>{{end}}</section>
<section class="card"><h2>Latest assignment</h2>{{if .Assignment}}<div class="kv"><span>Status</span><span>{{.Assignment.Status}}</span><span>Config ID</span><span><code>{{.Assignment.ConfigurationID}}</code></span><span>Hash</span><span><code>{{.Assignment.ConfigurationHash}}</code></span><span>Updated</span><span>{{.Assignment.UpdatedAt}}</span>{{if .Assignment.Error}}<span>Error</span><span class="bad">{{.Assignment.Error}}</span>{{end}}</div>{{else}}<p class="muted">No FleetAMP configuration has been assigned.</p>{{end}}</section>
<section class="card"><h2>Configuration drift</h2><div class="kv"><span>Status</span><span class="{{if .Drift.InSync}}ok{{else if eq .Drift.Status "drift"}}bad{{else}}warn{{end}}">{{.Drift.Status}}</span>{{if .Drift.Reason}}<span>Reason</span><span>{{.Drift.Reason}}</span>{{end}}</div>{{if .Drift.Differences}}<div style="margin-top:12px">{{range .Drift.Differences}}<div style="margin:8px 0;padding:10px;background:#0b1220;border-radius:8px"><code>{{.Path}}</code> <span class="warn">{{.Kind}}</span><br><span class="muted">Desired:</span> <code>{{printf "%v" .Desired}}</code><br><span class="muted">Effective:</span> <code>{{printf "%v" .Effective}}</code></div>{{end}}</div>{{end}}</section></div>
<div class="configgrid"><section class="card"><h2>Desired configuration</h2>{{if .DesiredConfig}}<p class="muted">{{.DesiredConfig.Name}} · version {{.DesiredConfig.Version}}</p><pre>{{.DesiredConfig.Content}}</pre>{{else}}<p class="muted">No desired FleetAMP configuration.</p>{{end}}</section>
<section class="card"><h2>Effective configuration</h2><p class="muted">Reported by the managed agent through OpAMP.</p>{{if .EffectiveConfig}}<pre>{{.EffectiveConfig}}</pre>{{else}}<p class="muted">No effective configuration has been reported yet.</p>{{end}}</section></div>
{{if .Deployments}}<section class="card" style="margin-top:16px"><h2>Last FleetAMP configuration deployments</h2><p class="muted">Latest 10 deployment attempts for this managed agent.</p><table style="width:100%;border-collapse:collapse"><thead><tr><th>Version</th><th>Action</th><th>Status</th><th>Created</th><th>Sent</th><th>Applied / Failed</th></tr></thead><tbody>{{range .Deployments}}<tr><td><strong>{{.ConfigurationName}} v{{.ConfigurationVersion}}</strong><br><code>{{.ID}}</code></td><td>{{.Action}}</td><td class="{{if eq .Status "applied"}}ok{{else if eq .Status "failed"}}bad{{else}}warn{{end}}">{{.Status}}{{if .Error}}<br><span class="bad">{{.Error}}</span>{{end}}</td><td>{{.CreatedAt}}</td><td>{{if .SentAt}}{{.SentAt}}{{else}}—{{end}}</td><td>{{if .AppliedAt}}{{.AppliedAt}}{{else if .FailedAt}}{{.FailedAt}}{{else}}—{{end}}</td></tr>{{end}}</tbody></table></section>{{else}}<section class="card" style="margin-top:16px"><h2>Last FleetAMP configuration deployments</h2><p class="muted">No FleetAMP deployment history has been recorded for this agent yet.</p></section>{{end}}
{{if .ConfigurationHistory}}<section class="card" style="margin-top:16px"><h2>Configuration history</h2><table style="width:100%;border-collapse:collapse"><thead><tr><th>Version</th><th>Created</th><th>Hash</th><th>Config ID</th></tr></thead><tbody>{{range .ConfigurationHistory}}<tr><td>{{.Version}}</td><td>{{.CreatedAt}}</td><td><code>{{.Hash}}</code></td><td><code>{{.ID}}</code></td></tr>{{end}}</tbody></table></section>{{end}}
</body></html>`))
