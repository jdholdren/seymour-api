package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/jdholdren/seymour/internal/seymour"
)

const promptNamespace = "-prompt"

func (r Repo) ActivePrompt(ctx context.Context) (*seymour.Prompt, error) {
	const q = `SELECT * FROM prompts WHERE active = 1 LIMIT 1;`

	var prompt seymour.Prompt
	err := r.db.GetContext(ctx, &prompt, q)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("error fetching active prompt: %w", err)
	}

	return &prompt, nil
}

func (r Repo) SetPrompt(ctx context.Context, content string) (seymour.Prompt, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return seymour.Prompt{}, fmt.Errorf("error beginning transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Deactivate all existing active prompts
	if _, err := tx.ExecContext(ctx, `UPDATE prompts SET active = 0 WHERE active = 1;`); err != nil {
		return seymour.Prompt{}, fmt.Errorf("error deactivating prompts: %w", err)
	}

	// Insert the new active prompt
	id := fmt.Sprintf("%s%s", uuid.NewString(), promptNamespace)
	const insertQ = `INSERT INTO prompts (id, content, active) VALUES (?, ?, 1);`
	if _, err := tx.ExecContext(ctx, insertQ, id, content); err != nil {
		return seymour.Prompt{}, fmt.Errorf("error inserting prompt: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return seymour.Prompt{}, fmt.Errorf("error committing transaction: %w", err)
	}

	// Fetch the newly created prompt
	const selectQ = `SELECT * FROM prompts WHERE id = ?;`
	var prompt seymour.Prompt
	if err := r.db.GetContext(ctx, &prompt, selectQ, id); err != nil {
		return seymour.Prompt{}, fmt.Errorf("error fetching created prompt: %w", err)
	}

	return prompt, nil
}
