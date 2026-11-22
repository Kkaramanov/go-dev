package postgres_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	moviepb "github.com/Kkaramanov/go-dev/hw5/api"
	"github.com/Kkaramanov/go-dev/hw5/internal/adapter/postgres"
	"github.com/Kkaramanov/go-dev/hw5/internal/adapter/stand"
	"github.com/Kkaramanov/go-dev/hw5/internal/domain"
	grpcapi "github.com/Kkaramanov/go-dev/hw5/internal/handler/grpc"
	httpapi "github.com/Kkaramanov/go-dev/hw5/internal/handler/http"
	"github.com/Kkaramanov/go-dev/hw5/internal/testutil"
	"github.com/Kkaramanov/go-dev/hw5/internal/usecase"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

func TestSynchronizationAndAPIsWithoutStand(t *testing.T) {
	pool := testutil.Postgres(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := postgres.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	repo := postgres.New(pool)
	var last atomic.Int64
	last.Store(3)
	var calls atomic.Int64
	external := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		var id int64
		if _, err := fmt.Sscanf(r.URL.Path, "/%d/info.0.json", &id); err != nil || id < 1 || id > last.Load() {
			http.NotFound(w, r)
			return
		}
		_, _ = fmt.Fprintf(w, `{"id":%d,"title":"Film","year":2020,"director":"Director"}`, id)
	}))
	t.Cleanup(external.Close)
	synchronizer := usecase.NewSynchronizer(stand.New(external.URL, external.Client(), time.Second), repo, 3)
	for _, tc := range []struct {
		last  int64
		saved int
	}{{3, 3}, {3, 0}, {5, 2}} {
		last.Store(tc.last)
		n, err := synchronizer.Sync(ctx)
		if err != nil || n != tc.saved {
			t.Fatalf("sync last=%d: %d, %v", tc.last, n, err)
		}
	}
	// A gap below MAX(id) must also be repaired on the next startup.
	if _, err := pool.Exec(ctx, `DELETE FROM movies WHERE id=2`); err != nil {
		t.Fatal(err)
	}
	if n, err := synchronizer.Sync(ctx); err != nil || n != 1 {
		t.Fatalf("repair gap: %d, %v", n, err)
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM movies`).Scan(&count); err != nil || count != 5 {
		t.Fatalf("duplicates: count=%d error=%v", count, err)
	}
	external.Close()
	callsBefore := calls.Load()
	service := usecase.New(repo, 3)
	handler := httpapi.New(service)
	for _, tc := range []struct {
		path string
		code int
	}{{"/last-movie", 200}, {"/load-movies?from=1&to=5", 200}, {"/load-movies?from=5&to=6", 207}, {"/load-movies?from=6&to=6", 404}} {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("GET", tc.path, nil))
		if w.Code != tc.code {
			t.Fatalf("HTTP %s: %d %s", tc.path, w.Code, w.Body.String())
		}
		if tc.path == "/last-movie" {
			var movie struct {
				ID       int64  `json:"id"`
				Title    string `json:"title"`
				Year     int32  `json:"year"`
				Director string `json:"director"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &movie); err != nil || movie.ID != 5 || movie.Title != "Film" || movie.Year != 2020 || movie.Director != "Director" {
				t.Fatalf("HTTP movie: %+v, %v", movie, err)
			}
		}
	}
	listener := bufconn.Listen(1 << 20)
	server := grpc.NewServer()
	moviepb.RegisterMovieServiceServer(server, grpcapi.New(service))
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)
	t.Cleanup(func() { _ = listener.Close() })
	connection, err := grpc.NewClient("passthrough:///movies", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return listener.DialContext(ctx) }))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = connection.Close() })
	client := moviepb.NewMovieServiceClient(connection)
	movie, err := client.GetLastMovie(ctx, &moviepb.GetLastMovieRequest{})
	if err != nil || movie.GetId() != 5 || movie.GetTitle() != "Film" || movie.GetYear() != 2020 || movie.GetDirector() != "Director" {
		t.Fatalf("gRPC movie: %v, %v", movie, err)
	}
	result, err := client.LoadMovies(ctx, &moviepb.LoadMoviesRequest{From: 1, To: 5})
	if err != nil || len(result.GetMovies()) != 5 {
		t.Fatalf("gRPC range: %v, %v", result, err)
	}
	for i, movie := range result.Movies {
		if movie.Id != int64(i+1) {
			t.Fatalf("ID order: %v", result)
		}
	}
	result, err = client.LoadMovies(ctx, &moviepb.LoadMoviesRequest{From: 5, To: 6})
	if err != nil || len(result.GetMovies()) != 1 || len(result.GetErrors()) != 1 || result.Errors[0].Code != string(domain.NotFound) {
		t.Fatalf("gRPC partial: %v, %v", result, err)
	}
	if _, err = client.LoadMovies(ctx, &moviepb.LoadMoviesRequest{From: 6, To: 6}); status.Code(err) != codes.NotFound || len(status.Convert(err).Details()) != 1 {
		t.Fatalf("gRPC missing: %v", err)
	}
	if calls.Load() != callsBefore {
		t.Fatal("API called the external stand")
	}
}
