package mysql

import (
	"context"

	"github.com/jdholdren/seymour/internal/seymour"
)

func (r Repo) User(ctx context.Context, id string) (seymour.User, error) {
	return seymour.User{}, ErrNotImplemented
}

func (r Repo) EnsureUser(ctx context.Context, idp seymour.Idp, id string) (seymour.User, seymour.UserLogin, error) {
	return seymour.User{}, seymour.UserLogin{}, ErrNotImplemented
}
