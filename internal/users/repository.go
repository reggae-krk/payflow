package users

import (
	"context"
	"errors"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/reggae-krk/payflow/internal/db"
)

var ErrEmailTaken = errors.New("email already registered")
var ErrNoRows = errors.New("no rows in result set")
var ErrNoRowToUpdate = errors.New("no row found to update")

type UserRepository interface {
    CreateUser(ctx context.Context, email, passwordHash string) (*User, error)
    GetByID(ctx context.Context, id int64) (*User, error)
    GetByEmail(ctx context.Context, email string) (*User, error)
    UpdateEmail(ctx context.Context, id int64, newEmail string) error
    UpdatePassword(ctx context.Context, id int64, newPasswordHash string) error
    DeleteByID(ctx context.Context, id int64) error
}

type userRepository struct {
	db db.Querier
}

func NewUserRepository(database db.Querier) *userRepository {
	return &userRepository{db: database}
}

func (userRepo *userRepository) CreateUser(ctx context.Context, email, passwordHash string) (*User, error) {
	var user User

	query := `
			INSERT INTO users (email, password_hash)
			VALUES ($1, $2)
			RETURNING id, email, password_hash, created_at
	`

	err := userRepo.db.QueryRow(ctx, query, email, passwordHash).Scan(&user.Id, &user.Email, &user.PasswordHash,
		&user.CreatedAt)

	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrEmailTaken
		}
		return nil, err
	}
	return &user, nil
}

func (userRepo *userRepository) GetByID(ctx context.Context, id int64) (*User, error) {
	var user User

	query := `
		SELECT id, email, password_hash, created_at FROM users WHERE id = $1
	`

	err := userRepo.db.QueryRow(ctx, query, id).Scan(&user.Id, &user.Email, &user.PasswordHash,
		&user.CreatedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNoRows
		}
		return nil, err
	}
	return &user, nil
}

func (userRepo *userRepository) GetByEmail(ctx context.Context, email string) (*User, error) {
	var user User

	query := `
		SELECT id, email, password_hash, created_at FROM users WHERE email = $1
	`

	err := userRepo.db.QueryRow(ctx, query, email).Scan(&user.Id, &user.Email, &user.PasswordHash,
		&user.CreatedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNoRows
		}
		return nil, err
	}
	return &user, nil
}

func (userRepo *userRepository) UpdatePassword(ctx context.Context, id int64, newPasswordHash string) error {
	query := `
		UPDATE users SET password_hash = $1 WHERE id = $2
	`

	cmdTag, err := userRepo.db.Exec(ctx, query, newPasswordHash, id)
	if err != nil {
		return err
	}

	if cmdTag.RowsAffected() == 0 {
		return ErrNoRowToUpdate
	}

	return nil
}

func (userRepo *userRepository) UpdateEmail(ctx context.Context, id int64, newEmail string) error {
	existingUser, err := userRepo.GetByEmail(ctx, newEmail)

	if err != nil {
		if !errors.Is(err, ErrNoRows) {
			return err
		}
	} else if existingUser != nil {
		return ErrEmailTaken
	}

	query := `
		UPDATE users SET email = $1 WHERE id = $2
	`

	cmdTag, err := userRepo.db.Exec(ctx, query, newEmail, id)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrEmailTaken
		}
		return err
	}

	if cmdTag.RowsAffected() == 0 {
		return ErrNoRowToUpdate
	}

	return nil
}

func (userRepo *userRepository) DeleteByID(ctx context.Context, id int64) error {
	query := `
		DELETE FROM users WHERE id = $1
	`
	cmdTag, err := userRepo.db.Exec(ctx, query, id)
	if err != nil {
		return err
	}

	if cmdTag.RowsAffected() == 0 {
		return ErrNoRows
	}

	return nil
}

func isUniqueViolation(err error) bool {
    var pgErr *pgconn.PgError
    if !errors.As(err, &pgErr) {
        return false
    }
    return pgErr.Code == pgerrcode.UniqueViolation
}
