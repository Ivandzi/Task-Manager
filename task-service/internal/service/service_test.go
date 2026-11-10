package service_test

import (
	"context"
	"testing"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"taskservice.com/internal/mocks"
	"taskservice.com/internal/repository"
	"taskservice.com/internal/service"
)

func TestUpdateTasks(t *testing.T) {
	mockRepo := new(mocks.TasksRepository)
	mockCache := new(mocks.CacheClient)
	existingTask := &repository.Tasks{
		ID:     1,
		UserID: 2,
		Title:  "Old title",
	}
	taskToUpdate := &repository.Tasks{
		ID:     1,
		UserID: 2,
		Title:  "New title",
	}
	mockRepo.On("GetTasksByID", context.Background(), 1).Return(existingTask, nil)
	mockRepo.On("UpdateTask", context.Background(), taskToUpdate).Return(nil)
	mockCache.On("Del", context.Background(), "tasks:user:2").Return(redis.NewIntResult(1, nil))
	serv := service.NewService(mockRepo, mockCache)
	err := serv.UpdateTask(context.Background(), 2, taskToUpdate)
	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
	mockCache.AssertExpectations(t)
}
func TestDeleteTask(t *testing.T) {
	mockRepo := new(mocks.TasksRepository)
	mockRepo.On("GetTasksByID", context.Background(), 1).Return(&repository.Tasks{ID: 1, UserID: 2}, nil)
	serv := service.NewService(mockRepo, nil)
	err := serv.DeleteTask(context.Background(), 99, 1)
	assert.ErrorIs(t, err, service.ErrAccessDenied)
	mockRepo.AssertNotCalled(t, "DeleteTask")
}
func TestGetTasks_WithCache(t *testing.T) {
	mockRepo := new(mocks.TasksRepository)
	mockCache := new(mocks.CacheClient)
	cachedJSON := `[{"id":1,"user_id":2,"title":"Cached Task"}]`
	mockCache.On("Get", context.Background(), "tasks:user:2").Return(redis.NewStringResult(cachedJSON, nil))
	serv := service.NewService(mockRepo, mockCache)
	tasks, err := serv.GetTasks(context.Background(), 2)
	assert.NoError(t, err)
	assert.Len(t, tasks, 1)
	assert.Equal(t, "Cached Task", tasks[0].Title)
	mockRepo.AssertNotCalled(t, "GetTasksByUserID")
}
func TestGetTasksEmptyList(t *testing.T) {
	mockRepo := new(mocks.TasksRepository)
	mockCache := new(mocks.CacheClient)
	mockCache.On("Get", context.Background(), "tasks:user:2").Return(redis.NewStringResult("", redis.Nil))
	mockRepo.On("GetTasksByUserID", context.Background(), 2).Return(nil, nil)
	mockCache.On("Set", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(redis.NewStatusResult("OK", nil))
	serv := service.NewService(mockRepo, mockCache)
	tasks, err := serv.GetTasks(context.Background(), 2)
	assert.NoError(t, err)
	assert.NotNil(t, tasks)
	assert.Len(t, tasks, 0)
}
