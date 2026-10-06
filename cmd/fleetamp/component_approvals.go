package main

import (
	"html/template"
	"net/http"
	"strings"

	"github.com/marellasunil/FleetAMP/internal/lifecycle"
	"github.com/marellasunil/FleetAMP/internal/storage"
)

type componentApprovalsView struct { Page string; Items []*lifecycle.Approval; Plans []*lifecycle.ExecutionPlan; Message string }

const componentApprovalsHTML = `<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Component approvals · FleetAMP</title><style>`+controlPlaneCSS+detailCSS+`</style></head><body><div class="shell">`+sideNav+`<main class="main"><header class="top"><div><div class="crumb">FleetAMP / Approvals / Components</div><div class="pagetitle">Component lifecycle approvals</div><div class="subtitle">Four-eyes review and executor-neutral preparation of component lifecycle requests.</div></div><div class="topactions"><a class="btn" href="/approvals">Configuration approvals</a><a class="btn" href="/deployments">Deployments</a></div></header><div class="content">{{if .Message}}<div class="notice">{{.Message}}</div>{{end}}<section class="card"><div class="cardhead"><div><div class="cardtitle">Approval queue</div><div class="cardsub">Approval records pin both proposal and validation hashes. Preparation verifies them again and never executes the operation.</div></div><span class="badge off">No executor registered</span></div>{{if .Items}}<div style="overflow:auto"><table><thead><tr><th>Group / component</th><th>Immutable evidence</th><th>Requester / reviewer</th><th>Status</th><th>Action</th></tr></thead><tbody>{{range .Items}}<tr><td><strong>{{.GroupName}}</strong><div class="tiny">{{.Operation}} · {{.ComponentType}} · {{.TargetCount}} target(s)</div><div class="tiny">{{.SubmissionComment}}</div></td><td><div class="tiny code">Proposal: {{.RequestSpecHash}}</div><div class="tiny code">Validation: {{.ValidationHash}}</div></td><td>{{.RequestedBy}}<div class="tiny">Reviewer: {{.AssignedReviewer}}</div></td><td><span class="badge {{if eq .Status "approved"}}ok{{else if eq .Status "pending_approval"}}warn{{else}}off{{end}}">{{.Status}}</span>{{if .ReviewedBy}}<div class="tiny">{{.ReviewedBy}} · {{.ReviewComment}}</div>{{end}}</td><td>{{if eq .Status "pending_approval"}}<form method="post" action="/component-approvals"><input type="hidden" name="approval_id" value="{{.ID}}"><input class="input" name="review_comment" maxlength="500" placeholder="Review comment"><div class="detailactions"><button class="btn primary" name="decision" value="approved">Approve</button><button class="btn" name="decision" value="rejected">Reject</button><button class="btn" name="decision" value="sent_back">Send back</button></div></form>{{else if eq .Status "approved"}}<form method="post" action="/component-approvals"><input type="hidden" name="action" value="prepare_execution"><input type="hidden" name="approval_id" value="{{.ID}}"><button class="btn primary" type="submit">Prepare execution contract</button></form>{{else}}<span class="tiny">Review complete</span>{{end}}</td></tr>{{end}}</tbody></table></div>{{else}}<div class="empty">No component lifecycle approval requests exist.</div>{{end}}</section><section class="card" style="margin-top:16px"><div class="cardhead"><div><div class="cardtitle">Prepared execution contracts</div><div class="cardsub">Immutable, content-addressed plans. No adapter is registered to execute them.</div></div></div>{{if .Plans}}<div style="overflow:auto"><table><thead><tr><th>Executor</th><th>Operation</th><th>Targets</th><th>Evidence</th><th>Prepared</th></tr></thead><tbody>{{range .Plans}}<tr><td><span class="badge off">{{.ExecutorKind}}</span></td><td>{{.Operation}} · {{.ComponentType}}</td><td>{{len .Targets}}</td><td><div class="tiny code">Plan: {{.PlanHash}}</div><div class="tiny code">Proposal: {{.RequestSpecHash}}</div><div class="tiny code">Validation: {{.ValidationHash}}</div></td><td>{{.PreparedBy}}<div class="tiny">{{.CreatedAt}}</div></td></tr>{{end}}</tbody></table></div>{{else}}<div class="empty">No execution contracts have been prepared.</div>{{end}}</section></div></main></div></body></html>`

var componentApprovalsPage=template.Must(template.New("component-approvals").Parse(componentApprovalsHTML))

