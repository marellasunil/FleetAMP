package main

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	sqlitestore "github.com/marellasunil/FleetAMP/internal/storage/sqlite"
	"golang.org/x/crypto/argon2"
)

const (
	serverPepperCredential = "fleetamp-server-pepper"
	sessionCookieName      = "fleetamp_session"
	minimumAdminPassword   = 16
)

type administratorStore interface {
	Exists(context.Context) (bool, error)
	Get(context.Context) (*sqlitestore.Administrator, error)
	Create(context.Context, sqlitestore.Administrator) error
	ReplacePassword(context.Context, string, []byte, []byte) error
}
type authSession struct {
	Username string
	Expires  time.Time
}

type authManager struct {
	store            administratorStore
	pepper           []byte
	bootstrapDigest  [sha256.Size]byte
	bootstrapExpires time.Time
	secureCookies    bool
	now              func() time.Time
	mu               sync.RWMutex
	sessions         map[string]authSession
}

// newAuthManager loads the server-bound pepper, determines secure-cookie behavior, and starts first-login setup when needed.
func newAuthManager(ctx context.Context, store administratorStore, dataDir, httpAddr string) (*authManager, error) {
	pepper, source, err := loadServerPepper(dataDir, httpAddr)
	if err != nil {
		return nil, err
	}
	manager := &authManager{
		store:         store,
		pepper:        pepper,
		secureCookies: !isLoopbackListener(httpAddr) || boolEnv("FLEETAMP_HTTP_TLS_ENABLED") || boolEnv("FLEETAMP_SECURE_COOKIES"),
		now:           time.Now,
		sessions:      make(map[string]authSession),
	}
	exists, err := store.Exists(ctx)
	if err != nil {
		return nil, err
	}
	slog.Info("authentication initialized", "component", "auth", "pepper_source", source, "administrator_configured", exists)
	if !exists {
		if err := manager.issueBootstrapToken(); err != nil {
			return nil, err
		}
	}
	return manager, nil
}

// loadServerPepper reads the systemd credential or configured pepper file, falling back to a local development file only on loopback.
func loadServerPepper(dataDir, httpAddr string) ([]byte, string, error) {
	if directory := strings.TrimSpace(os.Getenv("CREDENTIALS_DIRECTORY")); directory != "" {
		path := filepath.Join(directory, serverPepperCredential)
		value, err := os.ReadFile(path)
		if err == nil {
			return validatePepper(value, "systemd credential")
		}
		if !errors.Is(err, os.ErrNotExist) {
			return nil, "", fmt.Errorf("read systemd server pepper: %w", err)
		}
	}
	if configured := strings.TrimSpace(os.Getenv("FLEETAMP_SERVER_PEPPER_FILE")); configured != "" {
		value, err := os.ReadFile(configured)
		if err != nil {
			return nil, "", fmt.Errorf("read server pepper file: %w", err)
		}
		return validatePepper(value, "configured file")
	}
	if !isLoopbackListener(httpAddr) && !boolEnv("FLEETAMP_ALLOW_INSECURE") {
		return nil, "", fmt.Errorf("remote HTTP listener %q requires the %q systemd credential or FLEETAMP_SERVER_PEPPER_FILE", httpAddr, serverPepperCredential)
	}
	path := filepath.Join(dataDir, "server-pepper")
	value, err := os.ReadFile(path)
	if err == nil {
		return validatePepper(value, "local development file")
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, "", fmt.Errorf("read local server pepper: %w", err)
	}
	return createLocalPepper(path)
}

// validatePepper trims and checks pepper material before it participates in password derivation.
func validatePepper(value []byte, source string) ([]byte, string, error) {
	value = []byte(strings.TrimSpace(string(value)))
	if decoded, err := base64.RawURLEncoding.DecodeString(string(value)); err == nil {
		value = decoded
	}
	if len(value) < 32 {
		return nil, "", fmt.Errorf("server pepper from %s must contain at least 32 bytes", source)
	}
	return value, source, nil
}

