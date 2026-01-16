package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/sanderginn/clubhouse/internal/cache"
	"github.com/sanderginn/clubhouse/internal/db"
	"github.com/sanderginn/clubhouse/internal/handlers"
	"github.com/sanderginn/clubhouse/internal/middleware"
	"github.com/sanderginn/clubhouse/internal/observability"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize observability
	otelShutdown, err := observability.Init(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize observability: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		ctxShutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := otelShutdown(ctxShutdown); err != nil {
			fmt.Fprintf(os.Stderr, "failed to shutdown observability: %v\n", err)
		}
	}()

	// Initialize database
	dbConn, err := db.Init(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize database: %v\n", err)
		os.Exit(1)
	}
	defer dbConn.Close()

	// Initialize Redis
	redisConn, err := cache.Init(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize Redis: %v\n", err)
		os.Exit(1)
	}
	defer redisConn.Close()

	// Initialize HTTP server
	mux := http.NewServeMux()

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Initialize handlers
	authHandler := handlers.NewAuthHandler(dbConn, redisConn)
	postHandler := handlers.NewPostHandler(dbConn)
	commentHandler := handlers.NewCommentHandler(dbConn)
	adminHandler := handlers.NewAdminHandler(dbConn)

	// API routes
	mux.HandleFunc("/api/v1/auth/register", authHandler.Register)
	mux.HandleFunc("/api/v1/auth/login", authHandler.Login)
	mux.HandleFunc("/api/v1/auth/logout", authHandler.Logout)
	mux.HandleFunc("/api/v1/sections/", postHandler.GetFeed)
	mux.HandleFunc("/api/v1/posts/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/comments") {
			commentHandler.GetThread(w, r)
		} else {
			postHandler.GetPost(w, r)
		}
	})
	mux.HandleFunc("/api/v1/comments/", commentHandler.GetComment)

	// Protected post routes
	postCreateHandler := middleware.RequireAuth(redisConn)(
		http.HandlerFunc(postHandler.CreatePost),
	)
	mux.Handle("/api/v1/posts", postCreateHandler)

	// Protected comment routes
	commentCreateHandler := middleware.RequireAuth(redisConn)(
		http.HandlerFunc(commentHandler.CreateComment),
	)
	mux.Handle("/api/v1/comments", commentCreateHandler)

	// Admin routes (protected by RequireAdmin middleware)
	mux.Handle("/api/v1/admin/users", middleware.RequireAdmin(redisConn)(http.HandlerFunc(adminHandler.ListPendingUsers)))
	mux.Handle("/api/v1/admin/users/", middleware.RequireAdmin(redisConn)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/approve") {
			adminHandler.ApproveUser(w, r)
		} else {
			adminHandler.RejectUser(w, r)
		}
	})))

	// Apply middleware
	handler := middleware.ChainMiddleware(mux,
		middleware.RequestID,
		middleware.Observability,
	)

	// HTTP server config
	port := os.Getenv("HTTP_PORT")
	if port == "" {
		port = "8080"
	}

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		fmt.Printf("Starting HTTP server on %s\n", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "server error: %v\n", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	// Graceful shutdown
	fmt.Println("Shutting down server...")
	ctxShutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctxShutdown); err != nil {
		fmt.Fprintf(os.Stderr, "server shutdown error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Server stopped")
}
