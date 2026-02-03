package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	_ "time/tzdata"

	"github.com/sanderginn/clubhouse/internal/cache"
	"github.com/sanderginn/clubhouse/internal/db"
	"github.com/sanderginn/clubhouse/internal/handlers"
	"github.com/sanderginn/clubhouse/internal/middleware"
	"github.com/sanderginn/clubhouse/internal/observability"
	"github.com/sanderginn/clubhouse/internal/services"
)

func writeJSONBytes(ctx context.Context, w http.ResponseWriter, statusCode int, body []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if _, err := w.Write(body); err != nil {
		observability.LogError(ctx, observability.ErrorLog{
			Message:    "failed to write response",
			Code:       "WRITE_FAILED",
			StatusCode: statusCode,
			Err:        err,
		})
	}
}

func main() {
	bootstrapUsername := flag.String("bootstrap-admin-username", os.Getenv("CLUBHOUSE_BOOTSTRAP_ADMIN_USERNAME"), "username for initial admin bootstrap")
	bootstrapEmail := flag.String("bootstrap-admin-email", os.Getenv("CLUBHOUSE_BOOTSTRAP_ADMIN_EMAIL"), "email for initial admin bootstrap")
	bootstrapPassword := flag.String("bootstrap-admin-password", os.Getenv("CLUBHOUSE_BOOTSTRAP_ADMIN_PASSWORD"), "password for initial admin bootstrap")
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize observability
	otelShutdown, metricsHandler, err := observability.Init(ctx)
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
		observability.LogError(ctx, observability.ErrorLog{
			Message:    "failed to initialize database",
			Code:       "DB_INIT_FAILED",
			StatusCode: http.StatusInternalServerError,
			Err:        err,
		})
		os.Exit(1)
	}
	defer dbConn.Close()

	go observability.StartDBStatsReporter(ctx, dbConn, 15*time.Second)

	if err := services.InitConfigService(ctx, dbConn); err != nil {
		observability.LogError(ctx, observability.ErrorLog{
			Message:    "failed to initialize config service",
			Code:       "CONFIG_INIT_FAILED",
			StatusCode: http.StatusInternalServerError,
			Err:        err,
		})
		os.Exit(1)
	}

	userService := services.NewUserService(dbConn)
	adminExists, err := userService.AdminExists(ctx)
	if err != nil {
		observability.LogError(ctx, observability.ErrorLog{
			Message:    "failed to check admin existence",
			Code:       "ADMIN_CHECK_FAILED",
			StatusCode: http.StatusInternalServerError,
			Err:        err,
		})
		os.Exit(1)
	}

	if adminExists {
		if *bootstrapUsername != "" || *bootstrapPassword != "" || *bootstrapEmail != "" {
			observability.LogInfo(ctx, "admin already exists; bootstrap skipped")
		}
	} else if *bootstrapUsername == "" || *bootstrapPassword == "" {
		observability.LogInfo(ctx, "no admin user exists; set CLUBHOUSE_BOOTSTRAP_ADMIN_USERNAME and CLUBHOUSE_BOOTSTRAP_ADMIN_PASSWORD (or CLI flags) to create the first admin")
	} else {
		user, created, err := userService.BootstrapAdmin(ctx, *bootstrapUsername, *bootstrapEmail, *bootstrapPassword)
		if err != nil {
			observability.LogError(ctx, observability.ErrorLog{
				Message:    "failed to bootstrap admin user",
				Code:       "ADMIN_BOOTSTRAP_FAILED",
				StatusCode: http.StatusInternalServerError,
				Err:        err,
			})
			os.Exit(1)
		}
		if created {
			observability.LogInfo(ctx, "bootstrap admin created", "username", user.Username)
		}
	}

	// Initialize Redis
	redisConn, err := cache.Init(ctx)
	if err != nil {
		observability.LogError(ctx, observability.ErrorLog{
			Message:    "failed to initialize redis",
			Code:       "REDIS_INIT_FAILED",
			StatusCode: http.StatusInternalServerError,
			Err:        err,
		})
		os.Exit(1)
	}
	defer redisConn.Close()

	// Initialize HTTP server
	mux := http.NewServeMux()

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSONBytes(r.Context(), w, http.StatusOK, []byte(`{"status":"ok"}`))
	})
	if metricsHandler != nil {
		mux.Handle("/metrics", metricsHandler)
	}

	// Initialize handlers
	authHandler := handlers.NewAuthHandler(dbConn, redisConn)
	configHandler := handlers.NewConfigHandler()
	pushService := services.NewPushService(dbConn)
	postHandler := handlers.NewPostHandler(dbConn, redisConn, pushService)
	commentHandler := handlers.NewCommentHandler(dbConn, redisConn, pushService)
	adminHandler := handlers.NewAdminHandler(dbConn, redisConn)
	reactionHandler := handlers.NewReactionHandler(dbConn, redisConn, pushService)
	cookLogHandler := handlers.NewCookLogHandler(dbConn, redisConn)
	userHandler := handlers.NewUserHandler(dbConn)
	sectionHandler := handlers.NewSectionHandler(dbConn)
	searchHandler := handlers.NewSearchHandler(dbConn)
	notificationHandler := handlers.NewNotificationHandler(dbConn, redisConn, pushService)
	wsHandler := handlers.NewWebSocketHandler(redisConn)
	linkHandler := handlers.NewLinkHandler()
	frontendMetricsHandler := handlers.NewMetricsHandler()
	pushHandler := handlers.NewPushHandler(dbConn, pushService)
	uploadHandler := handlers.NewUploadHandler()
	savedRecipeHandler := handlers.NewSavedRecipeHandler(dbConn, redisConn)
	requireAuth := middleware.RequireAuth(redisConn, dbConn)
	requireCSRF := middleware.RequireCSRF(redisConn)
	requireAuthCSRF := func(h http.Handler) http.Handler {
		return requireAuth(requireCSRF(h))
	}
	requireAdmin := middleware.RequireAdmin(redisConn, dbConn)
	requireAdminCSRF := func(h http.Handler) http.Handler {
		return requireAdmin(requireCSRF(h))
	}

	// API routes
	mux.Handle("/api/v1/config", http.HandlerFunc(configHandler.GetPublicConfig))
	mux.HandleFunc("/api/v1/auth/register", authHandler.Register)
	mux.HandleFunc("/api/v1/auth/login", authHandler.Login)
	mux.Handle("/api/v1/auth/logout", requireAuthCSRF(http.HandlerFunc(authHandler.Logout)))
	mux.HandleFunc("/api/v1/auth/me", authHandler.GetMe)
	mux.Handle("/api/v1/auth/csrf", requireAuth(http.HandlerFunc(authHandler.GetCSRFToken)))
	mux.Handle("/api/v1/auth/logout-all", requireAuthCSRF(http.HandlerFunc(authHandler.LogoutAll)))
	mux.HandleFunc("/api/v1/auth/password-reset/redeem", authHandler.RedeemPasswordResetToken)
	mux.Handle("/api/v1/sections", requireAuth(http.HandlerFunc(sectionHandler.ListSections)))
	sectionRouteHandler := newSectionRouteHandler(requireAuth, sectionRouteDeps{
		listSections: sectionHandler.ListSections,
		getSection:   sectionHandler.GetSection,
		getFeed:      postHandler.GetFeed,
		getLinks:     sectionHandler.GetSectionLinks,
	})
	mux.Handle("/api/v1/sections/", sectionRouteHandler)

	// User routes (protected - requires auth)
	userRouteHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/users/autocomplete" {
			requireAuth(http.HandlerFunc(userHandler.AutocompleteUsers)).ServeHTTP(w, r)
			return
		}
		if r.URL.Path == "/api/v1/users/lookup" {
			requireAuth(http.HandlerFunc(userHandler.LookupUserByUsername)).ServeHTTP(w, r)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/v1/users/me/mfa/") {
			switch r.URL.Path {
			case "/api/v1/users/me/mfa/enable":
				requireAuthCSRF(http.HandlerFunc(userHandler.EnrollMFA)).ServeHTTP(w, r)
				return
			case "/api/v1/users/me/mfa/verify":
				requireAuthCSRF(http.HandlerFunc(userHandler.VerifyMFA)).ServeHTTP(w, r)
				return
			case "/api/v1/users/me/mfa/disable":
				requireAuthCSRF(http.HandlerFunc(userHandler.DisableMFA)).ServeHTTP(w, r)
				return
			default:
				writeJSONBytes(r.Context(), w, http.StatusNotFound, []byte(`{"error":"Not found","code":"NOT_FOUND"}`))
				return
			}
		}
		if strings.HasPrefix(r.URL.Path, "/api/v1/users/me/section-subscriptions") {
			if r.Method == http.MethodGet && r.URL.Path == "/api/v1/users/me/section-subscriptions" {
				requireAuth(http.HandlerFunc(userHandler.GetMySectionSubscriptions)).ServeHTTP(w, r)
				return
			}
			if r.Method == http.MethodPatch {
				requireAuthCSRF(http.HandlerFunc(userHandler.UpdateMySectionSubscription)).ServeHTTP(w, r)
				return
			}
			writeJSONBytes(r.Context(), w, http.StatusMethodNotAllowed, []byte(`{"error":"Method not allowed","code":"METHOD_NOT_ALLOWED"}`))
			return
		}
		// Check if this is the /api/v1/users/me endpoint
		if r.URL.Path == "/api/v1/users/me" {
			if r.Method == http.MethodPatch {
				updateMeHandler := requireAuthCSRF(http.HandlerFunc(userHandler.UpdateMe))
				updateMeHandler.ServeHTTP(w, r)
				return
			}
			writeJSONBytes(r.Context(), w, http.StatusMethodNotAllowed, []byte(`{"error":"Method not allowed","code":"METHOD_NOT_ALLOWED"}`))
			return
		}
		// GET /api/v1/users/{id}/posts
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/posts") {
			postsHandler := middleware.RequireAuth(redisConn, dbConn)(http.HandlerFunc(userHandler.GetUserPosts))
			postsHandler.ServeHTTP(w, r)
		} else if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/comments") {
			// GET /api/v1/users/{id}/comments
			commentsHandler := middleware.RequireAuth(redisConn, dbConn)(http.HandlerFunc(userHandler.GetUserComments))
			commentsHandler.ServeHTTP(w, r)
		} else if r.Method == http.MethodGet {
			// GET /api/v1/users/{id}
			profileHandler := middleware.RequireAuth(redisConn, dbConn)(http.HandlerFunc(userHandler.GetProfile))
			profileHandler.ServeHTTP(w, r)
		} else {
			writeJSONBytes(r.Context(), w, http.StatusMethodNotAllowed, []byte(`{"error":"Method not allowed","code":"METHOD_NOT_ALLOWED"}`))
		}
	})
	mux.Handle("/api/v1/users/", userRouteHandler)

	// Comment routes - route to appropriate handler based on method
	commentRouteHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/restore") {
			restoreHandler := requireAuthCSRF(http.HandlerFunc(commentHandler.RestoreComment))
			restoreHandler.ServeHTTP(w, r)
		} else if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/reactions") {
			// POST /api/v1/comments/{id}/reactions
			reactionAuthHandler := requireAuthCSRF(http.HandlerFunc(reactionHandler.AddReactionToComment))
			reactionAuthHandler.ServeHTTP(w, r)
		} else if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/reactions") {
			// GET /api/v1/comments/{id}/reactions
			reactionAuthHandler := requireAuth(http.HandlerFunc(reactionHandler.GetCommentReactions))
			reactionAuthHandler.ServeHTTP(w, r)
		} else if r.Method == http.MethodDelete && strings.Contains(r.URL.Path, "/reactions/") {
			// DELETE /api/v1/comments/{id}/reactions/{emoji}
			reactionAuthHandler := requireAuthCSRF(http.HandlerFunc(reactionHandler.RemoveReactionFromComment))
			reactionAuthHandler.ServeHTTP(w, r)
		} else if r.Method == http.MethodPatch && isCommentIDPath(r.URL.Path) {
			updateHandler := requireAuthCSRF(http.HandlerFunc(commentHandler.UpdateComment))
			updateHandler.ServeHTTP(w, r)
		} else if r.Method == http.MethodGet {
			requireAuth(http.HandlerFunc(commentHandler.GetComment)).ServeHTTP(w, r)
		} else if r.Method == http.MethodDelete {
			deleteHandler := requireAuthCSRF(http.HandlerFunc(commentHandler.DeleteComment))
			deleteHandler.ServeHTTP(w, r)
		} else {
			writeJSONBytes(r.Context(), w, http.StatusMethodNotAllowed, []byte(`{"error":"Method not allowed","code":"METHOD_NOT_ALLOWED"}`))
		}
	})
	mux.Handle("/api/v1/comments/", commentRouteHandler)

	// Post routes - route to appropriate handler
	postRouteHandler := newPostRouteHandler(requireAuth, requireAuthCSRF, postRouteDeps{
		getThread:              commentHandler.GetThread,
		restorePost:            postHandler.RestorePost,
		addReactionToPost:      reactionHandler.AddReactionToPost,
		removeReactionFromPost: reactionHandler.RemoveReactionFromPost,
		getReactions:           reactionHandler.GetPostReactions,
		saveRecipe:             savedRecipeHandler.SaveRecipe,
		unsaveRecipe:           savedRecipeHandler.UnsaveRecipe,
		getPostSaves:           savedRecipeHandler.GetPostSaves,
		logCook:                cookLogHandler.LogCook,
		updateCookLog:          cookLogHandler.UpdateCookLog,
		removeCookLog:          cookLogHandler.RemoveCookLog,
		getCookLogs:            cookLogHandler.GetPostCookLogs,
		getPost:                postHandler.GetPost,
		updatePost:             postHandler.UpdatePost,
		deletePost:             postHandler.DeletePost,
	})
	mux.Handle("/api/v1/posts/", postRouteHandler)

	// Protected post create route
	postCreateHandler := requireAuthCSRF(
		http.HandlerFunc(postHandler.CreatePost),
	)
	mux.Handle("/api/v1/posts", postCreateHandler)

	// Protected comment routes
	commentCreateHandler := requireAuthCSRF(
		http.HandlerFunc(commentHandler.CreateComment),
	)
	mux.Handle("/api/v1/comments", commentCreateHandler)

	// Saved recipe routes (protected)
	mux.Handle("/api/v1/me/saved-recipes", requireAuth(http.HandlerFunc(savedRecipeHandler.ListSavedRecipes)))
	mux.Handle("/api/v1/me/recipe-categories", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			requireAuth(http.HandlerFunc(savedRecipeHandler.ListRecipeCategories)).ServeHTTP(w, r)
			return
		}
		if r.Method == http.MethodPost {
			requireAuthCSRF(http.HandlerFunc(savedRecipeHandler.CreateRecipeCategory)).ServeHTTP(w, r)
			return
		}
		writeJSONBytes(r.Context(), w, http.StatusMethodNotAllowed, []byte(`{"error":"Method not allowed","code":"METHOD_NOT_ALLOWED"}`))
	}))
	mux.Handle("/api/v1/me/recipe-categories/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPatch {
			requireAuthCSRF(http.HandlerFunc(savedRecipeHandler.UpdateRecipeCategory)).ServeHTTP(w, r)
			return
		}
		if r.Method == http.MethodDelete {
			requireAuthCSRF(http.HandlerFunc(savedRecipeHandler.DeleteRecipeCategory)).ServeHTTP(w, r)
			return
		}
		writeJSONBytes(r.Context(), w, http.StatusMethodNotAllowed, []byte(`{"error":"Method not allowed","code":"METHOD_NOT_ALLOWED"}`))
	}))

	// Search routes (protected)
	mux.Handle("/api/v1/search", requireAuth(http.HandlerFunc(searchHandler.Search)))

	// Cook log routes (protected)
	mux.Handle("/api/v1/me/cook-logs", requireAuth(http.HandlerFunc(cookLogHandler.GetMyCookLogs)))

	// Link preview route (protected with CSRF - POST only, prevents SSRF)
	mux.Handle("/api/v1/links/preview", requireAuthCSRF(http.HandlerFunc(linkHandler.PreviewLink)))
	mux.Handle("/api/v1/metrics/vitals", requireAuth(http.HandlerFunc(frontendMetricsHandler.RecordFrontendMetrics)))

	// Notification routes (protected)
	mux.Handle("/api/v1/notifications", requireAuth(http.HandlerFunc(notificationHandler.GetNotifications)))
	mux.Handle("/api/v1/notifications/read", requireAuthCSRF(http.HandlerFunc(notificationHandler.MarkAllNotificationsRead)))
	mux.Handle("/api/v1/notifications/", requireAuthCSRF(http.HandlerFunc(notificationHandler.MarkNotificationRead)))

	// Push routes (protected)
	mux.Handle("/api/v1/push/vapid-key", requireAuth(http.HandlerFunc(pushHandler.GetVAPIDKey)))
	mux.Handle("/api/v1/push/subscribe", requireAuthCSRF(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			pushHandler.Subscribe(w, r)
			return
		}
		if r.Method == http.MethodDelete {
			pushHandler.Unsubscribe(w, r)
			return
		}
		writeJSONBytes(r.Context(), w, http.StatusMethodNotAllowed, []byte(`{"error":"Method not allowed","code":"METHOD_NOT_ALLOWED"}`))
	})))

	// Upload routes (protected)
	mux.Handle("/api/v1/uploads", requireAuthCSRF(http.HandlerFunc(uploadHandler.UploadImage)))
	uploadsFileServer := http.StripPrefix("/api/v1/uploads/", http.FileServer(http.Dir(uploadHandler.UploadDir())))
	mux.Handle("/api/v1/uploads/", requireAuth(uploadsFileServer))

	// Admin routes (protected by RequireAdmin middleware)
	mux.Handle("/api/v1/admin/users", requireAdmin(http.HandlerFunc(adminHandler.ListPendingUsers)))
	mux.Handle("/api/v1/admin/users/approved", requireAdmin(http.HandlerFunc(adminHandler.ListApprovedUsers)))
	mux.Handle("/api/v1/admin/users/", requireAdminCSRF(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/promote") {
			adminHandler.PromoteUser(w, r)
		} else if strings.Contains(r.URL.Path, "/approve") {
			adminHandler.ApproveUser(w, r)
		} else if strings.Contains(r.URL.Path, "/unsuspend") {
			adminHandler.UnsuspendUser(w, r)
		} else if strings.Contains(r.URL.Path, "/suspend") {
			adminHandler.SuspendUser(w, r)
		} else if r.Method == http.MethodDelete {
			adminHandler.RejectUser(w, r)
		} else {
			writeJSONBytes(r.Context(), w, http.StatusMethodNotAllowed, []byte(`{"error":"Method not allowed","code":"METHOD_NOT_ALLOWED"}`))
		}
	})))

	// Admin post routes (hard delete and restore)
	mux.Handle("/api/v1/admin/posts/", requireAdminCSRF(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/restore") {
			adminHandler.AdminRestorePost(w, r)
		} else if r.Method == http.MethodDelete {
			adminHandler.HardDeletePost(w, r)
		} else {
			writeJSONBytes(r.Context(), w, http.StatusMethodNotAllowed, []byte(`{"error":"Method not allowed","code":"METHOD_NOT_ALLOWED"}`))
		}
	})))

	// Admin comment routes (hard delete and restore)
	mux.Handle("/api/v1/admin/comments/", requireAdminCSRF(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/restore") {
			adminHandler.AdminRestoreComment(w, r)
		} else if r.Method == http.MethodDelete {
			adminHandler.HardDeleteComment(w, r)
		} else {
			writeJSONBytes(r.Context(), w, http.StatusMethodNotAllowed, []byte(`{"error":"Method not allowed","code":"METHOD_NOT_ALLOWED"}`))
		}
	})))

	// Admin config route
	mux.Handle("/api/v1/admin/config", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			requireAdmin(http.HandlerFunc(adminHandler.GetConfig)).ServeHTTP(w, r)
		} else if r.Method == http.MethodPatch {
			requireAdminCSRF(http.HandlerFunc(adminHandler.UpdateConfig)).ServeHTTP(w, r)
		} else {
			writeJSONBytes(r.Context(), w, http.StatusMethodNotAllowed, []byte(`{"error":"Method not allowed","code":"METHOD_NOT_ALLOWED"}`))
		}
	}))

	// Admin audit logs route
	mux.Handle("/api/v1/admin/audit-logs", requireAdmin(http.HandlerFunc(adminHandler.GetAuditLogs)))
	mux.Handle("/api/v1/admin/audit-logs/actions", requireAdmin(http.HandlerFunc(adminHandler.GetAuditLogActions)))
	// Admin auth events route
	mux.Handle("/api/v1/admin/auth-events", requireAdmin(http.HandlerFunc(adminHandler.GetAuthEvents)))

	// Admin password reset route
	mux.Handle("/api/v1/admin/password-reset/generate", requireAdminCSRF(http.HandlerFunc(adminHandler.GeneratePasswordResetToken)))

	// Admin TOTP routes
	mux.Handle("/api/v1/admin/totp/enroll", requireAdminCSRF(http.HandlerFunc(adminHandler.EnrollTOTP)))
	mux.Handle("/api/v1/admin/totp/verify", requireAdminCSRF(http.HandlerFunc(adminHandler.VerifyTOTP)))

	// WebSocket route (protected)
	mux.Handle("/api/v1/ws", requireAuth(http.HandlerFunc(wsHandler.HandleWS)))

	// Apply middleware
	handler := middleware.ChainMiddleware(mux,
		middleware.RequestID,
		middleware.CSPMiddleware,
		middleware.Observability,
	)

	// HTTP server config
	port := os.Getenv("HTTP_PORT")
	if port == "" {
		port = "8080"
	}

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           handler,
		ReadTimeout:       15 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		observability.LogInfo(ctx, "starting http server", "addr", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			observability.LogError(ctx, observability.ErrorLog{
				Message:    "server error",
				Code:       "SERVER_START_FAILED",
				StatusCode: http.StatusInternalServerError,
				Err:        err,
			})
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	// Graceful shutdown
	observability.LogInfo(ctx, "shutting down server")
	ctxShutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctxShutdown); err != nil {
		observability.LogError(ctx, observability.ErrorLog{
			Message:    "server shutdown error",
			Code:       "SERVER_SHUTDOWN_FAILED",
			StatusCode: http.StatusInternalServerError,
			Err:        err,
		})
		os.Exit(1)
	}

	observability.LogInfo(ctx, "server stopped")
}
