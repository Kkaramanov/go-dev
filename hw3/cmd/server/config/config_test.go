package config

import (
	"os"
	"testing"
)

func TestDefaultsAndEnvironmentOverrides(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://movies:movies@localhost:5432/movies")
	for _, key := range []string{"MOVIE_BASE_URL", "HTTP_ADDR", "GRPC_ADDR"} {
		t.Setenv(key, "")
		if err := os.Unsetenv(key); err != nil {
			t.Fatal(err)
		}
	}
	cfg, err := Load()
	if err != nil || cfg.MovieBaseURL != "https://homeworksite.site" || cfg.HTTPAddr != ":8080" || cfg.GRPCAddr != ":9090" || cfg.DatabaseURL != "postgres://movies:movies@localhost:5432/movies" {
		t.Fatalf("defaults: %+v %v", cfg, err)
	}
	t.Setenv("DATABASE_URL", "host=localhost port=5433 dbname=test user=test")
	t.Setenv("MOVIE_BASE_URL", "http://localhost:1234/stand/")
	t.Setenv("HTTP_ADDR", "127.0.0.1:8081")
	t.Setenv("GRPC_ADDR", "127.0.0.1:9091")
	cfg, err = Load()
	if err != nil || cfg.MovieBaseURL != "http://localhost:1234/stand/" || cfg.HTTPAddr != "127.0.0.1:8081" || cfg.GRPCAddr != "127.0.0.1:9091" || cfg.DatabaseURL != "host=localhost port=5433 dbname=test user=test" {
		t.Fatalf("overrides: %+v %v", cfg, err)
	}
}

func TestInvalidBaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/movies")
	for _, raw := range []string{"", "localhost:8000", "ftp://host", "https://", "https://host?x=1", "https://host/#fragment"} {
		t.Run(raw, func(t *testing.T) {
			t.Setenv("MOVIE_BASE_URL", raw)
			if _, err := Load(); err == nil {
				t.Fatalf("accepted invalid URL %q", raw)
			}
		})
	}
}

func TestMissingDatabaseURL(t *testing.T) {
	for _, value := range []string{"", "   "} {
		t.Setenv("DATABASE_URL", value)
		if _, err := Load(); err == nil {
			t.Fatal("accepted empty DATABASE_URL")
		}
	}
}
