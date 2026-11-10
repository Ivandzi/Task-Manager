package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
	"github.com/robfig/cron/v3"
	"taskservice.com/internal/repository"
)

type TasksRepository interface {
	CreateTask(ctx context.Context, tasks *repository.Tasks) error
	GetTasksByUserID(ctx context.Context, userID int) ([]repository.Tasks, error)
	GetTasksByID(ctx context.Context, taskID int) (*repository.Tasks, error)
	UpdateTask(ctx context.Context, task *repository.Tasks) error
	DeleteTask(ctx context.Context, taskID int) error
	DeleteOldCompletedTasks(ctx context.Context) error
}
type CacheClient interface {
	Get(ctx context.Context, key string) *redis.StringCmd
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd
	Del(ctx context.Context, keys ...string) *redis.IntCmd
}
type Service struct {
	Repo   TasksRepository
	Client CacheClient
	cron   *cron.Cron
}

func NewService(repo TasksRepository, client CacheClient) *Service {
	return &Service{
		Repo:   repo,
		Client: client,
	}
}

var ErrAccessDenied = errors.New("access denied")
var ErrNotFound = errors.New("task no found")

func (s *Service) CreateTask(ctx context.Context, tasks *repository.Tasks) error {
	err := s.Repo.CreateTask(ctx, tasks)
	if err != nil {
		return err
	}
	cacheKey := fmt.Sprintf("tasks:user:%v", tasks.UserID)
	err = s.Client.Del(ctx, cacheKey).Err()
	if err != nil {
		log.Printf("Redis del error: %v", err)
	}
	return nil
}
func (s *Service) GetTasks(ctx context.Context, userID int) ([]repository.Tasks, error) {
	cacheKey := fmt.Sprintf("tasks:user:%v", userID)
	var tasks []repository.Tasks
	val, err := s.Client.Get(ctx, cacheKey).Result()
	if err == nil {
		if err = json.Unmarshal([]byte(val), &tasks); err != nil {
			return nil, err
		}
		return tasks, nil
	}
	if err != redis.Nil {
		return nil, err
	}
	tasks, err = s.Repo.GetTasksByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if tasks == nil {
		tasks = []repository.Tasks{}
	}
	jsonData, err := json.Marshal(tasks)
	if err != nil {
		return nil, err
	}
	if err = s.Client.Set(ctx, cacheKey, jsonData, 10*time.Minute).Err(); err != nil {
		log.Printf("Redis set error: %v", err)
	}
	return tasks, nil
}
func (s *Service) GetTask(ctx context.Context, userID int, taskID int) (*repository.Tasks, error) {
	task, err := s.Repo.GetTasksByID(ctx, taskID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if task.UserID != userID {
		return nil, ErrAccessDenied
	}
	return task, nil
}
func (s *Service) UpdateTask(ctx context.Context, userID int, task *repository.Tasks) error {
	resolvedTask, err := s.Repo.GetTasksByID(ctx, task.ID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if resolvedTask.UserID != userID {
		return ErrAccessDenied
	}
	if task.Status == "" {
		task.Status = resolvedTask.Status
	}
	if err = s.Repo.UpdateTask(ctx, task); err != nil {
		return err
	}
	cacheKey := fmt.Sprintf("tasks:user:%v", task.UserID)
	err = s.Client.Del(ctx, cacheKey).Err()
	if err != nil {
		log.Printf("Redis set error: %v", err)
	}
	return nil
}
func (s *Service) DeleteTask(ctx context.Context, userID int, taskID int) error {
	resolvedTask, err := s.Repo.GetTasksByID(ctx, taskID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if resolvedTask.UserID != userID {
		return ErrAccessDenied
	}
	err = s.Repo.DeleteTask(ctx, taskID)
	if err != nil {
		return err
	}
	cacheKey := fmt.Sprintf("tasks:user:%v", userID)
	err = s.Client.Del(ctx, cacheKey).Err()
	if err != nil {
		log.Printf("Redis del error: %v", err)
	}
	return nil
}
func (s *Service) CleanupOldTasksWithRetry(ctx context.Context) error {
	const maxRetries = 3
	const retryDelay = 2 * time.Second
	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		newCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		err := s.Repo.DeleteOldCompletedTasks(newCtx)
		cancel()
		if err == nil {
			log.Printf("Cron: successfully cleaned up old tasks")
			return nil
		}
		lastErr = err
		log.Printf("Crone save failed (attempt %v/%v): %v", attempt, maxRetries, err)
		if attempt < maxRetries {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(retryDelay):
			}
		}
	}
	return fmt.Errorf("Cron failed after %v attempts: %v", maxRetries, lastErr)
}
func (s *Service) StartCleanupCron(ctx context.Context) {
	s.cron = cron.New(
		cron.WithChain(
			cron.Recover(cron.DefaultLogger),
			cron.SkipIfStillRunning(cron.DefaultLogger),
		),
	)
	_, err := s.cron.AddFunc("0 3 * * *", func() {
		log.Println("Cron: checking for old completed tasks...")
		if err := s.CleanupOldTasksWithRetry(ctx); err != nil {
			log.Printf("Cron error: failed to delete old tasks: %v", err)
		} else {
			log.Println("Cron: old completed tasks cleaned up successfully")
		}
	})
	if err != nil {
		log.Printf("Failed to add cron job: %v", err)
		return
	}
	s.cron.Start()
	log.Println("Cron: scheduler started")
	<-ctx.Done()
	log.Println("Cron: stopping scheduler...")
	stopCtx := s.cron.Stop()
	<-stopCtx.Done()
	log.Println("Cron: scheduler stopped gracefully")
}
func (s *Service) StopCron() <-chan struct{} {
	if s.cron != nil {
		return s.cron.Stop().Done()
	}
	ch := make(chan struct{})
	close(ch)
	return ch
}
