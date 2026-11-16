package postgres_test

import (
	"context"
	"reflect"
	"sync"
	"testing"

	"github.com/Kkaramanov/go-dev/hw3/internal/adapter/postgres"
	"github.com/Kkaramanov/go-dev/hw3/internal/domain"
	"github.com/Kkaramanov/go-dev/hw3/internal/testutil"
)

func TestRepositoryAndMigrations(t *testing.T) {
	pool := testutil.Postgres(t)
	ctx := context.Background()
	// Concurrent migration runners must apply the SQL file only once.
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := postgres.Migrate(ctx, pool); err != nil {
				t.Errorf("migration: %v", err)
			}
		}()
	}
	wg.Wait()
	var versions int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM schema_migrations`).Scan(&versions); err != nil || versions != 1 {
		t.Fatalf("versions=%d error=%v", versions, err)
	}
	repo := postgres.New(pool)
	if _, err := repo.GetLastMovie(ctx); err == nil || domain.Classify(err) != domain.NotFound {
		t.Fatalf("empty DB: %v", err)
	}
	movies := []domain.Movie{{ID: 1, Title: "Film", Year: 2020, Director: "Director"}, {ID: 4, Title: "Другой фильм", Year: 2021, Director: "Режиссёр"}}
	if err := repo.SaveMovies(ctx, movies); err != nil {
		t.Fatal(err)
	}
	if err := repo.SaveMovies(ctx, movies); err != nil {
		t.Fatalf("duplicate save: %v", err)
	}
	for _, want := range movies {
		got, err := repo.GetMovie(ctx, want.ID)
		if err != nil || got != want {
			t.Fatalf("movie: %+v, %v; want %+v", got, err, want)
		}
	}
	if got, err := repo.GetLastMovie(ctx); err != nil || got != movies[1] {
		t.Fatalf("last: %+v, %v", got, err)
	}
	if _, err := repo.GetMovie(ctx, 2); err == nil || domain.Classify(err) != domain.NotFound {
		t.Fatalf("missing: %v", err)
	}
	ids, err := repo.MissingIDs(ctx, 1, 5)
	if err != nil || !reflect.DeepEqual(ids, []int64{2, 3, 5}) {
		t.Fatalf("gaps: %v, %v", ids, err)
	}
	// All inserts in a failed batch roll back together.
	err = repo.SaveMovies(ctx, []domain.Movie{{ID: 2, Title: "Valid", Year: 2020, Director: "D"}, {ID: 3, Title: "Invalid", Year: 0, Director: "D"}})
	if err == nil {
		t.Fatal("invalid movie saved")
	}
	if _, err := repo.GetMovie(ctx, 2); err == nil || domain.Classify(err) != domain.NotFound {
		t.Fatalf("batch did not roll back: %v", err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := repo.GetMovie(canceled, 1); domain.Classify(err) != domain.Canceled {
		t.Fatalf("canceled: %v", err)
	}
	pool.Close()
	if _, err := repo.GetMovie(ctx, 1); domain.Classify(err) != domain.Unavailable {
		t.Fatalf("closed storage: %v", err)
	}
}