// createLocalPepper generates a restricted development pepper file without exposing its value in logs.
func createLocalPepper(path string) ([]byte, string, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, "", fmt.Errorf("create pepper directory: %w", err)
	}
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return nil, "", fmt.Errorf("generate local server pepper: %w", err)
	}
	encoded := []byte(base64.RawURLEncoding.EncodeToString(value))
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			existing, readErr := os.ReadFile(path)
			if readErr != nil {
				return nil, "", readErr
			}
			return validatePepper(existing, "local development file")
		}
		return nil, "", fmt.Errorf("create local server pepper: %w", err)
	}
	if _, err := file.Write(encoded); err != nil {
		_ = file.Close()
		return nil, "", fmt.Errorf("write local server pepper: %w", err)
	}
	if err := file.Close(); err != nil {
		return nil, "", err
	}
	slog.Warn("using file-backed development pepper; configure TPM-backed systemd credentials before remote deployment", "component", "auth")
	return value, "local development file", nil
}

// issueBootstrapToken creates an in-memory, time-limited token used exactly once to create the first administrator.
func (a *authManager) issueBootstrapToken() error {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return fmt.Errorf("generate bootstrap token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	a.bootstrapDigest = sha256.Sum256([]byte(token))
	a.bootstrapExpires = a.now().Add(15 * time.Minute)
	slog.Warn("FleetAMP administrator setup required",
		"component", "auth", "event", "bootstrap_required",
		"setup_url", "/setup", "bootstrap_token", token,
		"expires_at", a.bootstrapExpires.UTC().Format(time.RFC3339))
	return nil
}

// configured reports whether the singleton administrator record already exists.
func (a *authManager) configured(ctx context.Context) (bool, error) {
	return a.store.Exists(ctx)
}

// validBootstrapToken checks token expiry and value using a constant-time digest comparison.
func (a *authManager) validBootstrapToken(token string) bool {
	if token == "" || !a.now().Before(a.bootstrapExpires) {
		return false
	}
	actual := sha256.Sum256([]byte(token))
	return subtle.ConstantTimeCompare(actual[:], a.bootstrapDigest[:]) == 1
}

// passwordDigest derives the stored Argon2id verifier from the password, per-user salt, and server-specific pepper.
func passwordDigest(password string, pepper, salt []byte) []byte {
	mac := hmac.New(sha256.New, pepper)
	_, _ = mac.Write([]byte(password))
	material := mac.Sum(nil)
	return argon2.IDKey(material, salt, 3, 64*1024, 2, 32)
}

// newSalt returns cryptographically random salt material for a new administrator verifier.
func newSalt() ([]byte, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}
	return salt, nil
}

// createAdministrator validates the one-time setup request and atomically stores the first administrator verifier.
func (a *authManager) createAdministrator(ctx context.Context, username, password, token string) error {
	username = strings.TrimSpace(username)
	if len(username) < 3 || len(username) > 64 {
		return fmt.Errorf("username must contain between 3 and 64 characters")
	}
	if len(password) < minimumAdminPassword {
		return fmt.Errorf("password must contain at least %d characters", minimumAdminPassword)
	}
	if !a.validBootstrapToken(token) {
		return fmt.Errorf("bootstrap token is invalid or expired")
	}
	exists, err := a.store.Exists(ctx)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("administrator is already configured")
	}
	salt, err := newSalt()
	if err != nil {
		return err
	}
	if err := a.store.Create(ctx, sqlitestore.Administrator{
		Username: username, PasswordSalt: salt,
		PasswordHash: passwordDigest(password, a.pepper, salt),
	}); err != nil {
		return err
	}
	a.bootstrapDigest = [sha256.Size]byte{}
	a.bootstrapExpires = time.Time{}
	slog.Info("administrator created", "component", "auth", "event", "administrator_created", "username", username)
	return nil
}

// authenticate derives and compares the supplied password without revealing whether an account lookup failed.
func (a *authManager) authenticate(ctx context.Context, username, password string) bool {
	admin, err := a.store.Get(ctx)
	if err != nil {
		return false
	}
	actualUser := sha256.Sum256([]byte(strings.TrimSpace(username)))
	expectedUser := sha256.Sum256([]byte(admin.Username))
	actualHash := passwordDigest(password, a.pepper, admin.PasswordSalt)
	return subtle.ConstantTimeCompare(actualUser[:], expectedUser[:]) == 1 &&
		subtle.ConstantTimeCompare(actualHash, admin.PasswordHash) == 1
}

