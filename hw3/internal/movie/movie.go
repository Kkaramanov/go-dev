package movie

import (
	"context"
	"errors"
	"fmt"
)

type Movie struct {
	ID       int64  `json:"id"`
	Title    string `json:"title"`
	Year     int32  `json:"year"`
	Director string `json:"director"`
}

type Code string

const (
	InvalidArgument Code = "invalid_argument"
	NotFound        Code = "not_found"
	RateLimited     Code = "rate_limited"
	UpstreamError   Code = "upstream_error"
	InvalidResponse Code = "invalid_response"
	Unavailable     Code = "unavailable"
	Timeout         Code = "timeout"
	Canceled        Code = "canceled"
)

type Error struct {
	ID      int64  `json:"id,omitempty"`
	Code    Code   `json:"code"`
	Message string `json:"message"`
	Cause   error  `json:"-"`
}

func (e *Error) Error() string { return e.Message }
func (e *Error) Unwrap() error { return e.Cause }

func Failure(id int64, code Code, message string, cause error) *Error {
	return &Error{ID: id, Code: code, Message: message, Cause: cause}
}

func Classify(err error) Code {
	if errors.Is(err, context.Canceled) {
		return Canceled
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return Timeout
	}
	var failure *Error
	if errors.As(err, &failure) {
		return failure.Code
	}
	return UpstreamError
}

const MaxRange = 10000

func ValidateRange(from, to int64) error {
	if from < 1 || to < from || to-from >= MaxRange {
		return Failure(0, InvalidArgument,
			fmt.Sprintf("from and to must be positive, from <= to, at most %d movies per request", MaxRange), nil)
	}
	return nil
}

type Result struct {
	Movies []Movie  `json:"movies"`
	Errors []*Error `json:"errors"`
}

type Service interface {
	LoadMovies(context.Context, int64, int64) (Result, error)
	GetLastMovie(context.Context) (Movie, error)
}
