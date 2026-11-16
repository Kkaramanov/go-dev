package usecase

import (
	"context"

	"github.com/Kkaramanov/go-dev/hw3/internal/domain"
	"math"
)

// FindLastMovie uses the guarantee that IDs are contiguous and only NotFound means
// that the upper boundary has been reached. Other stand failures abort search.
func FindLastMovie(ctx context.Context, source MovieProvider) (domain.Movie, error) {
	ctx, cancel := context.WithTimeout(ctx, RequestTimeout)
	defer cancel()
	last, err := source.GetMovie(ctx, 1)
	if err != nil {
		return domain.Movie{}, err
	}
	low, high := int64(1), int64(2)
	for {
		candidate, err := source.GetMovie(ctx, high)
		if err != nil {
			if domain.Classify(err) == domain.NotFound {
				break
			}
			return domain.Movie{}, err
		}
		last, low = candidate, high
		if high == math.MaxInt64 {
			return last, nil
		}
		if high > math.MaxInt64/2 {
			high = math.MaxInt64
		} else {
			high *= 2
		}
	}
	for high-low > 1 {
		mid := low + (high-low)/2
		candidate, err := source.GetMovie(ctx, mid)
		if err != nil {
			if domain.Classify(err) != domain.NotFound {
				return domain.Movie{}, err
			}
			high = mid
		} else {
			low, last = mid, candidate
		}
	}
	return last, nil
}
