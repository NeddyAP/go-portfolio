package github_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/NeddyAP/go-portfolio/internal/github"
)

func TestParseUsername(t *testing.T) {
	cases := map[string]string{
		"https://github.com/neddyap":        "neddyap",
		"https://github.com/neddyap/":       "neddyap",
		"http://github.com/NeddyAP/repo":    "NeddyAP",
		"neddyap":                           "neddyap",
		"":                                  "",
		"https://github.com/":               "",
	}
	for in, want := range cases {
		if got := github.ParseUsername(in); got != want {
			t.Errorf("ParseUsername(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRefreshBlockingAndCache(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/neddyap" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"total": {"lastYear": 42},
			"contributions": [
				{"date": "2026-11-15", "count": 2, "level": 1},
				{"date": "2026-11-16", "count": 5, "level": 3},
				{"date": "2026-11-22", "count": 8, "level": 4}
			]
		}`))
	}))
	defer srv.Close()

	svc := github.NewService(srv.URL, nil)
	data, err := svc.RefreshBlocking(context.Background(), "neddyap")
	if err != nil {
		t.Fatalf("RefreshBlocking: %v", err)
	}
	if data.TotalContributions != 42 {
		t.Fatalf("expected 42 contributions, got %d", data.TotalContributions)
	}
	if len(data.Weeks) < 2 {
		t.Fatalf("expected at least 2 week columns, got %d", len(data.Weeks))
	}

	cached := svc.GetCached("neddyap")
	if cached == nil || cached.TotalContributions != 42 {
		t.Fatalf("expected cached data, got %+v", cached)
	}
}
