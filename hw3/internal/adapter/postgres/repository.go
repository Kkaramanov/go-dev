package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/Kkaramanov/go-dev/hw3/internal/domain"
	"github.com/Kkaramanov/go-dev/hw3/internal/usecase"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ pool *pgxpool.Pool }

var _ usecase.MovieRepository = (*Repository)(nil)

func New(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

func (r *Repository) GetMovie(ctx context.Context, id int64) (domain.Movie, error) {
	row := r.pool.QueryRow(ctx, `SELECT id,title,year,director FROM movies WHERE id=$1`, id)
	return scanMovie(row, id)
}

func (r *Repository) GetLastMovie(ctx context.Context) (domain.Movie, error) {
	row := r.pool.QueryRow(ctx, `SELECT id,title,year,director FROM movies ORDER BY id DESC LIMIT 1`)
	return scanMovie(row, 0)
}

func scanMovie(row pgx.Row, id int64) (domain.Movie, error) {
	var movie domain.Movie
	err := row.Scan(&movie.ID, &movie.Title, &movie.Year, &movie.Director)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Movie{}, domain.Failure(id, domain.NotFound, fmt.Sprintf("movie %d: not found", id), nil)
	}
	if err != nil {
		return domain.Movie{}, storageError(err)
	}
	return movie, nil
}

// MissingIDs returns gaps in a bounded range, not just IDs after MAX(id).
func (r *Repository) MissingIDs(ctx context.Context, from, to int64) ([]int64, error) {
	rows, err := r.pool.Query(ctx, `SELECT candidate.id
 FROM generate_series($1::bigint,$2::bigint) AS candidate(id)
 WHERE NOT EXISTS (SELECT 1 FROM movies WHERE movies.id=candidate.id)
 ORDER BY candidate.id`, from, to)
	if err != nil {
		return nil, storageError(err)
	}
	defer rows.Close()
	ids, err := pgx.CollectRows(rows, pgx.RowTo[int64])
	if err != nil {
		return nil, storageError(err)
	}
	return ids, nil
}

func (r *Repository) SaveMovies(ctx context.Context, movies []domain.Movie) error {
	if len(movies) == 0 {
		return ctx.Err()
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return storageError(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	batch := &pgx.Batch{}
	for _, movie := range movies {
		batch.Queue(`INSERT INTO movies (id,title,year,director) VALUES ($1,$2,$3,$4) ON CONFLICT (id) DO NOTHING`, movie.ID, movie.Title, movie.Year, movie.Director)
	}
	if err = tx.SendBatch(ctx, batch).Close(); err != nil {
		return storageError(err)
	}
	if err = tx.Commit(ctx); err != nil {
		return storageError(err)
	}
	return nil
}

func storageError(err error) error {
	code := domain.Unavailable
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		code = domain.Classify(err)
	}
	return domain.Failure(0, code, "movie storage unavailable", err)
}
