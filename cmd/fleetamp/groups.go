// FleetAMP group, label, and group-management HTTP/UI handlers.
//
// Purpose:
//
//	Implements controlled Application/Environment/Place groups, agent group
//	assignment, managed labels, group CRUD, enable/disable lifecycle, and
//	membership-aware deletion protection for both REST and web UI routes.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"html/template"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/marellasunil/FleetAMP/internal/agents"
	"github.com/marellasunil/FleetAMP/internal/configs"
	"github.com/marellasunil/FleetAMP/internal/groups"
	fleetopamp "github.com/marellasunil/FleetAMP/internal/opamp"
	"github.com/marellasunil/FleetAMP/internal/storage"
	"github.com/marellasunil/FleetAMP/internal/storage/memory"
)

type groupRequest struct {
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Selector    map[string]string `json:"selector"`
	Owners      []string          `json:"owners,omitempty"`
	Enabled     *bool             `json:"enabled,omitempty"`
}

type groupListItem struct {
	Group       *groups.Group
	MemberCount int
	Drifted     int
}

type groupsView struct {
	Page  string
	Items []groupListItem
}
type groupPreviewAgent struct {
	Agent  *agents.ManagedAgent
	Reason string
}

type groupDetailView struct {
	Page               string
	Group              *groups.Group
	Members            []*agents.ManagedAgent
	Configurations     []*configs.Configuration
	SelectedConfig     *configs.Configuration
	SelectedPipeline   *configs.PipelineModel
	PipelineError      string
	Preview            []groupPreviewAgent
	Eligible           int
	Requests           []*configs.GroupDeploymentRequest
	DeploymentHistory  []*configs.Deployment
	DriftSummary       groupDriftSummary
	ConfigurationSaved string
	RequestCreated     string
	RequestUpdated     string
	Error              string
	CanAdminister      bool
	OwnersText         string
}

func currentUsername(auth *authManager, r *http.Request) string {
	if auth != nil {
		if username, ok := auth.sessionUsername(r); ok {
			return username
		}
	}
	if username, _, ok := r.BasicAuth(); ok {
		return username
	}
	return ""
}

func isGroupOwner(group *groups.Group, username string) bool {
	for _, owner := range group.Owners {
		if strings.EqualFold(strings.TrimSpace(owner), strings.TrimSpace(username)) {
			return true
		}
	}
	return false
}

func canAccessGroup(auth *authManager, r *http.Request, group *groups.Group) bool {
	if currentRole(auth, r) != roleGroupOwner {
		return true
	}
	return isGroupOwner(group, currentUsername(auth, r))
}

func requireGroupAccess(w http.ResponseWriter, r *http.Request, auth *authManager, group *groups.Group) bool {
	if canAccessGroup(auth, r, group) {
		return true
	}
	http.Error(w, "forbidden: you are not an owner of this group", http.StatusForbidden)
	return false
}

func normalizeOwners(values []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		for _, item := range strings.Split(value, ",") {
			owner := strings.TrimSpace(item)
			key := strings.ToLower(owner)
			if owner == "" {
				continue
			}
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			result = append(result, owner)
		}
	}
	sort.Strings(result)
	return result
}

// validateConfigurationForApproval enforces the current validation policy before a request enters the approval queue.
func validateConfigurationForApproval(ctx context.Context, validator *configs.Validator, configuration *configs.Configuration) error {
	if configuration == nil {
		return errors.New("configuration is required")
	}
	validation := validator.Validate(ctx, configuration.Content)
	if validation.Valid {
		return nil
	}
	message := strings.TrimSpace(validation.Error)
	if message == "" {
		message = "configuration validation failed"
	}
	return errors.New(message)
}

func previewGroupMembers(members []*agents.ManagedAgent, enabled bool) ([]groupPreviewAgent, int) {
	result := make([]groupPreviewAgent, 0, len(members))
	eligible := 0
	for _, agent := range members {
		reason := "Ready"
		switch {
		case !enabled:
			reason = "Group disabled"
		case agent.Status == agents.LifecycleRetired:
			reason = "Retired"
		case !agent.Connected:
			reason = "Offline"
		case !hasCapability(agent.Capabilities, "accepts_remote_config"):
			reason = "Remote configuration unsupported"
		default:
			eligible++
		}
		result = append(result, groupPreviewAgent{Agent: agent, Reason: reason})
	}
	return result, eligible
}

func previewGroupConfiguration(ctx context.Context, members []*agents.ManagedAgent, enabled bool, configuration *configs.Configuration, assignmentStore storage.AssignmentStore) ([]groupPreviewAgent, int, error) {
	preview, _ := previewGroupMembers(members, enabled)
	eligible := 0
	for index := range preview {
		if preview[index].Reason != "Ready" {
			continue
		}
		latest, err := latestAssignmentForAgent(ctx, assignmentStore, preview[index].Agent.InstanceUID)
		switch {
		case err == nil && latest.ConfigurationHash == configuration.Hash && latest.Status == configs.DeliveryApplied:
			preview[index].Reason = "Already deployed · Latest"
		case err == nil && latest.ConfigurationHash == configuration.Hash &&
			(latest.Status == configs.DeliveryPending || latest.Status == configs.DeliverySent || latest.Status == configs.DeliveryApplying):
			preview[index].Reason = "Deployment already in progress"
		case err != nil && !errors.Is(err, storage.ErrAssignmentNotFound):
			return nil, 0, err
		default:
			eligible++
		}
	}
	return preview, eligible, nil
}

