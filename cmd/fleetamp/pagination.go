package main

import (
	"net/http"
	"strconv"
)

type paginationView struct {
	Page, PageSize, Total, TotalPages, From, To int
	PreviousURL, NextURL                        string
	Pages                                       []paginationLink
	Sizes                                       []paginationSize
}

type paginationLink struct {
	Number  int
	URL     string
	Current bool
}

type paginationSize struct {
	Size     int
	URL      string
	Selected bool
}

const paginationHTML = `{{with .Pagination}}<div class="pagination"><div class="result-count">{{if .Total}}Showing {{.From}}–{{.To}} of {{.Total}}{{else}}No matching results{{end}}</div><div class="page-links"><a class="page-link {{if not .PreviousURL}}disabled{{end}}" href="{{if .PreviousURL}}{{.PreviousURL}}{{else}}#{{end}}">‹</a>{{range .Pages}}<a class="page-link {{if .Current}}current{{end}}" href="{{.URL}}">{{.Number}}</a>{{end}}<a class="page-link {{if not .NextURL}}disabled{{end}}" href="{{if .NextURL}}{{.NextURL}}{{else}}#{{end}}">›</a></div><div class="page-size"><span class="tiny">Rows</span>{{range .Sizes}}<a class="page-link {{if .Selected}}current{{end}}" href="{{.URL}}">{{.Size}}</a>{{end}}</div></div>{{end}}`

func paginationFromRequest(r *http.Request, total int) paginationView {
	pageSize := queryInt(r, "page_size", 50)
	switch pageSize {
	case 25, 50, 100:
	default:
		pageSize = 50
	}
	totalPages := max(1, (total+pageSize-1)/pageSize)
	page := min(max(1, queryInt(r, "page", 1)), totalPages)
	p := paginationView{Page: page, PageSize: pageSize, Total: total, TotalPages: totalPages}
	if total > 0 {
		p.From = (page-1)*pageSize + 1
		p.To = min(page*pageSize, total)
	}
	if page > 1 {
		p.PreviousURL = pageURL(r, page-1)
	}
	if page < totalPages {
		p.NextURL = pageURL(r, page+1)
	}
	for number := max(1, page-2); number <= min(totalPages, page+2); number++ {
		p.Pages = append(p.Pages, paginationLink{Number: number, URL: pageURL(r, number), Current: number == page})
	}
	for _, size := range []int{25, 50, 100} {
		values := r.URL.Query()
		values.Set("page_size", strconv.Itoa(size))
		values.Set("page", "1")
		p.Sizes = append(p.Sizes, paginationSize{Size: size, URL: r.URL.Path + "?" + values.Encode(), Selected: size == pageSize})
	}
	return p
}

func paginateSlice[T any](items []T, p paginationView) []T {
	start := (p.Page - 1) * p.PageSize
	if start >= len(items) {
		return nil
	}
	return items[start:min(start+p.PageSize, len(items))]
}
func queryInt(r *http.Request, key string, fallback int) int {
	value, err := strconv.Atoi(r.URL.Query().Get(key))
	if err != nil {
		return fallback
	}
	return value
}

func pageURL(r *http.Request, page int) string {
	values := r.URL.Query()
	values.Set("page", strconv.Itoa(page))
	return r.URL.Path + "?" + values.Encode()
}
