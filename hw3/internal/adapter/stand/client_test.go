package stand_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Kkaramanov/go-dev/hw3/internal/adapter/stand"
	"github.com/Kkaramanov/go-dev/hw3/internal/domain"
)

func TestGetMovie(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
		code   domain.Code
	}{
		{"success", 200, `{"id":7,"title":"Film","year":2020,"director":"Director"}`, ""},
		{"not found", 404, "", domain.NotFound},
		{"rate limited", 429, "", domain.RateLimited},
		{"upstream failure", 500, "", domain.UpstreamError},
		{"malformed JSON", 200, `{`, domain.InvalidResponse},
		{"wrong ID", 200, `{"id":8,"title":"Film","year":2020,"director":"Director"}`, domain.InvalidResponse},
		{"missing fields", 200, `{"id":7}`, domain.InvalidResponse},
		{"oversized response", 200, strings.Repeat(" ", (1<<20)+1), domain.InvalidResponse},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "GET" || r.URL.Path != "/stand/7/info.0.json" {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
				}
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()
			client := stand.New(server.URL+"/stand/", server.Client(), time.Second)
			got, err := client.GetMovie(context.Background(), 7)
			if tc.code != "" {
				if err == nil || domain.Classify(err) != tc.code {
					t.Fatalf("got %v; want %s", err, tc.code)
				}
				return
			}
			want := domain.Movie{ID: 7, Title: "Film", Year: 2020, Director: "Director"}
			if err != nil || got != want {
				t.Fatalf("got %+v, %v", got, err)
			}
		})
	}
}

func TestCanceledRequest(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	client := stand.New("http://localhost:1", &http.Client{}, time.Second)
	_, err := client.GetMovie(ctx, 1)
	if !errors.Is(err, context.Canceled) || domain.Classify(err) != domain.Canceled {
		t.Fatalf("got %v", err)
	}
}
