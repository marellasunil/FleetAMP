package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	sqlitestore "github.com/marellasunil/FleetAMP/internal/storage/sqlite"
)

func newUserTestManager(t *testing.T) (*authManager, *sqlitestore.Database) {
	t.Helper()
	db, err := sqlitestore.Open(context.Background(), filepath.Join(t.TempDir(), "fleetamp.db"))
	if err != nil {
		t.Fatal(err)
	}
	manager := testAuthManager(db.Authentication(), strings.Repeat("p", 32), "bootstrap")
	if err := manager.createAdministrator(context.Background(), "admin",
		"a-strong-admin-password", "bootstrap"); err != nil {
		db.Close()
		t.Fatal(err)
	}
	return manager, db
}

func TestCreateUserAuthenticationAndDisable(t *testing.T) {
	manager, db := newUserTestManager(t)
	defer db.Close()
	if err := manager.createUser(context.Background(), "operator-one",
		"a-strong-operator-password", "operator"); err != nil {
		t.Fatal(err)
	}
	principalRole, ok := manager.authenticateRole(context.Background(),
		"operator-one", "a-strong-operator-password")
	if !ok || principalRole != roleOperator {
		t.Fatalf("operator authentication role=%q ok=%t", principalRole, ok)
	}
	if err := db.Authentication().SetEnabled(context.Background(), "operator-one", false); err != nil {
		t.Fatal(err)
	}
	if manager.authenticate(context.Background(), "operator-one", "a-strong-operator-password") {
		t.Fatal("disabled user authenticated")
	}
}

func TestRoleAndPasswordChangesRevokeSessions(t *testing.T) {
	manager, db := newUserTestManager(t)
	defer db.Close()
	if err := manager.createUser(context.Background(), "viewer-one",
		"a-strong-viewer-password", "viewer"); err != nil {
		t.Fatal(err)
	}
	token, err := manager.createSessionForRole("viewer-one", roleViewer)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/agents", nil)
	request.AddCookie(&http.Cookie{Name: manager.cookieName(), Value: token})
	if !manager.validSession(request) {
		t.Fatal("viewer session was not created")
	}
	if err := db.Authentication().UpdateRole(context.Background(), "viewer-one", "operator"); err != nil {
		t.Fatal(err)
	}
	manager.revokeUserSessions("viewer-one")
	if manager.validSession(request) {
		t.Fatal("role change did not revoke existing session")
	}

	token, err = manager.createSessionForRole("viewer-one", roleOperator)
	if err != nil {
		t.Fatal(err)
	}
	request = httptest.NewRequest(http.MethodGet, "/agents", nil)
	request.AddCookie(&http.Cookie{Name: manager.cookieName(), Value: token})
	if err := manager.resetUserPassword(context.Background(), "viewer-one",
		"a-new-strong-viewer-password"); err != nil {
		t.Fatal(err)
	}
	if manager.validSession(request) {
		t.Fatal("password reset did not revoke existing session")
	}
	if !manager.authenticate(context.Background(), "viewer-one", "a-new-strong-viewer-password") {
		t.Fatal("new password was rejected")
	}
}

func TestSavingUnchangedRoleKeepsSession(t *testing.T) {
	manager, db := newUserTestManager(t)
	defer db.Close()
	token, err := manager.createSessionForRole("admin", roleAdmin)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/settings/users",
		strings.NewReader("action=role&username=admin&role=admin"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.AddCookie(&http.Cookie{Name: manager.cookieName(), Value: token})
	response := httptest.NewRecorder()
	manager.handleUsers(response, request)
	if response.Code != http.StatusSeeOther {
		t.Fatalf("save unchanged role status=%d body=%s", response.Code, response.Body.String())
	}
	check := httptest.NewRequest(http.MethodGet, "/agents", nil)
	check.AddCookie(&http.Cookie{Name: manager.cookieName(), Value: token})
	if !manager.validSession(check) {
		t.Fatal("saving an unchanged role revoked the current session")
	}
}

func TestUsersPageDoesNotRenderPasswordMaterial(t *testing.T) {
	manager, db := newUserTestManager(t)
	defer db.Close()
	token, err := manager.createSessionForRole("admin", roleAdmin)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	manager.registerRoutes(mux)
	handler := securityMiddleware(securityConfig{MaxBodyBytes: defaultMaxRequestBodyBytes}, manager, mux)
	request := httptest.NewRequest(http.MethodGet, "/settings/users", nil)
	request.AddCookie(&http.Cookie{Name: manager.cookieName(), Value: token})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("users page status=%d body=%s", response.Code, response.Body.String())
	}
	body := response.Body.String()
	if !strings.Contains(body, "Users and roles") || !strings.Contains(body, "admin") {
		t.Fatal("users page is missing expected identity information")
	}
	user, err := db.Authentication().Get(context.Background(), "admin")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(body, string(user.PasswordHash)) ||
		strings.Contains(body, string(user.PasswordSalt)) {
		t.Fatal("users page exposed password material")
	}
}
