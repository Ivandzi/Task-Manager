package handler_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"taskservice.com/internal/handler"
	"taskservice.com/internal/mocks"
	"taskservice.com/internal/repository"
)

func TestCreateTask_WithRealJWT(t *testing.T) {
	jwtSecret := "super_secret_key"
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": 1,
		"exp":     time.Now().Add(time.Hour).Unix(),
	})
	tokenString, _ := token.SignedString([]byte(jwtSecret))
	mockServ := new(mocks.TaskService)
	mockServ.On("CreateTask", mock.Anything, mock.Anything).Return(nil)
	h := handler.NewHandler(mockServ)
	auth := handler.AuthMiddleware(jwtSecret)
	protectedHandler := auth(http.HandlerFunc(h.CreateTaskHandler))
	req := httptest.NewRequest(http.MethodPost, "/tasks", strings.NewReader(`{"title":"Test"}`))
	req.Header.Set("Authorization", "Bearer "+tokenString)
	w := httptest.NewRecorder()
	protectedHandler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)
	mockServ.AssertCalled(t, "CreateTask", mock.Anything, mock.MatchedBy(func(task *repository.Tasks) bool {
		return task.UserID == 1
	}))
}
