package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"os/signal"
	"syscall"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	httpSwagger "github.com/swaggo/http-swagger"
	_ "taskservice.com/docs"
	"taskservice.com/internal/config"
	"taskservice.com/internal/handler"
	"taskservice.com/internal/repository"
	"taskservice.com/internal/service"
)

// @title Task Service API
// @version 1.0
// @description API for task management. Requires a JWT token from Auth Service.
// @host localhost:8082
// @BasePath /
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
func main() {
	cfg := config.Load()
	httpHost := cfg.HTTPHost
	httpPort := cfg.TaskPort
	pool, err := cfg.NewPool(context.Background())
	if err != nil {
		log.Fatalf("Failed to create pool: %v", err)
	}
	defer pool.Close()
	if err := pool.Ping(context.Background()); err != nil {
		log.Fatalf("Database is not responding: %v", err)
	}
	log.Println("Successfully connected to PostgreSQL")
	repo := repository.NewRepository(pool)
	client := cfg.NewRedis(context.Background())
	defer client.Close()
	_, err = client.Ping(context.Background()).Result()
	if err != nil {
		log.Fatalf("Error connecting to redis: %v", err)
	}
	log.Println("Successfully connected to Redis")
	serv := service.NewService(repo, client)
	handl := handler.NewHandler(serv)
	portFlag := flag.String("httpPort", httpPort, "server port")
	flag.Parse()
	mux := http.NewServeMux()
	auth := handler.AuthMiddleware(cfg.JWTSecret)
	mux.Handle("POST /tasks", handler.MetricsMiddleware(auth(http.HandlerFunc(handl.CreateTaskHandler))))
	mux.Handle("GET /tasks", handler.MetricsMiddleware(auth(http.HandlerFunc(handl.GetTasksHandler))))
	mux.Handle("GET /tasks/{id}", handler.MetricsMiddleware(auth(http.HandlerFunc(handl.GetTaskHandler))))
	mux.Handle("PUT /tasks/{id}", handler.MetricsMiddleware(auth(http.HandlerFunc(handl.UpdateTaskHandler))))
	mux.Handle("DELETE /tasks/{id}", handler.MetricsMiddleware(auth(http.HandlerFunc(handl.DeleteTaskHandler))))
	mux.Handle("GET /metrics", promhttp.Handler())
	mux.Handle("GET /swagger/", httpSwagger.WrapHandler)
	addr := fmt.Sprintf("%v:%v", httpHost, *portFlag)
	srv := &http.Server{
		Addr:    addr,
		Handler: mux,
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go serv.StartCleanupCron(ctx)
	go func() {
		log.Printf("The server is running on: %v\n", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Server error: %v\n", err)
		}
	}()
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan
	log.Println("Shutting down server...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server forced to shutdown: %v\n", err)
	}
	cancel()
	log.Println("Waiting for Cron to finish...")
	<-serv.StopCron()
	log.Println("Server exited properly")
}