func registerComponentLifecycleApprovalRoutes(mux *http.ServeMux,store storage.ComponentLifecycleApprovalStore,requestStore storage.ComponentLifecycleRequestStore,validationStore storage.ComponentLifecycleValidationStore,executionStore storage.ComponentLifecycleExecutionStore,groupStore storage.GroupStore,auth *authManager){
	mux.HandleFunc("/component-approvals",func(w http.ResponseWriter,r *http.Request){
		if r.URL.Path!="/component-approvals"{http.NotFound(w,r);return}
		if r.Method==http.MethodPost{
			if err:=r.ParseForm();err!=nil{http.Error(w,"invalid review",http.StatusBadRequest);return}
			approval,err:=store.Get(r.Context(),strings.TrimSpace(r.FormValue("approval_id")));if err!=nil{http.NotFound(w,r);return}
			group,err:=groupStore.Get(r.Context(),approval.GroupID);if err!=nil||!canAccessGroup(auth,r,group){http.Error(w,"forbidden",http.StatusForbidden);return}
			if r.FormValue("action")=="prepare_execution"{
				if currentRole(auth,r)!=roleAdmin{http.Error(w,"administrator access required",http.StatusForbidden);return}
				request,err:=requestStore.Get(r.Context(),approval.RequestID);if err!=nil{http.Error(w,"proposal unavailable",http.StatusConflict);return};validation,err:=validationStore.Get(r.Context(),approval.ValidationID);if err!=nil{http.Error(w,"validation unavailable",http.StatusConflict);return}
				preparer:=currentUsername(auth,r);if preparer==""&&auth==nil{preparer="test-admin"};plan,err:=lifecycle.PrepareExecution(approval,request,validation,preparer);if err!=nil{http.Error(w,err.Error(),http.StatusUnprocessableEntity);return};if err:=executionStore.Create(r.Context(),plan);err!=nil{http.Error(w,"execution contract already prepared",http.StatusConflict);return};http.Redirect(w,r,"/component-approvals?prepared="+plan.ID,http.StatusSeeOther);return
			}
			reviewer:=currentUsername(auth,r);if reviewer==""&&auth==nil{reviewer=approval.AssignedReviewer}
			if !strings.EqualFold(reviewer,approval.AssignedReviewer)||strings.EqualFold(reviewer,approval.RequestedBy){http.Error(w,"assigned reviewer required",http.StatusForbidden);return}
			decision:=lifecycle.ApprovalStatus(r.FormValue("decision"));if decision!=lifecycle.ApprovalApproved&&decision!=lifecycle.ApprovalRejected&&decision!=lifecycle.ApprovalSentBack{http.Error(w,"invalid decision",http.StatusBadRequest);return}
			comment:=strings.TrimSpace(r.FormValue("review_comment"));if decision!=lifecycle.ApprovalApproved&&comment==""{http.Error(w,"review comment required",http.StatusUnprocessableEntity);return}
			if err:=store.Review(r.Context(),approval.ID,lifecycle.ApprovalPending,decision,reviewer,comment);err!=nil{http.Error(w,"approval state changed",http.StatusConflict);return}
			http.Redirect(w,r,"/component-approvals?reviewed="+approval.ID,http.StatusSeeOther);return
		}
		if r.Method!=http.MethodGet&&r.Method!=http.MethodHead{http.Error(w,"method not allowed",http.StatusMethodNotAllowed);return}
		items,err:=store.List(r.Context(),200);if err!=nil{internalServerError(w,err);return};plans,err:=executionStore.List(r.Context(),200);if err!=nil{internalServerError(w,err);return}
		visible:=make([]*lifecycle.Approval,0,len(items));for _,item:=range items{group,groupErr:=groupStore.Get(r.Context(),item.GroupID);if groupErr==nil&&(canAccessGroup(auth,r,group)||strings.EqualFold(item.AssignedReviewer,currentUsername(auth,r))){visible=append(visible,item)}}
		message:="";if r.URL.Query().Get("submitted")!=""{message="Component lifecycle request submitted for approval. No operation has been executed."};if r.URL.Query().Get("reviewed")!=""{message="Component lifecycle review recorded. No operation has been executed."}
		if r.URL.Query().Get("prepared")!=""{message="Immutable execution contract prepared. No executor has run."}
		w.Header().Set("Content-Type","text/html; charset=utf-8");if err:=componentApprovalsPage.Execute(w,componentApprovalsView{Page:"approvals",Items:visible,Plans:plans,Message:message});err!=nil{internalServerError(w,err)}
	})
}
