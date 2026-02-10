package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/sanderginn/clubhouse/internal/models"
)

func TestPostRouteHandlerDeletePost(t *testing.T) {
	deleteCalled := false
	authCalled := false

	requireAuth := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authCalled = true
			next.ServeHTTP(w, r)
		})
	}

	deps := postRouteDeps{
		getThread: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("getThread should not be called")
		},
		restorePost: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("restorePost should not be called")
		},
		addReactionToPost: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("addReactionToPost should not be called")
		},
		removeReactionFromPost: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("removeReactionFromPost should not be called")
		},
		saveRecipe: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("saveRecipe should not be called")
		},
		unsaveRecipe: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("unsaveRecipe should not be called")
		},
		getPostSaves: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("getPostSaves should not be called")
		},
		logCook: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("logCook should not be called")
		},
		updateCookLog: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("updateCookLog should not be called")
		},
		removeCookLog: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("removeCookLog should not be called")
		},
		getCookLogs: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("getCookLogs should not be called")
		},
		getPost: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("getPost should not be called")
		},
		updatePost: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("updatePost should not be called")
		},
		deletePost: func(w http.ResponseWriter, r *http.Request) {
			deleteCalled = true
			w.WriteHeader(http.StatusOK)
		},
	}

	handler := newPostRouteHandler(requireAuth, requireAuth, deps)
	postID := uuid.New()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/posts/"+postID.String(), nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Fatalf("expected status %v, got %v", http.StatusOK, status)
	}

	if !authCalled {
		t.Fatal("expected auth middleware to be called")
	}

	if !deleteCalled {
		t.Fatal("expected delete handler to be called")
	}
}

func TestPostRouteHandlerUpdatePost(t *testing.T) {
	authCalled := false
	updateCalled := false

	requireAuth := func(next http.Handler) http.Handler {
		return next
	}
	requireAuthCSRF := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authCalled = true
			next.ServeHTTP(w, r)
		})
	}

	deps := postRouteDeps{
		getThread: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("getThread should not be called")
		},
		restorePost: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("restorePost should not be called")
		},
		addReactionToPost: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("addReactionToPost should not be called")
		},
		removeReactionFromPost: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("removeReactionFromPost should not be called")
		},
		getReactions: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("getReactions should not be called")
		},
		saveRecipe: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("saveRecipe should not be called")
		},
		unsaveRecipe: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("unsaveRecipe should not be called")
		},
		getPostSaves: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("getPostSaves should not be called")
		},
		logCook: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("logCook should not be called")
		},
		updateCookLog: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("updateCookLog should not be called")
		},
		removeCookLog: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("removeCookLog should not be called")
		},
		getCookLogs: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("getCookLogs should not be called")
		},
		getPost: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("getPost should not be called")
		},
		updatePost: func(w http.ResponseWriter, r *http.Request) {
			updateCalled = true
			w.WriteHeader(http.StatusOK)
		},
		deletePost: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("deletePost should not be called")
		},
	}

	handler := newPostRouteHandler(requireAuth, requireAuthCSRF, deps)
	postID := uuid.New()
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/posts/"+postID.String(), nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Fatalf("expected status %v, got %v", http.StatusOK, status)
	}

	if !authCalled {
		t.Fatal("expected auth middleware to be called")
	}

	if !updateCalled {
		t.Fatal("expected update handler to be called")
	}
}

func TestPostRouteHandlerMethodNotAllowed(t *testing.T) {
	requireAuth := func(next http.Handler) http.Handler {
		return next
	}

	deps := postRouteDeps{
		getThread: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("getThread should not be called")
		},
		restorePost: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("restorePost should not be called")
		},
		addReactionToPost: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("addReactionToPost should not be called")
		},
		removeReactionFromPost: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("removeReactionFromPost should not be called")
		},
		saveRecipe: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("saveRecipe should not be called")
		},
		unsaveRecipe: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("unsaveRecipe should not be called")
		},
		getPostSaves: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("getPostSaves should not be called")
		},
		logCook: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("logCook should not be called")
		},
		updateCookLog: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("updateCookLog should not be called")
		},
		removeCookLog: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("removeCookLog should not be called")
		},
		getCookLogs: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("getCookLogs should not be called")
		},
		getPost: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("getPost should not be called")
		},
		updatePost: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("updatePost should not be called")
		},
		deletePost: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("deletePost should not be called")
		},
	}

	handler := newPostRouteHandler(requireAuth, requireAuth, deps)
	postID := uuid.New()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/posts/"+postID.String(), nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %v, got %v", http.StatusMethodNotAllowed, status)
	}

	var response models.ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Code != "METHOD_NOT_ALLOWED" {
		t.Fatalf("expected code METHOD_NOT_ALLOWED, got %s", response.Code)
	}
}

