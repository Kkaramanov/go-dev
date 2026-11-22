package usecase

import (
	"context"

	"github.com/Kkaramanov/go-dev/hw5/internal/domain"
)

type MovieProvider interface {
	GetMovie(context.Context, int64) (domain.Movie, error)
}

type MovieReader interface {
	MovieProvider
	GetLastMovie(context.Context) (domain.Movie, error)
}

// MovieRepository adds the storage operations needed during startup synchronization.
type MovieRepository interface {
	MovieReader
	MissingIDs(context.Context, int64, int64) ([]int64, error)
	SaveMovies(context.Context, []domain.Movie) error
}

// MovieUseCase preserves per-movie failures when a range is partially loaded.
type MovieUseCase interface {
	LoadMovies(context.Context, int64, int64) (domain.Result, error)
	GetLastMovie(context.Context) (domain.Movie, error)
}
