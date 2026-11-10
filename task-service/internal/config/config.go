package config

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
)

type Config struct {
	HTTPHost      string
	TaskPort      string
	DBUser        string
	DBPassword    string
	DBHost        string
	DBPort        string
	DBName        string
	SSLMode       string
	RedisHost     string
	RedisPort     string
	RedisPassword string
	JWTSecret     string
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
func Load() *Config {
	_ = godotenv.Load()
	dbUser := getEnv("DATABASE_LOGIN", "")
	if dbUser == "" {
		log.Fatal("DATABASE_LOGIN is not set")
	}
	dbPassword := getEnv("DATABASE_PASSWORD", "")
	if dbPassword == "" {
		log.Fatal("DATABASE_PASSWORD is not set")
	}
	jwtSecret := getEnv("JWT_SECRET", "")
	if jwtSecret == "" {
		log.Fatal("JWT_SECRET is not set")
	}
	return &Config{
		HTTPHost:      getEnv("HTTP_HOST", "0.0.0.0"),
		TaskPort:      getEnv("TASK_PORT", "8082"),
		DBUser:        dbUser,
		DBPassword:    dbPassword,
		DBHost:        getEnv("DATABASE_HOST", "localhost"),
		DBPort:        getEnv("DATABASE_PORT", "5432"),
		DBName:        getEnv("DATABASE_NAME", "task_manager"),
		SSLMode:       getEnv("DATABASE_SSLMODE", "disable"),
		RedisHost:     getEnv("REDIS_HOST", "localhost"),
		RedisPort:     getEnv("REDIS_PORT", "6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),
		JWTSecret:     jwtSecret,
	}
}
func (c *Config) NewPool(ctx context.Context) (*pgxpool.Pool, error) {
	conn := fmt.Sprintf("postgres://%v:%v@%v:%v/%v?sslmode=%v", c.DBUser, c.DBPassword, c.DBHost, c.DBPort, c.DBName, c.SSLMode)
	pool, err := pgxpool.New(ctx, conn)
	if err != nil {
		log.Printf("Unable to connect: %v", err)
		return nil, err
	}
	return pool, err
}
func (c *Config) NewRedis(ctx context.Context) *redis.Client {
	client := redis.NewClient(&redis.Options{
		Addr:     c.RedisHost + ":" + c.RedisPort,
		Password: c.RedisPassword,
		DB:       0,
	})
	return client
}