func TestPostRouteHandlerDeletePostReactionsMissingEmoji(t *testing.T) {
	authCalled := false
	deleteCalled := false

	requireAuth := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authCalled = true
			next.ServeHTTP(w, r)
		})
	}

	deps := postRouteDeps{
		getThread: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("getThread should not be called")
		},
		restorePost: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("restorePost should not be called")
		},
		addReactionToPost: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("addReactionToPost should not be called")
		},
		removeReactionFromPost: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("removeReactionFromPost should not be called")
		},
		saveRecipe: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("saveRecipe should not be called")
		},
		unsaveRecipe: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("unsaveRecipe should not be called")
		},
		getPostSaves: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("getPostSaves should not be called")
		},
		logCook: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("logCook should not be called")
		},
		updateCookLog: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("updateCookLog should not be called")
		},
		removeCookLog: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("removeCookLog should not be called")
		},
		getCookLogs: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("getCookLogs should not be called")
		},
		getPost: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("getPost should not be called")
		},
		updatePost: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("updatePost should not be called")
		},
		deletePost: func(w http.ResponseWriter, r *http.Request) {
			deleteCalled = true
			w.WriteHeader(http.StatusOK)
		},
	}

	handler := newPostRouteHandler(requireAuth, requireAuth, deps)
	postID := uuid.New()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/posts/"+postID.String()+"/reactions", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %v, got %v", http.StatusMethodNotAllowed, status)
	}

	if authCalled {
		t.Fatal("expected auth middleware not to be called")
	}

	if deleteCalled {
		t.Fatal("did not expect delete handler to be called")
	}
}

func TestPostRouteHandlerDeletePostCommentsPath(t *testing.T) {
	authCalled := false
	deleteCalled := false

	requireAuth := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authCalled = true
			next.ServeHTTP(w, r)
		})
	}

	deps := postRouteDeps{
		getThread: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("getThread should not be called")
		},
		restorePost: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("restorePost should not be called")
		},
		addReactionToPost: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("addReactionToPost should not be called")
		},
		removeReactionFromPost: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("removeReactionFromPost should not be called")
		},
		saveRecipe: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("saveRecipe should not be called")
		},
		unsaveRecipe: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("unsaveRecipe should not be called")
		},
		getPostSaves: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("getPostSaves should not be called")
		},
		logCook: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("logCook should not be called")
		},
		updateCookLog: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("updateCookLog should not be called")
		},
		removeCookLog: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("removeCookLog should not be called")
		},
		getCookLogs: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("getCookLogs should not be called")
		},
		getPost: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("getPost should not be called")
		},
		updatePost: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("updatePost should not be called")
		},
		deletePost: func(w http.ResponseWriter, r *http.Request) {
			deleteCalled = true
			w.WriteHeader(http.StatusOK)
		},
	}

	handler := newPostRouteHandler(requireAuth, requireAuth, deps)
	postID := uuid.New()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/posts/"+postID.String()+"/comments", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %v, got %v", http.StatusMethodNotAllowed, status)
	}

	if authCalled {
		t.Fatal("expected auth middleware not to be called")
	}

	if deleteCalled {
		t.Fatal("did not expect delete handler to be called")
	}
}

func TestPostRouteHandlerGetThreadRequiresAuth(t *testing.T) {
	authCalled := false

	requireAuth := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authCalled = true
			w.WriteHeader(http.StatusUnauthorized)
		})
	}

	deps := postRouteDeps{
		getThread: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("getThread should not be called without auth")
		},
		restorePost: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("restorePost should not be called")
		},
		addReactionToPost: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("addReactionToPost should not be called")
		},
		removeReactionFromPost: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("removeReactionFromPost should not be called")
		},
		saveRecipe: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("saveRecipe should not be called")
		},
		unsaveRecipe: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("unsaveRecipe should not be called")
		},
		getPostSaves: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("getPostSaves should not be called")
		},
		logCook: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("logCook should not be called")
		},
		updateCookLog: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("updateCookLog should not be called")
		},
		removeCookLog: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("removeCookLog should not be called")
		},
		getCookLogs: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("getCookLogs should not be called")
		},
		getPost: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("getPost should not be called")
		},
		updatePost: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("updatePost should not be called")
		},
		deletePost: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("deletePost should not be called")
		},
	}

	handler := newPostRouteHandler(requireAuth, requireAuth, deps)
	postID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/posts/"+postID.String()+"/comments", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusUnauthorized {
		t.Fatalf("expected status %v, got %v", http.StatusUnauthorized, status)
	}

	if !authCalled {
		t.Fatal("expected auth middleware to be called")
	}
}

func TestPostRouteHandlerCookLogUsesCSRF(t *testing.T) {
	authCalled := false
	logCalled := false

	requireAuth := func(next http.Handler) http.Handler {
		return next
	}
	requireAuthCSRF := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authCalled = true
			next.ServeHTTP(w, r)
		})
	}

	deps := postRouteDeps{
		getThread: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("getThread should not be called")
		},
		restorePost: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("restorePost should not be called")
		},
		addReactionToPost: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("addReactionToPost should not be called")
		},
		removeReactionFromPost: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("removeReactionFromPost should not be called")
		},
		saveRecipe: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("saveRecipe should not be called")
		},
		unsaveRecipe: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("unsaveRecipe should not be called")
		},
		getPostSaves: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("getPostSaves should not be called")
		},
		logCook: func(w http.ResponseWriter, r *http.Request) {
			logCalled = true
			w.WriteHeader(http.StatusOK)
		},
		updateCookLog: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("updateCookLog should not be called")
		},
		removeCookLog: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("removeCookLog should not be called")
		},
		getCookLogs: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("getCookLogs should not be called")
		},
		getPost: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("getPost should not be called")
		},
		updatePost: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("updatePost should not be called")
		},
		deletePost: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("deletePost should not be called")
		},
	}

	handler := newPostRouteHandler(requireAuth, requireAuthCSRF, deps)
	postID := uuid.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/posts/"+postID.String()+"/cook-log", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Fatalf("expected status %v, got %v", http.StatusOK, status)
	}

	if !authCalled {
		t.Fatal("expected CSRF auth middleware to be called")
	}

	if !logCalled {
		t.Fatal("expected cook log handler to be called")
	}
}

