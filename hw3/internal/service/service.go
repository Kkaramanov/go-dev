package service

import (
	"context"
	"math"
	"sync"
	"time"

	"github.com/Kkaramanov/go-dev/hw3/internal/movie"
)

const RequestTimeout = 30 * time.Second

type Source interface {
	GetMovie(context.Context, int64) (movie.Movie, error)
}

type Service struct {
	source  Source
	workers int
}

func New(source Source, workers int) *Service {
	if workers < 1 {
		workers = 1
	}
	return &Service{source: source, workers: workers}
}

func (s *Service) LoadMovies(ctx context.Context, from, to int64) (movie.Result, error) {
	if err := movie.ValidateRange(from, to); err != nil {
		return movie.Result{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, RequestTimeout)
	defer cancel()
	count := int(to-from) + 1
	type item struct {
		movie movie.Movie
		err   error
	}
	items := make([]item, count)
	jobs := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < min(s.workers, count); w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				if ctx.Err() != nil {
					return
				}
				items[i].movie, items[i].err = s.source.GetMovie(ctx, from+int64(i))
			}
		}()
	}
dispatch:
	for i := 0; i < count; i++ {
		select {
		case <-ctx.Done():
			break dispatch
		case jobs <- i:
		}
	}
	close(jobs)
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return movie.Result{}, err
	}
	result := movie.Result{Movies: make([]movie.Movie, 0, count), Errors: make([]*movie.Error, 0)}
	for i, item := range items {
		if item.err != nil {
			result.Errors = append(result.Errors, movie.Failure(from+int64(i), movie.Classify(item.err), item.err.Error(), item.err))
		} else {
			result.Movies = append(result.Movies, item.movie)
		}
	}
	return result, nil
}

// GetLastMovie uses the guarantee that IDs are contiguous and only 404 means
// that the upper boundary has been reached. Other stand failures abort search.
func (s *Service) GetLastMovie(ctx context.Context) (movie.Movie, error) {
	ctx, cancel := context.WithTimeout(ctx, RequestTimeout)
	defer cancel()
	last, err := s.source.GetMovie(ctx, 1)
	if err != nil {
		return movie.Movie{}, err
	}
	low, high := int64(1), int64(2)
	for {
		candidate, err := s.source.GetMovie(ctx, high)
		if err != nil {
			if movie.Classify(err) == movie.NotFound {
				break
			}
			return movie.Movie{}, err
		}
		last, low = candidate, high
		if high == math.MaxInt64 {
			return last, nil
		}
		if high > math.MaxInt64/2 {
			high = math.MaxInt64
		} else {
			high *= 2
		}
	}
	for high-low > 1 {
		mid := low + (high-low)/2
		candidate, err := s.source.GetMovie(ctx, mid)
		if err != nil {
			if movie.Classify(err) != movie.NotFound {
				return movie.Movie{}, err
			}
			high = mid
		} else {
			low, last = mid, candidate
		}
	}
	return last, nil
}
