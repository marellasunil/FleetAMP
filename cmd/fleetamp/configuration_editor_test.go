package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/marellasunil/FleetAMP/internal/agents"
	"github.com/marellasunil/FleetAMP/internal/configs"
	"github.com/marellasunil/FleetAMP/internal/storage/memory"
	sqlitestore "github.com/marellasunil/FleetAMP/internal/storage/sqlite"
)

func TestAgentConfigurationEditorSavesValidatedImmutableVersion(t *testing.T) {
	ctx := context.Background()
	db, err := sqlitestore.Open(ctx, filepath.Join(t.TempDir(), "fleetamp.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	agentStore := memory.NewAgentStore()
	if err := agentStore.Upsert(ctx, &agents.ManagedAgent{
		InstanceUID:  "agent-1",
		Name:         "collector-one",
		Capabilities: []string{"accepts_remote_config"},
	}); err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	registerConfigRoutes(
		mux,
		db.Configurations(),
		db.Assignments(),
		db.Deployments(),
		agentStore,
		configs.NewValidator(""),
		nil,
	)

	form := url.Values{
		"name":    {"collector.yaml"},
		"version": {"1.0.0"},
		"content": {"receivers: {}\nservice:\n  pipelines: {}\n"},
	}
	request := httptest.NewRequest(
		http.MethodPost,
		"/agents/agent-1/configurations",
		strings.NewReader(form.Encode()),
	)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	if response.Code != http.StatusSeeOther {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if location := response.Header().Get("Location"); !strings.HasPrefix(location, "/agents/agent-1?configuration_saved=") {
		t.Fatalf("location=%q", location)
	}
	items, err := db.Configurations().List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("configuration count=%d", len(items))
	}
	if items[0].Name != "collector.yaml" || items[0].Version != "1.0.0" {
		t.Fatalf("saved configuration=%#v", items[0])
	}
}

func TestAgentConfigurationEditorRejectsInvalidYAML(t *testing.T) {
	ctx := context.Background()
	db, err := sqlitestore.Open(ctx, filepath.Join(t.TempDir(), "fleetamp.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	agentStore := memory.NewAgentStore()
	if err := agentStore.Upsert(ctx, &agents.ManagedAgent{InstanceUID: "agent-1", Name: "collector-one"}); err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	registerConfigRoutes(
		mux,
		db.Configurations(),
		db.Assignments(),
		db.Deployments(),
		agentStore,
		configs.NewValidator(""),
		nil,
	)

	form := url.Values{
		"name":    {"collector.yaml"},
		"version": {"broken"},
		"content": {"service: ["},
	}
	request := httptest.NewRequest(
		http.MethodPost,
		"/agents/agent-1/configurations",
		strings.NewReader(form.Encode()),
	)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	items, err := db.Configurations().List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("invalid configuration was persisted: %#v", items)
	}
}
