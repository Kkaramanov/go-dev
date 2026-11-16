package usecase

import (
	"context"
	"fmt"
	"sync"

	"github.com/Kkaramanov/go-dev/hw3/internal/domain"
)

const syncBatchSize int64 = 1000

type Synchronizer struct {
	source     MovieProvider
	repository MovieRepository
	workers    int
}

func NewSynchronizer(source MovieProvider, repository MovieRepository, workers int) *Synchronizer {
	return &Synchronizer{source: source, repository: repository, workers: max(1, workers)}
}

// Sync fills every missing ID, including gaps below the highest stored ID.
// Completed batches survive a failed attempt and are skipped on the next startup.
func (s *Synchronizer) Sync(ctx context.Context) (int, error) {
	last, err := FindLastMovie(ctx, s.source)
	if err != nil {
		if domain.Classify(err) == domain.NotFound {
			return 0, nil
		}
		return 0, fmt.Errorf("find last stand movie: %w", err)
	}
	saved := 0
	for from := int64(1); from <= last.ID; {
		to := from + min(syncBatchSize-1, last.ID-from)
		ids, err := s.repository.MissingIDs(ctx, from, to)
		if err != nil {
			return saved, fmt.Errorf("find missing movies: %w", err)
		}
		count, err := s.loadBatch(ctx, ids)
		saved += count
		if err != nil {
			return saved, err
		}
		if to == last.ID {
			break
		}
		from = to + 1
	}
	return saved, nil
}

func (s *Synchronizer) loadBatch(ctx context.Context, ids []int64) (int, error) {
	if len(ids) == 0 {
		return 0, ctx.Err()
	}
	type item struct {
		movie domain.Movie
		err   error
	}
	items := make([]item, len(ids))
	jobs := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < min(s.workers, len(ids)); w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				items[i].movie, items[i].err = s.source.GetMovie(ctx, ids[i])
			}
		}()
	}
dispatch:
	for i := range ids {
		select {
		case <-ctx.Done():
			break dispatch
		case jobs <- i:
		}
	}
	close(jobs)
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	movies := make([]domain.Movie, 0, len(ids))
	var firstErr error
	for i, item := range items {
		if item.err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("sync movie %d: %w", ids[i], item.err)
			}
		} else {
			movies = append(movies, item.movie)
		}
	}
	if len(movies) > 0 {
		if err := s.repository.SaveMovies(ctx, movies); err != nil {
			return 0, fmt.Errorf("save movies: %w", err)
		}
	}
	return len(movies), firstErr
}
