package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/kitbuilder587/fintech-bot/internal/domain"
)

type WorldModelRepo struct {
	db *DB
}

func NewWorldModelRepo(db *DB) *WorldModelRepo {
	return &WorldModelRepo{db: db}
}

func (r *WorldModelRepo) CreateFact(ctx context.Context, fact *domain.Fact) error {
	query := `
		INSERT INTO facts (id, user_id, content, source_url, confidence, extracted_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING extracted_at
	`

	err := r.db.Pool.QueryRow(ctx, query,
		fact.ID,
		fact.UserID,
		fact.Content,
		nullString(fact.SourceURL),
		fact.Confidence,
		fact.ExtractedAt,
	).Scan(&fact.ExtractedAt)

	if err != nil {
		if isDuplicateError(err) {
			return domain.ErrDuplicateSource
		}
		return fmt.Errorf("create fact: %w", err)
	}

	return nil
}

func (r *WorldModelRepo) GetFactsByUser(ctx context.Context, userID int64, limit int) ([]domain.Fact, error) {
	query := `
		SELECT id, user_id, content, source_url, confidence, extracted_at
		FROM facts
		WHERE user_id = $1
		ORDER BY extracted_at DESC
		LIMIT $2
	`

	rows, err := r.db.Pool.Query(ctx, query, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("get facts by user: %w", err)
	}
	defer rows.Close()

	return scanFacts(rows)
}

func (r *WorldModelRepo) GetFactsBySession(ctx context.Context, sessionID string) ([]domain.Fact, error) {
	query := `
		SELECT f.id, f.user_id, f.content, f.source_url, f.confidence, f.extracted_at
		FROM facts f
		JOIN session_facts sf ON f.id = sf.fact_id
		WHERE sf.session_id = $1
		ORDER BY f.extracted_at DESC
	`

	rows, err := r.db.Pool.Query(ctx, query, sessionID)
	if err != nil {
		return nil, fmt.Errorf("get facts by session: %w", err)
	}
	defer rows.Close()

	return scanFacts(rows)
}

func (r *WorldModelRepo) SearchFacts(ctx context.Context, userID int64, query string) ([]domain.Fact, error) {
	sqlQuery := `
		SELECT id, user_id, content, source_url, confidence, extracted_at
		FROM facts
		WHERE user_id = $1
		  AND to_tsvector('russian', content) @@ plainto_tsquery('russian', $2)
		ORDER BY ts_rank(to_tsvector('russian', content), plainto_tsquery('russian', $2)) DESC
	`

	rows, err := r.db.Pool.Query(ctx, sqlQuery, userID, query)
	if err != nil {
		return nil, fmt.Errorf("search facts: %w", err)
	}
	defer rows.Close()

	return scanFacts(rows)
}

func (r *WorldModelRepo) FindFactByContent(ctx context.Context, userID int64, content string) (*domain.Fact, error) {
	query := `
		SELECT id, user_id, content, source_url, confidence, extracted_at
		FROM facts
		WHERE user_id = $1 AND content = $2
	`

	var fact domain.Fact
	var sourceURL *string
	err := r.db.Pool.QueryRow(ctx, query, userID, content).Scan(
		&fact.ID,
		&fact.UserID,
		&fact.Content,
		&sourceURL,
		&fact.Confidence,
		&fact.ExtractedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("find fact by content: %w", err)
	}

	if sourceURL != nil {
		fact.SourceURL = *sourceURL
	}
	return &fact, nil
}

