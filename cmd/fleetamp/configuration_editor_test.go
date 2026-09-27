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

func TestAgentConfigurationAuthoringHasMovedToGroups(t *testing.T) {
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
		nil,
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

	if response.Code != http.StatusGone {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), "moved to Groups") {
		t.Fatalf("body=%q", response.Body.String())
	}
	items, err := db.Configurations().List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("configuration count=%d", len(items))
	}
}

func TestAgentConfigurationAuthoringReturnsNotFoundForUnknownCollector(t *testing.T) {
	ctx := context.Background()
	db, err := sqlitestore.Open(ctx, filepath.Join(t.TempDir(), "fleetamp.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	agentStore := memory.NewAgentStore()
	mux := http.NewServeMux()
	registerConfigRoutes(
		mux,
		db.Configurations(),
		db.Assignments(),
		db.Deployments(),
		agentStore,
		configs.NewValidator(""),
		nil,
		nil,
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

	if response.Code != http.StatusNotFound {
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

func TestConfigurationEditorJavaScriptIsServed(t *testing.T) {
	mux := http.NewServeMux()
	registerUIRoutes(mux)

	request := httptest.NewRequest(http.MethodGet, "/assets/configuration-editor.js", nil)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if contentType := response.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "text/javascript") {
		t.Fatalf("content-type=%q", contentType)
	}
	script := response.Body.String()
	for _, expected := range []string{
		"configuration-editor-form",
		"configuration-validation-error",
		"/api/v1/configurations/sections/compose",
		"form.requestSubmit",
	} {
		if !strings.Contains(script, expected) {
			t.Fatalf("script is missing %q", expected)
		}
	}
}

func TestConfigurationAuthoringIsOnlyShownOnGroupDetail(t *testing.T) {
	if strings.Contains(agentDetailHTML, "Create configuration version") {
		t.Fatal("agent detail must not offer configuration authoring")
	}
	if strings.Contains(agentDetailHTML, "Saved configuration versions") {
		t.Fatal("agent detail must not list group-scoped configuration versions")
	}
	if !strings.Contains(groupDetailHTML, "Create configuration version") {
		t.Fatal("group detail is missing configuration authoring")
	}
}
