package main

import (
	"errors"
	"html/template"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/marellasunil/FleetAMP/internal/distributions"
	"github.com/marellasunil/FleetAMP/internal/storage"
)

type artifactPromotionView struct {
	Page                     string
	Artifacts                []*distributions.BuildArtifact
	PromotionByArtifact      map[string]*distributions.ArtifactPromotion
	ReviewByPromotion        map[string]*distributions.ArtifactPromotionReview
	PublicationByPromo       map[string]*distributions.ArtifactPublication
	PreflightByPublication   map[string]*distributions.ArtifactPublicationPreflight
	ExecutionByPublication   map[string]*distributions.ArtifactPublicationExecution
	PublicationWorkerEnabled bool
	Username                 string
	Message, Error           string
}

const artifactPromotionHTML = `<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Artifact Promotions · FleetAMP</title><style>` + controlPlaneCSS + detailCSS + `.promotion-stack{display:grid;gap:16px}.promotion-card{padding:16px;border:1px solid var(--line);border-radius:10px;background:#0a1626}.promotion-grid{display:grid;grid-template-columns:minmax(0,1.2fr) minmax(300px,.8fr);gap:16px}@media(max-width:1000px){.promotion-grid{grid-template-columns:1fr}}</style></head><body><div class="shell">` + sideNav + `<main class="main"><header class="top"><div><div class="crumb">FleetAMP / Lifecycle / Distributions / Promotions</div><div class="pagetitle">Artifact Promotions</div><div class="subtitle">Approve, preflight and publish the exact governed artifact.</div></div><div class="topactions"><a class="btn" href="/collector-distributions/build-plans">Build plans</a><span class="badge {{if .PublicationWorkerEnabled}}ok{{else}}off{{end}}">Registry worker {{if .PublicationWorkerEnabled}}enabled{{else}}disabled{{end}}</span></div></header><div class="content">{{if .Message}}<div class="notice">{{.Message}}</div>{{end}}{{if .Error}}<div class="configerror" role="alert">{{.Error}}</div>{{end}}<div class="promotion-grid"><section class="card"><div class="cardhead"><div><div class="cardtitle">Generated artifacts</div><div class="cardsub">Destination and credential references come from the immutable build plan; users cannot replace them here.</div></div></div><div class="cardbody promotion-stack">{{range .Artifacts}}{{$artifact := .}}{{$promotion := index $.PromotionByArtifact .ID}}<article class="promotion-card"><div><strong>Artifact</strong><div class="tiny code">{{.ID}}</div><div class="tiny code">SHA-256: {{.ArtifactSHA256}}</div></div>{{if $promotion}}{{$review := index $.ReviewByPromotion $promotion.ID}}{{$publication := index $.PublicationByPromo $promotion.ID}}<div class="kv" style="margin-top:12px"><span>Destination</span><span class="code">{{$promotion.DestinationRef}}</span><span>Requested by</span><span>{{$promotion.RequestedBy}}</span><span>Reason</span><span>{{$promotion.Reason}}</span><span>Proposal evidence</span><span class="code">{{$promotion.EvidenceHash}}</span></div>{{if $publication}}{{$preflight := index $.PreflightByPublication $publication.ID}}{{$execution := index $.ExecutionByPublication $publication.ID}}<div class="notice" style="margin-top:12px"><strong>Publication queued</strong><br><span class="code">{{$publication.ID}}</span><br>Exact digest and destination are frozen.</div>{{if $preflight}}<div class="notice" style="margin-top:12px"><strong>Safe preflight passed</strong><br>Registry host: <span class="code">{{$preflight.DestinationHost}}</span><br>Artifact size: {{$preflight.ArtifactSize}} bytes<br>Credential reference available; secret was not stored.<br><span class="tiny code">Evidence: {{$preflight.EvidenceHash}}</span></div>{{if $execution}}<div class="notice" style="margin-top:12px"><strong>Publication {{$execution.Status}}</strong><br>Worker: {{$execution.WorkerID}}<br>{{if $execution.FailureCode}}Failure: {{$execution.FailureCode}} · <span class="code">{{$execution.FailureDigest}}</span><br>{{end}}<span class="tiny code">Evidence: {{$execution.EvidenceHash}}</span></div>{{else if not $.PublicationWorkerEnabled}}<div class="tiny" style="margin-top:12px">Preflight is ready. Enable the registry worker explicitly to publish.</div>{{end}}{{else}}<form method="post" style="margin-top:12px"><input type="hidden" name="action" value="preflight"><input type="hidden" name="publication_id" value="{{$publication.ID}}"><input type="hidden" name="artifact_id" value="{{$artifact.ID}}"><button class="btn primary" type="submit">Run safe publication preflight</button></form>{{end}}{{else if $review}}<div class="notice" style="margin-top:12px"><strong>Review: {{$review.Decision}}</strong> by {{$review.ReviewedBy}}<br>{{$review.Reason}}</div>{{if eq $review.Decision "approved"}}<form method="post" style="margin-top:12px"><input type="hidden" name="action" value="queue"><input type="hidden" name="promotion_id" value="{{$promotion.ID}}"><button class="btn primary" type="submit">Queue exact publication</button></form>{{end}}{{else if eq $.Username $promotion.RequestedBy}}<div class="tiny" style="margin-top:12px">A different administrator must review this proposal.</div>{{else}}<form method="post" style="margin-top:12px"><input type="hidden" name="action" value="review"><input type="hidden" name="promotion_id" value="{{$promotion.ID}}"><textarea class="input" name="reason" rows="2" maxlength="500" required placeholder="Independent review reason"></textarea><div class="detailactions"><button class="btn primary" name="decision" value="approved">Approve exact evidence</button><button class="btn" name="decision" value="rejected">Reject</button></div></form>{{end}}{{else}}<form method="post" style="margin-top:12px"><input type="hidden" name="action" value="propose"><input type="hidden" name="artifact_id" value="{{.ID}}"><textarea class="input" name="reason" rows="2" maxlength="500" required placeholder="Why should this artifact be promoted?"></textarea><button class="btn primary" type="submit">Create immutable promotion proposal</button></form>{{end}}</article>{{else}}<div class="empty">No successful sandboxed artifacts are available.</div>{{end}}</div></section><aside class="card"><div class="cardhead"><div><div class="cardtitle">Publication boundary</div><div class="cardsub">Controls completed by this stage</div></div></div><div class="cardbody"><ul><li>Exact artifact SHA-256 frozen and rechecked</li><li>Destination inherited from approved build plan</li><li>Registry host explicitly allowlisted by preflight</li><li>Credential passed to ORAS on stdin and never persisted</li><li>Atomic worker lease prevents concurrent publishers</li><li>Successful and failed outcomes are immutable receipts</li><li>Worker remains disabled unless explicitly enabled</li></ul></div></aside></div></div></main></div></body></html>`

