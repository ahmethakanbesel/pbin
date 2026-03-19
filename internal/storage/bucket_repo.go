package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/ahmethakanbesel/pbin/internal/domain/bucket"
)

// bucketRepo implements bucket.Repository using a DBPair.
type bucketRepo struct {
	db *DBPair
}

// NewBucketRepo returns a bucketRepo that satisfies bucket.Repository.
func NewBucketRepo(db *DBPair) *bucketRepo {
	return &bucketRepo{db: db}
}

// Compile-time check: bucketRepo satisfies bucket.Repository.
var _ bucket.Repository = (*bucketRepo)(nil)

func (r *bucketRepo) Create(ctx context.Context, b bucket.Bucket, expiresAt *time.Time) error {
	var expiresAtUnix *int64
	if expiresAt != nil {
		v := expiresAt.Unix()
		expiresAtUnix = &v
	}

	_, err := r.db.WriteDB.ExecContext(ctx, `
		INSERT INTO buckets (slug, password, one_use, expires_at, delete_secret)
		VALUES (?, ?, ?, ?, ?)`,
		b.Slug,
		nullableString(b.PasswordHash),
		boolToInt(b.OneUse),
		expiresAtUnix,
		nullableString(b.DeleteSecret),
	)
	if err != nil {
		return fmt.Errorf("bucket repo create: %w", err)
	}
	return nil
}

func (r *bucketRepo) AddFile(ctx context.Context, bf bucket.BucketFile) error {
	_, err := r.db.WriteDB.ExecContext(ctx, `
		INSERT INTO bucket_files (bucket_slug, filename, size, mime_type, storage_key)
		VALUES (?, ?, ?, ?, ?)`,
		bf.BucketSlug, bf.Filename, bf.Size, bf.MimeType, bf.StorageKey,
	)
	if err != nil {
		return fmt.Errorf("bucket repo add file: %w", err)
	}
	return nil
}

func (r *bucketRepo) GetBySlug(ctx context.Context, slug string) (bucket.Bucket, error) {
	row := r.db.ReadDB.QueryRowContext(ctx, `
		SELECT slug, COALESCE(password, ''), one_use, expires_at, COALESCE(delete_secret, '')
		FROM buckets WHERE slug = ?`, slug)

	var (
		b            bucket.Bucket
		oneUseInt    int
		expiresAtRaw sql.NullInt64
	)
	if err := row.Scan(
		&b.Slug,
		&b.PasswordHash,
		&oneUseInt,
		&expiresAtRaw,
		&b.DeleteSecret,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return bucket.Bucket{}, bucket.ErrNotFound
		}
		return bucket.Bucket{}, fmt.Errorf("bucket repo get: %w", err)
	}

	b.OneUse = oneUseInt == 1
	if expiresAtRaw.Valid {
		t := time.Unix(expiresAtRaw.Int64, 0).UTC()
		b.ExpiresAt = &t
	}

	rows, err := r.db.ReadDB.QueryContext(ctx, `
		SELECT id, bucket_slug, filename, size, mime_type, storage_key
		FROM bucket_files WHERE bucket_slug = ? ORDER BY id`, slug)
	if err != nil {
		return bucket.Bucket{}, fmt.Errorf("bucket repo get files: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var bf bucket.BucketFile
		if err := rows.Scan(&bf.ID, &bf.BucketSlug, &bf.Filename, &bf.Size, &bf.MimeType, &bf.StorageKey); err != nil {
			return bucket.Bucket{}, fmt.Errorf("bucket repo get files scan: %w", err)
		}
		b.Files = append(b.Files, bf)
	}
	if err := rows.Err(); err != nil {
		return bucket.Bucket{}, fmt.Errorf("bucket repo get files rows: %w", err)
	}

	return b, nil
}

func (r *bucketRepo) MarkDownloaded(ctx context.Context, slug string) (bool, error) {
	res, err := r.db.WriteDB.ExecContext(ctx,
		`UPDATE buckets SET downloaded_at = unixepoch() WHERE slug = ? AND downloaded_at IS NULL`,
		slug)
	if err != nil {
		return false, fmt.Errorf("bucket repo mark downloaded: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("bucket repo mark downloaded rows: %w", err)
	}
	return n == 1, nil
}

func (r *bucketRepo) Delete(ctx context.Context, slug string) error {
	_, err := r.db.WriteDB.ExecContext(ctx, `DELETE FROM buckets WHERE slug = ?`, slug)
	if err != nil {
		return fmt.Errorf("bucket repo delete: %w", err)
	}
	return nil
}

func (r *bucketRepo) ListExpired(ctx context.Context) ([]bucket.Bucket, error) {
	rows, err := r.db.ReadDB.QueryContext(ctx, `
		SELECT slug, COALESCE(password, ''), one_use, expires_at, COALESCE(delete_secret, '')
		FROM buckets
		WHERE expires_at IS NOT NULL AND expires_at < unixepoch()`)
	if err != nil {
		return nil, fmt.Errorf("bucket repo list expired: %w", err)
	}
	defer rows.Close()

	var buckets []bucket.Bucket
	for rows.Next() {
		var (
			b            bucket.Bucket
			oneUseInt    int
			expiresAtRaw sql.NullInt64
		)
		if err := rows.Scan(
			&b.Slug,
			&b.PasswordHash,
			&oneUseInt,
			&expiresAtRaw,
			&b.DeleteSecret,
		); err != nil {
			return nil, fmt.Errorf("bucket repo list expired scan: %w", err)
		}
		b.OneUse = oneUseInt == 1
		if expiresAtRaw.Valid {
			t := time.Unix(expiresAtRaw.Int64, 0).UTC()
			b.ExpiresAt = &t
		}

		// Fetch files for each expired bucket.
		fileRows, err := r.db.ReadDB.QueryContext(ctx, `
			SELECT id, bucket_slug, filename, size, mime_type, storage_key
			FROM bucket_files WHERE bucket_slug = ? ORDER BY id`, b.Slug)
		if err != nil {
			return nil, fmt.Errorf("bucket repo list expired files: %w", err)
		}
		for fileRows.Next() {
			var bf bucket.BucketFile
			if err := fileRows.Scan(&bf.ID, &bf.BucketSlug, &bf.Filename, &bf.Size, &bf.MimeType, &bf.StorageKey); err != nil {
				fileRows.Close()
				return nil, fmt.Errorf("bucket repo list expired files scan: %w", err)
			}
			b.Files = append(b.Files, bf)
		}
		fileRows.Close()
		if err := fileRows.Err(); err != nil {
			return nil, fmt.Errorf("bucket repo list expired files rows: %w", err)
		}

		buckets = append(buckets, b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("bucket repo list expired rows: %w", err)
	}
	return buckets, nil
}