func commonAppliedConfiguration(ctx context.Context, preview []groupPreviewAgent, assignmentStore storage.AssignmentStore) (string, string, error) {
	baseID, baseHash := "", ""
	for _, item := range preview {
		if item.Reason != "Ready" {
			continue
		}
		assignment, err := latestAssignmentForAgent(ctx, assignmentStore, item.Agent.InstanceUID)
		if errors.Is(err, storage.ErrAssignmentNotFound) {
			continue
		}
		if err != nil {
			return "", "", err
		}
		if assignment.Status != configs.DeliveryApplied {
			continue
		}
		if baseID == "" {
			baseID, baseHash = assignment.ConfigurationID, assignment.ConfigurationHash
			continue
		}
		if assignment.ConfigurationID != baseID || assignment.ConfigurationHash != baseHash {
			return "", "", nil
		}
	}
	return baseID, baseHash, nil
}

func lastSuccessfulConfigurationID(ctx context.Context, agentUID string, deploymentStore storage.DeploymentStore) (string, error) {
	deployments, err := deploymentStore.ListByAgent(ctx, agentUID, 100)
	if err != nil {
		return "", err
	}
	for _, deployment := range deployments {
		if deployment.Status == configs.DeliveryApplied {
			return deployment.ConfigurationID, nil
		}
	}
	return "", nil
}

const maxManagedLabels = 5

