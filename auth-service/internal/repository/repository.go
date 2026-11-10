package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	Pool *pgxpool.Pool
}
type User struct {
	ID           int       `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"password_hash"`
	Role         string    `json:"role"`
	CreatedAt    time.Time `json:"created_at"`
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{
		Pool: pool,
	}
}
func (r *Repository) CreateUser(ctx context.Context, email, passwordHash string) error {
	_, err := r.Pool.Exec(ctx,
		"INSERT INTO users (email, password_hash) VALUES ($1, $2)",
		email, passwordHash,
	)
	return err
}
func (r *Repository) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	var u User
	err := r.Pool.QueryRow(ctx, "SELECT id, email, password_hash, role FROM users WHERE email = $1", email).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Role)
	if err != nil {
		return nil, err
	}
	return &u, nil
}
