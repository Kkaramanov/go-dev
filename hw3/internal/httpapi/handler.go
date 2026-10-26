package httpapi

import (
	"embed"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/Kkaramanov/go-dev/hw3/internal/movie"
	swaggerFiles "github.com/swaggo/files/v2"
)

//go:embed docs/*
var docs embed.FS

func New(service movie.Service) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /load-movies", func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		from, errFrom := strconv.ParseInt(query.Get("from"), 10, 64)
		to, errTo := strconv.ParseInt(query.Get("to"), 10, 64)
		if errFrom != nil || errTo != nil || len(query["from"]) != 1 || len(query["to"]) != 1 {
			writeJSON(w, http.StatusBadRequest, movie.Failure(0, movie.InvalidArgument, "from and to must each be supplied once as integers", nil))
			return
		}
		if err := movie.ValidateRange(from, to); err != nil {
			writeError(w, err)
			return
		}
		result, err := service.LoadMovies(r.Context(), from, to)
		if err != nil {
			writeError(w, err)
			return
		}
		if len(result.Errors) == 0 {
			writeJSON(w, http.StatusOK, result.Movies)
			return
		}
		code := http.StatusMultiStatus
		if len(result.Movies) == 0 {
			code = statusCode(result.Errors[0].Code)
		}
		writeJSON(w, code, result)
	})
	mux.HandleFunc("GET /last-movie", func(w http.ResponseWriter, r *http.Request) {
		result, err := service.GetLastMovie(r.Context())
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
	})
	mux.HandleFunc("GET /swagger/index.html", func(w http.ResponseWriter, r *http.Request) {
		data, err := docs.ReadFile("docs/index.html")
		if err != nil {
			http.Error(w, "Swagger UI unavailable", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(data)
	})
	mux.HandleFunc("GET /swagger/openapi.json", func(w http.ResponseWriter, r *http.Request) {
		data, err := docs.ReadFile("docs/openapi.json")
		if err != nil {
			http.Error(w, "OpenAPI schema unavailable", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(data)
	})
	mux.Handle("GET /swagger/", http.StripPrefix("/swagger/", http.FileServer(http.FS(swaggerFiles.FS))))
	return mux
}

func writeError(w http.ResponseWriter, err error) {
	code := movie.Classify(err)
	writeJSON(w, statusCode(code), movie.Failure(0, code, err.Error(), nil))
}

func statusCode(code movie.Code) int {
	switch code {
	case movie.InvalidArgument:
		return http.StatusBadRequest
	case movie.NotFound:
		return http.StatusNotFound
	case movie.Timeout:
		return http.StatusGatewayTimeout
	case movie.Canceled:
		return http.StatusRequestTimeout
	case movie.RateLimited, movie.Unavailable:
		return http.StatusServiceUnavailable
	default:
		return http.StatusBadGateway
	}
}

func writeJSON(w http.ResponseWriter, code int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(data)
}
