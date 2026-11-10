package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	Pool *pgxpool.Pool
}
type Tasks struct {
	ID          int       `json:"id"`
	UserID      int       `json:"user_id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{Pool: pool}
}
func (r *Repository) CreateTask(ctx context.Context, tasks *Tasks) error {
	_, err := r.Pool.Exec(ctx,
		"INSERT INTO tasks (user_id, title, description) VALUES ($1, $2, $3)",
		tasks.UserID, tasks.Title, tasks.Description,
	)
	if err != nil {
		return err
	}
	return nil
}
func (r *Repository) GetTasksByUserID(ctx context.Context, userID int) ([]Tasks, error) {
	rows, err := r.Pool.Query(ctx,
		"SELECT id, user_id, title, description, status, created_at, updated_at FROM tasks WHERE user_id = $1",
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tasks []Tasks
	for rows.Next() {
		var t Tasks
		err = rows.Scan(&t.ID, &t.UserID, &t.Title, &t.Description, &t.Status, &t.CreatedAt, &t.UpdatedAt)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, nil
}
func (r *Repository) GetTasksByID(ctx context.Context, taskID int) (*Tasks, error) {
	var t Tasks
	err := r.Pool.QueryRow(ctx,
		"SELECT id, user_id, title, description, status, created_at, updated_at FROM tasks WHERE id = $1",
		taskID,
	).Scan(&t.ID, &t.UserID, &t.Title, &t.Description, &t.Status, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &t, nil
}
func (r *Repository) UpdateTask(ctx context.Context, task *Tasks) error {
	_, err := r.Pool.Exec(ctx,
		"UPDATE tasks SET title = $1, description = $2, status = $3, updated_at = NOW() WHERE id = $4",
		task.Title, task.Description, task.Status, task.ID,
	)
	if err != nil {
		return err
	}
	return nil
}
func (r *Repository) DeleteTask(ctx context.Context, taskID int) error {
	_, err := r.Pool.Exec(ctx,
		"DELETE FROM tasks WHERE id = $1",
		taskID,
	)
	if err != nil {
		return err
	}
	return nil
}
func (r *Repository) DeleteOldCompletedTasks(ctx context.Context) error {
	_, err := r.Pool.Exec(ctx,
		"DELETE FROM tasks WHERE status = 'completed' AND updated_at < NOW() - INTERVAL '1 day'",
	)
	return err
}
