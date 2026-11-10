package service_test

import (
	"context"
	"testing"

	"authservice.com/internal/mocks"
	"authservice.com/internal/repository"
	"authservice.com/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"golang.org/x/crypto/bcrypt"
)

func TestRegister(t *testing.T) {
	rawPassword := "super_secret_password"
	mockRepo := new(mocks.UserRepository)
	mockRepo.On("CreateUser", context.Background(), "test@mail.com", mock.Anything).Run(func(args mock.Arguments) {
		passwordHash := args.Get(2).(string)
		assert.NotEqual(t, rawPassword, passwordHash, "The password must not be transmitted in plain text!")
		err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(rawPassword))
		assert.NoError(t, err, "The passed password must be a valid bcrypt hash")

	}).
		Return(nil)
	serv := service.NewService(mockRepo, "secret")
	err := serv.Register(context.Background(), "test@mail.com", rawPassword)
	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}
func TestLogin(t *testing.T) {
	rawPassword := "super_secret_password"
	hash, _ := bcrypt.GenerateFromPassword([]byte(rawPassword), bcrypt.DefaultCost)
	mockRepo := new(mocks.UserRepository)
	mockRepo.On("GetUserByEmail", mock.Anything, "test@mail.com").Return(&repository.User{
		ID:           1,
		Email:        "test@mail.com",
		PasswordHash: string(hash),
		Role:         "user",
	}, nil)
	serv := service.NewService(mockRepo, "super_secret_jwt_key")
	tests := []struct {
		name        string
		password    string
		expectError bool
		expectToken bool
	}{
		{
			name:        "Success",
			password:    rawPassword,
			expectError: false,
			expectToken: true,
		},
		{
			name:        "Wrong Password",
			password:    "wrong_password",
			expectError: true,
			expectToken: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := serv.Login(context.Background(), "test@mail.com", tt.password)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			if tt.expectToken {
				assert.NotEmpty(t, token)
			} else {
				assert.Empty(t, token)
			}
		})
	}
}
