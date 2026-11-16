package usecase_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Kkaramanov/go-dev/hw3/internal/domain"
	"github.com/Kkaramanov/go-dev/hw3/internal/usecase"
)

type memoryRepository struct {
	movies              map[int64]domain.Movie
	missingErr, saveErr error
}

func (r *memoryRepository) GetMovie(ctx context.Context, id int64) (domain.Movie, error) {
	if m, ok := r.movies[id]; ok {
		return m, nil
	}
	return domain.Movie{}, domain.Failure(id, domain.NotFound, "not found", nil)
}
func (r *memoryRepository) GetLastMovie(ctx context.Context) (domain.Movie, error) {
	var last domain.Movie
	for id, m := range r.movies {
		if id > last.ID {
			last = m
		}
	}
	if last.ID == 0 {
		return domain.Movie{}, domain.Failure(0, domain.NotFound, "not found", nil)
	}
	return last, nil
}
func (r *memoryRepository) MissingIDs(ctx context.Context, from, to int64) ([]int64, error) {
	if r.missingErr != nil {
		return nil, r.missingErr
	}
	var ids []int64
	for id := from; id <= to; id++ {
		if _, ok := r.movies[id]; !ok {
			ids = append(ids, id)
		}
	}
	return ids, nil
}
func (r *memoryRepository) SaveMovies(ctx context.Context, movies []domain.Movie) error {
	if r.saveErr != nil {
		return r.saveErr
	}
	for _, m := range movies {
		r.movies[m.ID] = m
	}
	return nil
}

func TestSyncFillsGapsAndSkipsExisting(t *testing.T) {
	repository := &memoryRepository{movies: map[int64]domain.Movie{2: {ID: 2}, 7: {ID: 7}}}
	last := int64(10005)
	calls := map[int64]int{}
	var mu sync.Mutex
	source := providerFunc(func(ctx context.Context, id int64) (domain.Movie, error) {
		mu.Lock()
		calls[id]++
		mu.Unlock()
		if id > last {
			return domain.Movie{}, domain.Failure(id, domain.NotFound, "not found", nil)
		}
		return domain.Movie{ID: id, Title: fmt.Sprint(id), Year: 2020, Director: "Director"}, nil
	})
	syncer := usecase.NewSynchronizer(source, repository, 4)
	n, err := syncer.Sync(context.Background())
	if err != nil || n != 10003 || len(repository.movies) != 10005 {
		t.Fatalf("first sync: saved=%d total=%d error=%v", n, len(repository.movies), err)
	}
	if calls[7] != 0 {
		t.Fatalf("existing non-probe movie fetched: %d", calls[7])
	}
	n, err = syncer.Sync(context.Background())
	if err != nil || n != 0 {
		t.Fatalf("second sync: %d, %v", n, err)
	}
	last = 10007
	n, err = syncer.Sync(context.Background())
	if err != nil || n != 2 || len(repository.movies) != 10007 {
		t.Fatalf("new movies: %d, %v", n, err)
	}
}

func TestSyncPreservesProgressOnFailure(t *testing.T) {
	failure := errors.New("stand failed")
	fail := true
	source := providerFunc(func(ctx context.Context, id int64) (domain.Movie, error) {
		if id > 10 {
			return domain.Movie{}, domain.Failure(id, domain.NotFound, "not found", nil)
		}
		if id == 7 && fail {
			return domain.Movie{}, failure
		}
		return domain.Movie{ID: id}, nil
	})
	repository := &memoryRepository{movies: map[int64]domain.Movie{}}
	syncer := usecase.NewSynchronizer(source, repository, 3)
	n, err := syncer.Sync(context.Background())
	if !errors.Is(err, failure) || n != 9 || len(repository.movies) != 9 {
		t.Fatalf("partial sync: n=%d error=%v movies=%v", n, err, repository.movies)
	}
	fail = false
	n, err = syncer.Sync(context.Background())
	if err != nil || n != 1 || len(repository.movies) != 10 {
		t.Fatalf("retry: %d, %v", n, err)
	}
}

func TestSyncEmptyStandAndErrors(t *testing.T) {
	failure := errors.New("failure")
	for _, tc := range []struct {
		name                           string
		sourceErr, missingErr, saveErr error
		want                           error
	}{
		{name: "empty", sourceErr: domain.Failure(1, domain.NotFound, "empty", nil)},
		{name: "search error", sourceErr: failure, want: failure},
		{name: "read error", missingErr: failure, want: failure},
		{name: "write error", saveErr: failure, want: failure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repository := &memoryRepository{movies: map[int64]domain.Movie{}, missingErr: tc.missingErr, saveErr: tc.saveErr}
			source := providerFunc(func(ctx context.Context, id int64) (domain.Movie, error) {
				if tc.sourceErr != nil {
					return domain.Movie{}, tc.sourceErr
				}
				if id > 1 {
					return domain.Movie{}, domain.Failure(id, domain.NotFound, "missing", nil)
				}
				return domain.Movie{ID: id}, nil
			})
			_, err := usecase.NewSynchronizer(source, repository, 0).Sync(context.Background())
			if !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
		})
	}
}

func TestSyncCancellationInFlight(t *testing.T) {
	entered := make(chan struct{}, 1)
	source := providerFunc(func(ctx context.Context, id int64) (domain.Movie, error) {
		if id > 10 {
			return domain.Movie{}, domain.Failure(id, domain.NotFound, "missing", nil)
		}
		if id == 7 {
			entered <- struct{}{}
			<-ctx.Done()
			return domain.Movie{}, ctx.Err()
		}
		return domain.Movie{ID: id}, nil
	})
	repository := &memoryRepository{movies: map[int64]domain.Movie{}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	finished := make(chan error, 1)
	go func() { _, err := usecase.NewSynchronizer(source, repository, 2).Sync(ctx); finished <- err }()
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("sync did not begin loading")
	}
	cancel()
	select {
	case err := <-finished:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("got %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("sync did not stop on cancellation")
	}
}