// createSession creates an opaque browser token while storing only its digest in the in-memory session table.
func (a *authManager) createSession(username string) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	key := sessionKey(token)
	a.mu.Lock()
	a.sessions[key] = authSession{Username: username, Expires: a.now().Add(12 * time.Hour)}
	a.mu.Unlock()
	return token, nil
}

// sessionKey hashes a raw session token so plaintext tokens are never retained server-side.
func sessionKey(token string) string {
	digest := sha256.Sum256([]byte(token))
	return hex.EncodeToString(digest[:])
}

// validSession verifies the cookie-backed session and removes it when it has expired.
func (a *authManager) validSession(r *http.Request) bool {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil || cookie.Value == "" {
		return false
	}
	key := sessionKey(cookie.Value)
	a.mu.RLock()
	session, ok := a.sessions[key]
	a.mu.RUnlock()
	if !ok {
		return false
	}
	if !a.now().Before(session.Expires) {
		a.mu.Lock()
		delete(a.sessions, key)
		a.mu.Unlock()
		return false
	}
	return true
}

// setSessionCookie writes a restricted HttpOnly, SameSite cookie and enables Secure when HTTPS is expected.
func (a *authManager) setSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookieName, Value: token, Path: "/",
		HttpOnly: true, Secure: a.secureCookies, SameSite: http.SameSiteStrictMode,
		MaxAge: int((12 * time.Hour).Seconds()),
	})
}

// clearSession deletes the server-side session and expires the corresponding browser cookie.
func (a *authManager) clearSession(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookieName); err == nil {
		a.mu.Lock()
		delete(a.sessions, sessionKey(cookie.Value))
		a.mu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookieName, Path: "/", HttpOnly: true,
		Secure: a.secureCookies, SameSite: http.SameSiteStrictMode, MaxAge: -1,
	})
}

// authorize permits a valid application session or legacy Basic Auth and otherwise returns an HTML redirect or API 401.
func (a *authManager) authorize(w http.ResponseWriter, r *http.Request, legacy securityConfig) bool {
	if a.validSession(r) {
		return true
	}
	if legacy.HTTPUsername != "" && validBasicAuth(r, legacy) {
		return true
	}
	configured, err := a.configured(r.Context())
	if err != nil {
		internalServerError(w, err)
		return false
	}
	if !configured {
		if r.Method == http.MethodGet || r.Method == http.MethodHead {
			http.Redirect(w, r, "/setup", http.StatusSeeOther)
		} else {
			http.Error(w, "FleetAMP administrator setup is required", http.StatusServiceUnavailable)
		}
		return false
	}
	if acceptsHTML(r) {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	} else {
		w.Header().Set("WWW-Authenticate", `Basic realm="FleetAMP", charset="UTF-8"`)
		http.Error(w, "authentication required", http.StatusUnauthorized)
	}
	return false
}

// acceptsHTML distinguishes browser navigation from API clients for redirect-versus-JSON authentication behavior.
func acceptsHTML(r *http.Request) bool {
	return strings.Contains(r.Header.Get("Accept"), "text/html") ||
		(!strings.HasPrefix(r.URL.Path, "/api/") && r.Header.Get("Accept") == "")
}

type authPageData struct {
	Title   string
	Message string
	Setup   bool
}

const authPageHTML = `<!doctype html><html><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>{{.Title}} · FleetAMP</title><style>` + controlPlaneCSS + `
.authshell{min-height:100vh;display:grid;place-items:center;padding:24px}
.authcard{width:min(460px,100%);padding:26px}.authbrand{text-align:center;margin-bottom:22px}
.authform{display:grid;gap:14px}.authform label{display:grid;gap:7px;color:var(--muted)}
.authform .input{width:100%}.autherror{border:1px solid #70404a;background:#351923;color:#ffabb5;padding:11px;border-radius:8px}
.authhelp{font-size:11px;color:var(--muted);line-height:1.6}</style></head>
<body><main class="authshell"><section class="card authcard"><div class="authbrand">
<div class="brandmark" style="margin:auto">∿</div><h1>FleetAMP</h1>
<div class="subtitle">{{if .Setup}}Secure administrator setup{{else}}Control-plane login{{end}}</div></div>
{{if .Message}}<div class="autherror">{{.Message}}</div>{{end}}
<form class="authform" method="post" action="{{if .Setup}}/setup{{else}}/login{{end}}">
<label>Username<input class="input" name="username" autocomplete="username" required minlength="3" maxlength="64"></label>
<label>Password<input class="input" type="password" name="password" autocomplete="{{if .Setup}}new-password{{else}}current-password{{end}}" required minlength="16"></label>
{{if .Setup}}<label>Confirm password<input class="input" type="password" name="confirm_password" autocomplete="new-password" required minlength="16"></label>
<label>One-time setup token<input class="input" type="password" name="bootstrap_token" autocomplete="off" required></label>{{end}}
<button class="btn primary" type="submit">{{if .Setup}}Create administrator{{else}}Sign in{{end}}</button>
</form><p class="authhelp">{{if .Setup}}Retrieve the short-lived token from the FleetAMP systemd journal. It expires after 15 minutes and is consumed once.{{else}}Use the administrator credentials created during FleetAMP setup.{{end}}</p>
</section></main></body></html>`

