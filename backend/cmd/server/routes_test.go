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
