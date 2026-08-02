package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"modernc.org/sqlite"

	"github.com/jdholdren/seymour/internal/seymour"
)

const (
	userNamespace      = "-usr"
	userLoginNamespace = "-lgn"
)

func (r Repo) User(ctx context.Context, id string) (seymour.User, error) {
	const q = `SELECT * FROM users WHERE id = ?;`

	var user seymour.User
	err := r.db.GetContext(ctx, &user, q, id)
	if errors.Is(err, sql.ErrNoRows) {
		return seymour.User{}, seymour.ErrNotFound
	}
	if err != nil {
		return seymour.User{}, fmt.Errorf("error fetching user: %s", err)
	}

	return user, nil
}

// userLoginByIdp fetches a user and their login by idp/idp id using the given queryer,
// so it can be reused both inside and outside of a transaction.
func userLoginByIdp(ctx context.Context, q sqlx.QueryerContext, idp seymour.Idp, idpID string) (seymour.User, seymour.UserLogin, error) {
	const loginQ = `SELECT * FROM user_logins WHERE idp = ? AND idp_id = ?;`

	var login seymour.UserLogin
	if err := sqlx.GetContext(ctx, q, &login, loginQ, idp, idpID); err != nil {
		return seymour.User{}, seymour.UserLogin{}, err
	}

	const userQ = `SELECT * FROM users WHERE id = ?;`
	var user seymour.User
	if err := sqlx.GetContext(ctx, q, &user, userQ, login.UserID); err != nil {
		return seymour.User{}, seymour.UserLogin{}, err
	}

	return user, login, nil
}

// EnsureUser ensures a user exists for the given idp/idp id, creating both the user and
// the login if this is the first time we've seen it.
func (r Repo) EnsureUser(ctx context.Context, idp seymour.Idp, idpID string) (seymour.User, seymour.UserLogin, error) {
	user, login, err := userLoginByIdp(ctx, r.db, idp, idpID)
	if err == nil {
		return user, login, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return seymour.User{}, seymour.UserLogin{}, fmt.Errorf("error fetching user login: %s", err)
	}

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return seymour.User{}, seymour.UserLogin{}, fmt.Errorf("error starting transaction: %s", err)
	}
	defer func() { _ = tx.Rollback() }()

	userID := fmt.Sprintf("%s%s", uuid.NewString(), userNamespace)
	const insertUserQ = `INSERT INTO users (id) VALUES (?);`
	if _, err := tx.ExecContext(ctx, insertUserQ, userID); err != nil {
		return seymour.User{}, seymour.UserLogin{}, fmt.Errorf("error inserting user: %s", err)
	}

	loginID := fmt.Sprintf("%s%s", uuid.NewString(), userLoginNamespace)
	const insertLoginQ = `INSERT INTO user_logins (id, user_id, idp, idp_id) VALUES (?, ?, ?, ?);`
	_, err = tx.ExecContext(ctx, insertLoginQ, loginID, userID, idp, idpID)
	if sqliteErr := (&sqlite.Error{}); errors.As(err, &sqliteErr) && sqliteErr.Code() == 2067 {
		// Someone else beat us to it: roll back our half-finished insert and read what they created.
		_ = tx.Rollback()
		return userLoginByIdp(ctx, r.db, idp, idpID)
	}
	if err != nil {
		return seymour.User{}, seymour.UserLogin{}, fmt.Errorf("error inserting user login: %s", err)
	}

	if err := tx.Commit(); err != nil {
		return seymour.User{}, seymour.UserLogin{}, fmt.Errorf("error committing transaction: %s", err)
	}

	return userLoginByIdp(ctx, r.db, idp, idpID)
}
