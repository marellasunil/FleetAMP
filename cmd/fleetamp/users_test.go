package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/marellasunil/FleetAMP/internal/groups"
	sqlitestore "github.com/marellasunil/FleetAMP/internal/storage/sqlite"
)

func newUserTestManager(t *testing.T) (*authManager, *sqlitestore.Database) {
	t.Helper()
	db, err := sqlitestore.Open(context.Background(), filepath.Join(t.TempDir(), "fleetamp.db"))
	if err != nil {
		t.Fatal(err)
	}
	manager := testAuthManager(db.Authentication(), strings.Repeat("p", 32), "bootstrap")
	manager.groupStore = db.Groups()
	group, err := groups.New("Test group", "", map[string]string{"team": "test"})
	if err != nil {
		db.Close()
		t.Fatal(err)
	}
	group.ID = "test-group"
	if err := db.Groups().Create(context.Background(), group); err != nil {
		db.Close()
		t.Fatal(err)
	}
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
	if err := manager.createUser(context.Background(), "operator-one", "operator@example.com",
		"a-strong-operator-password", "member", []string{"test-group"}, []string{"configuration_editor", "deployment_operator"}); err != nil {
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
	if err := manager.createUser(context.Background(), "viewer-one", "",
		"a-strong-viewer-password", "member", []string{"test-group"}, []string{"viewer"}); err != nil {
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
		"a-new-strong-viewer-password", false); err != nil {
		t.Fatal(err)
	}
	if manager.validSession(request) {
		t.Fatal("password reset did not revoke existing session")
	}
	if !manager.authenticate(context.Background(), "viewer-one", "a-new-strong-viewer-password") {
		t.Fatal("new password was rejected")
	}
}

func TestAccountPasswordChangeRequiresCurrentPassword(t *testing.T) {
	manager, db := newUserTestManager(t)
	defer db.Close()
	token, err := manager.createSessionForRole("admin", roleAdmin)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/account", strings.NewReader(
		"action=password&current_password=wrong-password&password=a-new-strong-admin-password&confirm_password=a-new-strong-admin-password"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.AddCookie(&http.Cookie{Name: manager.cookieName(), Value: token})
	response := httptest.NewRecorder()
	manager.handleAccount(response, request)
	if !strings.Contains(response.Header().Get("Location"), "current+password+is+incorrect") {
		t.Fatalf("unexpected redirect: %s", response.Header().Get("Location"))
	}
	if !manager.authenticate(context.Background(), "admin", "a-strong-admin-password") {
		t.Fatal("wrong current password changed the stored verifier")
	}
}

func TestAdminResetRequiresReauthenticationAndForcesChange(t *testing.T) {
	manager, db := newUserTestManager(t)
	defer db.Close()
	if err := manager.createUser(context.Background(), "viewer-one", "", "a-strong-viewer-password", "member", []string{"test-group"}, []string{"viewer"}); err != nil {
		t.Fatal(err)
	}
	token, err := manager.createSessionForRole("admin", roleAdmin)
	if err != nil {
		t.Fatal(err)
	}
	post := func(adminPassword string) *httptest.ResponseRecorder {
		form := "action=reset_password&username=viewer-one&admin_password=" + adminPassword + "&password=a-temporary-viewer-password&confirm_password=a-temporary-viewer-password"
		request := httptest.NewRequest(http.MethodPost, "/settings/users", strings.NewReader(form))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		request.AddCookie(&http.Cookie{Name: manager.cookieName(), Value: token})
		response := httptest.NewRecorder()
		manager.handleUsers(response, request)
		return response
	}
	if location := post("wrong-password").Header().Get("Location"); !strings.Contains(location, "Admin+password+is+incorrect") {
		t.Fatalf("missing reauthentication error: %s", location)
	}
	post("a-strong-admin-password")
	user, err := db.Authentication().Get(context.Background(), "viewer-one")
	if err != nil {
		t.Fatal(err)
	}
	if !user.MustChangePassword {
		t.Fatal("admin reset did not require a password change")
	}
	if !manager.authenticate(context.Background(), "viewer-one", "a-temporary-viewer-password") {
		t.Fatal("temporary password rejected")
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

func TestSavingCurrentUserGroupsKeepsSession(t *testing.T) {
	manager, db := newUserTestManager(t)
	defer db.Close()
	token, err := manager.createSessionForRole("admin", roleAdmin)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/settings/users",
		strings.NewReader("action=groups&username=admin&group_ids=test-group"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.AddCookie(&http.Cookie{Name: manager.cookieName(), Value: token})
	response := httptest.NewRecorder()
	manager.handleUsers(response, request)
	if response.Code != http.StatusSeeOther {
		t.Fatalf("save groups status=%d body=%s", response.Code, response.Body.String())
	}
	check := httptest.NewRequest(http.MethodGet, "/agents", nil)
	check.AddCookie(&http.Cookie{Name: manager.cookieName(), Value: token})
	if !manager.validSession(check) {
		t.Fatal("saving group membership revoked the current session")
	}
	user, err := db.Authentication().Get(context.Background(), "admin")
	if err != nil {
		t.Fatal(err)
	}
	if len(user.GroupIDs) != 1 || user.GroupIDs[0] != "test-group" {
		t.Fatalf("saved group IDs = %v", user.GroupIDs)
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
	if !strings.Contains(body, "Users, groups and roles") || !strings.Contains(body, "admin") {
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

func TestGroupOwnershipFollowsPerGroupRole(t *testing.T) {
	manager, db := newUserTestManager(t)
	defer db.Close()
	ctx := context.Background()
	second, err := groups.New("Read only group", "", map[string]string{"team": "readonly"})
	if err != nil {
		t.Fatal(err)
	}
	second.ID = "read-only-group"
	if err := db.Groups().Create(ctx, second); err != nil {
		t.Fatal(err)
	}
	if err := manager.createUser(ctx, "alice", "alice@example.com", "a-strong-alice-password",
		"member", []string{"test-group"}, []string{"group_owner"}); err != nil {
		t.Fatal(err)
	}
	if err := manager.updateGroupMembership(ctx, "alice", second.ID, []string{"viewer"}, false); err != nil {
		t.Fatal(err)
	}
	owned, err := db.Groups().Get(ctx, "test-group")
	if err != nil || !isGroupOwner(owned, "alice") {
		t.Fatalf("alice should own test-group: group=%+v err=%v", owned, err)
	}
	readOnly, err := db.Groups().Get(ctx, second.ID)
	if err != nil {
		t.Fatal(err)
	}
	if isGroupOwner(readOnly, "alice") {
		t.Fatal("viewer membership incorrectly granted group ownership")
	}
}
