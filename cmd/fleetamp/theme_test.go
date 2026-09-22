package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestThemeAssetAndControlsAreRendered(t *testing.T) {
	mux := http.NewServeMux()
	registerUIRoutes(mux)

	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/assets/theme.js", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("theme asset status = %d", response.Code)
	}
	if got := response.Header().Get("Content-Type"); !strings.Contains(got, "text/javascript") {
		t.Fatalf("theme asset content type = %q", got)
	}
	for _, want := range []string{"fleetamp-theme", "localStorage.setItem", "root.dataset.theme", "\"light\"", "\"dark\""} {
		if !strings.Contains(response.Body.String(), want) {
			t.Errorf("theme asset does not contain %q", want)
		}
	}

	var page bytes.Buffer
	if err := agentsPage.Execute(&page, agentListView{Page: "fleet"}); err != nil {
		t.Fatal(err)
	}
	html := page.String()
	for _, want := range []string{
		"id=\"theme-toggle\"",
		"src=\"/assets/theme.js\"",
		":root[data-theme=\"light\"]",
		"--theme-bg:#0c0c0e",
		"--theme-bg:#f5f5f6",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("fleet page does not contain %q", want)
		}
	}
	pages := map[string]string{
		"fleet":                  fleetHTML + fleetTail,
		"groups":                 groupsHTML,
		"upcoming":               upcomingHTML,
		"agent detail":           agentDetailHTML,
		"group detail":           groupDetailHTML,
		"configuration pipeline": configurationDetailHTML,
	}
	for name, markup := range pages {
		for _, want := range []string{"id=\"theme-toggle\"", "/assets/theme.js", ":root[data-theme=\"light\"]", "--theme-bg:#0c0c0e"} {
			if !strings.Contains(markup, want) {
				t.Errorf("%s page does not contain %q", name, want)
			}
		}
	}

	page.Reset()
	if err := authPage.Execute(&page, authPageData{Title: "Login"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(page.String(), "id=\"theme-toggle\"") || !strings.Contains(page.String(), "/assets/theme.js") {
		t.Fatal("authentication page does not expose the theme control")
	}
}
