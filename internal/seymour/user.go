package seymour

import (
	"context"
	"database/sql"
	"time"
)

// User holds the mainly the id of the account interacting with the app.
//
// It has possibly many UserLogins that tie it to multiple ways of signing in,
// although we only support github for now.
type User struct {
	ID             string    `db:"id"`
	PreferredName  *string   `db:"preferred_name"`
	TimelinePrompt *string   `db:"timeline_prompt"`
	CreatedAt      time.Time `db:"created_at"`
	UpdatedAt      time.Time `db:"updated_at"`
}

// UpdateUserArgs contains the user fields to update. Nil fields are left unchanged;
// non-nil invalid values clear the field, while valid values (including empty strings) are stored.
type UpdateUserArgs struct {
	PreferredName  *sql.NullString
	TimelinePrompt *sql.NullString
}

// Idp is an enum of the supported identity providers for login.
type Idp string

// UserLogin attaches a user to an IDP such as github.
type UserLogin struct {
	ID     string `db:"id"`      // The id of the row for a deletion, for example.
	UserID string `db:"user_id"` // The user this login belongs to.
	Idp    Idp    `db:"idp"`     // The source of the login.
	IdpID  string `db:"idp_id"`  // The identifier of the user in the Idp's system.

	LastLogin time.Time `db:"last_login"`
	CreatedAt time.Time `db:"created_at"`
}

type UserService interface {
	// Returns a user by their primary ID.
	User(ctx context.Context, id string) (User, error)
	// Ensures that a user is created based on the given user login details.
	//
	// If the user login is already found, the found login is just returned with the user (no modification).
	// Otherwise, both are created and returned.
	EnsureUser(ctx context.Context, idp Idp, id string) (User, UserLogin, error)
	// Updates the supplied user fields. Nil values are left unchanged, invalid nullable values clear the field,
	// and valid values (including empty strings) are stored.
	UpdateUser(ctx context.Context, id string, args UpdateUserArgs) error
}
