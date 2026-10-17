package main

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/marellasunil/FleetAMP/internal/agents"
	"github.com/marellasunil/FleetAMP/internal/configs"
	"github.com/marellasunil/FleetAMP/internal/groups"
	fleetopamp "github.com/marellasunil/FleetAMP/internal/opamp"
	"github.com/marellasunil/FleetAMP/internal/storage"
	"github.com/marellasunil/FleetAMP/internal/storage/memory"
)

type membershipView struct {
	Page        string
	Group       *groups.Group
	Members     []*agents.ManagedAgent
	Available   []*agents.ManagedAgent
	OtherGroups []*groups.Group
	Reviewers   []reviewerOption
	Requests    []*configs.MembershipChange
	Message     string
	Error       string
	CanManage   bool
}

func latestApprovedGroupConfiguration(ctx context.Context, configsStore storage.ConfigurationStore, requests storage.GroupDeploymentRequestStore, groupID string) (*configs.Configuration, error) {
	items, err := requests.ListByGroup(ctx, groupID, 100)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if item.Status == configs.GroupDeploymentCompleted {
			return configsStore.Get(ctx, item.ConfigurationID)
		}
	}
	return nil, nil
}

func unassignedAgents(r *http.Request, agentStore *memory.AgentStore, groupStore storage.GroupStore) ([]*agents.ManagedAgent, error) {
	all, err := agentStore.List(r.Context())
	if err != nil {
		return nil, err
	}
	allGroups, err := groupStore.List(r.Context())
	if err != nil {
		return nil, err
	}
	var result []*agents.ManagedAgent
	for _, agent := range all {
		assigned := false
		for _, group := range allGroups {
			if groups.Matches(group, agent) {
				assigned = true
				break
			}
		}
		if !assigned {
			result = append(result, agent)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}

func membershipBaseline(agentUID string, history []*configs.MembershipChange) string {
	for _, change := range history {
		if change.Status != configs.MembershipCompleted {
			continue
		}
		for _, target := range change.Targets {
			if target.AgentInstanceUID == agentUID && (change.Operation == configs.MembershipAssign || change.Operation == configs.MembershipMove) {
				return target.PreviousConfigurationID
			}
		}
	}
	return ""
}

func registerMembershipRoutes(mux *http.ServeMux, changes storage.MembershipChangeStore, groupStore storage.GroupStore, agentStore *memory.AgentStore, configStore storage.ConfigurationStore, assignmentStore storage.AssignmentStore, deploymentStore storage.DeploymentStore, deploymentRequests storage.GroupDeploymentRequestStore, adapter *fleetopamp.Adapter, auth *authManager, dataDir string) {
	mux.HandleFunc("/groups/{id}/collectors", func(w http.ResponseWriter, r *http.Request) {
		group, err := groupStore.Get(r.Context(), r.PathValue("id"))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		if !canAccessGroup(auth, r, group) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		canManage := canManageGroup(auth, r, group)
		if r.Method == http.MethodPost {
			if !canManage {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			if err := r.ParseForm(); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			action := r.FormValue("action")
			switch action {
			case "request":
				operation := configs.MembershipOperation(r.FormValue("operation"))
				uids := r.Form["agent_uid"]
				if len(uids) == 0 {
					http.Error(w, "select at least one Collector", http.StatusBadRequest)
					return
				}
				targetGroup := group
				sourceGroup := (*groups.Group)(nil)
				if operation == configs.MembershipRemove || operation == configs.MembershipMove {
					sourceGroup = group
				}
				if operation == configs.MembershipMove {
					targetGroup, err = groupStore.Get(r.Context(), r.FormValue("target_group_id"))
					if err != nil {
						http.Error(w, "target group not found", http.StatusBadRequest)
						return
					}
					if !canManageGroup(auth, r, targetGroup) {
						http.Error(w, "permission to manage both source and target groups is required", http.StatusForbidden)
						return
					}
				}
				reviewer := strings.TrimSpace(r.FormValue("assigned_reviewer"))
				if err := validateAssignedReviewer(r.Context(), auth, group, currentUsername(auth, r), reviewer); err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				allHistory, _ := changes.List(r.Context(), 500)
				allGroups, _ := groupStore.List(r.Context())
				targets := make([]configs.MembershipTarget, 0, len(uids))
				for _, uid := range uids {
					agent, getErr := agentStore.Get(r.Context(), uid)
					if getErr != nil {
						http.Error(w, "Collector not found", http.StatusBadRequest)
						return
					}
					isMember := groups.MatchesIdentity(group, agent)
					if operation == configs.MembershipAssign {
						for _, existingGroup := range allGroups {
							if groups.Matches(existingGroup, agent) {
								http.Error(w, "Collector is already assigned to "+existingGroup.Name+"; use Move instead", http.StatusConflict)
								return
							}
						}
					}
					if (operation == configs.MembershipRemove || operation == configs.MembershipMove) && !isMember {
						http.Error(w, "Collector membership changed; refresh and retry", http.StatusConflict)
						return
					}
					previousID := ""
					if latest, e := latestAssignmentForAgent(r.Context(), assignmentStore, uid); e == nil {
						previousID = latest.ConfigurationID
					}
					if operation == configs.MembershipRemove && configs.RemovalPolicy(r.FormValue("removal_policy")) == configs.RemovalRestore {
						if baseline := membershipBaseline(uid, allHistory); baseline != "" {
							previousID = baseline
						}
					}
					targets = append(targets, configs.MembershipTarget{AgentInstanceUID: uid, AgentName: agent.Name, PreviousGroupFields: copyStringMap(agent.GroupFields), PreviousConfigurationID: previousID})
				}
				targetConfigID := ""
				if operation != configs.MembershipRemove {
					c, lookupErr := latestApprovedGroupConfiguration(r.Context(), configStore, deploymentRequests, targetGroup.ID)
					if lookupErr != nil {
						internalServerError(w, lookupErr)
						return
					}
					if c != nil {
						targetConfigID = c.ID
					}
				}
				proposal := configs.MembershipChange{Operation: operation, RemovalPolicy: configs.RemovalPolicy(r.FormValue("removal_policy")), TargetConfigurationID: targetConfigID, Targets: targets, RequestedBy: currentUsername(auth, r), AssignedReviewer: reviewer, Reason: strings.TrimSpace(r.FormValue("reason"))}
				if operation != configs.MembershipRemove {
					proposal.TargetGroupID, proposal.TargetGroupName = targetGroup.ID, targetGroup.Name
				}
				if sourceGroup != nil {
					proposal.SourceGroupID, proposal.SourceGroupName = sourceGroup.ID, sourceGroup.Name
				}
				value, createErr := configs.NewMembershipChange(proposal)
				if createErr != nil {
					http.Error(w, createErr.Error(), http.StatusBadRequest)
					return
				}
				if err := changes.Create(r.Context(), value); err != nil {
					internalServerError(w, err)
					return
				}
				http.Redirect(w, r, "/groups/"+group.ID+"/collectors?message=Request+created+and+awaiting+approval", http.StatusSeeOther)
				return
			case "reject":
				value, err := changes.Get(r.Context(), r.FormValue("request_id"))
				if err != nil {
					http.NotFound(w, r)
					return
				}
				if !strings.EqualFold(value.AssignedReviewer, currentUsername(auth, r)) {
					http.Error(w, "only the assigned reviewer can reject", http.StatusForbidden)
					return
				}
				if err := changes.Update(r.Context(), value.ID, configs.MembershipPending, configs.MembershipRejected, currentUsername(auth, r), r.FormValue("review_comment"), ""); err != nil {
					http.Error(w, err.Error(), http.StatusConflict)
					return
				}
				http.Redirect(w, r, "/groups/"+group.ID+"/collectors?message=Request+rejected", http.StatusSeeOther)
				return
			case "approve":
				value, err := changes.Get(r.Context(), r.FormValue("request_id"))
				if err != nil {
					http.NotFound(w, r)
					return
				}
				if !strings.EqualFold(value.AssignedReviewer, currentUsername(auth, r)) {
					http.Error(w, "only the assigned reviewer can approve", http.StatusForbidden)
					return
				}
				if err := changes.Update(r.Context(), value.ID, configs.MembershipPending, configs.MembershipExecuting, currentUsername(auth, r), r.FormValue("review_comment"), ""); err != nil {
					http.Error(w, err.Error(), http.StatusConflict)
					return
				}
				execErr := executeMembershipChange(r, value, groupStore, agentStore, configStore, assignmentStore, deploymentStore, adapter, dataDir)
				if execErr != nil {
					_ = changes.Update(r.Context(), value.ID, configs.MembershipExecuting, configs.MembershipFailed, "", "", execErr.Error())
					http.Redirect(w, r, "/groups/"+group.ID+"/collectors?error="+urlQuery(execErr.Error()), http.StatusSeeOther)
					return
				}
				_ = changes.Update(r.Context(), value.ID, configs.MembershipExecuting, configs.MembershipCompleted, "", "", "")
				http.Redirect(w, r, "/groups/"+group.ID+"/collectors?message=Membership+change+completed", http.StatusSeeOther)
				return
			}
		}
		members, err := membersForGroupIdentity(r.Context(), group, agentStore)
		if err != nil {
			internalServerError(w, err)
			return
		}
		available, err := unassignedAgents(r, agentStore, groupStore)
		if err != nil {
			internalServerError(w, err)
			return
		}
		allGroups, _ := groupStore.List(r.Context())
		var others []*groups.Group
		for _, candidate := range allGroups {
			if candidate.ID != group.ID && candidate.Enabled {
				others = append(others, candidate)
			}
		}
		requests, _ := changes.List(r.Context(), 100)
		var scoped []*configs.MembershipChange
		for _, item := range requests {
			if item.SourceGroupID == group.ID || item.TargetGroupID == group.ID {
				scoped = append(scoped, item)
			}
		}
		reviewers, _ := eligibleDeploymentReviewers(r.Context(), auth, group, currentUsername(auth, r), currentRole(auth, r))
		view := membershipView{Page: "groups", Group: group, Members: members, Available: available, OtherGroups: others, Reviewers: reviewers, Requests: scoped, Message: r.URL.Query().Get("message"), Error: r.URL.Query().Get("error"), CanManage: canManage}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = membershipPage.Execute(w, view)
	})
}

func urlQuery(value string) string { return url.QueryEscape(value) }

func executeMembershipChange(r *http.Request, value *configs.MembershipChange, groupStore storage.GroupStore, agentStore *memory.AgentStore, configStore storage.ConfigurationStore, assignmentStore storage.AssignmentStore, deploymentStore storage.DeploymentStore, adapter *fleetopamp.Adapter, dataDir string) error {
	var targetGroup *groups.Group
	var targetConfig *configs.Configuration
	var err error
	if value.Operation != configs.MembershipRemove {
		targetGroup, err = groupStore.Get(r.Context(), value.TargetGroupID)
		if err != nil {
			return err
		}
		if value.TargetConfigurationID != "" {
			targetConfig, err = configStore.Get(r.Context(), value.TargetConfigurationID)
			if err != nil {
				return err
			}
		}
	}
	allGroups, err := groupStore.List(r.Context())
	if err != nil {
		return err
	}
	var sourceGroup *groups.Group
	if value.SourceGroupID != "" {
		sourceGroup, err = groupStore.Get(r.Context(), value.SourceGroupID)
		if err != nil {
			return err
		}
	}
	for _, snapshot := range value.Targets {
		agent, err := agentStore.Get(r.Context(), snapshot.AgentInstanceUID)
		if err != nil {
			return err
		}
		if value.Operation == configs.MembershipAssign {
			for _, candidate := range allGroups {
				if groups.Matches(candidate, agent) {
					return fmt.Errorf("%s is now assigned to %s; create a new proposal", agent.Name, candidate.Name)
				}
			}
		} else if sourceGroup == nil || !groups.MatchesIdentity(sourceGroup, agent) {
			return fmt.Errorf("%s no longer belongs to the approved source group", agent.Name)
		}
		oldFields := copyStringMap(agent.GroupFields)
		if targetConfig != nil {
			_, _, err = deliverConfigurationWithRollback(r.Context(), agent.InstanceUID, targetConfig, configs.DeploymentActionDeploy, snapshot.PreviousConfigurationID, value.ID, assignmentStore, deploymentStore, adapter)
			if err != nil && !errors.Is(err, configs.ErrConfigurationCurrent) {
				return fmt.Errorf("%s: %w", agent.Name, err)
			}
		}
		if value.Operation == configs.MembershipRemove && value.RemovalPolicy == configs.RemovalRestore && snapshot.PreviousConfigurationID != "" {
			previous, e := configStore.Get(r.Context(), snapshot.PreviousConfigurationID)
			if e != nil {
				return e
			}
			_, _, e = deliverConfigurationWithRollback(r.Context(), agent.InstanceUID, previous, configs.DeploymentActionRollback, "", value.ID, assignmentStore, deploymentStore, adapter)
			if e != nil && !errors.Is(e, configs.ErrConfigurationCurrent) {
				return e
			}
		}
		if value.Operation == configs.MembershipRemove {
			agent.GroupFields = map[string]string{}
		} else {
			agent.GroupFields = copyStringMap(targetGroup.Selector)
		}
		if err = agentStore.Upsert(r.Context(), agent); err != nil {
			agent.GroupFields = oldFields
			return err
		}
	}
	return saveAgentSnapshot(r.Context(), agentStore, dataDir)
}

const membershipHTML = `<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Collector membership · FleetAMP</title><style>` + controlPlaneCSS + detailCSS + `.pick{display:grid;gap:8px}.pick label{display:flex;gap:9px;align-items:center;padding:10px;border:1px solid var(--line);border-radius:8px}.membership-actions{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:14px}@media(max-width:950px){.membership-actions{grid-template-columns:1fr}}</style></head><body><div class="shell">` + sideNav + `<main class="main"><header class="top"><div><div class="crumb">FleetAMP / Groups / Collector membership</div><div class="pagetitle">{{.Group.Name}}</div><div class="subtitle">Governed assignment, removal and movement of Collectors.</div></div><a class="btn" href="/groups/{{.Group.ID}}">← Group</a></header><div class="content">{{if .Message}}<div class="notice">✓ {{.Message}}</div>{{end}}{{if .Error}}<div class="configerror">{{.Error}}</div>{{end}}
<section class="card"><div class="cardhead"><div><div class="cardtitle">Create membership proposal</div><div class="cardsub">Every configuration-affecting change requires approval.</div></div></div><div class="cardbody membership-actions">
<form class="detailform" method="post"><input type="hidden" name="action" value="request"><input type="hidden" name="operation" value="assign"><strong>Assign available Collectors</strong><div class="pick">{{range .Available}}<label><input type="checkbox" name="agent_uid" value="{{.InstanceUID}}"> {{if .DisplayName}}{{.DisplayName}}{{else}}{{.Name}}{{end}}</label>{{else}}<span class="tiny">No unassigned Collectors.</span>{{end}}</div><label>Reviewer<select class="select" name="assigned_reviewer" required>{{range .Reviewers}}<option>{{.Username}}</option>{{end}}</select></label><label>Reason<textarea class="input" name="reason" required></textarea></label><button class="btn primary">Request assignment</button></form>
<form class="detailform" method="post"><input type="hidden" name="action" value="request"><input type="hidden" name="operation" value="remove"><strong>Remove from this group</strong><div class="pick">{{range .Members}}<label><input type="checkbox" name="agent_uid" value="{{.InstanceUID}}"> {{if .DisplayName}}{{.DisplayName}}{{else}}{{.Name}}{{end}}</label>{{else}}<span class="tiny">No members.</span>{{end}}</div><label>Configuration after removal<select class="select" name="removal_policy"><option value="keep_current">Keep current configuration</option><option value="restore_pre_group">Restore pre-group configuration</option></select></label><label>Reviewer<select class="select" name="assigned_reviewer" required>{{range .Reviewers}}<option>{{.Username}}</option>{{end}}</select></label><label>Reason<textarea class="input" name="reason" required></textarea></label><button class="btn">Request removal</button></form>
<form class="detailform" method="post"><input type="hidden" name="action" value="request"><input type="hidden" name="operation" value="move"><strong>Move to another group</strong><div class="pick">{{range .Members}}<label><input type="checkbox" name="agent_uid" value="{{.InstanceUID}}"> {{if .DisplayName}}{{.DisplayName}}{{else}}{{.Name}}{{end}}</label>{{else}}<span class="tiny">No members.</span>{{end}}</div><label>Target group<select class="select" name="target_group_id" required>{{range .OtherGroups}}<option value="{{.ID}}">{{.Name}}</option>{{end}}</select></label><label>Reviewer<select class="select" name="assigned_reviewer" required>{{range .Reviewers}}<option>{{.Username}}</option>{{end}}</select></label><label>Reason<textarea class="input" name="reason" required></textarea></label><button class="btn">Request move</button></form>
</div></section><section class="card" style="margin-top:16px"><div class="cardhead"><div><div class="cardtitle">Membership history</div><div class="cardsub">Immutable target snapshots and governed decisions.</div></div></div>{{if .Requests}}<div style="overflow:auto"><table><thead><tr><th>Operation</th><th>Scope</th><th>Collectors</th><th>Requester / reviewer</th><th>Status</th><th>Action</th></tr></thead><tbody>{{range .Requests}}<tr><td>{{.Operation}}</td><td>{{if .SourceGroupName}}{{.SourceGroupName}}{{end}}{{if .TargetGroupName}} → {{.TargetGroupName}}{{end}}</td><td>{{len .Targets}}</td><td>{{.RequestedBy}}<div class="tiny">{{.AssignedReviewer}}</div></td><td><span class="badge {{if eq .Status "completed"}}ok{{else}}warn{{end}}">{{.Status}}</span>{{if .Error}}<div class="tiny red">{{.Error}}</div>{{end}}</td><td>{{if eq .Status "pending_approval"}}<form method="post"><input type="hidden" name="request_id" value="{{.ID}}"><button class="btn primary" name="action" value="approve">Approve</button><button class="btn" name="action" value="reject">Reject</button></form>{{else}}—{{end}}</td></tr>{{end}}</tbody></table></div>{{else}}<div class="cardbody tiny">No membership proposals yet.</div>{{end}}</section></div></main></div></body></html>`

var membershipPage = template.Must(template.New("membership").Parse(membershipHTML))
