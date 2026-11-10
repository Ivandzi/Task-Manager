package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "authservice.com/docs"
	"authservice.com/internal/config"
	"authservice.com/internal/handler"
	"authservice.com/internal/repository"
	"authservice.com/internal/service"
	httpSwagger "github.com/swaggo/http-swagger"
)

// @title Auth Service API
// @version 1.0
// @description API for user registration and authentication (issuance of JWT tokens).
// @host localhost:8081
// @BasePath /
func main() {
	cfg := config.Load()
	httpHost := cfg.HTTPHost
	httpPort := cfg.AuthPort
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
	serv := service.NewService(repo, cfg.JWTSecret)
	handl := handler.NewHandler(serv)
	portFlag := flag.String("httpPort", httpPort, "server port")
	flag.Parse()
	http.HandleFunc("/register", handl.RegisterHandler)
	http.HandleFunc("/login", handl.LoginHandler)
	http.HandleFunc("/swagger/", httpSwagger.WrapHandler)
	addr := fmt.Sprintf("%v:%v", httpHost, *portFlag)
	srv := &http.Server{
		Addr:    addr,
		Handler: nil,
	}
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
	log.Println("Server exited properly")
}
