package usecase

import (
	"fmt"

	"github.com/Kkaramanov/go-dev/hw3/internal/domain"
)

const MaxRange = 10000

func ValidateRange(from, to int64) error {
	if from < 1 || to < from || to-from >= MaxRange {
		return domain.Failure(0, domain.InvalidArgument,
			fmt.Sprintf("from and to must be positive, from <= to, at most %d movies per request", MaxRange), nil)
	}
	return nil
}
