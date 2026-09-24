package main

import (
	"context"
	"crypto/sha256"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	sqlitestore "github.com/marellasunil/FleetAMP/internal/storage/sqlite"
)

type memoryAdministratorStore struct {
	mu   sync.Mutex
	user *sqlitestore.User
}

func (s *memoryAdministratorStore) Exists(context.Context) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.user != nil, nil
}

func (s *memoryAdministratorStore) Get(_ context.Context, username string) (*sqlitestore.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.user == nil || !strings.EqualFold(s.user.Username, username) {
		return nil, sqlitestore.ErrUserNotFound
	}
	copy := *s.user
	copy.PasswordSalt = append([]byte(nil), s.user.PasswordSalt...)
	copy.PasswordHash = append([]byte(nil), s.user.PasswordHash...)
	return &copy, nil
}

func (s *memoryAdministratorStore) List(context.Context) ([]*sqlitestore.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.user == nil {
		return nil, nil
	}
	copy := *s.user
	return []*sqlitestore.User{&copy}, nil
}

func (s *memoryAdministratorStore) Create(_ context.Context, user sqlitestore.User) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.user != nil {
		return errors.New("user exists")
	}
	copy := user
	s.user = &copy
	return nil
}

func (s *memoryAdministratorStore) ReplacePassword(_ context.Context, username string, salt, hash []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.user == nil || !strings.EqualFold(s.user.Username, username) {
		return sqlitestore.ErrUserNotFound
	}
	s.user.PasswordSalt = append([]byte(nil), salt...)
	s.user.PasswordHash = append([]byte(nil), hash...)
	return nil
}

func (s *memoryAdministratorStore) UpdateRole(_ context.Context, username, nextRole string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.user == nil || !strings.EqualFold(s.user.Username, username) {
		return sqlitestore.ErrUserNotFound
	}
	s.user.Role = nextRole
	return nil
}

func (s *memoryAdministratorStore) SetEnabled(_ context.Context, username string, enabled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.user == nil || !strings.EqualFold(s.user.Username, username) {
		return sqlitestore.ErrUserNotFound
	}
	s.user.Enabled = enabled
	return nil
}

func testAuthManager(store userStore, pepper, bootstrapToken string) *authManager {
	return &authManager{
		store: store, pepper: []byte(pepper), now: time.Now,
		bootstrapDigest:  sha256.Sum256([]byte(bootstrapToken)),
		bootstrapExpires: time.Now().Add(time.Minute),
		sessions:         make(map[string]authSession),
	}
}
func TestAdministratorPasswordIsBoundToServerPepper(t *testing.T) {
	store := &memoryAdministratorStore{}
	first := testAuthManager(store, strings.Repeat("a", 32), "one-time-token")
	if err := first.createAdministrator(context.Background(), "admin", "a-strong-admin-password", "one-time-token"); err != nil {
		t.Fatalf("create administrator: %v", err)
	}
	if !first.authenticate(context.Background(), "admin", "a-strong-admin-password") {
		t.Fatal("password rejected on original server")
	}

	copiedDatabase := testAuthManager(store, strings.Repeat("b", 32), "unused")
	if copiedDatabase.authenticate(context.Background(), "admin", "a-strong-admin-password") {
		t.Fatal("copied database authenticated with a different server pepper")
	}
}

func TestBootstrapTokenIsSingleUse(t *testing.T) {
	store := &memoryAdministratorStore{}
	manager := testAuthManager(store, strings.Repeat("p", 32), "bootstrap")
	if err := manager.createAdministrator(context.Background(), "admin", "a-strong-admin-password", "bootstrap"); err != nil {
		t.Fatal(err)
	}
	if manager.validBootstrapToken("bootstrap") {
		t.Fatal("bootstrap token remained valid after setup")
	}
	if err := manager.createAdministrator(context.Background(), "other", "another-strong-password", "bootstrap"); err == nil {
		t.Fatal("second administrator creation succeeded")
	}
}
func TestRemoteListenerRequiresServerPepper(t *testing.T) {
	t.Setenv("CREDENTIALS_DIRECTORY", "")
	t.Setenv("FLEETAMP_SERVER_PEPPER_FILE", "")
	t.Setenv("FLEETAMP_ALLOW_INSECURE", "")
	if _, _, err := loadServerPepper(t.TempDir(), "0.0.0.0:8080"); err == nil {
		t.Fatal("remote listener accepted without server pepper")
	}
}

func TestSessionCookieSecurity(t *testing.T) {
	manager := testAuthManager(&memoryAdministratorStore{}, strings.Repeat("p", 32), "bootstrap")
	manager.secureCookies = true
	token, err := manager.createSession("admin")
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	manager.setSessionCookie(response, token)
	cookies := response.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookies=%d", len(cookies))
	}
	cookie := cookies[0]
	if !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteStrictMode {
		t.Fatalf("insecure cookie attributes: %#v", cookie)
	}
	request := httptest.NewRequest(http.MethodGet, "/agents", nil)
	request.AddCookie(cookie)
	if !manager.validSession(request) {
		t.Fatal("valid session rejected")
	}
	if cookie.Expires.IsZero() {
		t.Fatal("persistent session cookie is missing an explicit expiry")
	}
}

func TestCreateSessionBoundsAndPrunesSessionTable(t *testing.T) {
	manager := testAuthManager(&memoryAdministratorStore{}, strings.Repeat("p", 32), "bootstrap")
	now := time.Date(2026, time.September, 6, 12, 0, 0, 0, time.UTC)
	manager.now = func() time.Time { return now }
	manager.sessions["expired"] = authSession{Username: "admin", Expires: now.Add(-time.Minute)}

	for i := 0; i < maximumActiveSessions+5; i++ {
		if _, err := manager.createSession("admin"); err != nil {
			t.Fatal(err)
		}
	}
	if got := len(manager.sessions); got != maximumActiveSessions {
		t.Fatalf("active sessions=%d, want %d", got, maximumActiveSessions)
	}
	if _, ok := manager.sessions["expired"]; ok {
		t.Fatal("expired session was not pruned")
	}
}

func TestSessionsOnSameHostDoNotCollideAcrossInstallations(t *testing.T) {
	first := testAuthManager(&memoryAdministratorStore{}, strings.Repeat("a", 32), "bootstrap")
	second := testAuthManager(&memoryAdministratorStore{}, strings.Repeat("b", 32), "bootstrap")
	if first.cookieName() == second.cookieName() {
		t.Fatal("different installations share a session cookie name")
	}
	firstToken, err := first.createSession("admin")
	if err != nil {
		t.Fatal(err)
	}
	secondToken, err := second.createSession("admin")
	if err != nil {
		t.Fatal(err)
	}
	firstResponse := httptest.NewRecorder()
	first.setSessionCookie(firstResponse, firstToken)
	secondResponse := httptest.NewRecorder()
	second.setSessionCookie(secondResponse, secondToken)
	request := httptest.NewRequest(http.MethodGet, "http://localhost:8080/agents", nil)
	request.AddCookie(firstResponse.Result().Cookies()[0])
	request.AddCookie(secondResponse.Result().Cookies()[0])
	if !first.validSession(request) || !second.validSession(request) {
		t.Fatal("one installation invalidated the other installation's session")
	}
	first.clearSession(httptest.NewRecorder(), request)
	if first.validSession(request) || !second.validSession(request) {
		t.Fatal("logging out of one installation changed the other installation's session")
	}
}
