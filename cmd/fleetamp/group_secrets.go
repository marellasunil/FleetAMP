package main

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/marellasunil/FleetAMP/internal/configs"
	"github.com/marellasunil/FleetAMP/internal/groups"
	groupsecrets "github.com/marellasunil/FleetAMP/internal/secrets"
	"github.com/marellasunil/FleetAMP/internal/storage"
)

const groupSecretPrefix = "enc:group-secret:v1:"

var secretReferencePattern = regexp.MustCompile(`\$\{secret:([A-Za-z0-9][A-Za-z0-9._/-]{0,127})\}`)
var secretKeyPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/-]{0,127}$`)

type groupSecretService struct {
	store  storage.GroupSecretStore
	pepper []byte
}

var runtimeGroupSecrets *groupSecretService

func (s *groupSecretService) encryptionKey() []byte {
	digest := sha256.Sum256(append([]byte("fleetamp-group-secret-v1:"), s.pepper...))
	return digest[:]
}

func (s *groupSecretService) encrypt(value string) (string, error) {
	block, err := aes.NewCipher(s.encryptionKey()); if err != nil { return "", err }
	gcm, err := cipher.NewGCM(block); if err != nil { return "", err }
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil { return "", err }
	sealed := gcm.Seal(nonce, nonce, []byte(value), []byte(groupSecretPrefix))
	return groupSecretPrefix + base64.RawStdEncoding.EncodeToString(sealed), nil
}

func (s *groupSecretService) decrypt(stored string) (string, error) {
	if !strings.HasPrefix(stored, groupSecretPrefix) { return "", fmt.Errorf("unsupported group secret format") }
	sealed, err := base64.RawStdEncoding.DecodeString(strings.TrimPrefix(stored, groupSecretPrefix)); if err != nil { return "", err }
	block, err := aes.NewCipher(s.encryptionKey()); if err != nil { return "", err }
	gcm, err := cipher.NewGCM(block); if err != nil { return "", err }
	if len(sealed) < gcm.NonceSize() { return "", fmt.Errorf("encrypted group secret is truncated") }
	value, err := gcm.Open(nil, sealed[:gcm.NonceSize()], sealed[gcm.NonceSize():], []byte(groupSecretPrefix))
	if err != nil { return "", fmt.Errorf("authenticate encrypted group secret: %w", err) }
	return string(value), nil
}

func (s *groupSecretService) values(ctx context.Context, groupID string) (map[string]string, error) {
	items, err := s.store.ListByGroup(ctx, groupID); if err != nil { return nil, err }
	result := make(map[string]string, len(items))
	for _, item := range items { value, decryptErr := s.decrypt(item.Ciphertext); if decryptErr != nil { return nil, decryptErr }; result[item.Key] = value }
	return result, nil
}

func (s *groupSecretService) materialize(ctx context.Context, groupID, content string) (string, error) {
	if groupID == "" || !secretReferencePattern.MatchString(content) { return content, nil }
	values, err := s.values(ctx, groupID); if err != nil { return "", err }
	var missing string
	resolved := secretReferencePattern.ReplaceAllStringFunc(content, func(reference string) string {
		parts := secretReferencePattern.FindStringSubmatch(reference)
		value, ok := values[parts[1]]
		if !ok { missing = parts[1]; return reference }
		return value
	})
	if missing != "" { return "", fmt.Errorf("group secret %q is not configured", missing) }
	return resolved, nil
}

// validateReferences confirms that every placeholder has a group-scoped key
// without decrypting values or passing plaintext through validation errors.
func (s *groupSecretService) validateReferences(ctx context.Context, groupID, content string) error {
	if groupID == "" || !secretReferencePattern.MatchString(content) {
		return nil
	}
	items, err := s.store.ListByGroup(ctx, groupID)
	if err != nil {
		return err
	}
	available := make(map[string]bool, len(items))
	for _, item := range items {
		available[item.Key] = true
	}
	for _, match := range secretReferencePattern.FindAllStringSubmatch(content, -1) {
		if !available[match[1]] {
			return fmt.Errorf("group secret %q is not configured", match[1])
		}
	}
	return nil
}

// normalize replaces both stored references and decrypted values with the same
// marker. It is safe for drift comparisons and user-facing effective config.
func (s *groupSecretService) normalize(ctx context.Context, groupID, content string) (string, error) {
	if groupID == "" { return content, nil }
	values, err := s.values(ctx, groupID); if err != nil { return "", err }
	for key, value := range values {
		marker := "<secret:" + key + ">"
		content = strings.ReplaceAll(content, "${secret:"+key+"}", marker)
		if value != "" { content = strings.ReplaceAll(content, value, marker) }
	}
	return content, nil
}

func configurationForDelivery(ctx context.Context, configuration *configs.Configuration) (*configs.Configuration, error) {
	if configuration == nil || runtimeGroupSecrets == nil || configuration.GroupID == "" { return configuration, nil }
	content, err := runtimeGroupSecrets.materialize(ctx, configuration.GroupID, configuration.Content); if err != nil { return nil, err }
	if content == configuration.Content { return configuration, nil }
	resolved := configs.NewGroupConfiguration(configuration.GroupID, configuration.Name, configuration.Version, content, configuration.ContentType)
	resolved.ID, resolved.CreatedAt = configuration.ID, configuration.CreatedAt
	return resolved, nil
}

func configurationContentForValidation(ctx context.Context, groupID, content string) (string, error) {
	if runtimeGroupSecrets == nil || groupID == "" { return content, nil }
	if err := runtimeGroupSecrets.validateReferences(ctx, groupID, content); err != nil { return "", err }
	return content, nil
}

type groupSecretsView struct {
	Page string
	Group *groups.Group
	Items []*groupsecrets.GroupSecret
	Message, Error string
}

const groupSecretsHTML = `<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Group secrets · FleetAMP</title><style>` + controlPlaneCSS + detailCSS + `</style></head><body><div class="shell">` + sideNav + `<main class="main"><header class="top"><div><div class="crumb">FleetAMP / Groups &amp; Labels / Secrets</div><div class="pagetitle">{{.Group.Name}} secrets</div><div class="subtitle">Encrypted group-scoped values resolved only when configuration is deployed.</div></div><div class="topactions"><a class="btn" href="/groups/{{.Group.ID}}">← Group</a></div></header><div class="content">{{if .Message}}<div class="notice">✓ {{.Message}}</div>{{end}}{{if .Error}}<div class="configerror">{{.Error}}</div>{{end}}<section class="card"><div class="cardhead"><div><div class="cardtitle">Add or replace secret</div><div class="cardsub">Existing values are never displayed. Saving the same key safely replaces its value.</div></div></div><div class="cardbody"><form class="detailform" method="post"><input type="hidden" name="action" value="upsert"><label>Key<input class="input" name="key" required pattern="[A-Za-z0-9][A-Za-z0-9._/-]{0,127}" placeholder="hec_token"></label><label>Secret value<input class="input" type="password" name="value" required autocomplete="new-password"></label><button class="btn primary" type="submit">Add or replace</button></form><p class="tiny">Reference this value in YAML as <code>${secret:key}</code>. The saved version, preview, approval diff and audit log retain only the reference.</p></div></section><section class="card" style="margin-top:16px"><div class="cardhead"><div><div class="cardtitle">Available secret keys</div><div class="cardsub">Metadata only—values cannot be read back from this page.</div></div></div>{{if .Items}}<table><thead><tr><th>Key</th><th>Reference</th><th>Updated by</th><th>Updated</th><th>Action</th></tr></thead><tbody>{{range .Items}}<tr><td><strong>{{.Key}}</strong></td><td><code>${secret:{{.Key}}}</code></td><td>{{.UpdatedBy}}</td><td>{{.UpdatedAt}}</td><td><form method="post" onsubmit="return confirm('Delete this secret key? Deployments using it will fail until it is restored.')"><input type="hidden" name="action" value="delete"><input type="hidden" name="key" value="{{.Key}}"><button class="btn" type="submit">Delete</button></form></td></tr>{{end}}</tbody></table>{{else}}<div class="cardbody tiny">No secret keys configured for this group.</div>{{end}}</section></div></main></div></body></html>`

var groupSecretsPage = template.Must(template.New("group-secrets").Parse(groupSecretsHTML))

func registerGroupSecretRoutes(mux *http.ServeMux, store storage.GroupSecretStore, groupStore storage.GroupStore, auth *authManager) {
	service := &groupSecretService{store: store, pepper: auth.pepper}
	runtimeGroupSecrets = service
	mux.HandleFunc("/groups/{id}/secrets", func(w http.ResponseWriter, r *http.Request) {
		group, err := groupStore.Get(r.Context(), r.PathValue("id")); if err != nil { http.NotFound(w, r); return }
		if !requireGroupAccess(w, r, auth, group) { return }
		if !canManageGroup(auth, r, group) { http.Error(w, "only an Admin or assigned Group Owner can manage group secrets", http.StatusForbidden); return }
		if r.Method == http.MethodPost {
			if err := r.ParseForm(); err != nil { http.Error(w, "invalid form", http.StatusBadRequest); return }
			key := strings.TrimSpace(r.FormValue("key"))
			if !secretKeyPattern.MatchString(key) { http.Redirect(w, r, "/groups/"+group.ID+"/secrets?error="+url.QueryEscape("Secret key contains unsupported characters."), http.StatusSeeOther); return }
			switch r.FormValue("action") {
			case "upsert":
				value := r.FormValue("value"); if value == "" || len(value) > 16384 { http.Redirect(w, r, "/groups/"+group.ID+"/secrets?error="+url.QueryEscape("Secret value is required and must not exceed 16 KiB."), http.StatusSeeOther); return }
				ciphertext, encryptErr := service.encrypt(value); if encryptErr != nil { internalServerError(w, encryptErr); return }
				now := time.Now().UTC(); item := &groupsecrets.GroupSecret{GroupID: group.ID, Key: key, Ciphertext: ciphertext, UpdatedBy: currentUsername(auth, r), CreatedAt: now, UpdatedAt: now}
				if err := store.Upsert(r.Context(), item); err != nil { internalServerError(w, err); return }
				http.Redirect(w, r, "/groups/"+group.ID+"/secrets?message="+url.QueryEscape("Secret key saved."), http.StatusSeeOther); return
			case "delete":
				if err := store.Delete(r.Context(), group.ID, key); err != nil { http.Error(w, err.Error(), http.StatusNotFound); return }
				http.Redirect(w, r, "/groups/"+group.ID+"/secrets?message="+url.QueryEscape("Secret key deleted."), http.StatusSeeOther); return
			default: http.Error(w, "unsupported action", http.StatusBadRequest); return
			}
		}
		if r.Method != http.MethodGet { w.WriteHeader(http.StatusMethodNotAllowed); return }
		items, err := store.ListByGroup(r.Context(), group.ID); if err != nil { internalServerError(w, err); return }
		if err := groupSecretsPage.Execute(w, groupSecretsView{Page: "groups", Group: group, Items: items, Message: r.URL.Query().Get("message"), Error: r.URL.Query().Get("error")}); err != nil { internalServerError(w, err) }
	})
}
