package credentials

import (
	"github.com/coding-cave-dev/nimbul/internal/db"
)

type Service struct {
	queries *db.Queries
}

func NewService(queries *db.Queries) *Service {
	return &Service{queries: queries}
}
