package httpapi

import "github.com/Kkaramanov/go-dev/hw3/internal/domain"

type movieResponse struct {
	ID       int64  `json:"id"`
	Title    string `json:"title"`
	Year     int32  `json:"year"`
	Director string `json:"director"`
}

type errorResponse struct {
	ID      int64       `json:"id,omitempty"`
	Code    domain.Code `json:"code"`
	Message string      `json:"message"`
}

type loadResponse struct {
	Movies []movieResponse `json:"movies"`
	Errors []errorResponse `json:"errors"`
}

func toMovie(movie domain.Movie) movieResponse {
	return movieResponse{ID: movie.ID, Title: movie.Title, Year: movie.Year, Director: movie.Director}
}

func toMovies(movies []domain.Movie) []movieResponse {
	if movies == nil {
		return nil
	}
	result := make([]movieResponse, 0, len(movies))
	for _, movie := range movies {
		result = append(result, toMovie(movie))
	}
	return result
}

func toResult(result domain.Result) loadResponse {
	response := loadResponse{Movies: toMovies(result.Movies), Errors: make([]errorResponse, 0, len(result.Errors))}
	for _, failure := range result.Errors {
		response.Errors = append(response.Errors, errorResponse{ID: failure.ID, Code: failure.Code, Message: failure.Message})
	}
	return response
}
