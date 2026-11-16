package handler_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	moviepb "github.com/Kkaramanov/go-dev/hw3/api"
	"github.com/Kkaramanov/go-dev/hw3/internal/adapter/stand"
	grpcapi "github.com/Kkaramanov/go-dev/hw3/internal/handler/grpc"
	httpapi "github.com/Kkaramanov/go-dev/hw3/internal/handler/http"
	"github.com/Kkaramanov/go-dev/hw3/internal/usecase"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

func service(t *testing.T) *usecase.Service {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/1/info.0.json", "/2/info.0.json", "/3/info.0.json":
			var id int
			_, _ = fmt.Sscanf(r.URL.Path, "/%d/info.0.json", &id)
			_, _ = fmt.Fprintf(w, `{"id":%d,"title":"Film","year":2020,"director":"Director"}`, id)
		case "/5/info.0.json":
			w.WriteHeader(http.StatusTooManyRequests)
		case "/6/info.0.json":
			_, _ = w.Write([]byte(`{`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	return usecase.New(stand.New(server.URL, server.Client(), time.Second), 3)
}

func TestHTTPContract(t *testing.T) {
	handler := httpapi.New(service(t))
	for _, tc := range []struct {
		path string
		code int
		body string
	}{
		{"/load-movies?from=1&to=1", 200, `[{"id":1,"title":"Film","year":2020,"director":"Director"}]`},
		{"/last-movie", 200, `{"id":3,"title":"Film","year":2020,"director":"Director"}`},
		{"/load-movies?from=3&to=4", 207, `{"movies":[{"id":3,"title":"Film","year":2020,"director":"Director"}],"errors":[{"id":4,"code":"not_found","message":"movie 4: stand returned HTTP 404"}]}`},
		{"/load-movies?from=4&to=4", 404, `{"movies":[],"errors":[{"id":4,"code":"not_found","message":"movie 4: stand returned HTTP 404"}]}`},
		{"/load-movies?from=5&to=5", 503, `{"movies":[],"errors":[{"id":5,"code":"rate_limited","message":"movie 5: stand returned HTTP 429"}]}`},
		{"/load-movies?from=6&to=6", 502, `{"movies":[],"errors":[{"id":6,"code":"invalid_response","message":"movie 6: invalid movie JSON from stand"}]}`},
		{"/load-movies?from=0&to=1", 400, `{"code":"invalid_argument","message":"from and to must be positive, from <= to, at most 10000 movies per request"}`},
		{"/load-movies?from=1", 400, `{"code":"invalid_argument","message":"from and to must each be supplied once as integers"}`},
		{"/load-movies?from=1&from=2&to=3", 400, `{"code":"invalid_argument","message":"from and to must each be supplied once as integers"}`},
		{"/load-movies?from=x&to=3", 400, `{"code":"invalid_argument","message":"from and to must each be supplied once as integers"}`},
	} {
		t.Run(tc.path, func(t *testing.T) {
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, tc.path, nil))
			if w.Code != tc.code {
				t.Fatalf("status %d: %s", w.Code, w.Body.String())
			}
			if w.Header().Get("Content-Type") != "application/json; charset=utf-8" {
				t.Fatalf("content type: %s", w.Header())
			}
			var got, want any
			if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(tc.body), &want); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("got %s; want %s", w.Body.String(), tc.body)
			}
		})
	}
	for _, path := range []string{"/swagger/index.html", "/swagger/openapi.json", "/swagger/swagger-ui.css"} {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 200 || w.Body.Len() == 0 {
			t.Fatalf("swagger %s: %d", path, w.Code)
		}
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("POST", "/last-movie", nil))
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST: %d", w.Code)
	}
}

func TestGRPCContract(t *testing.T) {
	listener := bufconn.Listen(1 << 20)
	server := grpc.NewServer()
	moviepb.RegisterMovieServiceServer(server, grpcapi.New(service(t)))
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)
	t.Cleanup(func() { _ = listener.Close() })
	conn, err := grpc.NewClient("passthrough:///movies", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return listener.DialContext(ctx) }))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	client := moviepb.NewMovieServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	last, err := client.GetLastMovie(ctx, &moviepb.GetLastMovieRequest{})
	if err != nil || last.GetId() != 3 || last.GetTitle() != "Film" || last.GetYear() != 2020 || last.GetDirector() != "Director" {
		t.Fatalf("last: %v, %v", last, err)
	}
	for _, tc := range []struct {
		from, to         int64
		code             codes.Code
		movies, failures int
		failureID        int64
		failureCode      string
	}{
		{1, 3, codes.OK, 3, 0, 0, ""},
		{3, 4, codes.OK, 1, 1, 4, "not_found"},
		{4, 4, codes.NotFound, 0, 1, 4, "not_found"},
		{5, 5, codes.ResourceExhausted, 0, 1, 5, "rate_limited"},
		{6, 6, codes.DataLoss, 0, 1, 6, "invalid_response"},
		{0, 1, codes.InvalidArgument, 0, 0, 0, ""},
	} {
		t.Run(fmt.Sprintf("%d-%d", tc.from, tc.to), func(t *testing.T) {
			result, err := client.LoadMovies(ctx, &moviepb.LoadMoviesRequest{From: tc.from, To: tc.to})
			if status.Code(err) != tc.code {
				t.Fatalf("status: %v", err)
			}
			if tc.code == codes.InvalidArgument {
				return
			}
			if tc.code != codes.OK {
				details := status.Convert(err).Details()
				if len(details) != 1 {
					t.Fatalf("missing error details: %v", details)
				}
				var ok bool
				result, ok = details[0].(*moviepb.LoadMoviesResponse)
				if !ok {
					t.Fatalf("unexpected details: %T", details[0])
				}
			}
			if len(result.GetMovies()) != tc.movies || len(result.GetErrors()) != tc.failures {
				t.Fatalf("result: %v", result)
			}
			if tc.failures > 0 && (result.Errors[0].Id != tc.failureID || result.Errors[0].Code != tc.failureCode) {
				t.Fatalf("errors: %v", result.Errors)
			}
			for i, m := range result.Movies {
				if m.Id != tc.from+int64(i) {
					t.Fatalf("movie order: %v", result.Movies)
				}
			}
		})
	}
	canceled, stop := context.WithCancel(ctx)
	stop()
	if _, err := client.GetLastMovie(canceled, &moviepb.GetLastMovieRequest{}); status.Code(err) != codes.Canceled {
		t.Fatalf("cancellation: %v", err)
	}
}