func TestPostRouteHandlerGetCookLogsRequiresAuth(t *testing.T) {
	authCalled := false

	requireAuth := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authCalled = true
			w.WriteHeader(http.StatusUnauthorized)
		})
	}

	deps := postRouteDeps{
		getThread: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("getThread should not be called")
		},
		restorePost: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("restorePost should not be called")
		},
		addReactionToPost: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("addReactionToPost should not be called")
		},
		removeReactionFromPost: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("removeReactionFromPost should not be called")
		},
		saveRecipe: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("saveRecipe should not be called")
		},
		unsaveRecipe: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("unsaveRecipe should not be called")
		},
		getPostSaves: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("getPostSaves should not be called")
		},
		logCook: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("logCook should not be called")
		},
		updateCookLog: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("updateCookLog should not be called")
		},
		removeCookLog: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("removeCookLog should not be called")
		},
		getCookLogs: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("getCookLogs should not be called without auth")
		},
		getPost: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("getPost should not be called")
		},
		updatePost: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("updatePost should not be called")
		},
		deletePost: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("deletePost should not be called")
		},
	}

	handler := newPostRouteHandler(requireAuth, requireAuth, deps)
	postID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/posts/"+postID.String()+"/cook-logs", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusUnauthorized {
		t.Fatalf("expected status %v, got %v", http.StatusUnauthorized, status)
	}

	if !authCalled {
		t.Fatal("expected auth middleware to be called")
	}
}

func TestPostRouteHandlerAddToWatchlistUsesCSRFAuth(t *testing.T) {
	authCalled := false
	handlerCalled := false

	requireAuth := func(next http.Handler) http.Handler {
		return next
	}
	requireAuthCSRF := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authCalled = true
			next.ServeHTTP(w, r)
		})
	}

	deps := postRouteDeps{
		addToWatchlist: func(w http.ResponseWriter, r *http.Request) {
			handlerCalled = true
			w.WriteHeader(http.StatusOK)
		},
		getPost: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("getPost should not be called")
		},
	}

	handler := newPostRouteHandler(requireAuth, requireAuthCSRF, deps)
	postID := uuid.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/posts/"+postID.String()+"/watchlist", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Fatalf("expected status %v, got %v", http.StatusOK, status)
	}

	if !authCalled {
		t.Fatal("expected CSRF auth middleware to be called")
	}

	if !handlerCalled {
		t.Fatal("expected addToWatchlist handler to be called")
	}
}

func TestPostRouteHandlerRemoveFromWatchlistUsesCSRFAuth(t *testing.T) {
	authCalled := false
	handlerCalled := false

	requireAuth := func(next http.Handler) http.Handler {
		return next
	}
	requireAuthCSRF := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authCalled = true
			next.ServeHTTP(w, r)
		})
	}

	deps := postRouteDeps{
		removeFromWatchlist: func(w http.ResponseWriter, r *http.Request) {
			handlerCalled = true
			w.WriteHeader(http.StatusNoContent)
		},
		getPost: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("getPost should not be called")
		},
	}

	handler := newPostRouteHandler(requireAuth, requireAuthCSRF, deps)
	postID := uuid.New()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/posts/"+postID.String()+"/watchlist", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusNoContent {
		t.Fatalf("expected status %v, got %v", http.StatusNoContent, status)
	}

	if !authCalled {
		t.Fatal("expected CSRF auth middleware to be called")
	}

	if !handlerCalled {
		t.Fatal("expected removeFromWatchlist handler to be called")
	}
}

func TestPostRouteHandlerGetPostWatchlistInfoRequiresAuth(t *testing.T) {
	authCalled := false

	requireAuth := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authCalled = true
			w.WriteHeader(http.StatusUnauthorized)
		})
	}

	requireAuthCSRF := func(next http.Handler) http.Handler {
		return next
	}

	deps := postRouteDeps{
		getPostWatchlistInfo: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("getPostWatchlistInfo should not be called without auth")
		},
		getPost: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("getPost should not be called")
		},
	}

	handler := newPostRouteHandler(requireAuth, requireAuthCSRF, deps)
	postID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/posts/"+postID.String()+"/watchlist-info", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusUnauthorized {
		t.Fatalf("expected status %v, got %v", http.StatusUnauthorized, status)
	}

	if !authCalled {
		t.Fatal("expected auth middleware to be called")
	}
}

