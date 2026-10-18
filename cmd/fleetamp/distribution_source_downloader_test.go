package main

import (
	"context"
	"github.com/marellasunil/FleetAMP/internal/distributions"
	"github.com/marellasunil/FleetAMP/internal/storage/memory"
	"strings"
	"testing"
)

type fakeSourceFetcher struct{}

func (fakeSourceFetcher) Fetch(context.Context, *distributions.BuildSourceResolution) (string, int64, string, error) {
	sha := strings.Repeat("a", 64)
	return sha, 123, "source-cache://" + sha + ".tar.gz", nil
}
func TestDistributionSourceDownloaderCreatesOneAcquisition(t *testing.T) {
	sources := memory.NewDistributionBuildSourceStore()
	acquisitions := memory.NewDistributionBuildSourceAcquisitionStore()
	source := &distributions.BuildSourceResolution{ID: "source-1", ExecutionID: "execution-1", BuildPlanHash: "plan", Status: distributions.BuildSourceResolved}
	if err := sources.Create(t.Context(), source); err != nil {
		t.Fatal(err)
	}
	worker := &distributionSourceDownloader{ID: "downloader-1", Sources: sources, Acquisitions: acquisitions, Fetcher: fakeSourceFetcher{}}
	if err := worker.runOnce(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := worker.runOnce(t.Context()); err != nil {
		t.Fatal(err)
	}
	items, err := acquisitions.List(t.Context(), 10)
	if err != nil || len(items) != 1 {
		t.Fatalf("items=%d err=%v", len(items), err)
	}
	if items[0].Status != distributions.BuildSourceAcquired {
		t.Fatalf("unexpected acquisition: %#v", items[0])
	}
}
