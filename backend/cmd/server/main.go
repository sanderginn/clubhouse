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
	postHandler := handlers.NewPostHandler(dbConn, redisConn)
	commentHandler := handlers.NewCommentHandler(dbConn, redisConn)
	adminHandler := handlers.NewAdminHandler(dbConn)
	reactionHandler := handlers.NewReactionHandler(dbConn, redisConn)
	userHandler := handlers.NewUserHandler(dbConn)
	sectionHandler := handlers.NewSectionHandler(dbConn)
	wsHandler := handlers.NewWebSocketHandler(redisConn)

	// API routes
	mux.HandleFunc("/api/v1/auth/register", authHandler.Register)
	mux.HandleFunc("/api/v1/auth/login", authHandler.Login)
	mux.HandleFunc("/api/v1/auth/logout", authHandler.Logout)
	mux.HandleFunc("/api/v1/auth/me", authHandler.GetMe)
	mux.HandleFunc("/api/v1/sections", sectionHandler.ListSections)
	sectionRouteHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/feed") {
			postHandler.GetFeed(w, r)
		} else {
			sectionHandler.GetSection(w, r)
		}
	})
	mux.Handle("/api/v1/sections/", sectionRouteHandler)

	// User routes (protected - requires auth)
	userRouteHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if this is the /api/v1/users/me endpoint
		if r.URL.Path == "/api/v1/users/me" {
			if r.Method == http.MethodPatch {
				updateMeHandler := middleware.RequireAuth(redisConn)(http.HandlerFunc(userHandler.UpdateMe))
				updateMeHandler.ServeHTTP(w, r)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusMethodNotAllowed)
			w.Write([]byte(`{"error":"Method not allowed","code":"METHOD_NOT_ALLOWED"}`))
			return
		}
		// GET /api/v1/users/{id}/posts
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/posts") {
			postsHandler := middleware.RequireAuth(redisConn)(http.HandlerFunc(userHandler.GetUserPosts))
			postsHandler.ServeHTTP(w, r)
		} else if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/comments") {
			// GET /api/v1/users/{id}/comments
			commentsHandler := middleware.RequireAuth(redisConn)(http.HandlerFunc(userHandler.GetUserComments))
			commentsHandler.ServeHTTP(w, r)
		} else if r.Method == http.MethodGet {
			// GET /api/v1/users/{id}
			profileHandler := middleware.RequireAuth(redisConn)(http.HandlerFunc(userHandler.GetProfile))
			profileHandler.ServeHTTP(w, r)
		} else {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusMethodNotAllowed)
			w.Write([]byte(`{"error":"Method not allowed","code":"METHOD_NOT_ALLOWED"}`))
		}
	})
	mux.Handle("/api/v1/users/", userRouteHandler)

	// Comment routes - route to appropriate handler based on method
	commentRouteHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/restore") {
			restoreHandler := middleware.RequireAuth(redisConn)(http.HandlerFunc(commentHandler.RestoreComment))
			restoreHandler.ServeHTTP(w, r)
		} else if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/reactions") {
			// POST /api/v1/comments/{id}/reactions
			reactionAuthHandler := middleware.RequireAuth(redisConn)(http.HandlerFunc(reactionHandler.AddReactionToComment))
			reactionAuthHandler.ServeHTTP(w, r)
		} else if r.Method == http.MethodDelete && strings.Contains(r.URL.Path, "/reactions/") {
			// DELETE /api/v1/comments/{id}/reactions/{emoji}
			reactionAuthHandler := middleware.RequireAuth(redisConn)(http.HandlerFunc(reactionHandler.RemoveReactionFromComment))
			reactionAuthHandler.ServeHTTP(w, r)
		} else if r.Method == http.MethodGet {
			commentHandler.GetComment(w, r)
		} else if r.Method == http.MethodDelete {
			deleteHandler := middleware.RequireAuth(redisConn)(http.HandlerFunc(commentHandler.DeleteComment))
			deleteHandler.ServeHTTP(w, r)
		} else {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusMethodNotAllowed)
			w.Write([]byte(`{"error":"Method not allowed","code":"METHOD_NOT_ALLOWED"}`))
		}
	})
	mux.Handle("/api/v1/comments/", commentRouteHandler)

	// Post routes - route to appropriate handler
	postRouteHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if this is a thread comments request (GET /api/v1/posts/{id}/comments)
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/comments") {
			commentHandler.GetThread(w, r)
		} else if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/restore") {
			// Check if this is a restore request (POST /api/v1/posts/{id}/restore)
			// Apply auth middleware and call RestorePost
			authHandler := middleware.RequireAuth(redisConn)(http.HandlerFunc(postHandler.RestorePost))
			authHandler.ServeHTTP(w, r)
		} else if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/reactions") {
			// Check if this is an add reaction request (POST /api/v1/posts/{id}/reactions)
			// Apply auth middleware and call AddReactionToPost
			reactionAuthHandler := middleware.RequireAuth(redisConn)(http.HandlerFunc(reactionHandler.AddReactionToPost))
			reactionAuthHandler.ServeHTTP(w, r)
		} else if r.Method == http.MethodDelete && strings.Contains(r.URL.Path, "/reactions/") {
			// DELETE /api/v1/posts/{id}/reactions/{emoji}
			reactionAuthHandler := middleware.RequireAuth(redisConn)(http.HandlerFunc(reactionHandler.RemoveReactionFromPost))
			reactionAuthHandler.ServeHTTP(w, r)
		} else if r.Method == http.MethodGet {
			postHandler.GetPost(w, r)
		}
	})
	mux.Handle("/api/v1/posts/", postRouteHandler)

	// Protected post create route
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

	// WebSocket route (protected)
	mux.Handle("/api/v1/ws", middleware.RequireAuth(redisConn)(http.HandlerFunc(wsHandler.HandleWS)))

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
