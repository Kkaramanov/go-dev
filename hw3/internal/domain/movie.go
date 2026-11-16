package domain

import (
	"context"
	"errors"
)

type Movie struct {
	ID       int64
	Title    string
	Year     int32
	Director string
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
	ID      int64
	Code    Code
	Message string
	Cause   error
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

type Result struct {
	Movies []Movie
	Errors []*Error
}
