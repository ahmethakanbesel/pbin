// Package storage implements domain repository interfaces using SQLite.
package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/ahmethakanbesel/pbin/internal/domain/file"
)

// fileRepo implements file.Repository using a DBPair.
type fileRepo struct {
	db *DBPair
}

// NewFileRepo returns a fileRepo that satisfies file.Repository.
func NewFileRepo(db *DBPair) *fileRepo {
	return &fileRepo{db: db}
}

// Compile-time check: fileRepo satisfies file.Repository.
var _ file.Repository = (*fileRepo)(nil)

func (r *fileRepo) Create(ctx context.Context, f file.File, expiresAt *time.Time) error {
	var expiresAtUnix *int64
	if expiresAt != nil {
		v := expiresAt.Unix()
		expiresAtUnix = &v
	}

	_, err := r.db.WriteDB.ExecContext(ctx, `
		INSERT INTO files (slug, filename, size, mime_type, password, one_use, expires_at, delete_secret)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		f.Slug, f.Filename, f.Size, f.MimeType,
		nullableString(f.PasswordHash),
		boolToInt(f.OneUse),
		expiresAtUnix,
		nullableString(f.DeleteSecret),
	)
	if err != nil {
		return fmt.Errorf("file repo create: %w", err)
	}
	return nil
}

func (r *fileRepo) GetBySlug(ctx context.Context, slug string) (file.File, error) {
	row := r.db.ReadDB.QueryRowContext(ctx, `
		SELECT slug, filename, size, mime_type,
		       COALESCE(password, ''),
		       one_use,
		       expires_at,
		       COALESCE(delete_secret, '')
		FROM files WHERE slug = ?`, slug)

	var (
		f            file.File
		oneUse       int
		expiresAtRaw sql.NullInt64
	)
	if err := row.Scan(
		&f.Slug, &f.Filename, &f.Size, &f.MimeType,
		&f.PasswordHash,
		&oneUse,
		&expiresAtRaw,
		&f.DeleteSecret,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return file.File{}, file.ErrNotFound
		}
		return file.File{}, fmt.Errorf("file repo get: %w", err)
	}

	f.OneUse = oneUse == 1
	if expiresAtRaw.Valid {
		t := time.Unix(expiresAtRaw.Int64, 0).UTC()
		f.ExpiresAt = &t
	}

	return f, nil
}

func (r *fileRepo) MarkDownloaded(ctx context.Context, slug string) (bool, error) {
	res, err := r.db.WriteDB.ExecContext(ctx,
		`UPDATE files SET downloaded_at = unixepoch() WHERE slug = ? AND downloaded_at IS NULL`,
		slug)
	if err != nil {
		return false, fmt.Errorf("file repo mark downloaded: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("file repo mark downloaded rows: %w", err)
	}
	return n == 1, nil
}

func (r *fileRepo) Delete(ctx context.Context, slug string) error {
	_, err := r.db.WriteDB.ExecContext(ctx, `DELETE FROM files WHERE slug = ?`, slug)
	if err != nil {
		return fmt.Errorf("file repo delete: %w", err)
	}
	return nil
}

func (r *fileRepo) ListExpired(ctx context.Context) ([]file.File, error) {
	rows, err := r.db.ReadDB.QueryContext(ctx, `
		SELECT slug, filename, size, mime_type,
		       COALESCE(password, ''),
		       one_use, expires_at,
		       COALESCE(delete_secret, '')
		FROM files
		WHERE expires_at IS NOT NULL AND expires_at < unixepoch()`)
	if err != nil {
		return nil, fmt.Errorf("file repo list expired: %w", err)
	}
	defer rows.Close()

	var files []file.File
	for rows.Next() {
		var (
			f            file.File
			oneUse       int
			expiresAtRaw sql.NullInt64
		)
		if err := rows.Scan(
			&f.Slug, &f.Filename, &f.Size, &f.MimeType,
			&f.PasswordHash,
			&oneUse,
			&expiresAtRaw,
			&f.DeleteSecret,
		); err != nil {
			return nil, fmt.Errorf("file repo list expired scan: %w", err)
		}
		f.OneUse = oneUse == 1
		if expiresAtRaw.Valid {
			t := time.Unix(expiresAtRaw.Int64, 0).UTC()
			f.ExpiresAt = &t
		}
		files = append(files, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("file repo list expired rows: %w", err)
	}
	return files, nil
}

// nullableString converts an empty string to nil (SQL NULL) and non-empty to a string pointer.
func nullableString(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

// boolToInt converts bool to SQLite integer (0/1).
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
