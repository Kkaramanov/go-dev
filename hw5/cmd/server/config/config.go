package config

import (
	"fmt"
	"net/url"
	"os"
	"strings"
)

type Config struct {
	MovieBaseURL string
	HTTPAddr     string
	GRPCAddr     string
	DatabaseURL  string
}

func Load() (Config, error) {
	cfg := Config{
		DatabaseURL:  os.Getenv("DATABASE_URL"),
		MovieBaseURL: value("MOVIE_BASE_URL", "https://homeworksite.site"),
		HTTPAddr:     value("HTTP_ADDR", ":8080"),
		GRPCAddr:     value("GRPC_ADDR", ":9090"),
	}
	if strings.TrimSpace(cfg.DatabaseURL) == "" {
		return Config{}, fmt.Errorf("DATABASE_URL must not be empty")
	}
	u, err := url.Parse(cfg.MovieBaseURL)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.RawQuery != "" || u.Fragment != "" {
		return Config{}, fmt.Errorf("MOVIE_BASE_URL must be an absolute HTTP(S) URL without query or fragment")
	}
	if cfg.HTTPAddr == "" || cfg.GRPCAddr == "" {
		return Config{}, fmt.Errorf("HTTP_ADDR and GRPC_ADDR must not be empty")
	}
	return cfg, nil
}

func value(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}
