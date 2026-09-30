package main

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPaginationPreservesFiltersAndSlicesRequestedPage(t *testing.T) {
	request := httptest.NewRequest("GET", "/agents?status=active&page=2&page_size=25", nil)
	page := paginationFromRequest(request, 61)

	if page.Page != 2 || page.TotalPages != 3 || page.From != 26 || page.To != 50 {
		t.Fatalf("unexpected pagination: %+v", page)
	}
	if !strings.Contains(page.PreviousURL, "status=active") || !strings.Contains(page.NextURL, "page=3") {
		t.Fatalf("pagination URLs did not preserve state: previous=%q next=%q", page.PreviousURL, page.NextURL)
	}

	items := make([]int, 61)
	for i := range items {
		items[i] = i + 1
	}
	result := paginateSlice(items, page)
	if len(result) != 25 || result[0] != 26 || result[24] != 50 {
		t.Fatalf("unexpected page slice: first=%d last=%d length=%d", result[0], result[len(result)-1], len(result))
	}
}

func TestPaginationNormalizesUnsupportedValues(t *testing.T) {
	request := httptest.NewRequest("GET", "/groups?page=999&page_size=13", nil)
	page := paginationFromRequest(request, 70)
	if page.PageSize != 50 || page.Page != 2 || page.To != 70 {
		t.Fatalf("unexpected normalized pagination: %+v", page)
	}
}