func TestPostRouteHandlerAddToBookshelfUsesCSRFAuth(t *testing.T) {
	authCalled := false
	handlerCalled := false

	requireAuth := func(next http.Handler) http.Handler {
		return next
	}
	requireAuthCSRF := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authCalled = true
			next.ServeHTTP(w, r)
		})
	}

	deps := postRouteDeps{
		addToBookshelf: func(w http.ResponseWriter, r *http.Request) {
			handlerCalled = true
			w.WriteHeader(http.StatusCreated)
		},
		getPost: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("getPost should not be called")
		},
	}

	handler := newPostRouteHandler(requireAuth, requireAuthCSRF, deps)
	postID := uuid.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/posts/"+postID.String()+"/bookshelf", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusCreated {
		t.Fatalf("expected status %v, got %v", http.StatusCreated, status)
	}

	if !authCalled {
		t.Fatal("expected CSRF auth middleware to be called")
	}

	if !handlerCalled {
		t.Fatal("expected addToBookshelf handler to be called")
	}
}

func TestPostRouteHandlerRemoveFromBookshelfUsesCSRFAuth(t *testing.T) {
	authCalled := false
	handlerCalled := false

	requireAuth := func(next http.Handler) http.Handler {
		return next
	}
	requireAuthCSRF := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authCalled = true
			next.ServeHTTP(w, r)
		})
	}

	deps := postRouteDeps{
		removeFromBookshelf: func(w http.ResponseWriter, r *http.Request) {
			handlerCalled = true
			w.WriteHeader(http.StatusNoContent)
		},
		getPost: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("getPost should not be called")
		},
	}

	handler := newPostRouteHandler(requireAuth, requireAuthCSRF, deps)
	postID := uuid.New()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/posts/"+postID.String()+"/bookshelf", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusNoContent {
		t.Fatalf("expected status %v, got %v", http.StatusNoContent, status)
	}

	if !authCalled {
		t.Fatal("expected CSRF auth middleware to be called")
	}

	if !handlerCalled {
		t.Fatal("expected removeFromBookshelf handler to be called")
	}
}

func TestPostRouteHandlerLogWatchUsesCSRFAuth(t *testing.T) {
	authCalled := false
	logCalled := false

	requireAuth := func(next http.Handler) http.Handler {
		return next
	}
	requireAuthCSRF := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authCalled = true
			next.ServeHTTP(w, r)
		})
	}

	deps := postRouteDeps{
		getThread: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("getThread should not be called")
		},
		restorePost: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("restorePost should not be called")
		},
		addReactionToPost: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("addReactionToPost should not be called")
		},
		removeReactionFromPost: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("removeReactionFromPost should not be called")
		},
		saveRecipe: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("saveRecipe should not be called")
		},
		unsaveRecipe: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("unsaveRecipe should not be called")
		},
		getPostSaves: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("getPostSaves should not be called")
		},
		logCook: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("logCook should not be called")
		},
		updateCookLog: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("updateCookLog should not be called")
		},
		removeCookLog: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("removeCookLog should not be called")
		},
		getCookLogs: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("getCookLogs should not be called")
		},
		logWatch: func(w http.ResponseWriter, r *http.Request) {
			logCalled = true
			w.WriteHeader(http.StatusOK)
		},
		getPost: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("getPost should not be called")
		},
		updatePost: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("updatePost should not be called")
		},
		deletePost: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("deletePost should not be called")
		},
	}

	handler := newPostRouteHandler(requireAuth, requireAuthCSRF, deps)
	postID := uuid.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/posts/"+postID.String()+"/watch-log", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Fatalf("expected status %v, got %v", http.StatusOK, status)
	}

	if !authCalled {
		t.Fatal("expected CSRF auth middleware to be called")
	}

	if !logCalled {
		t.Fatal("expected watch log handler to be called")
	}
}

func TestPostRouteHandlerGetWatchLogsRequiresAuth(t *testing.T) {
	authCalled := false

	requireAuth := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authCalled = true
			w.WriteHeader(http.StatusUnauthorized)
		})
	}

	deps := postRouteDeps{
		getThread: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("getThread should not be called")
		},
		restorePost: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("restorePost should not be called")
		},
		addReactionToPost: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("addReactionToPost should not be called")
		},
		removeReactionFromPost: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("removeReactionFromPost should not be called")
		},
		saveRecipe: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("saveRecipe should not be called")
		},
		unsaveRecipe: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("unsaveRecipe should not be called")
		},
		getPostSaves: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("getPostSaves should not be called")
		},
		logCook: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("logCook should not be called")
		},
		updateCookLog: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("updateCookLog should not be called")
		},
		removeCookLog: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("removeCookLog should not be called")
		},
		getCookLogs: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("getCookLogs should not be called")
		},
		getWatchLogs: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("getWatchLogs should not be called without auth")
		},
		getPost: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("getPost should not be called")
		},
		updatePost: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("updatePost should not be called")
		},
		deletePost: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("deletePost should not be called")
		},
	}

	handler := newPostRouteHandler(requireAuth, requireAuth, deps)
	postID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/posts/"+postID.String()+"/watch-logs", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusUnauthorized {
		t.Fatalf("expected status %v, got %v", http.StatusUnauthorized, status)
	}

	if !authCalled {
		t.Fatal("expected auth middleware to be called")
	}
}

