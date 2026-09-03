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
	mu    sync.Mutex
	admin *sqlitestore.Administrator
}

func (s *memoryAdministratorStore) Exists(context.Context) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.admin != nil, nil
}

func (s *memoryAdministratorStore) Get(context.Context) (*sqlitestore.Administrator, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.admin == nil {
		return nil, sqlitestore.ErrAdministratorNotFound
	}
	copy := *s.admin
	copy.PasswordSalt = append([]byte(nil), s.admin.PasswordSalt...)
	copy.PasswordHash = append([]byte(nil), s.admin.PasswordHash...)
	return &copy, nil
}
func (s *memoryAdministratorStore) Create(_ context.Context, admin sqlitestore.Administrator) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.admin != nil {
		return errors.New("administrator exists")
	}
	copy := admin
	s.admin = &copy
	return nil
}

func (s *memoryAdministratorStore) ReplacePassword(_ context.Context, username string, salt, hash []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.admin == nil || s.admin.Username != username {
		return sqlitestore.ErrAdministratorNotFound
	}
	s.admin.PasswordSalt = append([]byte(nil), salt...)
	s.admin.PasswordHash = append([]byte(nil), hash...)
	return nil
}

func testAuthManager(store administratorStore, pepper, bootstrapToken string) *authManager {
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
}
