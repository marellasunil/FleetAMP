package main

import (
	"errors"
	"github.com/marellasunil/FleetAMP/internal/distributions"
	"github.com/marellasunil/FleetAMP/internal/storage"
	"html/template"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type distributionBuildPlanView struct {
	Page                 string
	Distributions        []*distributions.Distribution
	Plans                []*distributions.BuildPlan
	ApprovalByPlan       map[string]*distributions.BuildApproval
	ExecutionByPlan      map[string]*distributions.BuildExecution
	PreflightByExecution map[string]*distributions.BuildPreflight
	ResultByExecution    map[string]*distributions.BuildWorkerResult
	Sources              []*distributions.BuildSourceResolution
	Acquisitions         []*distributions.BuildSourceAcquisition
	Inspections          []*distributions.BuildSourceInspection
	Artifacts            []*distributions.BuildArtifact
	Attempts             []*distributions.BuildAttempt
	RetryRequests        []*distributions.BuildRetryRequest
	CanRetryAttempt      map[string]bool
	RetryPendingAttempt  map[string]bool
	Username             string
	Message, Error       string
}

const distributionBuildPlanHTML = `<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Collector Build Plans · FleetAMP</title><style>` + controlPlaneCSS + detailCSS + `.build-layout{display:grid;grid-template-columns:minmax(0,1fr) minmax(360px,.8fr);gap:16px}.checks{display:grid;gap:8px}@media(max-width:1050px){.build-layout{grid-template-columns:1fr}}</style></head><body><div class="shell">` + sideNav + `<main class="main"><header class="top"><div><div class="crumb">FleetAMP / Lifecycle / Distributions / Build plans</div><div class="pagetitle">Collector Build Plans</div><div class="subtitle">Freeze, approve, queue and trace reproducible sandboxed Collector builds.</div></div><a class="btn" href="/collector-distributions">Distribution catalog</a></header><div class="content">{{if .Message}}<div class="notice">{{.Message}}</div>{{end}}{{if .Error}}<div class="configerror" role="alert">{{.Error}}</div>{{end}}<div class="build-layout"><section class="card"><div class="cardhead"><div><div class="cardtitle">Create immutable build plan</div><div class="cardsub">Admin-only. Planning performs no clone, command, container build, registry push or deployment.</div></div><span class="badge off">Builds opt-in</span></div><div class="cardbody"><form method="post"><input type="hidden" name="action" value="create"><label>Published distribution<select class="select" name="distribution_id" required><option value="">Select distribution</option>{{range .Distributions}}<option value="{{.ID}}">{{.Name}} · Collector {{.CollectorVersion}}</option>{{end}}</select></label><fieldset class="checks" style="margin-top:14px"><legend>Build outputs</legend><label><input type="checkbox" name="outputs" value="oci-image"> OCI image</label><label><input type="checkbox" name="outputs" value="binary-archive"> Binary archive</label></fieldset><label style="display:block;margin-top:14px">Destination reference<input class="input code" name="destination_ref" required placeholder="oci://registry.example/platform/collector:0.140.0"></label><label style="display:block;margin-top:14px">Credential reference<input class="input code" name="credential_ref" required placeholder="secret://registry/platform"></label><div class="notice" style="margin-top:14px">Only references are stored. Never enter a token, password or private key.</div><div class="detailactions"><button class="btn primary" type="submit">Save immutable build plan</button></div></form></div></section><aside class="card"><div class="cardhead"><div><div class="cardtitle">Governance boundary</div><div class="cardsub">What this stage guarantees</div></div></div><div class="cardbody"><ul><li>Pinned official source reference</li><li>Frozen component inventory</li><li>Admin baseline evidence</li><li>Exact approval binding</li><li>Immutable execution and attempt receipts</li><li>Bounded sandboxed retries</li><li>No registry write or deployment</li></ul></div></aside></div><section class="card" style="margin-top:16px"><div class="cardhead"><div><div class="cardtitle">Immutable plans and execution outbox</div><div class="cardsub">A queued plan must pass preflight before an enabled worker can claim it.</div></div></div>{{if .Plans}}<div style="overflow:auto"><table><thead><tr><th>Plan</th><th>Distribution</th><th>Outputs</th><th>Destination</th><th>Requested</th><th>Status</th><th>Governance action</th></tr></thead><tbody>{{range .Plans}}{{$approval := index $.ApprovalByPlan .ID}}{{$execution := index $.ExecutionByPlan .ID}}<tr><td class="code">{{.PlanHash}}</td><td class="code">{{.Manifest.DistributionSpecHash}}</td><td>{{range .Manifest.Outputs}}<span class="chip">{{.}}</span>{{end}}</td><td class="code">{{.Manifest.DestinationRef}}</td><td>{{.RequestedBy}}<div class="tiny">{{.RequestedAt}}</div></td><td>{{if $execution}}{{with index $.PreflightByExecution $execution.ID}}<span class="badge ok">{{.Status}}</span><div class="tiny">Manifest hash verified<br>Checked by {{.CheckedBy}}</div>{{else}}<span class="badge off">{{$execution.Status}}</span><div class="tiny">Awaiting preflight</div>{{end}}{{else if $approval}}<span class="badge {{if eq $approval.Decision "approved"}}ok{{else}}off{{end}}">{{$approval.Decision}}</span><div class="tiny">{{$approval.Reason}}</div>{{else}}<span class="badge off">{{.Status}}</span>{{end}}</td><td>{{if $execution}}{{with index $.PreflightByExecution $execution.ID}}<span class="tiny">Immutable preflight evidence<br><span class="code">{{.ID}}</span><br>{{.CheckedAt}}</span>{{else}}<form method="post"><input type="hidden" name="action" value="preflight"><input type="hidden" name="execution_id" value="{{$execution.ID}}"><button class="btn primary" type="submit">Run safe preflight</button></form><div class="tiny">Verifies evidence only; no build runs during this request.</div>{{end}}{{else if and $approval (eq $approval.Decision "approved")}}<form method="post"><input type="hidden" name="action" value="queue"><input type="hidden" name="plan_id" value="{{.ID}}"><button class="btn primary" type="submit">Queue approved plan</button></form><div class="tiny">Records intent for opt-in workers.</div>{{else if $approval}}<span class="tiny">Rejected plans cannot be queued.<br>Reviewed by {{$approval.ReviewedBy}}</span>{{else if eq $.Username .RequestedBy}}<span class="tiny">A different administrator must review this plan.</span>{{else}}<form method="post"><input type="hidden" name="action" value="review"><input type="hidden" name="plan_id" value="{{.ID}}"><textarea class="input" name="reason" rows="2" required placeholder="Review reason"></textarea><div class="detailactions"><button class="btn primary" name="decision" value="approved">Approve</button><button class="btn" name="decision" value="rejected">Reject</button></div></form>{{end}}</td></tr>{{end}}</tbody></table></div>{{else}}<div class="empty">No build plans have been created.</div>{{end}}</section></div></main></div></body></html>`

const distributionBuildSourceHTML = `<section class="card" style="margin-top:16px"><div class="cardhead"><div><div class="cardtitle">Official source resolutions</div><div class="cardsub">Immutable, allow-listed archive specifications. Source content has not been downloaded or executed.</div></div><span class="badge off">Network fetch disabled</span></div>{{if .Sources}}<div style="overflow:auto"><table><thead><tr><th>Release</th><th>Repository</th><th>Archive specification</th><th>Evidence</th><th>Resolved</th></tr></thead><tbody>{{range .Sources}}<tr><td><span class="badge ok">{{.SourceRef}}</span><div class="tiny">{{.Status}}</div></td><td class="code">{{.Repository}}</td><td class="code">{{.ArchiveURL}}</td><td class="code">{{.EvidenceHash}}</td><td>{{.ResolvedBy}}<div class="tiny">{{.ResolvedAt}}</div></td></tr>{{end}}</tbody></table></div>{{else}}<div class="empty">No official source has been resolved by the dry-run worker.</div>{{end}}</section>`

const distributionBuildAcquisitionHTML = `<section class="card" style="margin-top:16px"><div class="cardhead"><div><div class="cardtitle">Verified source cache</div><div class="cardsub">Archives downloaded from the fixed official host with bounded streaming and SHA-256 verification.</div></div></div>{{if .Acquisitions}}<div style="overflow:auto"><table><thead><tr><th>Status</th><th>SHA-256</th><th>Size</th><th>Local reference</th><th>Acquired</th></tr></thead><tbody>{{range .Acquisitions}}<tr><td><span class="badge ok">{{.Status}}</span></td><td class="code">{{.ArchiveSHA256}}</td><td>{{.ArchiveSize}} bytes</td><td class="code">{{.LocalReference}}</td><td>{{.AcquiredBy}}<div class="tiny">{{.AcquiredAt}}</div></td></tr>{{end}}</tbody></table></div>{{else}}<div class="empty">No source archives acquired. Downloads are disabled by default.</div>{{end}}</section>`

const distributionBuildInspectionHTML = `<section class="card" style="margin-top:16px"><div class="cardhead"><div><div class="cardtitle">Safe archive inspections and OCB manifests</div><div class="cardsub">Tarballs are inspected without extraction. Unsafe paths, links, devices and size-limit violations are rejected before a deterministic allow-listed manifest is recorded.</div></div><span class="badge off">Safe inspection</span></div>{{if .Inspections}}<div style="overflow:auto"><table><thead><tr><th>Status</th><th>Archive inventory</th><th>Manifest evidence</th><th>Generated OCB manifest</th><th>Inspected</th></tr></thead><tbody>{{range .Inspections}}<tr><td><span class="badge ok">{{.Status}}</span><div class="tiny code">{{.EvidenceHash}}</div></td><td>{{.FileCount}} files · {{.DirectoryCount}} directories<div class="tiny">{{.UncompressedBytes}} uncompressed bytes<br>Root: <span class="code">{{.RootPrefix}}</span></div></td><td class="code">{{.ManifestSHA256}}</td><td><details><summary>View immutable manifest</summary><pre class="code" style="white-space:pre-wrap">{{.ManifestYAML}}</pre></details></td><td>{{.InspectedBy}}<div class="tiny">{{.InspectedAt}}</div></td></tr>{{end}}</tbody></table></div>{{else}}<div class="empty">No archive inspections recorded. Inspection is disabled by default.</div>{{end}}</section>`

const distributionBuildArtifactHTML = `<section class="card" style="margin-top:16px"><div class="cardhead"><div><div class="cardtitle">Sandboxed OCB artifacts</div><div class="cardsub">Offline builds run in a digest-pinned container with no network, no added capabilities, bounded resources and no host runtime socket mount.</div></div></div>{{if .Artifacts}}<div style="overflow:auto"><table><thead><tr><th>Status</th><th>Builder</th><th>Artifact</th><th>Evidence</th><th>Built</th></tr></thead><tbody>{{range .Artifacts}}<tr><td><span class="badge ok">{{.Status}}</span></td><td>{{.Runtime}}<div class="tiny code">{{.BuilderImage}}</div></td><td class="code">{{.ArtifactReference}}<div class="tiny">{{.ArtifactSize}} bytes<br>SHA-256: {{.ArtifactSHA256}}</div></td><td class="code">{{.EvidenceHash}}</td><td>{{.BuiltBy}}<div class="tiny">{{.BuiltAt}}</div></td></tr>{{end}}</tbody></table></div>{{else}}<div class="empty">No Collector artifact has been generated. The offline OCB runner is disabled by default.</div>{{end}}</section>`

const distributionBuildAttemptHTML = `<section class="card" style="margin-top:16px"><div class="cardhead"><div><div class="cardtitle">Immutable build attempts</div><div class="cardsub">Every completed OCB run is retained. Automatic retries are bounded; a manual retry creates a separate immutable authorization.</div></div></div>{{if .Attempts}}<div style="overflow:auto"><table><thead><tr><th>Attempt</th><th>Outcome</th><th>Trigger</th><th>Evidence</th><th>Completed</th><th>Action</th></tr></thead><tbody>{{range .Attempts}}<tr><td><strong>#{{.AttemptNumber}}</strong><div class="tiny code">{{.InspectionID}}</div></td><td><span class="badge {{if eq .Status "succeeded"}}ok{{else}}off{{end}}">{{.Status}}</span>{{if .FailureCode}}<div class="tiny"><strong>{{.FailureCode}}</strong><br>{{.FailureMessage}}<br>Diagnostic fingerprint: <span class="code">{{.FailureDigest}}</span></div>{{end}}{{if not .NextRetryAt.IsZero}}<div class="tiny">Automatic retry after {{.NextRetryAt}}</div>{{end}}</td><td>{{.Trigger}}{{if .RetryRequestID}}<div class="tiny code">{{.RetryRequestID}}</div>{{end}}</td><td class="code">{{.EvidenceHash}}{{if .ArtifactSHA256}}<div class="tiny">Artifact: {{.ArtifactSHA256}}</div>{{end}}</td><td>{{.WorkerID}}<div class="tiny">{{.CompletedAt}}</div></td><td>{{if index $.CanRetryAttempt .ID}}<form method="post"><input type="hidden" name="action" value="retry-build"><input type="hidden" name="inspection_id" value="{{.InspectionID}}"><input type="hidden" name="attempt_id" value="{{.ID}}"><textarea class="input" name="reason" rows="2" required maxlength="500" placeholder="Reason for manual retry"></textarea><button class="btn primary" type="submit">Authorize retry</button></form>{{else if index $.RetryPendingAttempt .ID}}<span class="badge off">Retry queued</span>{{else}}<span class="tiny">No action</span>{{end}}</td></tr>{{end}}</tbody></table></div>{{else}}<div class="empty">No sandboxed build attempts have completed.</div>{{end}}</section>`

var distributionBuildPlanPage = template.Must(template.New("distribution-build-plans").Parse(strings.Replace(strings.Replace(distributionBuildPlanHTML, `<a class="btn" href="/collector-distributions">Distribution catalog</a>`, `<div class="topactions"><a class="btn" href="/collector-distributions/promotions">Artifact promotions</a><a class="btn" href="/collector-distributions">Distribution catalog</a></div>`, 1), `</div></main></div></body></html>`, distributionBuildSourceHTML+distributionBuildAcquisitionHTML+distributionBuildInspectionHTML+distributionBuildAttemptHTML+distributionBuildArtifactHTML+`</div></main></div></body></html>`, 1)))

func registerDistributionBuildPlanRoutes(mux *http.ServeMux, distributionStore storage.DistributionStore, planStore storage.DistributionBuildPlanStore, approvalStore storage.DistributionBuildApprovalStore, executionStore storage.DistributionBuildExecutionStore, preflightStore storage.DistributionBuildPreflightStore, resultStore storage.DistributionBuildWorkerResultStore, sourceStore storage.DistributionBuildSourceStore, acquisitionStore storage.DistributionBuildSourceAcquisitionStore, inspectionStore storage.DistributionBuildSourceInspectionStore, artifactStore storage.DistributionBuildArtifactStore, attemptStore storage.DistributionBuildAttemptStore, retryStore storage.DistributionBuildRetryRequestStore, auth *authManager) {
	mux.HandleFunc("/collector-distributions/build-plans", func(w http.ResponseWriter, r *http.Request) {
		if auth != nil && currentRole(auth, r) != roleAdmin {
			http.Error(w, "administrator access required", http.StatusForbidden)
			return
		}
		if r.Method == http.MethodPost {
			if err := r.ParseForm(); err != nil {
				http.Error(w, "invalid form", http.StatusBadRequest)
				return
			}
			action := strings.TrimSpace(r.FormValue("action"))
			var err error
			message := "Immutable build plan saved. No execution was performed."
			if action == "retry-build" {
				var inspections []*distributions.BuildSourceInspection
				inspections, err = inspectionStore.List(r.Context(), 1000)
				var inspection *distributions.BuildSourceInspection
				for _, candidate := range inspections {
					if candidate.ID == strings.TrimSpace(r.FormValue("inspection_id")) {
						inspection = candidate
						break
					}
				}
				if err == nil && inspection == nil {
					err = errors.New("source inspection not found")
				}
				var attempts []*distributions.BuildAttempt
				if err == nil {
					attempts, err = attemptStore.List(r.Context(), 10000)
				}
				var latest *distributions.BuildAttempt
				for _, candidate := range attempts {
					if candidate.InspectionID == inspection.ID && (latest == nil || candidate.AttemptNumber > latest.AttemptNumber) {
						latest = candidate
					}
				}
				if err == nil && (latest == nil || latest.ID != strings.TrimSpace(r.FormValue("attempt_id"))) {
					err = errors.New("latest failed build attempt not found")
				}
				if err == nil {
					var request *distributions.BuildRetryRequest
					request, err = distributions.NewBuildRetryRequest(inspection, latest, r.FormValue("reason"), currentUsername(auth, r), time.Now())
					if err == nil {
						err = retryStore.Create(r.Context(), request)
					}
				}
				message = "Manual retry authorized as immutable evidence. The worker will create a new attempt."
			} else if action == "preflight" {
				var execution *distributions.BuildExecution
				execution, err = executionStore.Get(r.Context(), strings.TrimSpace(r.FormValue("execution_id")))
				if err == nil {
					var receipt *distributions.BuildPreflight
					receipt, err = distributions.NewBuildPreflight(execution, currentUsername(auth, r), time.Now())
					if err == nil {
						err = preflightStore.Create(r.Context(), receipt)
					}
				}
				message = "Immutable build preflight recorded. No build or external write was performed."
			} else if action == "queue" {
				var plan *distributions.BuildPlan
				plan, err = planStore.Get(r.Context(), strings.TrimSpace(r.FormValue("plan_id")))
				if err == nil {
					var approvals []*distributions.BuildApproval
					approvals, err = approvalStore.List(r.Context(), 1000)
					if err == nil {
						var approval *distributions.BuildApproval
						for _, candidate := range approvals {
							if candidate.BuildPlanID == plan.ID {
								approval = candidate
								break
							}
						}
						var execution *distributions.BuildExecution
						execution, err = distributions.NewBuildExecution(plan, approval, currentUsername(auth, r), time.Now())
						if err == nil {
							err = executionStore.Create(r.Context(), execution)
						}
					}
				}
				message = "Approved build plan queued as immutable evidence. The executor remains disabled."
			} else if action == "review" {
				var plan *distributions.BuildPlan
				plan, err = planStore.Get(r.Context(), strings.TrimSpace(r.FormValue("plan_id")))
				if err == nil {
					var approval *distributions.BuildApproval
					approval, err = distributions.NewBuildApproval(plan, r.FormValue("decision"), r.FormValue("reason"), currentUsername(auth, r), time.Now())
					if err == nil {
						err = approvalStore.Create(r.Context(), approval)
					}
				}
				message = "Final build-plan review recorded. Execution remains disabled."
			} else {
				var distribution *distributions.Distribution
				distribution, err = distributionStore.Get(r.Context(), strings.TrimSpace(r.FormValue("distribution_id")))
				if err == nil {
					var plan *distributions.BuildPlan
					plan, err = distributions.NewBuildPlan(distribution, r.Form["outputs"], r.FormValue("destination_ref"), r.FormValue("credential_ref"), currentUsername(auth, r), time.Now())
					if err == nil {
						err = planStore.Create(r.Context(), plan)
					}
				}
			}
			if err != nil {
				renderDistributionBuildPlans(w, r, distributionStore, planStore, approvalStore, executionStore, preflightStore, resultStore, sourceStore, acquisitionStore, inspectionStore, artifactStore, attemptStore, retryStore, auth, "", err.Error())
				return
			}
			http.Redirect(w, r, "/collector-distributions/build-plans?message="+url.QueryEscape(message), http.StatusSeeOther)
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		renderDistributionBuildPlans(w, r, distributionStore, planStore, approvalStore, executionStore, preflightStore, resultStore, sourceStore, acquisitionStore, inspectionStore, artifactStore, attemptStore, retryStore, auth, r.URL.Query().Get("message"), "")
	})
}
func renderDistributionBuildPlans(w http.ResponseWriter, r *http.Request, distributionStore storage.DistributionStore, planStore storage.DistributionBuildPlanStore, approvalStore storage.DistributionBuildApprovalStore, executionStore storage.DistributionBuildExecutionStore, preflightStore storage.DistributionBuildPreflightStore, resultStore storage.DistributionBuildWorkerResultStore, sourceStore storage.DistributionBuildSourceStore, acquisitionStore storage.DistributionBuildSourceAcquisitionStore, inspectionStore storage.DistributionBuildSourceInspectionStore, artifactStore storage.DistributionBuildArtifactStore, attemptStore storage.DistributionBuildAttemptStore, retryStore storage.DistributionBuildRetryRequestStore, auth *authManager, message, pageError string) {
	items, err := distributionStore.List(r.Context())
	if err != nil {
		internalServerError(w, err)
		return
	}
	plans, err := planStore.List(r.Context(), 100)
	if err != nil {
		internalServerError(w, err)
		return
	}
	approvals, err := approvalStore.List(r.Context(), 100)
	if err != nil {
		internalServerError(w, err)
		return
	}
	byPlan := map[string]*distributions.BuildApproval{}
	for _, approval := range approvals {
		byPlan[approval.BuildPlanID] = approval
	}
	executions, err := executionStore.List(r.Context(), 100)
	if err != nil {
		internalServerError(w, err)
		return
	}
	executionByPlan := map[string]*distributions.BuildExecution{}
	for _, execution := range executions {
		executionByPlan[execution.BuildPlanID] = execution
	}
	preflights, err := preflightStore.List(r.Context(), 100)
	if err != nil {
		internalServerError(w, err)
		return
	}
	preflightByExecution := map[string]*distributions.BuildPreflight{}
	for _, receipt := range preflights {
		preflightByExecution[receipt.ExecutionID] = receipt
	}
	results, err := resultStore.List(r.Context(), 100)
	if err != nil {
		internalServerError(w, err)
		return
	}
	resultByExecution := map[string]*distributions.BuildWorkerResult{}
	for _, result := range results {
		resultByExecution[result.ExecutionID] = result
	}
	sources, err := sourceStore.List(r.Context(), 100)
	if err != nil {
		internalServerError(w, err)
		return
	}
	acquisitions, err := acquisitionStore.List(r.Context(), 100)
	if err != nil {
		internalServerError(w, err)
		return
	}
	inspections, err := inspectionStore.List(r.Context(), 100)
	if err != nil {
		internalServerError(w, err)
		return
	}
	artifacts, err := artifactStore.List(r.Context(), 100)
	if err != nil {
		internalServerError(w, err)
		return
	}
	attempts, err := attemptStore.List(r.Context(), 1000)
	if err != nil {
		internalServerError(w, err)
		return
	}
	retryRequests, err := retryStore.List(r.Context(), 1000)
	if err != nil {
		internalServerError(w, err)
		return
	}
	latest := map[string]*distributions.BuildAttempt{}
	for _, attempt := range attempts {
		if current := latest[attempt.InspectionID]; current == nil || attempt.AttemptNumber > current.AttemptNumber {
			latest[attempt.InspectionID] = attempt
		}
	}
	pending := map[string]bool{}
	consumed := map[string]bool{}
	for _, attempt := range attempts {
		if attempt.RetryRequestID != "" {
			consumed[attempt.RetryRequestID] = true
		}
	}
	for _, request := range retryRequests {
		if !consumed[request.ID] {
			pending[request.PreviousAttemptID] = true
		}
	}
	canRetry := map[string]bool{}
	for _, attempt := range latest {
		if attempt.Status == distributions.BuildAttemptFailed && !pending[attempt.ID] {
			canRetry[attempt.ID] = true
		}
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := distributionBuildPlanPage.Execute(w, distributionBuildPlanView{Page: "collector-distributions", Distributions: items, Plans: plans, ApprovalByPlan: byPlan, ExecutionByPlan: executionByPlan, PreflightByExecution: preflightByExecution, ResultByExecution: resultByExecution, Sources: sources, Acquisitions: acquisitions, Inspections: inspections, Artifacts: artifacts, Attempts: attempts, RetryRequests: retryRequests, CanRetryAttempt: canRetry, RetryPendingAttempt: pending, Username: currentUsername(auth, r), Message: message, Error: pageError}); err != nil {
		internalServerError(w, err)
	}
}