// copyStringMap returns an independent map so request updates cannot mutate stored agent metadata by aliasing.
func copyStringMap(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func sameSelector(left, right map[string]string) bool {
	if len(left) != len(right) {
		return false
	}
	for key, value := range left {
		if right[key] != value {
			return false
		}
	}
	return true
}

// registerGroupRoutes exposes group CRUD APIs, agent metadata updates, membership previews, and group UI pages.
func registerGroupRoutes(mux *http.ServeMux, groupStore storage.GroupStore, agentStore *memory.AgentStore, configStore storage.ConfigurationStore, assignmentStore storage.AssignmentStore, deploymentStore storage.DeploymentStore, requestStore storage.GroupDeploymentRequestStore, validator *configs.Validator, adapter *fleetopamp.Adapter, auth *authManager, dataDir string) {
	// /agents/{uid}/group updates operator-managed group identity fields for an agent.
	mux.HandleFunc("/agents/{uid}/group", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		agent, err := agentStore.Get(r.Context(), r.PathValue("uid"))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		group, err := groupStore.Get(r.Context(), strings.TrimSpace(r.FormValue("group_id")))
		if err != nil {
			http.Error(w, "group not found", http.StatusNotFound)
			return
		}
		if agent.GroupFields == nil {
			agent.GroupFields = map[string]string{}
		}
		for key, value := range group.Selector {
			agent.GroupFields[key] = value
		}
		if err := agentStore.Upsert(r.Context(), agent); err != nil {
			internalServerError(w, err)
			return
		}
		if err := saveAgentSnapshot(r.Context(), agentStore, dataDir); err != nil {
			internalServerError(w, err)
			return
		}
		http.Redirect(w, r, "/agents/"+agent.InstanceUID, http.StatusSeeOther)
	})
	// /agents/{uid}/labels replaces the complete operator-label set submitted by the agent detail form.
	mux.HandleFunc("/agents/{uid}/labels", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		agent, err := agentStore.Get(r.Context(), r.PathValue("uid"))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		labels, err := parseSelectorText(r.FormValue("labels"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		agent.Labels = labels
		if err := agentStore.Upsert(r.Context(), agent); err != nil {
			internalServerError(w, err)
			return
		}
		if err := saveAgentSnapshot(r.Context(), agentStore, dataDir); err != nil {
			internalServerError(w, err)
			return
		}
		http.Redirect(w, r, "/agents/"+agent.InstanceUID, http.StatusSeeOther)
	})
	// /agents/{uid}/label adds or removes one label without replacing unrelated labels.
	mux.HandleFunc("/agents/{uid}/label", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		agent, err := agentStore.Get(r.Context(), r.PathValue("uid"))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		key, value := strings.TrimSpace(r.FormValue("key")), strings.TrimSpace(r.FormValue("value"))
		if key == "" || value == "" {
			http.Error(w, "label key and value are required", http.StatusBadRequest)
			return
		}
		if agent.Labels == nil {
			agent.Labels = map[string]string{}
		}
		if _, exists := agent.Labels[key]; !exists && len(agent.Labels) >= maxManagedLabels {
			http.Redirect(w, r, "/agents/"+agent.InstanceUID+"?error=Maximum+of+5+managed+labels+allowed", http.StatusSeeOther)
			return
		}
		agent.Labels[key] = value
		if err := agentStore.Upsert(r.Context(), agent); err != nil {
			internalServerError(w, err)
			return
		}
		if err := saveAgentSnapshot(r.Context(), agentStore, dataDir); err != nil {
			internalServerError(w, err)
			return
		}
		http.Redirect(w, r, "/agents/"+agent.InstanceUID, http.StatusSeeOther)
	})

	// /api/v1/agents/{uid}/labels provides JSON label replacement for automation clients.
	mux.HandleFunc("/api/v1/agents/{uid}/labels", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut && r.Method != http.MethodPatch {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		uid := r.PathValue("uid")
		agent, err := agentStore.Get(r.Context(), uid)
		if err != nil {
			http.Error(w, "managed agent not found", http.StatusNotFound)
			return
		}
		var labels map[string]string
		if err := json.NewDecoder(r.Body).Decode(&labels); err != nil {
			http.Error(w, "invalid JSON body", http.StatusBadRequest)
			return
		}
		next := copyStringMap(agent.Labels)
		if r.Method == http.MethodPut || next == nil {
			next = map[string]string{}
		}
		for k, v := range labels {
			k, v = strings.TrimSpace(k), strings.TrimSpace(v)
			if k == "" {
				continue
			}
			if v == "" {
				delete(next, k)
			} else {
				next[k] = v
			}
		}
		if len(next) > maxManagedLabels {
			http.Error(w, "maximum of 5 managed labels allowed", http.StatusUnprocessableEntity)
			return
		}
		agent.Labels = next
		if err := agentStore.Upsert(r.Context(), agent); err != nil {
			internalServerError(w, err)
			return
		}
		if err := saveAgentSnapshot(r.Context(), agentStore, dataDir); err != nil {
			internalServerError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, agent)
	})

	// /api/v1/groups lists groups or creates a validated selector-based group.
	mux.HandleFunc("/api/v1/groups", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			items, err := groupStore.List(r.Context())
			if err != nil {
				internalServerError(w, err)
				return
			}
			if currentRole(auth, r) == roleGroupOwner {
				username := currentUsername(auth, r)
				filtered := make([]*groups.Group, 0, len(items))
				for _, item := range items {
					if isGroupOwner(item, username) {
						filtered = append(filtered, item)
					}
				}
				items = filtered
			}
			writeJSON(w, http.StatusOK, items)
		case http.MethodPost:
			var req groupRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "invalid JSON body", 400)
				return
			}
			group, err := newValidatedGroup(req)
			if err != nil {
				http.Error(w, err.Error(), 400)
				return
			}
			if err := groupStore.Create(r.Context(), group); err != nil {
				http.Error(w, err.Error(), 409)
				return
			}
			writeJSON(w, http.StatusCreated, group)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	// /api/v1/groups/{id} reads, updates, deletes, or previews membership for one group.
	mux.HandleFunc("/api/v1/groups/", func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/groups/"), "/"), "/")
		if len(parts) == 0 || parts[0] == "" {
			http.NotFound(w, r)
			return
		}
		group, err := groupStore.Get(r.Context(), parts[0])
		if err != nil {
			http.Error(w, "group not found", 404)
			return
		}
		if !requireGroupAccess(w, r, auth, group) {
			return
		}
		if len(parts) == 2 && parts[1] == "members" && r.Method == http.MethodGet {
			members, err := membersForGroupIdentity(r.Context(), group, agentStore)
			if err != nil {
				internalServerError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, members)
			return
		}
		if len(parts) != 1 {
			http.NotFound(w, r)
			return
		}
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, http.StatusOK, group)
		case http.MethodPut:
			var req groupRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "invalid JSON body", 400)
				return
			}
			selector := cleanSelector(req.Selector)
			if err := validateGroupSelector(selector); err != nil {
				http.Error(w, err.Error(), 400)
				return
			}
			group.Name, group.Description, group.Selector, group.UpdatedAt = canonicalGroupName(selector), strings.TrimSpace(req.Description), selector, time.Now().UTC()
			if currentRole(auth, r) == roleAdmin && req.Owners != nil {
				group.Owners = normalizeOwners(req.Owners)
			}
			if req.Enabled != nil {
				group.Enabled = *req.Enabled
			}
			if err := groupStore.Update(r.Context(), group); err != nil {
				internalServerError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, group)
		case http.MethodDelete:
			members, memberErr := membersForGroupIdentity(r.Context(), group, agentStore)
			if memberErr != nil {
				http.Error(w, memberErr.Error(), 500)
				return
			}
			if len(members) > 0 {
				http.Error(w, "group cannot be deleted while agents are assigned; unassign all agents first", http.StatusConflict)
				return
			}
			if err := groupStore.Delete(r.Context(), group.ID); err != nil {
				internalServerError(w, err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	registerGroupUI(mux, groupStore, agentStore, configStore, assignmentStore, deploymentStore, requestStore, validator, adapter, auth)
}

// newValidatedGroup normalizes a request, validates its selector, and constructs the domain group.
func newValidatedGroup(req groupRequest) (*groups.Group, error) {
	req.Name = strings.TrimSpace(req.Name)
	req.Description = strings.TrimSpace(req.Description)
	req.Selector = cleanSelector(req.Selector)
	if err := validateGroupSelector(req.Selector); err != nil {
		return nil, err
	}
	req.Name = canonicalGroupName(req.Selector)
	return groups.New(req.Name, req.Description, req.Selector)
}

// validateGroupSelector rejects empty keys and values that cannot safely target agents.
func validateGroupSelector(selector map[string]string) error {
	if strings.TrimSpace(selector["application"]) == "" {
		return errors.New("application selector is required")
	}
	if strings.TrimSpace(selector["environment"]) == "" {
		return errors.New("environment selector is required")
	}
	if strings.TrimSpace(selector["place"]) == "" {
		return errors.New("place selector is required")
	}
	return nil
}

// canonicalGroupName derives a stable display name from a selector when the request omits one.
func canonicalGroupName(selector map[string]string) string {
	parts := []string{selector["application"], selector["environment"], selector["place"]}
	for i, part := range parts {
		part = strings.ToLower(strings.TrimSpace(part))
		var b strings.Builder
		dash := false
		for _, r := range part {
			if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
				b.WriteRune(r)
				dash = false
			} else if !dash && b.Len() > 0 {
				b.WriteByte('-')
				dash = true
			}
		}
		parts[i] = strings.Trim(b.String(), "-")
	}
	return strings.Join(parts, "-")
}

