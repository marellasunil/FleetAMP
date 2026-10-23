package main

import (
	"net/http"

	"github.com/marellasunil/FleetAMP/internal/distributions"
)

func loadDistributionLifecycleStats(r *http.Request, specifications, releases int, stores distributionLifecycleStores) (distributionLifecycleStats, error) {
	stats := distributionLifecycleStats{Specifications: specifications, Releases: releases}
	if stores.BuildPlans != nil {
		items, err := stores.BuildPlans.List(r.Context(), 10000)
		if err != nil {
			return stats, err
		}
		stats.BuildPlans = len(items)
	}
	if stores.BuildAttempts != nil {
		items, err := stores.BuildAttempts.List(r.Context(), 10000)
		if err != nil {
			return stats, err
		}
		for _, item := range items {
			if item.Status == distributions.BuildAttemptSucceeded {
				stats.BuildsSucceeded++
			} else if item.Status == distributions.BuildAttemptFailed {
				stats.BuildsFailed++
			}
		}
	}
	if stores.Promotions != nil {
		items, err := stores.Promotions.List(r.Context(), 10000)
		if err != nil {
			return stats, err
		}
		reviewed := map[string]bool{}
		if stores.PromotionReviews != nil {
			reviews, listErr := stores.PromotionReviews.List(r.Context(), 10000)
			if listErr != nil {
				return stats, listErr
			}
			for _, review := range reviews {
				reviewed[review.PromotionID] = true
			}
		}
		for _, item := range items {
			if item.Status == distributions.ArtifactPromotionPending && !reviewed[item.ID] {
				stats.PromotionsPending++
			}
		}
	}
	if stores.Publications != nil {
		items, err := stores.Publications.List(r.Context(), 10000)
		if err != nil {
			return stats, err
		}
		completed := map[string]bool{}
		if stores.PublicationExecutions != nil {
			executions, listErr := stores.PublicationExecutions.List(r.Context(), 10000)
			if listErr != nil {
				return stats, listErr
			}
			for _, execution := range executions {
				if execution.Status == distributions.ArtifactPublicationSucceeded {
					completed[execution.PublicationID] = true
				}
				if execution.Status == distributions.ArtifactPublicationFailed {
					stats.PublicationsFailed++
				}
			}
		}
		for _, item := range items {
			if item.Status == distributions.ArtifactPublicationQueued && !completed[item.ID] {
				stats.PublicationsQueued++
			}
		}
	}
	if stores.Previews != nil {
		items, err := stores.Previews.List(r.Context(), 10000)
		if err != nil {
			return stats, err
		}
		stats.Previews = len(items)
	}
	if stores.Handoffs != nil {
		items, err := stores.Handoffs.List(r.Context(), 10000)
		if err != nil {
			return stats, err
		}
		stats.Handoffs = len(items)
	}
	return stats, nil
}
