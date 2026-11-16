package httpapi

import (
	"embed"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/Kkaramanov/go-dev/hw3/internal/domain"
	"github.com/Kkaramanov/go-dev/hw3/internal/usecase"
	swaggerFiles "github.com/swaggo/files/v2"
)

//go:embed docs/*
var docs embed.FS

func New(service usecase.MovieUseCase) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /load-movies", func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		from, errFrom := strconv.ParseInt(query.Get("from"), 10, 64)
		to, errTo := strconv.ParseInt(query.Get("to"), 10, 64)
		if errFrom != nil || errTo != nil || len(query["from"]) != 1 || len(query["to"]) != 1 {
			writeError(w, domain.Failure(0, domain.InvalidArgument, "from and to must each be supplied once as integers", nil))
			return
		}
		result, err := service.LoadMovies(r.Context(), from, to)
		if err != nil {
			writeError(w, err)
			return
		}
		if len(result.Errors) == 0 {
			writeJSON(w, http.StatusOK, toMovies(result.Movies))
			return
		}
		code := http.StatusMultiStatus
		if len(result.Movies) == 0 {
			code = statusCode(result.Errors[0].Code)
		}
		writeJSON(w, code, toResult(result))
	})
	mux.HandleFunc("GET /last-movie", func(w http.ResponseWriter, r *http.Request) {
		result, err := service.GetLastMovie(r.Context())
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, toMovie(result))
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
	code := domain.Classify(err)
	writeJSON(w, statusCode(code), errorResponse{Code: code, Message: err.Error()})
}

func statusCode(code domain.Code) int {
	switch code {
	case domain.InvalidArgument:
		return http.StatusBadRequest
	case domain.NotFound:
		return http.StatusNotFound
	case domain.Timeout:
		return http.StatusGatewayTimeout
	case domain.Canceled:
		return http.StatusRequestTimeout
	case domain.RateLimited, domain.Unavailable:
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