// cleanSelector trims selector keys and values and drops empty pairs before validation.
func cleanSelector(in map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range in {
		k, v = strings.TrimSpace(k), strings.TrimSpace(v)
		if k != "" && v != "" {
			out[k] = v
		}
	}
	return out
}

// membersForGroup returns agents whose effective targeting metadata matches the group's selector.
func membersForGroup(ctx context.Context, group *groups.Group, store *memory.AgentStore) ([]*agents.ManagedAgent, error) {
	if group != nil && !group.Enabled {
		return []*agents.ManagedAgent{}, nil
	}
	return membersByMatcher(ctx, group, store, groups.Matches)
}

// membersForGroupIdentity previews membership using agent-reported identity fields before operator labels are considered.
func membersForGroupIdentity(ctx context.Context, group *groups.Group, store *memory.AgentStore) ([]*agents.ManagedAgent, error) {
	return membersByMatcher(ctx, group, store, groups.MatchesIdentity)
}

// membersByMatcher centralizes agent listing and filtering for the two supported membership semantics.
func membersByMatcher(ctx context.Context, group *groups.Group, store *memory.AgentStore, matcher func(*groups.Group, *agents.ManagedAgent) bool) ([]*agents.ManagedAgent, error) {
	all, err := store.List(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]*agents.ManagedAgent, 0)
	for _, agent := range all {
		if matcher(group, agent) {
			result = append(result, agent)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}

// registerGroupUI serves the group list, create/edit form, and group detail pages with current member counts.
func registerGroupUI(mux *http.ServeMux, groupStore storage.GroupStore, agentStore *memory.AgentStore, configStore storage.ConfigurationStore, assignmentStore storage.AssignmentStore, deploymentStore storage.DeploymentStore, requestStore storage.GroupDeploymentRequestStore, validator *configs.Validator, adapter *fleetopamp.Adapter, auth *authManager) {
	// /groups displays all groups and accepts creation form submissions.
	mux.HandleFunc("/groups", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/groups" {
			http.NotFound(w, r)
			return
		}
		if r.Method == http.MethodPost {
			selector, err := parseGroupSelectorForm(r)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			group, err := newValidatedGroup(groupRequest{Selector: selector})
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if err := groupStore.Create(r.Context(), group); err != nil {
				http.Error(w, err.Error(), http.StatusConflict)
				return
			}
			http.Redirect(w, r, "/groups/"+group.ID, http.StatusSeeOther)
			return
		}
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		groupsList, err := groupStore.List(r.Context())
		if err != nil {
			internalServerError(w, err)
			return
		}
		if currentRole(auth, r) == roleGroupOwner {
			username := currentUsername(auth, r)
			filtered := make([]*groups.Group, 0, len(groupsList))
			for _, item := range groupsList {
				if isGroupOwner(item, username) {
					filtered = append(filtered, item)
				}
			}
			groupsList = filtered
		}
		view := groupsView{Page: "groups", Items: make([]groupListItem, 0, len(groupsList))}
		for _, group := range groupsList {
			members, memberErr := membersForGroupIdentity(r.Context(), group, agentStore)
			if memberErr != nil {
				internalServerError(w, memberErr)
				return
			}
			drift, driftErr := buildGroupDrift(r.Context(), members, configStore, assignmentStore, adapter)
			if driftErr != nil {
				internalServerError(w, driftErr)
				return
			}
			view.Items = append(view.Items, groupListItem{Group: group, MemberCount: len(members), Drifted: drift.Drifted})
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = groupsPage.Execute(w, view)
	})

	// /groups/new renders the creation form; /groups/{id} shows details, editing, membership, and deletion.
	mux.HandleFunc("/groups/", func(w http.ResponseWriter, r *http.Request) {
		id := strings.Trim(strings.TrimPrefix(r.URL.Path, "/groups/"), "/")
		if id == "" {
			http.Redirect(w, r, "/groups", http.StatusPermanentRedirect)
			return
		}
		if strings.Contains(id, "/") {
			http.NotFound(w, r)
			return
		}
		group, err := groupStore.Get(r.Context(), id)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		if !requireGroupAccess(w, r, auth, group) {
			return
		}
		if r.Method == http.MethodPost {
			action := strings.TrimSpace(r.FormValue("action"))
			if action == "set_owners" {
				if currentRole(auth, r) != roleAdmin {
					http.Error(w, "only Admins can change group owners", http.StatusForbidden)
					return
				}
				owners := normalizeOwners([]string{r.FormValue("owners")})
				for _, owner := range owners {
					user, err := auth.store.Get(r.Context(), owner)
					if err != nil || !user.Enabled || user.Role != string(roleGroupOwner) {
						http.Error(w, "group owner must be an enabled user with the group_owner role: "+owner, http.StatusUnprocessableEntity)
						return
					}
				}
				group.Owners = owners
				group.UpdatedAt = time.Now().UTC()
				if err := groupStore.Update(r.Context(), group); err != nil {
					internalServerError(w, err)
					return
				}
				http.Redirect(w, r, "/groups/"+group.ID, http.StatusSeeOther)
				return
			}
			if action == "create_configuration" {
				name := strings.TrimSpace(r.FormValue("name"))
				version := strings.TrimSpace(r.FormValue("version"))
				content := r.FormValue("content")
				if name == "" || version == "" || strings.TrimSpace(content) == "" {
					http.Redirect(w, r, "/groups/"+group.ID+"?error="+url.QueryEscape("Name, version and configuration YAML are required."), http.StatusSeeOther)
					return
				}
				validation := validator.Validate(r.Context(), content)
				if !validation.Valid {
					message := strings.TrimSpace(validation.Error)
					if message == "" {
						message = "configuration validation failed"
					}
					http.Redirect(w, r, "/groups/"+group.ID+"?error="+url.QueryEscape(message), http.StatusSeeOther)
					return
				}
				configuration := configs.NewConfiguration(name, version, content, "text/yaml")
				if err := configStore.Put(r.Context(), configuration); err != nil {
					internalServerError(w, err)
					return
				}
				http.Redirect(w, r, "/groups/"+group.ID+"?configuration_saved="+configuration.ID, http.StatusSeeOther)
				return
			}
			if action == "reject_deployment" {
				request, err := requestStore.Get(r.Context(), strings.TrimSpace(r.FormValue("request_id")))
				if err != nil || request.GroupID != group.ID {
					http.Error(w, "deployment request not found", http.StatusNotFound)
					return
				}
				reviewer := currentUsername(auth, r)
				if strings.EqualFold(reviewer, request.RequestedBy) {
					http.Error(w, "requesters cannot review their own deployment request", http.StatusForbidden)
					return
				}
				if err := requestStore.Review(r.Context(), request.ID, configs.GroupDeploymentPendingApproval, configs.GroupDeploymentRejected, reviewer, strings.TrimSpace(r.FormValue("review_comment"))); err != nil {
					http.Error(w, "deployment request is no longer pending approval", http.StatusConflict)
					return
				}
				http.Redirect(w, r, "/groups/"+group.ID+"?request_updated=rejected", http.StatusSeeOther)
				return
			}
			if action == "approve_deployment" {
				request, err := requestStore.Get(r.Context(), strings.TrimSpace(r.FormValue("request_id")))
				if err != nil || request.GroupID != group.ID {
					http.Error(w, "deployment request not found", http.StatusNotFound)
					return
				}
				if request.Status != configs.GroupDeploymentPendingApproval {
					http.Error(w, "deployment request is no longer pending approval", http.StatusConflict)
					return
				}
				reviewer := currentUsername(auth, r)
				if strings.EqualFold(reviewer, request.RequestedBy) {
					http.Error(w, "requesters cannot approve their own deployment request", http.StatusForbidden)
					return
				}
				if !group.Enabled || !sameSelector(group.Selector, request.GroupSelector) {
					http.Error(w, "group changed after preview; create a new deployment request", http.StatusConflict)
					return
				}
				configuration, err := configStore.Get(r.Context(), request.ConfigurationID)
				if err != nil || configuration.Hash != request.ConfigurationHash {
					http.Error(w, "approved configuration no longer matches the request", http.StatusConflict)
					return
				}
				if err := validateConfigurationForApproval(r.Context(), validator, configuration); err != nil {
					http.Error(w, "configuration is not eligible for deployment: "+err.Error(), http.StatusUnprocessableEntity)
					return
				}
				eligibleTargets := make([]configs.GroupDeploymentTarget, 0, len(request.Targets))
				for _, target := range request.Targets {
					if target.Eligible {
						eligibleTargets = append(eligibleTargets, target)
					}
				}
				if len(eligibleTargets) == 0 {
					http.Error(w, "approval request has no eligible targets", http.StatusConflict)
					return
				}
				currentMembers, err := membersForGroupIdentity(r.Context(), group, agentStore)
				if err != nil {
					internalServerError(w, err)
					return
				}
				currentByID := make(map[string]*agents.ManagedAgent, len(currentMembers))
				for _, member := range currentMembers {
					currentByID[member.InstanceUID] = member
				}
				readyMembers := make([]*agents.ManagedAgent, 0, len(eligibleTargets))
				for _, target := range eligibleTargets {
					current := currentByID[target.AgentInstanceUID]
					if current == nil {
						http.Error(w, "target agent membership changed; create a new deployment request", http.StatusConflict)
						return
					}
					preview, ready, err := previewGroupConfiguration(r.Context(), []*agents.ManagedAgent{current}, group.Enabled, configuration, assignmentStore)
					if err != nil {
						internalServerError(w, err)
						return
					}
					if ready != 1 || len(preview) != 1 || preview[0].Reason != "Ready" {
						http.Error(w, "target agent membership or readiness changed; create a new deployment request", http.StatusConflict)
						return
					}
					readyMembers = append(readyMembers, current)
				}
				if err := requestStore.Review(r.Context(), request.ID, configs.GroupDeploymentPendingApproval, configs.GroupDeploymentDeploying, reviewer, strings.TrimSpace(r.FormValue("review_comment"))); err != nil {
					http.Error(w, "deployment request is no longer pending approval", http.StatusConflict)
					return
				}
				failures := make([]string, 0)
				for _, member := range readyMembers {
					previousID, err := lastSuccessfulConfigurationID(r.Context(), member.InstanceUID, deploymentStore)
					if err != nil {
						failures = append(failures, member.Name+": "+err.Error())
						continue
					}
					if _, _, err := deliverConfigurationWithRollback(r.Context(), member.InstanceUID, configuration, configs.DeploymentActionDeploy, previousID, request.ID, assignmentStore, deploymentStore, adapter); err != nil {
						attemptAutomaticRollback(r.Context(), member.InstanceUID, configuration.Hash, configStore, assignmentStore, deploymentStore, adapter)
						failures = append(failures, member.Name+": "+err.Error())
					}
				}
				if len(failures) > 0 {
					_ = requestStore.UpdateStatus(r.Context(), request.ID, configs.GroupDeploymentDeploying, configs.GroupDeploymentFailed)
					http.Redirect(w, r, "/groups/"+group.ID+"?error="+url.QueryEscape("Deployment failed for "+strings.Join(failures, "; ")), http.StatusSeeOther)
					return
				}
				http.Redirect(w, r, "/groups/"+group.ID+"?request_updated=deploying", http.StatusSeeOther)
				return
			}
			if action == "request_deployment" {
				if !group.Enabled {
					http.Error(w, "group must be enabled before requesting deployment", http.StatusConflict)
					return
				}
				configuration, err := configStore.Get(r.Context(), strings.TrimSpace(r.FormValue("configuration_id")))
				if err != nil {
					http.Error(w, "configuration not found", http.StatusNotFound)
					return
				}
				// Revalidate the immutable artifact at the approval boundary. This
				// protects the queue if validation policy or Collector binaries
				// changed after the version was originally saved.
				if err := validateConfigurationForApproval(r.Context(), validator, configuration); err != nil {
					http.Error(w, "configuration is not eligible for approval: "+err.Error(), http.StatusUnprocessableEntity)
					return
				}
				members, err := membersForGroupIdentity(r.Context(), group, agentStore)
				if err != nil {
					internalServerError(w, err)
					return
				}
				preview, eligible, err := previewGroupConfiguration(r.Context(), members, group.Enabled, configuration, assignmentStore)
				if err != nil {
					internalServerError(w, err)
					return
				}
				if eligible == 0 {
					http.Error(w, "selected configuration is already deployed and latest, already in progress, or has no ready target", http.StatusConflict)
					return
				}
				targets := make([]configs.GroupDeploymentTarget, 0, len(preview))
				for _, item := range preview {
					targets = append(targets, configs.GroupDeploymentTarget{
						AgentInstanceUID: item.Agent.InstanceUID, AgentName: item.Agent.Name,
						Readiness: item.Reason, Eligible: item.Reason == "Ready",
					})
				}
				requestedBy, ok := auth.sessionUsername(r)
				if !ok {
					requestedBy, _, ok = r.BasicAuth()
				}
				if !ok || strings.TrimSpace(requestedBy) == "" {
					http.Error(w, "authenticated requester identity is required", http.StatusUnauthorized)
					return
				}
				request, err := configs.NewGroupDeploymentRequest(group.ID, group.Name, group.Selector, configuration, targets, requestedBy)
				if err != nil {
					internalServerError(w, err)
					return
				}
				request.BaseConfigurationID, request.BaseConfigurationHash, err = commonAppliedConfiguration(r.Context(), preview, assignmentStore)
				if err != nil {
					internalServerError(w, err)
					return
				}
				if err := requestStore.Create(r.Context(), request); err != nil {
					internalServerError(w, err)
					return
				}
				http.Redirect(w, r, "/groups/"+group.ID+"?request_created="+request.ID, http.StatusSeeOther)
				return
			}
			if action == "delete" {
				members, err := membersForGroupIdentity(r.Context(), group, agentStore)
				if err != nil {
					internalServerError(w, err)
					return
				}
				if len(members) > 0 {
					http.Redirect(w, r, "/groups/"+group.ID+"?error=Cannot+delete+group%3A+unassign+all+agents+from+this+group+first", http.StatusSeeOther)
					return
				}
				requests, err := requestStore.ListByGroup(r.Context(), group.ID, 1)
				if err != nil {
					internalServerError(w, err)
					return
				}
				if len(requests) > 0 {
					http.Redirect(w, r, "/groups/"+group.ID+"?error=Cannot+delete+group%3A+approval+history+must+be+preserved", http.StatusSeeOther)
					return
				}
				if err := groupStore.Delete(r.Context(), group.ID); err != nil {
					internalServerError(w, err)
					return
				}
				http.Redirect(w, r, "/groups", http.StatusSeeOther)
				return
			}
			if action == "disable" || action == "enable" {
				group.Enabled = action == "enable"
				group.UpdatedAt = time.Now().UTC()
				if err := groupStore.Update(r.Context(), group); err != nil {
					internalServerError(w, err)
					return
				}
				http.Redirect(w, r, "/groups/"+group.ID, http.StatusSeeOther)
				return
			}
			if action == "update" {
				selector, err := parseGroupSelectorForm(r)
				if err != nil {
					http.Error(w, err.Error(), 400)
					return
				}
				group.Selector = selector
				group.Name = canonicalGroupName(selector)
				group.UpdatedAt = time.Now().UTC()
				if err := groupStore.Update(r.Context(), group); err != nil {
					http.Error(w, err.Error(), 409)
					return
				}
				http.Redirect(w, r, "/groups/"+group.ID, http.StatusSeeOther)
				return
			}
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		members, err := membersForGroupIdentity(r.Context(), group, agentStore)
		if err != nil {
			internalServerError(w, err)
			return
		}
		available, err := configStore.List(r.Context())
		if err != nil {
			internalServerError(w, err)
			return
		}
		requests, err := requestStore.ListByGroup(r.Context(), group.ID, 20)
		if err != nil {
			internalServerError(w, err)
			return
		}
		for _, request := range requests {
			if request.Status != configs.GroupDeploymentDeploying {
				continue
			}
			eligible, applied, failed := 0, 0, 0
			for index, target := range request.Targets {
				if !target.Eligible {
					continue
				}
				eligible++
				deployments, listErr := deploymentStore.ListByAgent(r.Context(), target.AgentInstanceUID, 20)
				if listErr != nil {
					internalServerError(w, listErr)
					return
				}
				for _, deployment := range deployments {
					if deployment.ConfigurationHash != request.ConfigurationHash || deployment.CreatedAt.Before(request.CreatedAt) {
						continue
					}
					request.Targets[index].Readiness = string(deployment.Status)
					switch deployment.Status {
					case configs.DeliveryApplied:
						applied++
					case configs.DeliveryFailed, configs.DeliveryUnsupported:
						failed++
					}
					break
				}
			}
			switch {
			case failed > 0:
				_ = requestStore.UpdateStatus(r.Context(), request.ID, configs.GroupDeploymentDeploying, configs.GroupDeploymentFailed)
				request.Status = configs.GroupDeploymentFailed
			case eligible > 0 && applied == eligible:
				_ = requestStore.UpdateStatus(r.Context(), request.ID, configs.GroupDeploymentDeploying, configs.GroupDeploymentCompleted)
				request.Status = configs.GroupDeploymentCompleted
			}
		}
		driftSummary, err := buildGroupDrift(r.Context(), members, configStore, assignmentStore, adapter)
		if err != nil {
			internalServerError(w, err)
			return
		}
		deploymentHistory, err := groupDeploymentHistory(r.Context(), members, deploymentStore, 50)
		if err != nil {
			internalServerError(w, err)
			return
		}
		view := groupDetailView{
			Page: "groups", Group: group, Members: members, Configurations: available,
			Requests: requests, DeploymentHistory: deploymentHistory, DriftSummary: driftSummary,
			CanAdminister:      currentRole(auth, r) == roleAdmin,
			OwnersText:         strings.Join(group.Owners, ", "),
			ConfigurationSaved: r.URL.Query().Get("configuration_saved"),
			RequestCreated:     r.URL.Query().Get("request_created"),
			RequestUpdated:     r.URL.Query().Get("request_updated"), Error: r.URL.Query().Get("error"),
		}
		if configID := strings.TrimSpace(r.URL.Query().Get("configuration_id")); configID != "" {
			for _, configuration := range available {
				if configuration.ID == configID {
					view.SelectedConfig = configuration
					break
				}
			}
			if view.SelectedConfig == nil {
				http.Error(w, "configuration not found", http.StatusNotFound)
				return
			}
			view.Preview, view.Eligible, err = previewGroupConfiguration(r.Context(), members, group.Enabled, view.SelectedConfig, assignmentStore)
			if err != nil {
				internalServerError(w, err)
				return
			}
			view.SelectedPipeline, err = configs.ParsePipelineModel(view.SelectedConfig.Content)
			if err != nil {
				view.PipelineError = err.Error()
			}
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = groupDetailPage.Execute(w, view)
	})
}

// parseGroupSelectorForm accepts structured form fields and converts them into a normalized selector map.
func parseGroupSelectorForm(r *http.Request) (map[string]string, error) {
	selector := map[string]string{
		"application": strings.TrimSpace(r.FormValue("application")),
		"environment": strings.TrimSpace(r.FormValue("environment")),
		"place":       strings.TrimSpace(r.FormValue("place")),
	}
	if err := validateGroupSelector(selector); err != nil {
		return nil, err
	}
	return selector, nil
}

// parseSelectorText parses one key=value selector per line and reports malformed or duplicate entries.
func parseSelectorText(input string) (map[string]string, error) {
	result := map[string]string{}
	for _, part := range strings.Split(input, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		pieces := strings.SplitN(part, "=", 2)
		if len(pieces) != 2 || strings.TrimSpace(pieces[0]) == "" || strings.TrimSpace(pieces[1]) == "" {
			return nil, errors.New("selector must use key=value pairs separated by commas")
		}
		result[strings.TrimSpace(pieces[0])] = strings.TrimSpace(pieces[1])
	}
	if len(result) == 0 {
		return nil, errors.New("at least one selector label is required")
	}
	return result, nil
}

var legacyGroupDetailPage = template.Must(template.New("group-detail").Parse(`<!doctype html><html><head><meta charset="utf-8"><title>FleetAMP Group</title><style>
body{font-family:system-ui,sans-serif;background:#0b1220;color:#e5e7eb;margin:0;padding:32px;max-width:1400px}a{color:#93c5fd;text-decoration:none}.nav{display:flex;gap:16px;margin-bottom:22px}.card{background:#111827;border:1px solid #1f2937;border-radius:12px;padding:18px;margin-bottom:18px}table{width:100%;border-collapse:collapse}th,td{padding:12px;text-align:left;border-bottom:1px solid #1f2937}th{color:#93c5fd}code{color:#c4b5fd}.muted{color:#94a3b8}.ok{color:#86efac}.bad{color:#fca5a5}input{background:#0b1220;color:#e5e7eb;border:1px solid #334155;border-radius:7px;padding:9px;margin:4px;min-width:220px}button{padding:9px 14px;border-radius:7px;border:0;cursor:pointer}.danger{background:#7f1d1d;color:white}.warning{background:#422006;border:1px solid #92400e;color:#fde68a}.status{display:inline-block;padding:4px 9px;border-radius:999px;font-size:12px;font-weight:700}.enabled{background:#14532d;color:#bbf7d0}.disabled{background:#3f3f46;color:#d4d4d8}.actionrow{display:flex;gap:8px;flex-wrap:wrap}.iconaction{display:inline-flex;gap:7px;align-items:center}
</style></head><body><div class="nav"><a href="/agents">Agents</a><a href="/groups">Groups</a></div><h1>{{.Group.Name}}</h1><p>{{if .Group.Enabled}}<span class="status enabled">Enabled</span>{{else}}<span class="status disabled">Disabled</span>{{end}}</p>{{if .Error}}<section class="card warning"><strong>⚠️ {{.Error}}</strong></section>{{end}}
<section class="card"><h2>✏️ Edit group</h2><form method="post" action="/groups/{{.Group.ID}}"><input type="hidden" name="action" value="update"><label>Application<br><input name="application" value="{{index .Group.Selector "application"}}" required></label><label>Environment<br><input name="environment" value="{{index .Group.Selector "environment"}}" required></label><label>Place<br><input name="place" value="{{index .Group.Selector "place"}}" required></label><p class="muted">Group name is regenerated automatically from Application, Environment and Place.</p><button type="submit">💾 Update group</button></form><div class="actionrow" style="margin-top:14px"><form method="post" action="/groups/{{.Group.ID}}"><input type="hidden" name="action" value="{{if .Group.Enabled}}disable{{else}}enable{{end}}"><button type="submit" class="iconaction">{{if .Group.Enabled}}⏸️ Disable group{{else}}▶️ Enable group{{end}}</button></form><form method="post" action="/groups/{{.Group.ID}}"><input type="hidden" name="action" value="delete"><button class="danger iconaction" type="submit">🗑️ Delete group</button></form></div>{{if .Members}}<p class="muted" style="margin-top:12px">Delete protection is active: {{len .Members}} agent(s) currently match this group identity. Unassign them before deleting the group.</p>{{end}}</section>
<section class="card"><h2>Assigned agents ({{len .Members}})</h2>{{if .Members}}<table><thead><tr><th>Name</th><th>Type</th><th>Status</th><th>Health</th><th>Managed labels</th></tr></thead><tbody>{{range .Members}}<tr><td><a href="/agents/{{.InstanceUID}}">{{.Name}}</a></td><td>{{.Type}}</td><td>{{.Status}}</td><td class="{{if .Healthy}}ok{{else}}bad{{end}}">{{if .Healthy}}Healthy{{else}}Unhealthy{{end}}</td><td>{{range $k,$v := .Labels}}<code>{{$k}}={{$v}}</code> {{end}}</td></tr>{{end}}</tbody></table>{{else}}<p class="muted">No agents currently match this group identity.</p>{{end}}</section></body></html>`))
