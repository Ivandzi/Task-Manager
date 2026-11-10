package service

import (
	"context"
	"time"

	"authservice.com/internal/repository"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type UserRepository interface {
	CreateUser(ctx context.Context, email, passwordHash string) error
	GetUserByEmail(ctx context.Context, email string) (*repository.User, error)
}
type Service struct {
	Repo      UserRepository
	JWTSecret string
}

func NewService(repo UserRepository, jwtSecret string) *Service {
	return &Service{
		Repo:      repo,
		JWTSecret: jwtSecret,
	}
}
func (s *Service) Register(ctx context.Context, email, password string) error {
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	err = s.Repo.CreateUser(ctx, email, string(passwordHash))
	if err != nil {
		return err
	}
	return nil
}
func (s *Service) Login(ctx context.Context, email, password string) (string, error) {
	user, err := s.Repo.GetUserByEmail(ctx, email)
	if err != nil {
		return "", err
	}
	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	if err != nil {
		return "", err
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": user.ID,
		"role":    user.Role,
		"exp":     time.Now().Add(time.Hour * 24).Unix(),
	})
	tokenString, err := token.SignedString([]byte(s.JWTSecret))
	if err != nil {
		return "", err
	}
	return tokenString, err
}
