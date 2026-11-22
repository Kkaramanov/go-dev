package grpcapi

import (
	"context"

	moviepb "github.com/Kkaramanov/go-dev/hw5/api"
	"github.com/Kkaramanov/go-dev/hw5/internal/domain"
	"github.com/Kkaramanov/go-dev/hw5/internal/usecase"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Server struct {
	moviepb.UnimplementedMovieServiceServer
	service usecase.MovieUseCase
}

func New(service usecase.MovieUseCase) *Server { return &Server{service: service} }

func (s *Server) LoadMovies(ctx context.Context, req *moviepb.LoadMoviesRequest) (*moviepb.LoadMoviesResponse, error) {
	result, err := s.service.LoadMovies(ctx, req.GetFrom(), req.GetTo())
	if err != nil {
		return nil, rpcError(err)
	}
	response := &moviepb.LoadMoviesResponse{}
	for _, item := range result.Movies {
		response.Movies = append(response.Movies, toProto(item))
	}
	for _, failure := range result.Errors {
		response.Errors = append(response.Errors, &moviepb.MovieError{
			Id: failure.ID, Code: string(failure.Code), Message: failure.Message,
		})
	}
	if len(result.Movies) == 0 && len(result.Errors) > 0 {
		// Preserve per-ID errors as structured status details even if all failed.
		st := status.New(rpcCode(result.Errors[0].Code), "no movies could be loaded from stand")
		withDetails, err := st.WithDetails(response)
		if err == nil {
			st = withDetails
		}
		return nil, st.Err()
	}
	return response, nil
}

func (s *Server) GetLastMovie(ctx context.Context, _ *moviepb.GetLastMovieRequest) (*moviepb.Movie, error) {
	result, err := s.service.GetLastMovie(ctx)
	if err != nil {
		return nil, rpcError(err)
	}
	return toProto(result), nil
}

func toProto(item domain.Movie) *moviepb.Movie {
	return &moviepb.Movie{Id: item.ID, Title: item.Title, Year: item.Year, Director: item.Director}
}

func rpcError(err error) error {
	return status.Error(rpcCode(domain.Classify(err)), err.Error())
}

func rpcCode(code domain.Code) codes.Code {
	switch code {
	case domain.InvalidArgument:
		return codes.InvalidArgument
	case domain.NotFound:
		return codes.NotFound
	case domain.Timeout:
		return codes.DeadlineExceeded
	case domain.Canceled:
		return codes.Canceled
	case domain.RateLimited:
		return codes.ResourceExhausted
	case domain.InvalidResponse:
		return codes.DataLoss
	default:
		return codes.Unavailable
	}
}
