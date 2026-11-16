package usecase_test

import (
	"context"
	"errors"
	"math"
	"reflect"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Kkaramanov/go-dev/hw3/internal/domain"
	"github.com/Kkaramanov/go-dev/hw3/internal/usecase"
)

type providerFunc func(context.Context, int64) (domain.Movie, error)

func (f providerFunc) GetMovie(ctx context.Context, id int64) (domain.Movie, error) {
	return f(ctx, id)
}

func TestLoadMoviesPreservesOrderAndPartialErrors(t *testing.T) {
	var active, peak atomic.Int32
	provider := providerFunc(func(ctx context.Context, id int64) (domain.Movie, error) {
		n := active.Add(1)
		defer active.Add(-1)
		for old := peak.Load(); n > old; old = peak.Load() {
			if peak.CompareAndSwap(old, n) {
				break
			}
		}
		if id == 2 {
			return domain.Movie{}, domain.Failure(id, domain.NotFound, "missing", nil)
		}
		return domain.Movie{ID: id}, nil
	})
	result, err := usecase.New(provider, 2).LoadMovies(context.Background(), 1, 4)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(result.Movies, []domain.Movie{{ID: 1}, {ID: 3}, {ID: 4}}) {
		t.Fatalf("movies: %+v", result.Movies)
	}
	if len(result.Errors) != 1 || result.Errors[0].ID != 2 || result.Errors[0].Code != domain.NotFound {
		t.Fatalf("errors: %+v", result.Errors)
	}
	if peak.Load() > 2 {
		t.Fatalf("worker limit exceeded: %d", peak.Load())
	}
}

func TestInvalidRangesDoNotCallProvider(t *testing.T) {
	provider := providerFunc(func(context.Context, int64) (domain.Movie, error) {
		t.Error("provider called for invalid input")
		return domain.Movie{}, nil
	})
	for _, r := range [][2]int64{{0, 1}, {1, 0}, {3, 2}, {1, 10001}, {math.MinInt64, math.MaxInt64}} {
		_, err := usecase.New(provider, 1).LoadMovies(context.Background(), r[0], r[1])
		if err == nil || domain.Classify(err) != domain.InvalidArgument {
			t.Fatalf("range %v: %v", r, err)
		}
	}
	for _, r := range [][2]int64{{1, 1}, {1, 10000}, {math.MaxInt64, math.MaxInt64}} {
		if err := usecase.ValidateRange(r[0], r[1]); err != nil {
			t.Fatalf("valid range %v: %v", r, err)
		}
	}
}

func TestGetLastMovie(t *testing.T) {
	for _, last := range []int64{0, 1, 2, 3, 100, math.MaxInt64} {
		t.Run(strconv.FormatInt(last, 10), func(t *testing.T) {
			calls := 0
			provider := providerFunc(func(ctx context.Context, id int64) (domain.Movie, error) {
				calls++
				if id > last {
					return domain.Movie{}, domain.Failure(id, domain.NotFound, "missing", nil)
				}
				return domain.Movie{ID: id}, nil
			})
			got, err := usecase.New(provider, 1).GetLastMovie(context.Background())
			if last == 0 {
				if err == nil || domain.Classify(err) != domain.NotFound {
					t.Fatalf("empty stand: %v", err)
				}
				return
			}
			if err != nil || got.ID != last {
				t.Fatalf("got %+v, %v; want %d", got, err, last)
			}
			if calls > 128 {
				t.Fatalf("search is not logarithmic: %d calls", calls)
			}
		})
	}
}

func TestGetLastMovieAbortsOnProviderFailure(t *testing.T) {
	for _, failingID := range []int64{1, 4, 6} {
		failure := domain.Failure(failingID, domain.RateLimited, "rate limited", nil)
		provider := providerFunc(func(ctx context.Context, id int64) (domain.Movie, error) {
			if id == failingID {
				return domain.Movie{}, failure
			}
			if id >= 8 {
				return domain.Movie{}, domain.Failure(id, domain.NotFound, "missing", nil)
			}
			return domain.Movie{ID: id}, nil
		})
		_, err := usecase.New(provider, 1).GetLastMovie(context.Background())
		if !errors.Is(err, failure) {
			t.Fatalf("failure at %d: %v", failingID, err)
		}
	}
}

func TestCancellationAndDeadline(t *testing.T) {
	provider := providerFunc(func(ctx context.Context, id int64) (domain.Movie, error) {
		<-ctx.Done()
		return domain.Movie{}, ctx.Err()
	})
	for _, deadline := range []bool{false, true} {
		ctx, cancel := context.WithCancel(context.Background())
		want := error(context.Canceled)
		if deadline {
			cancel()
			ctx, cancel = context.WithDeadline(context.Background(), time.Time{})
			want = context.DeadlineExceeded
		} else {
			cancel()
		}
		for _, workers := range []int{0, 2} {
			svc := usecase.New(provider, workers)
			if _, err := svc.LoadMovies(ctx, 1, 10); !errors.Is(err, want) {
				t.Fatalf("load: %v", err)
			}
			if _, err := svc.GetLastMovie(ctx); !errors.Is(err, want) {
				t.Fatalf("last: %v", err)
			}
		}
		cancel()
	}
}

func TestLoadMoviesCancellationInFlight(t *testing.T) {
	entered := make(chan struct{}, 1)
	provider := providerFunc(func(ctx context.Context, id int64) (domain.Movie, error) {
		select {
		case entered <- struct{}{}:
		default:
		}
		<-ctx.Done()
		return domain.Movie{}, ctx.Err()
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	finished := make(chan error, 1)
	go func() {
		_, err := usecase.New(provider, 2).LoadMovies(ctx, 1, 10)
		finished <- err
	}()
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("provider was never called")
	}
	cancel()
	select {
	case err := <-finished:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("got %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("load did not stop after cancellation")
	}
}
