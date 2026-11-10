package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"

	"taskservice.com/internal/repository"
	"taskservice.com/internal/service"
)

type TaskService interface {
	CreateTask(ctx context.Context, tasks *repository.Tasks) error
	GetTasks(ctx context.Context, userID int) ([]repository.Tasks, error)
	GetTask(ctx context.Context, userID int, taskID int) (*repository.Tasks, error)
	UpdateTask(ctx context.Context, userID int, task *repository.Tasks) error
	DeleteTask(ctx context.Context, userID int, taskID int) error
	CleanupOldTasksWithRetry(ctx context.Context) error
	StartCleanupCron(ctx context.Context)
	StopCron() <-chan struct{}
}
type Handler struct {
	Serv TaskService
}
type TaskRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      string `json:"status"`
}

func NewHandler(serv TaskService) *Handler {
	return &Handler{Serv: serv}
}

// CreateTaskHandler godoc
// @Summary Create a new task
// @Description Creates a task for an authorized user. The default status is “pending”.
// @Tags tasks
// @Accept json
// @Produce json
// @Param request body TaskRequest true "Task data (title, description)"
// @Success 201 {string} string "Created"
// @Failure 400 {string} string "Bad Request (invalid JSON)"
// @Failure 401 {string} string "Unauthorized (no token)"
// @Failure 500 {string} string "Internal Server Error"
// @Security BearerAuth
// @Router /tasks [post]
func (h *Handler) CreateTaskHandler(w http.ResponseWriter, r *http.Request) {
	var t TaskRequest
	err := json.NewDecoder(r.Body).Decode(&t)
	if err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		log.Printf("Decoding error: %v\n", err)
		return
	}
	userID, ok := r.Context().Value("user_id").(int)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	task := &repository.Tasks{
		UserID:      userID,
		Title:       t.Title,
		Description: t.Description,
		Status:      "pending",
	}
	if err = h.Serv.CreateTask(r.Context(), task); err != nil {
		http.Error(w, "error 500: Internal Server Error", http.StatusInternalServerError)
		log.Printf("Database creation error: %v\n", err)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

// GetTasksHandler godoc
// @Summary Get a list of tasks
// @Description Returns a list of tasks for the current user (with caching in Redis).
// @Tags tasks
// @Produce json
// @Success 200 {array} repository.Tasks "List of tasks"
// @Failure 401 {string} string "Unauthorized (no token)"
// @Failure 500 {string} string "Internal Server Error"
// @Security BearerAuth
// @Router /tasks [get]
func (h *Handler) GetTasksHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value("user_id").(int)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	tasks, err := h.Serv.GetTasks(r.Context(), userID)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		log.Printf("Error in tasks retrieval: %v\n", err)
		return
	}
	if tasks == nil {
		tasks = []repository.Tasks{}
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(tasks)
	if err != nil {
		log.Printf("Encoding error: %v\n", err)
	}
}

// GetTaskHandler godoc
// @Summary Get a task by ID
// @Description Returns one task. Checks that the task belongs to the current user (IDOR protection).
// @Tags tasks
// @Produce json
// @Param id path int true "Task ID"
// @Success 200 {object} repository.Tasks "Задача"
// @Failure 400 {string} string "Bad Request (invalid ID)"
// @Failure 401 {string} string "Unauthorized (no token)"
// @Failure 403 {string} string "Forbidden (someone else’s task)"
// @Failure 404 {string} string "Not Found (the task was not found)"
// @Failure 500 {string} string "Internal Server Error"
// @Security BearerAuth
// @Router /tasks/{id} [get]
func (h *Handler) GetTaskHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value("user_id").(int)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	taskID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || taskID <= 0 {
		http.Error(w, "error converting", http.StatusBadRequest)
		log.Printf("Error converting string to number: %v\n", err)
		return
	}
	task, err := h.Serv.GetTask(r.Context(), userID, taskID)
	if err != nil {
		if errors.Is(err, service.ErrAccessDenied) {
			http.Error(w, "access denied", http.StatusForbidden)
			return
		}
		if errors.Is(err, service.ErrNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, "internal server error", http.StatusInternalServerError)
		log.Printf("Error while retrieving the task: %v\n", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(task)
	if err != nil {
		log.Printf("Encoding error: %v\n", err)
	}
}

// UpdateTaskHandler godoc
// @Summary Update the task
// @Description Updates the task. Checks permissions (IDOR protection). Flushes the cache in Redis.
// @Tags tasks
// @Accept json
// @Produce json
// @Param id path int true "Task ID"
// @Param request body TaskRequest true "New task data (title, description, status)"
// @Success 200 {string} string "OK"
// @Failure 400 {string} string "Bad Request (invalid ID or JSON)"
// @Failure 401 {string} string "Unauthorized (no token)"
// @Failure 403 {string} string "Forbidden (someone else’s task)"
// @Failure 404 {string} string "Not Found (the task was not found)"
// @Failure 500 {string} string "Internal Server Error"
// @Security BearerAuth
// @Router /tasks/{id} [put]
func (h *Handler) UpdateTaskHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value("user_id").(int)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	taskID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || taskID <= 0 {
		http.Error(w, "error converting", http.StatusBadRequest)
		log.Printf("Error converting string to number: %v\n", err)
		return
	}
	var t TaskRequest
	err = json.NewDecoder(r.Body).Decode(&t)
	if err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		log.Printf("Decoding error: %v\n", err)
		return
	}
	task := &repository.Tasks{
		ID:          taskID,
		UserID:      userID,
		Title:       t.Title,
		Description: t.Description,
		Status:      t.Status,
	}
	if err = h.Serv.UpdateTask(r.Context(), userID, task); err != nil {
		if errors.Is(err, service.ErrNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if errors.Is(err, service.ErrAccessDenied) {
			http.Error(w, "access denied", http.StatusForbidden)
			return
		}
		http.Error(w, "internal server error", http.StatusInternalServerError)
		log.Printf("Update error: %v\n", err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// DeleteTaskHandler godoc
// @Summary Delete the task
// @Description Deletes the task. Checks permissions (IDOR protection). Flushes the cache in Redis.
// @Tags tasks
// @Param id path int true "Task ID"
// @Success 204 {string} string "No Content"
// @Failure 400 {string} string "Bad Request (invalid ID)"
// @Failure 401 {string} string "Unauthorized (no token)"
// @Failure 403 {string} string "Forbidden (someone else’s task)"
// @Failure 404 {string} string "Not Found (the task was not found)"
// @Failure 500 {string} string "Internal Server Error"
// @Security BearerAuth
// @Router /tasks/{id} [delete]
func (h *Handler) DeleteTaskHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value("user_id").(int)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	taskID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || taskID <= 0 {
		http.Error(w, "error converting", http.StatusBadRequest)
		log.Printf("Error converting string to number: %v\n", err)
		return
	}
	if err = h.Serv.DeleteTask(r.Context(), userID, taskID); err != nil {
		if errors.Is(err, service.ErrNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if errors.Is(err, service.ErrAccessDenied) {
			http.Error(w, "access denied", http.StatusForbidden)
			return
		}
		http.Error(w, "internal server error", http.StatusInternalServerError)
		log.Printf("Delete error: %v\n", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
