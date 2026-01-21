package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/kitbuilder587/fintech-bot/internal/domain"
)

type SourceRepo struct {
	db *DB
}

func NewSourceRepo(db *DB) *SourceRepo {
	return &SourceRepo{db: db}
}

func (r *SourceRepo) Create(ctx context.Context, source *domain.Source) error {
	query := `
        INSERT INTO sources (user_id, url, name, trust_level, is_user_added)
        VALUES ($1, $2, $3, $4, $5)
        RETURNING id, created_at
    `

	err := r.db.Pool.QueryRow(ctx, query,
		source.UserID,
		source.URL,
		source.Name,
		source.TrustLevel.String(),
		source.IsUserAdded,
	).Scan(&source.ID, &source.CreatedAt)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.ErrDuplicateSource
		}
		return fmt.Errorf("create source: %w", err)
	}

	return nil
}

func (r *SourceRepo) Delete(ctx context.Context, userID, sourceID int64) error {
	query := `DELETE FROM sources WHERE id = $1 AND user_id = $2`

	result, err := r.db.Pool.Exec(ctx, query, sourceID, userID)
	if err != nil {
		return fmt.Errorf("delete source: %w", err)
	}

	if result.RowsAffected() == 0 {
		return domain.ErrSourceNotFound
	}

	return nil
}

func (r *SourceRepo) ListByUser(ctx context.Context, userID int64) ([]domain.Source, error) {
	query := `
        SELECT id, user_id, url, name, trust_level, is_user_added, created_at
        FROM sources
        WHERE user_id = $1
        ORDER BY created_at DESC
    `

	rows, err := r.db.Pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("list sources: %w", err)
	}
	defer rows.Close()

	var sources []domain.Source
	for rows.Next() {
		var s domain.Source
		var trustLevel string
		err := rows.Scan(
			&s.ID,
			&s.UserID,
			&s.URL,
			&s.Name,
			&trustLevel,
			&s.IsUserAdded,
			&s.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan source: %w", err)
		}
		s.TrustLevel = domain.TrustLevel(trustLevel)
		sources = append(sources, s)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return sources, nil
}

func (r *SourceRepo) GetByID(ctx context.Context, sourceID int64) (*domain.Source, error) {
	query := `
        SELECT id, user_id, url, name, trust_level, is_user_added, created_at
        FROM sources
        WHERE id = $1
    `

	var s domain.Source
	var trustLevel string
	err := r.db.Pool.QueryRow(ctx, query, sourceID).Scan(
		&s.ID,
		&s.UserID,
		&s.URL,
		&s.Name,
		&trustLevel,
		&s.IsUserAdded,
		&s.CreatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrSourceNotFound
		}
		return nil, fmt.Errorf("get source: %w", err)
	}

	s.TrustLevel = domain.TrustLevel(trustLevel)
	return &s, nil
}

func (r *SourceRepo) ExistsByURL(ctx context.Context, userID int64, url string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM sources WHERE user_id = $1 AND url = $2)`

	var exists bool
	err := r.db.Pool.QueryRow(ctx, query, userID, url).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check source exists: %w", err)
	}

	return exists, nil
}

func (r *SourceRepo) CountByUser(ctx context.Context, userID int64) (int, error) {
	query := `SELECT COUNT(*) FROM sources WHERE user_id = $1`

	var count int
	err := r.db.Pool.QueryRow(ctx, query, userID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count sources: %w", err)
	}

	return count, nil
}

func (r *SourceRepo) GetDomainsByUserID(ctx context.Context, userID int64) ([]string, error) {
	query := `
        SELECT DISTINCT
            COALESCE(substring(url FROM '^https?://([^/]+)'), url) as domain
        FROM sources
        WHERE user_id = $1
    `

	rows, err := r.db.Pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("get domains: %w", err)
	}
	defer rows.Close()

	var domains []string
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			return nil, fmt.Errorf("scan domain: %w", err)
		}
		domains = append(domains, d)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return domains, nil
}

func (r *SourceRepo) UpdateTrustLevel(ctx context.Context, userID, sourceID int64, level domain.TrustLevel) error {
	query := `UPDATE sources SET trust_level = $1 WHERE id = $2 AND user_id = $3`

	result, err := r.db.Pool.Exec(ctx, query, string(level), sourceID, userID)
	if err != nil {
		return fmt.Errorf("update trust level: %w", err)
	}

	if result.RowsAffected() == 0 {
		return domain.ErrSourceNotFound
	}

	return nil
}
