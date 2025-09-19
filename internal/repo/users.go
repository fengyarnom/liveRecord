package repo

import (
	"context"
	"database/sql"
)

func (r *Repo) FindUserByUsername(ctx context.Context, username string) (int64, string, error) {
	row := r.DB.QueryRowContext(ctx, `SELECT id, password_hash FROM users WHERE username=$1 AND is_active=TRUE LIMIT 1`, username)
	var id int64
	var hash string
	switch err := row.Scan(&id, &hash); err {
	case nil:
		return id, hash, nil
	case sql.ErrNoRows:
		return 0, "", nil
	default:
		return 0, "", err
	}
}