var authPage = template.Must(template.New("auth").Parse(authPageHTML))

// registerRoutes exposes the first-login setup, normal login, and logout endpoints.
func (a *authManager) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/setup", a.handleSetup)
	mux.HandleFunc("/login", a.handleLogin)
	mux.HandleFunc("/logout", a.handleLogout)
}

// handleSetup renders the first-administrator page on GET and processes the one-time setup form on POST.
func (a *authManager) handleSetup(w http.ResponseWriter, r *http.Request) {
	configured, err := a.configured(r.Context())
	if err != nil {
		internalServerError(w, err)
		return
	}
	if configured {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if r.Method == http.MethodGet || r.Method == http.MethodHead {
		a.renderAuthPage(w, authPageData{Title: "Administrator setup", Setup: true})
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		a.renderAuthPage(w, authPageData{Title: "Administrator setup", Setup: true, Message: "Invalid setup request."})
		return
	}
	password := r.FormValue("password")
	if password != r.FormValue("confirm_password") {
		a.renderAuthPage(w, authPageData{Title: "Administrator setup", Setup: true, Message: "Passwords do not match."})
		return
	}
	if err := a.createAdministrator(r.Context(), r.FormValue("username"), password, r.FormValue("bootstrap_token")); err != nil {
		slog.Warn("administrator setup rejected", "component", "auth", "event", "bootstrap_rejected")
		a.renderAuthPage(w, authPageData{Title: "Administrator setup", Setup: true, Message: err.Error()})
		return
	}
	token, err := a.createSession(strings.TrimSpace(r.FormValue("username")))
	if err != nil {
		internalServerError(w, err)
		return
	}
	a.setSessionCookie(w, token)
	http.Redirect(w, r, "/agents", http.StatusSeeOther)
}

// handleLogin renders the sign-in page on GET and creates an authenticated session after a valid POST.
func (a *authManager) handleLogin(w http.ResponseWriter, r *http.Request) {
	configured, err := a.configured(r.Context())
	if err != nil {
		internalServerError(w, err)
		return
	}
	if !configured {
		http.Redirect(w, r, "/setup", http.StatusSeeOther)
		return
	}
	if r.Method == http.MethodGet || r.Method == http.MethodHead {
		a.renderAuthPage(w, authPageData{Title: "Sign in"})
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil || !a.authenticate(r.Context(), r.FormValue("username"), r.FormValue("password")) {
		time.Sleep(300 * time.Millisecond)
		slog.Warn("login rejected", "component", "auth", "event", "login_failed")
		a.renderAuthPage(w, authPageData{Title: "Sign in", Message: "Invalid username or password."})
		return
	}
	token, err := a.createSession(strings.TrimSpace(r.FormValue("username")))
	if err != nil {
		internalServerError(w, err)
		return
	}
	a.setSessionCookie(w, token)
	slog.Info("administrator signed in", "component", "auth", "event", "login_succeeded", "username", strings.TrimSpace(r.FormValue("username")))
	http.Redirect(w, r, "/agents", http.StatusSeeOther)
}

// handleLogout invalidates the current session and returns the browser to the login page.
func (a *authManager) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	a.clearSession(w, r)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// renderAuthPage executes the shared authentication template with a safe generic error model.
func (a *authManager) renderAuthPage(w http.ResponseWriter, data authPageData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if err := authPage.Execute(w, data); err != nil {
		slog.Error("render authentication page", "component", "auth", "error", err)
	}
}
