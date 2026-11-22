package usecase

import (
	"context"
	"sync"
	"time"

	"github.com/Kkaramanov/go-dev/hw5/internal/domain"
)

const RequestTimeout = 30 * time.Second

var _ MovieUseCase = (*Service)(nil)

type Service struct {
	repository MovieReader
	workers    int
}

func New(repository MovieReader, workers int) *Service {
	if workers < 1 {
		workers = 1
	}
	return &Service{repository: repository, workers: workers}
}

func (s *Service) LoadMovies(ctx context.Context, from, to int64) (domain.Result, error) {
	if err := ValidateRange(from, to); err != nil {
		return domain.Result{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, RequestTimeout)
	defer cancel()
	count := int(to-from) + 1
	type item struct {
		movie domain.Movie
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
				items[i].movie, items[i].err = s.repository.GetMovie(ctx, from+int64(i))
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
		return domain.Result{}, err
	}
	result := domain.Result{Movies: make([]domain.Movie, 0, count), Errors: make([]*domain.Error, 0)}
	for i, item := range items {
		if item.err != nil {
			result.Errors = append(result.Errors, domain.Failure(from+int64(i), domain.Classify(item.err), item.err.Error(), item.err))
		} else {
			result.Movies = append(result.Movies, item.movie)
		}
	}
	return result, nil
}

// GetLastMovie reads the highest stored ID without contacting the external stand.
func (s *Service) GetLastMovie(ctx context.Context) (domain.Movie, error) {
	ctx, cancel := context.WithTimeout(ctx, RequestTimeout)
	defer cancel()
	return s.repository.GetLastMovie(ctx)
}
