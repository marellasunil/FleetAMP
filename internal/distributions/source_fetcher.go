package distributions

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	officialSourceHost = "codeload.github.com"
	officialSourcePath = "/open-telemetry/opentelemetry-collector-releases/tar.gz/refs/tags/"
)

type SourceFetcher interface {
	Fetch(context.Context, *BuildSourceResolution) (archiveSHA string, archiveSize int64, localReference string, err error)
}

type RestrictedHTTPSourceFetcher struct {
	Client   *http.Client
	CacheDir string
	MaxBytes int64
}

func NewRestrictedHTTPSourceFetcher(cacheDir string, timeout time.Duration, maxBytes int64) *RestrictedHTTPSourceFetcher {
	if timeout <= 0 {
		timeout = 2 * time.Minute
	}
	if maxBytes <= 0 {
		maxBytes = 256 << 20
	}
	client := &http.Client{Timeout: timeout}
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 3 {
			return errors.New("too many source redirects")
		}
		return validateOfficialSourceURL(req.URL.String())
	}
	return &RestrictedHTTPSourceFetcher{Client: client, CacheDir: cacheDir, MaxBytes: maxBytes}
}

func validateOfficialSourceURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return errors.New("invalid source URL")
	}
	if u.Scheme != "https" || u.Hostname() != officialSourceHost || u.Port() != "" || u.User != nil {
		return errors.New("source URL must use the approved HTTPS host")
	}
	if !strings.HasPrefix(u.EscapedPath(), officialSourcePath) || strings.Contains(u.EscapedPath(), "..") || u.RawQuery != "" || u.Fragment != "" {
		return errors.New("source URL path is not allowed")
	}
	return nil
}

func (f *RestrictedHTTPSourceFetcher) Fetch(ctx context.Context, source *BuildSourceResolution) (string, int64, string, error) {
	if source == nil || source.Status != BuildSourceResolved {
		return "", 0, "", errors.New("resolved source is required")
	}
	if err := validateOfficialSourceURL(source.ArchiveURL); err != nil {
		return "", 0, "", err
	}
	if f.Client == nil {
		return "", 0, "", errors.New("source HTTP client is required")
	}
	if f.MaxBytes <= 0 {
		return "", 0, "", errors.New("source download limit is required")
	}
	if err := os.MkdirAll(f.CacheDir, 0o700); err != nil {
		return "", 0, "", fmt.Errorf("create source cache: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, source.ArchiveURL, nil)
	if err != nil {
		return "", 0, "", err
	}
	resp, err := f.Client.Do(req)
	if err != nil {
		return "", 0, "", fmt.Errorf("download official source: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", 0, "", fmt.Errorf("official source returned HTTP %d", resp.StatusCode)
	}
	if resp.ContentLength > f.MaxBytes {
		return "", 0, "", errors.New("source archive exceeds configured size limit")
	}
	tmp, err := os.CreateTemp(f.CacheDir, "source-*.partial")
	if err != nil {
		return "", 0, "", err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return "", 0, "", err
	}
	hash := sha256.New()
	written, copyErr := io.Copy(io.MultiWriter(tmp, hash), io.LimitReader(resp.Body, f.MaxBytes+1))
	syncErr := tmp.Sync()
	closeErr := tmp.Close()
	if copyErr != nil {
		return "", 0, "", copyErr
	}
	if written > f.MaxBytes {
		return "", 0, "", errors.New("source archive exceeds configured size limit")
	}
	if syncErr != nil {
		return "", 0, "", syncErr
	}
	if closeErr != nil {
		return "", 0, "", closeErr
	}
	sha := hex.EncodeToString(hash.Sum(nil))
	filename := sha + ".tar.gz"
	finalPath := filepath.Join(f.CacheDir, filename)
	if err := os.Rename(tmpName, finalPath); err != nil {
		return "", 0, "", err
	}
	return sha, written, "source-cache://" + filename, nil
}