func (r *WorldModelRepo) CreateEntity(ctx context.Context, entity *domain.Entity) error {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	query := `
		INSERT INTO entities (id, user_id, name, type, first_seen_at, last_seen_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING first_seen_at, last_seen_at
	`

	err = tx.QueryRow(ctx, query,
		entity.ID,
		entity.UserID,
		entity.Name,
		entity.Type.String(),
		entity.FirstSeenAt,
		entity.LastSeenAt,
	).Scan(&entity.FirstSeenAt, &entity.LastSeenAt)

	if err != nil {
		if isDuplicateError(err) {
			return domain.ErrDuplicateSource
		}
		return fmt.Errorf("create entity: %w", err)
	}

	if err = insertEntityAttributes(ctx, tx, entity.ID, entity.Attributes); err != nil {
		return err
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

func (r *WorldModelRepo) GetEntityByName(ctx context.Context, userID int64, name string) (*domain.Entity, error) {
	query := `
		SELECT id, user_id, name, type, first_seen_at, last_seen_at
		FROM entities
		WHERE user_id = $1 AND name = $2
	`

	var entity domain.Entity
	var entityType string
	err := r.db.Pool.QueryRow(ctx, query, userID, name).Scan(
		&entity.ID,
		&entity.UserID,
		&entity.Name,
		&entityType,
		&entity.FirstSeenAt,
		&entity.LastSeenAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("get entity by name: %w", err)
	}

	entity.Type = domain.EntityType(entityType)

	entity.Attributes, err = r.loadEntityAttributes(ctx, entity.ID)
	if err != nil {
		return nil, err
	}

	return &entity, nil
}

func (r *WorldModelRepo) GetEntitiesByUser(ctx context.Context, userID int64) ([]domain.Entity, error) {
	query := `
		SELECT id, user_id, name, type, first_seen_at, last_seen_at
		FROM entities
		WHERE user_id = $1
		ORDER BY last_seen_at DESC
	`

	rows, err := r.db.Pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("get entities by user: %w", err)
	}
	defer rows.Close()

	var entities []domain.Entity
	for rows.Next() {
		var e domain.Entity
		var entityType string
		err := rows.Scan(
			&e.ID,
			&e.UserID,
			&e.Name,
			&entityType,
			&e.FirstSeenAt,
			&e.LastSeenAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan entity: %w", err)
		}
		e.Type = domain.EntityType(entityType)

		e.Attributes, err = r.loadEntityAttributes(ctx, e.ID)
		if err != nil {
			return nil, err
		}

		entities = append(entities, e)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return entities, nil
}

func (r *WorldModelRepo) UpdateEntity(ctx context.Context, entity *domain.Entity) error {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	query := `
		UPDATE entities
		SET name = $2, type = $3, last_seen_at = $4
		WHERE id = $1
	`

	result, err := tx.Exec(ctx, query, entity.ID, entity.Name, entity.Type.String(), entity.LastSeenAt)
	if err != nil {
		return fmt.Errorf("update entity: %w", err)
	}

	if result.RowsAffected() == 0 {
		return domain.ErrNotFound
	}

	_, err = tx.Exec(ctx, `DELETE FROM entity_attributes WHERE entity_id = $1`, entity.ID)
	if err != nil {
		return fmt.Errorf("delete entity attributes: %w", err)
	}

	if err = insertEntityAttributes(ctx, tx, entity.ID, entity.Attributes); err != nil {
		return err
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

func (r *WorldModelRepo) CreateSession(ctx context.Context, session *domain.ResearchSession) error {
	query := `
		INSERT INTO research_sessions (id, user_id, question, strategy, created_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING created_at
	`

	err := r.db.Pool.QueryRow(ctx, query,
		session.ID,
		session.UserID,
		session.Question,
		nullString(session.Strategy),
		session.CreatedAt,
	).Scan(&session.CreatedAt)

	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}

	return nil
}

func (r *WorldModelRepo) GetRecentSessions(ctx context.Context, userID int64, limit int) ([]domain.ResearchSession, error) {
	query := `
		SELECT id, user_id, question, strategy, created_at
		FROM research_sessions
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`

	rows, err := r.db.Pool.Query(ctx, query, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("get recent sessions: %w", err)
	}
	defer rows.Close()

	return scanSessions(rows)
}

func (r *WorldModelRepo) AddFactToSession(ctx context.Context, sessionID, factID string) error {
	query := `
		INSERT INTO session_facts (session_id, fact_id)
		VALUES ($1, $2)
		ON CONFLICT DO NOTHING
	`

	_, err := r.db.Pool.Exec(ctx, query, sessionID, factID)
	if err != nil {
		return fmt.Errorf("add fact to session: %w", err)
	}

	return nil
}

func (r *WorldModelRepo) AddEntityToSession(ctx context.Context, sessionID, entityID string) error {
	query := `
		INSERT INTO session_entities (session_id, entity_id)
		VALUES ($1, $2)
		ON CONFLICT DO NOTHING
	`

	_, err := r.db.Pool.Exec(ctx, query, sessionID, entityID)
	if err != nil {
		return fmt.Errorf("add entity to session: %w", err)
	}

	return nil
}

func (r *WorldModelRepo) loadEntityAttributes(ctx context.Context, entityID string) (map[string]string, error) {
	query := `SELECT key, value FROM entity_attributes WHERE entity_id = $1`

	rows, err := r.db.Pool.Query(ctx, query, entityID)
	if err != nil {
		return nil, fmt.Errorf("load entity attributes: %w", err)
	}
	defer rows.Close()

	attrs := make(map[string]string)
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, fmt.Errorf("scan attribute: %w", err)
		}
		attrs[key] = value
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return attrs, nil
}

func scanFacts(rows pgx.Rows) ([]domain.Fact, error) {
	var facts []domain.Fact
	for rows.Next() {
		var f domain.Fact
		var sourceURL *string
		err := rows.Scan(
			&f.ID,
			&f.UserID,
			&f.Content,
			&sourceURL,
			&f.Confidence,
			&f.ExtractedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan fact: %w", err)
		}
		if sourceURL != nil {
			f.SourceURL = *sourceURL
		}
		facts = append(facts, f)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return facts, nil
}

func nullString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// isDuplicateError checks if the error is a PostgreSQL unique constraint violation
func isDuplicateError(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func insertEntityAttributes(ctx context.Context, tx pgx.Tx, entityID string, attrs map[string]string) error {
	for key, value := range attrs {
		_, err := tx.Exec(ctx,
			`INSERT INTO entity_attributes (entity_id, key, value) VALUES ($1, $2, $3)`,
			entityID, key, value,
		)
		if err != nil {
			return fmt.Errorf("insert entity attribute: %w", err)
		}
	}
	return nil
}

func scanSessions(rows pgx.Rows) ([]domain.ResearchSession, error) {
	var sessions []domain.ResearchSession
	for rows.Next() {
		var s domain.ResearchSession
		var strategy *string
		err := rows.Scan(
			&s.ID,
			&s.UserID,
			&s.Question,
			&strategy,
			&s.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan session: %w", err)
		}
		if strategy != nil {
			s.Strategy = *strategy
		}
		sessions = append(sessions, s)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return sessions, nil
}
