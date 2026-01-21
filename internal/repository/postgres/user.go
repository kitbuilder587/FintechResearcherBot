package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/kitbuilder587/fintech-bot/internal/domain"
)

type UserRepo struct {
	db *DB
}

func NewUserRepo(db *DB) *UserRepo {
	return &UserRepo{db: db}
}

func (r *UserRepo) GetOrCreate(ctx context.Context, telegramID int64, username string) (*domain.User, error) {
	query := `
        INSERT INTO users (id, username)
        VALUES ($1, $2)
        ON CONFLICT (id) DO UPDATE SET username = EXCLUDED.username
        RETURNING id, username, created_at
    `

	var user domain.User
	err := r.db.Pool.QueryRow(ctx, query, telegramID, username).Scan(
		&user.ID,
		&user.Username,
		&user.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get or create user: %w", err)
	}

	user.TelegramID = user.ID
	return &user, nil
}

func (r *UserRepo) GetByID(ctx context.Context, id int64) (*domain.User, error) {
	query := `SELECT id, username, created_at FROM users WHERE id = $1`

	var user domain.User
	err := r.db.Pool.QueryRow(ctx, query, id).Scan(
		&user.ID,
		&user.Username,
		&user.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrUserNotFound
		}
		return nil, fmt.Errorf("get user by id: %w", err)
	}

	user.TelegramID = user.ID
	return &user, nil
}

func (r *UserRepo) GetByTelegramID(ctx context.Context, telegramID int64) (*domain.User, error) {
	return r.GetByID(ctx, telegramID)
}

func (r *UserRepo) Update(ctx context.Context, user *domain.User) error {
	query := `UPDATE users SET username = $2 WHERE id = $1`

	result, err := r.db.Pool.Exec(ctx, query, user.ID, user.Username)
	if err != nil {
		return fmt.Errorf("update user: %w", err)
	}

	if result.RowsAffected() == 0 {
		return domain.ErrUserNotFound
	}

	return nil
}

func (r *UserRepo) Create(ctx context.Context, user *domain.User) error {
	query := `INSERT INTO users (id, username) VALUES ($1, $2) RETURNING created_at`

	err := r.db.Pool.QueryRow(ctx, query, user.TelegramID, user.Username).Scan(&user.CreatedAt)
	if err != nil {
		return fmt.Errorf("create user: %w", err)
	}

	user.ID = user.TelegramID
	return nil
}