func TestPostRouteHandlerGetReadLogsRequiresAuth(t *testing.T) {
	authCalled := false

	requireAuth := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authCalled = true
			w.WriteHeader(http.StatusUnauthorized)
		})
	}

	deps := postRouteDeps{
		getThread: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("getThread should not be called")
		},
		restorePost: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("restorePost should not be called")
		},
		addReactionToPost: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("addReactionToPost should not be called")
		},
		removeReactionFromPost: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("removeReactionFromPost should not be called")
		},
		saveRecipe: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("saveRecipe should not be called")
		},
		unsaveRecipe: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("unsaveRecipe should not be called")
		},
		getPostSaves: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("getPostSaves should not be called")
		},
		logCook: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("logCook should not be called")
		},
		updateCookLog: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("updateCookLog should not be called")
		},
		removeCookLog: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("removeCookLog should not be called")
		},
		getCookLogs: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("getCookLogs should not be called")
		},
		getWatchLogs: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("getWatchLogs should not be called")
		},
		getReadLogs: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("getReadLogs should not be called without auth")
		},
		getPost: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("getPost should not be called")
		},
		updatePost: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("updatePost should not be called")
		},
		deletePost: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("deletePost should not be called")
		},
	}

	handler := newPostRouteHandler(requireAuth, requireAuth, deps)
	postID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/posts/"+postID.String()+"/read", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusUnauthorized {
		t.Fatalf("expected status %v, got %v", http.StatusUnauthorized, status)
	}

	if !authCalled {
		t.Fatal("expected auth middleware to be called")
	}
}

func TestPostRouteHandlerReadMutationsUseCSRFAuth(t *testing.T) {
	authCalled := false
	csrfAuthCalled := false
	calledHandler := ""

	requireAuth := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authCalled = true
			next.ServeHTTP(w, r)
		})
	}
	requireAuthCSRF := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			csrfAuthCalled = true
			next.ServeHTTP(w, r)
		})
	}

	deps := postRouteDeps{
		getThread: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("getThread should not be called")
		},
		restorePost: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("restorePost should not be called")
		},
		addReactionToPost: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("addReactionToPost should not be called")
		},
		removeReactionFromPost: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("removeReactionFromPost should not be called")
		},
		getReactions: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("getReactions should not be called")
		},
		saveRecipe: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("saveRecipe should not be called")
		},
		unsaveRecipe: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("unsaveRecipe should not be called")
		},
		getPostSaves: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("getPostSaves should not be called")
		},
		logCook: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("logCook should not be called")
		},
		updateCookLog: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("updateCookLog should not be called")
		},
		removeCookLog: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("removeCookLog should not be called")
		},
		getCookLogs: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("getCookLogs should not be called")
		},
		logWatch: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("logWatch should not be called")
		},
		updateWatchLog: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("updateWatchLog should not be called")
		},
		removeWatchLog: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("removeWatchLog should not be called")
		},
		logRead: func(w http.ResponseWriter, r *http.Request) {
			calledHandler = "POST"
			w.WriteHeader(http.StatusCreated)
		},
		updateReadLog: func(w http.ResponseWriter, r *http.Request) {
			calledHandler = "PUT"
			w.WriteHeader(http.StatusOK)
		},
		removeReadLog: func(w http.ResponseWriter, r *http.Request) {
			calledHandler = "DELETE"
			w.WriteHeader(http.StatusNoContent)
		},
		getReadLogs: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("getReadLogs should not be called")
		},
		getPost: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("getPost should not be called")
		},
		updatePost: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("updatePost should not be called")
		},
		deletePost: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("deletePost should not be called")
		},
	}

	handler := newPostRouteHandler(requireAuth, requireAuthCSRF, deps)
	postID := uuid.New()

	tests := []struct {
		method          string
		expectedStatus  int
		expectedHandler string
	}{
		{method: http.MethodPost, expectedStatus: http.StatusCreated, expectedHandler: "POST"},
		{method: http.MethodPut, expectedStatus: http.StatusOK, expectedHandler: "PUT"},
		{method: http.MethodDelete, expectedStatus: http.StatusNoContent, expectedHandler: "DELETE"},
	}

	for _, tc := range tests {
		authCalled = false
		csrfAuthCalled = false
		calledHandler = ""

		req := httptest.NewRequest(tc.method, "/api/v1/posts/"+postID.String()+"/read", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if status := rr.Code; status != tc.expectedStatus {
			t.Fatalf("method %s: expected status %v, got %v", tc.method, tc.expectedStatus, status)
		}
		if calledHandler != tc.expectedHandler {
			t.Fatalf("method %s: expected %s handler, got %s", tc.method, tc.expectedHandler, calledHandler)
		}
		if !csrfAuthCalled {
			t.Fatalf("method %s: expected CSRF auth middleware to be called", tc.method)
		}
		if authCalled {
			t.Fatalf("method %s: did not expect auth-only middleware to be called", tc.method)
		}
	}
}