var artifactPromotionPage = template.Must(template.New("artifact-promotions").Parse(artifactPromotionHTML))

func registerDistributionArtifactPromotionRoutes(mux *http.ServeMux, artifactStore storage.DistributionBuildArtifactStore, planStore storage.DistributionBuildPlanStore, promotionStore storage.DistributionArtifactPromotionStore, reviewStore storage.DistributionArtifactPromotionReviewStore, publicationStore storage.DistributionArtifactPublicationStore, preflightStore storage.DistributionArtifactPublicationPreflightStore, executionStore storage.DistributionArtifactPublicationExecutionStore, verifier *distributions.ArtifactPublicationPreflightVerifier, publicationWorkerEnabled bool, auth *authManager) {
	mux.HandleFunc("/collector-distributions/promotions", func(w http.ResponseWriter, r *http.Request) {
		if auth != nil && currentRole(auth, r) != roleAdmin {
			http.Error(w, "administrator access required", http.StatusForbidden)
			return
		}
		if r.Method == http.MethodPost {
			if err := r.ParseForm(); err != nil {
				http.Error(w, "invalid form", http.StatusBadRequest)
				return
			}
			var err error
			message := ""
			switch strings.TrimSpace(r.FormValue("action")) {
			case "propose":
				var artifact *distributions.BuildArtifact
				artifact, err = findBuildArtifact(r, artifactStore, r.FormValue("artifact_id"))
				var plan *distributions.BuildPlan
				if err == nil {
					plan, err = findBuildPlanByHash(r, planStore, artifact.BuildPlanHash)
				}
				if err == nil {
					var promotion *distributions.ArtifactPromotion
					promotion, err = distributions.NewArtifactPromotion(artifact, plan, r.FormValue("reason"), currentUsername(auth, r), time.Now())
					if err == nil {
						err = promotionStore.Create(r.Context(), promotion)
					}
				}
				message = "Immutable artifact promotion proposed. A different administrator must review it."
			case "review":
				var promotion *distributions.ArtifactPromotion
				promotion, err = findArtifactPromotion(r, promotionStore, r.FormValue("promotion_id"))
				if err == nil {
					var review *distributions.ArtifactPromotionReview
					review, err = distributions.NewArtifactPromotionReview(promotion, r.FormValue("decision"), r.FormValue("reason"), currentUsername(auth, r), time.Now())
					if err == nil {
						err = reviewStore.Create(r.Context(), review)
					}
				}
				message = "Immutable promotion review recorded."
			case "queue":
				var promotion *distributions.ArtifactPromotion
				promotion, err = findArtifactPromotion(r, promotionStore, r.FormValue("promotion_id"))
				var review *distributions.ArtifactPromotionReview
				if err == nil {
					review, err = findArtifactPromotionReview(r, reviewStore, promotion.ID)
				}
				if err == nil {
					var publication *distributions.ArtifactPublication
					publication, err = distributions.NewArtifactPublication(promotion, review, currentUsername(auth, r), time.Now())
					if err == nil {
						err = publicationStore.Create(r.Context(), publication)
					}
				}
				message = "Exact artifact publication queued as immutable evidence. No registry write was performed."
			case "preflight":
				var publication *distributions.ArtifactPublication
				publication, err = findArtifactPublication(r, publicationStore, r.FormValue("publication_id"))
				var artifact *distributions.BuildArtifact
				if err == nil {
					artifact, err = findBuildArtifact(r, artifactStore, r.FormValue("artifact_id"))
				}
				if err == nil {
					var preflight *distributions.ArtifactPublicationPreflight
					preflight, err = verifier.Verify(r.Context(), publication, artifact, currentUsername(auth, r), time.Now())
					if err == nil {
						err = preflightStore.Create(r.Context(), preflight)
					}
				}
				message = "Safe publication preflight passed and immutable evidence was recorded. No registry write was performed."
			default:
				err = errors.New("unsupported promotion action")
			}
			if err != nil {
				renderArtifactPromotions(w, r, artifactStore, promotionStore, reviewStore, publicationStore, preflightStore, executionStore, publicationWorkerEnabled, auth, "", err.Error())
				return
			}
			http.Redirect(w, r, "/collector-distributions/promotions?message="+url.QueryEscape(message), http.StatusSeeOther)
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		renderArtifactPromotions(w, r, artifactStore, promotionStore, reviewStore, publicationStore, preflightStore, executionStore, publicationWorkerEnabled, auth, r.URL.Query().Get("message"), "")
	})
}

func renderArtifactPromotions(w http.ResponseWriter, r *http.Request, artifactStore storage.DistributionBuildArtifactStore, promotionStore storage.DistributionArtifactPromotionStore, reviewStore storage.DistributionArtifactPromotionReviewStore, publicationStore storage.DistributionArtifactPublicationStore, preflightStore storage.DistributionArtifactPublicationPreflightStore, executionStore storage.DistributionArtifactPublicationExecutionStore, publicationWorkerEnabled bool, auth *authManager, message, pageError string) {
	artifacts, err := artifactStore.List(r.Context(), 100)
	if err != nil {
		internalServerError(w, err)
		return
	}
	promotions, err := promotionStore.List(r.Context(), 100)
	if err != nil {
		internalServerError(w, err)
		return
	}
	reviews, err := reviewStore.List(r.Context(), 100)
	if err != nil {
		internalServerError(w, err)
		return
	}
	publications, err := publicationStore.List(r.Context(), 100)
	if err != nil {
		internalServerError(w, err)
		return
	}
	preflights, err := preflightStore.List(r.Context(), 100)
	if err != nil {
		internalServerError(w, err)
		return
	}
	executions, err := executionStore.List(r.Context(), 100)
	if err != nil {
		internalServerError(w, err)
		return
	}
	promotionByArtifact := map[string]*distributions.ArtifactPromotion{}
	for _, item := range promotions {
		promotionByArtifact[item.ArtifactID] = item
	}
	reviewByPromotion := map[string]*distributions.ArtifactPromotionReview{}
	for _, item := range reviews {
		reviewByPromotion[item.PromotionID] = item
	}
	publicationByPromotion := map[string]*distributions.ArtifactPublication{}
	for _, item := range publications {
		publicationByPromotion[item.PromotionID] = item
	}
	preflightByPublication := map[string]*distributions.ArtifactPublicationPreflight{}
	for _, item := range preflights {
		preflightByPublication[item.PublicationID] = item
	}
	executionByPublication := map[string]*distributions.ArtifactPublicationExecution{}
	for _, item := range executions {
		executionByPublication[item.PublicationID] = item
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := artifactPromotionPage.Execute(w, artifactPromotionView{Page: "collector-distributions", Artifacts: artifacts, PromotionByArtifact: promotionByArtifact, ReviewByPromotion: reviewByPromotion, PublicationByPromo: publicationByPromotion, PreflightByPublication: preflightByPublication, ExecutionByPublication: executionByPublication, PublicationWorkerEnabled: publicationWorkerEnabled, Username: currentUsername(auth, r), Message: message, Error: pageError}); err != nil {
		internalServerError(w, err)
	}
}

func findArtifactPublication(r *http.Request, store storage.DistributionArtifactPublicationStore, id string) (*distributions.ArtifactPublication, error) {
	items, err := store.List(r.Context(), 1000)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if item.ID == strings.TrimSpace(id) {
			return item, nil
		}
	}
	return nil, errors.New("artifact publication not found")
}

func findBuildArtifact(r *http.Request, store storage.DistributionBuildArtifactStore, id string) (*distributions.BuildArtifact, error) {
	items, err := store.List(r.Context(), 1000)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if item.ID == strings.TrimSpace(id) {
			return item, nil
		}
	}
	return nil, errors.New("build artifact not found")
}

func findBuildPlanByHash(r *http.Request, store storage.DistributionBuildPlanStore, hash string) (*distributions.BuildPlan, error) {
	items, err := store.List(r.Context(), 1000)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if item.PlanHash == hash {
			return item, nil
		}
	}
	return nil, errors.New("immutable build plan not found")
}

func findArtifactPromotion(r *http.Request, store storage.DistributionArtifactPromotionStore, id string) (*distributions.ArtifactPromotion, error) {
	items, err := store.List(r.Context(), 1000)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if item.ID == strings.TrimSpace(id) {
			return item, nil
		}
	}
	return nil, errors.New("artifact promotion not found")
}

func findArtifactPromotionReview(r *http.Request, store storage.DistributionArtifactPromotionReviewStore, promotionID string) (*distributions.ArtifactPromotionReview, error) {
	items, err := store.List(r.Context(), 1000)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if item.PromotionID == promotionID {
			return item, nil
		}
	}
	return nil, errors.New("artifact promotion review not found")
}
