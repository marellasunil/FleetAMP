package integrations

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type validationSecretResolver struct{ value string }

func (r validationSecretResolver) Resolve(context.Context, string) (string, error) {
	return r.value, nil
}

func TestGitHubConnectionValidationChecksRepositoryBranchAndWritePermission(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Header.Get("Authorization") != "Bearer token" {
			t.Errorf("missing bearer credential")
		}
		switch r.URL.Path {
		case "/repos/acme/telemetry":
			_, _ = w.Write([]byte(`{"full_name":"acme/telemetry","permissions":{"pull":true,"push":true,"admin":false}}`))
		case "/repos/acme/telemetry/branches/main":
			_, _ = w.Write([]byte(`{"name":"main"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	connection := &Connection{ID: "connection", Provider: GitHub, BaseURL: server.URL, Organization: "acme", Repository: "telemetry", Branch: "main", Mode: ModePullRequest, CredentialRef: "secret://github/prod", Enabled: true}
	validator := &ConnectionValidator{Secrets: validationSecretResolver{"token"}, Client: server.Client()}
	evidence, err := validator.Validate(t.Context(), connection)
	if err != nil {
		t.Fatal(err)
	}
	if requests != 2 || evidence["read"] != "true" || evidence["write"] != "true" {
		t.Fatalf("requests=%d evidence=%#v", requests, evidence)
	}
}

func TestConnectionValidationExpires(t *testing.T) {
	connection := &Connection{ID: "connection", Provider: GitHub}
	record, err := NewConnectionValidation(connection, "passed", "admin", "ok", nil, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if !record.Usable(record.CreatedAt.Add(30*time.Second)) || record.Usable(record.ExpiresAt) {
		t.Fatal("unexpected validation expiry behavior")
	}
}