func TestPostRouteHandlerWatchLogsRejectsMutatingMethods(t *testing.T) {
	authCalled := false
	csrfAuthCalled := false

	requireAuth := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authCalled = true
			next.ServeHTTP(w, r)
		})
	}
	requireAuthCSRF := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			csrfAuthCalled = true
			next.ServeHTTP(w, r)
		})
	}

	deps := postRouteDeps{
		getThread: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("getThread should not be called")
		},
		restorePost: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("restorePost should not be called")
		},
		addReactionToPost: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("addReactionToPost should not be called")
		},
		removeReactionFromPost: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("removeReactionFromPost should not be called")
		},
		getReactions: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("getReactions should not be called")
		},
		saveRecipe: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("saveRecipe should not be called")
		},
		unsaveRecipe: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("unsaveRecipe should not be called")
		},
		getPostSaves: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("getPostSaves should not be called")
		},
		logCook: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("logCook should not be called")
		},
		updateCookLog: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("updateCookLog should not be called")
		},
		removeCookLog: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("removeCookLog should not be called")
		},
		getCookLogs: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("getCookLogs should not be called")
		},
		logWatch: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("logWatch should not be called for /watch-logs")
		},
		updateWatchLog: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("updateWatchLog should not be called for /watch-logs")
		},
		removeWatchLog: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("removeWatchLog should not be called for /watch-logs")
		},
		getWatchLogs: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("getWatchLogs should not be called for mutating methods")
		},
		getPost: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("getPost should not be called")
		},
		updatePost: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("updatePost should not be called")
		},
		deletePost: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("deletePost should not be called")
		},
	}

	handler := newPostRouteHandler(requireAuth, requireAuthCSRF, deps)
	postID := uuid.New()
	methods := []string{http.MethodPost, http.MethodPut, http.MethodDelete}

	for _, method := range methods {
		authCalled = false
		csrfAuthCalled = false

		req := httptest.NewRequest(method, "/api/v1/posts/"+postID.String()+"/watch-logs", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusMethodNotAllowed {
			t.Fatalf("method %s: expected status %v, got %v", method, http.StatusMethodNotAllowed, status)
		}

		var response models.ErrorResponse
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("method %s: failed to decode response: %v", method, err)
		}
		if response.Code != "METHOD_NOT_ALLOWED" {
			t.Fatalf("method %s: expected METHOD_NOT_ALLOWED, got %s", method, response.Code)
		}

		if authCalled || csrfAuthCalled {
			t.Fatalf("method %s: expected no auth middleware to be invoked", method)
		}
	}
}

func TestSectionRouteHandlerFeedRequiresAuth(t *testing.T) {
	authCalled := false

	requireAuth := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authCalled = true
			w.WriteHeader(http.StatusUnauthorized)
		})
	}

	deps := sectionRouteDeps{
		listSections: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("listSections should not be called without auth")
		},
		getSection: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("getSection should not be called without auth")
		},
		getFeed: func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("getFeed should not be called without auth")
		},
	}

	handler := newSectionRouteHandler(requireAuth, deps)
	sectionID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sections/"+sectionID.String()+"/feed", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusUnauthorized {
		t.Fatalf("expected status %v, got %v", http.StatusUnauthorized, status)
	}

	if !authCalled {
		t.Fatal("expected auth middleware to be called")
	}
}

