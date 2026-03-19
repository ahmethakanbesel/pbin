package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/ahmethakanbesel/pbin/internal/domain/paste"
)

// pasteRepo implements paste.Repository using a DBPair.
type pasteRepo struct {
	db *DBPair
}

// NewPasteRepo returns a pasteRepo that satisfies paste.Repository.
func NewPasteRepo(db *DBPair) *pasteRepo {
	return &pasteRepo{db: db}
}

// Compile-time check: pasteRepo satisfies paste.Repository.
var _ paste.Repository = (*pasteRepo)(nil)

func (r *pasteRepo) Create(ctx context.Context, p paste.Paste, expiresAt *time.Time) error {
	var expiresAtUnix *int64
	if expiresAt != nil {
		v := expiresAt.Unix()
		expiresAtUnix = &v
	}

	_, err := r.db.WriteDB.ExecContext(ctx, `
		INSERT INTO pastes (slug, title, content, lang, password, one_use, expires_at, delete_secret)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		p.Slug, p.Title, p.Content, p.Lang,
		nullableString(p.PasswordHash),
		boolToInt(p.OneUse),
		expiresAtUnix,
		nullableString(p.DeleteSecret),
	)
	if err != nil {
		return fmt.Errorf("paste repo create: %w", err)
	}
	return nil
}

func (r *pasteRepo) GetBySlug(ctx context.Context, slug string) (paste.Paste, error) {
	row := r.db.ReadDB.QueryRowContext(ctx, `
		SELECT slug, title, content, lang,
		       COALESCE(password, ''),
		       one_use,
		       expires_at,
		       COALESCE(delete_secret, '')
		FROM pastes WHERE slug = ?`, slug)

	var (
		p            paste.Paste
		oneUse       int
		expiresAtRaw sql.NullInt64
	)
	if err := row.Scan(
		&p.Slug, &p.Title, &p.Content, &p.Lang,
		&p.PasswordHash,
		&oneUse,
		&expiresAtRaw,
		&p.DeleteSecret,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return paste.Paste{}, paste.ErrNotFound
		}
		return paste.Paste{}, fmt.Errorf("paste repo get: %w", err)
	}

	p.OneUse = oneUse == 1
	if expiresAtRaw.Valid {
		t := time.Unix(expiresAtRaw.Int64, 0).UTC()
		p.ExpiresAt = &t
	}

	return p, nil
}

func (r *pasteRepo) MarkViewed(ctx context.Context, slug string) (bool, error) {
	res, err := r.db.WriteDB.ExecContext(ctx,
		`UPDATE pastes SET viewed_at = unixepoch() WHERE slug = ? AND viewed_at IS NULL`,
		slug)
	if err != nil {
		return false, fmt.Errorf("paste repo mark viewed: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("paste repo mark viewed rows: %w", err)
	}
	return n == 1, nil
}

func (r *pasteRepo) Delete(ctx context.Context, slug string) error {
	_, err := r.db.WriteDB.ExecContext(ctx, `DELETE FROM pastes WHERE slug = ?`, slug)
	if err != nil {
		return fmt.Errorf("paste repo delete: %w", err)
	}
	return nil
}

func (r *pasteRepo) ListExpired(ctx context.Context) ([]paste.Paste, error) {
	rows, err := r.db.ReadDB.QueryContext(ctx, `
		SELECT slug, title, content, lang,
		       COALESCE(password, ''),
		       one_use, expires_at,
		       COALESCE(delete_secret, '')
		FROM pastes
		WHERE expires_at IS NOT NULL AND expires_at < unixepoch()`)
	if err != nil {
		return nil, fmt.Errorf("paste repo list expired: %w", err)
	}
	defer rows.Close()

	var pastes []paste.Paste
	for rows.Next() {
		var (
			p            paste.Paste
			oneUse       int
			expiresAtRaw sql.NullInt64
		)
		if err := rows.Scan(
			&p.Slug, &p.Title, &p.Content, &p.Lang,
			&p.PasswordHash,
			&oneUse,
			&expiresAtRaw,
			&p.DeleteSecret,
		); err != nil {
			return nil, fmt.Errorf("paste repo list expired scan: %w", err)
		}
		p.OneUse = oneUse == 1
		if expiresAtRaw.Valid {
			t := time.Unix(expiresAtRaw.Int64, 0).UTC()
			p.ExpiresAt = &t
		}
		pastes = append(pastes, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("paste repo list expired rows: %w", err)
	}
	return pastes, nil
}