func TestRegisterBookshelfRoutesWiresHandlersAndMiddleware(t *testing.T) {
	mux := http.NewServeMux()

	authCalled := false
	csrfAuthCalled := false
	calledHandler := ""

	requireAuth := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authCalled = true
			next.ServeHTTP(w, r)
		})
	}
	requireAuthCSRF := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			csrfAuthCalled = true
			next.ServeHTTP(w, r)
		})
	}

	handler := func(name string, status int) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			calledHandler = name
			w.WriteHeader(status)
		}
	}

	registerBookshelfRoutes(mux, requireAuth, requireAuthCSRF, bookshelfRouteDeps{
		getMyBookshelf:    handler("getMyBookshelf", http.StatusOK),
		getAllBookshelf:   handler("getAllBookshelf", http.StatusOK),
		listCategories:    handler("listCategories", http.StatusOK),
		createCategory:    handler("createCategory", http.StatusCreated),
		reorderCategories: handler("reorderCategories", http.StatusOK),
		updateCategory:    handler("updateCategory", http.StatusOK),
		deleteCategory:    handler("deleteCategory", http.StatusNoContent),
	})

	tests := []struct {
		name               string
		method             string
		path               string
		expectedStatus     int
		expectedHandler    string
		expectAuth         bool
		expectAuthWithCSRF bool
	}{
		{
			name:               "GET /api/v1/bookshelf",
			method:             http.MethodGet,
			path:               "/api/v1/bookshelf",
			expectedStatus:     http.StatusOK,
			expectedHandler:    "getMyBookshelf",
			expectAuth:         true,
			expectAuthWithCSRF: false,
		},
		{
			name:               "GET /api/v1/bookshelf/all",
			method:             http.MethodGet,
			path:               "/api/v1/bookshelf/all",
			expectedStatus:     http.StatusOK,
			expectedHandler:    "getAllBookshelf",
			expectAuth:         true,
			expectAuthWithCSRF: false,
		},
		{
			name:               "GET /api/v1/bookshelf/categories",
			method:             http.MethodGet,
			path:               "/api/v1/bookshelf/categories",
			expectedStatus:     http.StatusOK,
			expectedHandler:    "listCategories",
			expectAuth:         true,
			expectAuthWithCSRF: false,
		},
		{
			name:               "POST /api/v1/bookshelf/categories",
			method:             http.MethodPost,
			path:               "/api/v1/bookshelf/categories",
			expectedStatus:     http.StatusCreated,
			expectedHandler:    "createCategory",
			expectAuth:         false,
			expectAuthWithCSRF: true,
		},
		{
			name:               "POST /api/v1/bookshelf/categories/reorder",
			method:             http.MethodPost,
			path:               "/api/v1/bookshelf/categories/reorder",
			expectedStatus:     http.StatusOK,
			expectedHandler:    "reorderCategories",
			expectAuth:         false,
			expectAuthWithCSRF: true,
		},
		{
			name:               "PUT /api/v1/bookshelf/categories/{id}",
			method:             http.MethodPut,
			path:               "/api/v1/bookshelf/categories/" + uuid.New().String(),
			expectedStatus:     http.StatusOK,
			expectedHandler:    "updateCategory",
			expectAuth:         false,
			expectAuthWithCSRF: true,
		},
		{
			name:               "DELETE /api/v1/bookshelf/categories/{id}",
			method:             http.MethodDelete,
			path:               "/api/v1/bookshelf/categories/" + uuid.New().String(),
			expectedStatus:     http.StatusNoContent,
			expectedHandler:    "deleteCategory",
			expectAuth:         false,
			expectAuthWithCSRF: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			authCalled = false
			csrfAuthCalled = false
			calledHandler = ""

			req := httptest.NewRequest(tc.method, tc.path, nil)
			rr := httptest.NewRecorder()
			mux.ServeHTTP(rr, req)

			if rr.Code != tc.expectedStatus {
				t.Fatalf("expected status %d, got %d", tc.expectedStatus, rr.Code)
			}
			if calledHandler != tc.expectedHandler {
				t.Fatalf("expected handler %q, got %q", tc.expectedHandler, calledHandler)
			}
			if authCalled != tc.expectAuth {
				t.Fatalf("expected auth middleware called=%t, got %t", tc.expectAuth, authCalled)
			}
			if csrfAuthCalled != tc.expectAuthWithCSRF {
				t.Fatalf("expected CSRF auth middleware called=%t, got %t", tc.expectAuthWithCSRF, csrfAuthCalled)
			}
		})
	}
}

func TestRegisterBookshelfRoutesRejectsUnsupportedMethods(t *testing.T) {
	mux := http.NewServeMux()

	authCalled := false
	csrfAuthCalled := false

	requireAuth := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authCalled = true
			next.ServeHTTP(w, r)
		})
	}
	requireAuthCSRF := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			csrfAuthCalled = true
			next.ServeHTTP(w, r)
		})
	}

	registerBookshelfRoutes(mux, requireAuth, requireAuthCSRF, bookshelfRouteDeps{
		getMyBookshelf:    func(w http.ResponseWriter, r *http.Request) { t.Fatal("unexpected getMyBookshelf call") },
		getAllBookshelf:   func(w http.ResponseWriter, r *http.Request) { t.Fatal("unexpected getAllBookshelf call") },
		listCategories:    func(w http.ResponseWriter, r *http.Request) { t.Fatal("unexpected listCategories call") },
		createCategory:    func(w http.ResponseWriter, r *http.Request) { t.Fatal("unexpected createCategory call") },
		reorderCategories: func(w http.ResponseWriter, r *http.Request) { t.Fatal("unexpected reorderCategories call") },
		updateCategory:    func(w http.ResponseWriter, r *http.Request) { t.Fatal("unexpected updateCategory call") },
		deleteCategory:    func(w http.ResponseWriter, r *http.Request) { t.Fatal("unexpected deleteCategory call") },
	})

	tests := []struct {
		name   string
		method string
		path   string
	}{
		{
			name:   "PUT /api/v1/bookshelf/categories",
			method: http.MethodPut,
			path:   "/api/v1/bookshelf/categories",
		},
		{
			name:   "GET /api/v1/bookshelf/categories/reorder",
			method: http.MethodGet,
			path:   "/api/v1/bookshelf/categories/reorder",
		},
		{
			name:   "POST /api/v1/bookshelf/categories/{id}",
			method: http.MethodPost,
			path:   "/api/v1/bookshelf/categories/" + uuid.New().String(),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			authCalled = false
			csrfAuthCalled = false

			req := httptest.NewRequest(tc.method, tc.path, nil)
			rr := httptest.NewRecorder()
			mux.ServeHTTP(rr, req)

			if rr.Code != http.StatusMethodNotAllowed {
				t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rr.Code)
			}

			var response models.ErrorResponse
			if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}
			if response.Code != "METHOD_NOT_ALLOWED" {
				t.Fatalf("expected METHOD_NOT_ALLOWED code, got %q", response.Code)
			}

			if authCalled {
				t.Fatal("expected auth middleware to not be called")
			}
			if csrfAuthCalled {
				t.Fatal("expected CSRF auth middleware to not be called")
			}
		})
	}
}

func TestRegisterReadHistoryRouteRequiresAuth(t *testing.T) {
	mux := http.NewServeMux()
	authCalled := false

	requireAuth := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authCalled = true
			w.WriteHeader(http.StatusUnauthorized)
		})
	}

	registerReadHistoryRoute(mux, requireAuth, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("GetReadHistory handler should not be called when auth middleware blocks")
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/read-history", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}
	if !authCalled {
		t.Fatal("expected auth middleware to be called")
	}
}

func TestPostRouteHandlerCreateQuoteUsesCSRFAuth(t *testing.T) {
	authCalled := false
	createQuoteCalled := false

	requireAuth := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("requireAuth should not be called for quote creation")
		})
	}
	requireAuthCSRF := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authCalled = true
			next.ServeHTTP(w, r)
		})
	}

	handler := newPostRouteHandler(requireAuth, requireAuthCSRF, postRouteDeps{
		createQuote: func(w http.ResponseWriter, r *http.Request) {
			createQuoteCalled = true
			w.WriteHeader(http.StatusCreated)
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/posts/"+uuid.New().String()+"/quotes", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, rr.Code)
	}
	if !authCalled {
		t.Fatal("expected CSRF auth middleware to be called")
	}
	if !createQuoteCalled {
		t.Fatal("expected createQuote handler to be called")
	}
}

func TestPostRouteHandlerGetPostQuotesUsesAuth(t *testing.T) {
	authCalled := false
	getQuotesCalled := false

	requireAuth := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authCalled = true
			next.ServeHTTP(w, r)
		})
	}
	requireAuthCSRF := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("requireAuthCSRF should not be called for quote listing")
		})
	}

	handler := newPostRouteHandler(requireAuth, requireAuthCSRF, postRouteDeps{
		getPostQuotes: func(w http.ResponseWriter, r *http.Request) {
			getQuotesCalled = true
			w.WriteHeader(http.StatusOK)
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/posts/"+uuid.New().String()+"/quotes", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
	if !authCalled {
		t.Fatal("expected auth middleware to be called")
	}
	if !getQuotesCalled {
		t.Fatal("expected getPostQuotes handler to be called")
	}
}

func TestRegisterBookQuoteRoutesWiresHandlersAndMiddleware(t *testing.T) {
	mux := http.NewServeMux()
	csrfAuthCalled := false
	calledHandler := ""

	requireAuthCSRF := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			csrfAuthCalled = true
			next.ServeHTTP(w, r)
		})
	}

	handler := func(name string, status int) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			calledHandler = name
			w.WriteHeader(status)
		}
	}

	registerBookQuoteRoutes(mux, requireAuthCSRF, bookQuoteRouteDeps{
		updateQuote: handler("updateQuote", http.StatusOK),
		deleteQuote: handler("deleteQuote", http.StatusNoContent),
	})

	tests := []struct {
		name               string
		method             string
		path               string
		expectedStatus     int
		expectedHandler    string
		expectAuthWithCSRF bool
	}{
		{
			name:               "PUT /api/v1/quotes/{id}",
			method:             http.MethodPut,
			path:               "/api/v1/quotes/" + uuid.New().String(),
			expectedStatus:     http.StatusOK,
			expectedHandler:    "updateQuote",
			expectAuthWithCSRF: true,
		},
		{
			name:               "DELETE /api/v1/quotes/{id}",
			method:             http.MethodDelete,
			path:               "/api/v1/quotes/" + uuid.New().String(),
			expectedStatus:     http.StatusNoContent,
			expectedHandler:    "deleteQuote",
			expectAuthWithCSRF: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			csrfAuthCalled = false
			calledHandler = ""

			req := httptest.NewRequest(tc.method, tc.path, nil)
			rr := httptest.NewRecorder()
			mux.ServeHTTP(rr, req)

			if rr.Code != tc.expectedStatus {
				t.Fatalf("expected status %d, got %d", tc.expectedStatus, rr.Code)
			}
			if calledHandler != tc.expectedHandler {
				t.Fatalf("expected handler %q, got %q", tc.expectedHandler, calledHandler)
			}
			if csrfAuthCalled != tc.expectAuthWithCSRF {
				t.Fatalf("expected CSRF auth middleware called=%t, got %t", tc.expectAuthWithCSRF, csrfAuthCalled)
			}
		})
	}
}

func TestRegisterBookQuoteRoutesRejectsUnsupportedMethods(t *testing.T) {
	mux := http.NewServeMux()
	csrfAuthCalled := false

	requireAuthCSRF := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			csrfAuthCalled = true
			next.ServeHTTP(w, r)
		})
	}

	registerBookQuoteRoutes(mux, requireAuthCSRF, bookQuoteRouteDeps{
		updateQuote: func(w http.ResponseWriter, r *http.Request) { t.Fatal("unexpected updateQuote call") },
		deleteQuote: func(w http.ResponseWriter, r *http.Request) { t.Fatal("unexpected deleteQuote call") },
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/quotes/"+uuid.New().String(), nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rr.Code)
	}

	var response models.ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response.Code != "METHOD_NOT_ALLOWED" {
		t.Fatalf("expected METHOD_NOT_ALLOWED code, got %q", response.Code)
	}
	if csrfAuthCalled {
		t.Fatal("expected CSRF auth middleware to not be called")
	}
}
