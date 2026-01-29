package main

import (
	"net/http"
	"strings"

	"github.com/sanderginn/clubhouse/internal/middleware"
)

type authMiddleware = middleware.Middleware

type postRouteDeps struct {
	getThread              http.HandlerFunc
	restorePost            http.HandlerFunc
	addReactionToPost      http.HandlerFunc
	removeReactionFromPost http.HandlerFunc
	getReactions           http.HandlerFunc
	getPost                http.HandlerFunc
	updatePost             http.HandlerFunc
	deletePost             http.HandlerFunc
}

func newPostRouteHandler(requireAuth authMiddleware, requireAuthCSRF authMiddleware, deps postRouteDeps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if this is a thread comments request (GET /api/v1/posts/{id}/comments)
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/comments") {
			requireAuth(http.HandlerFunc(deps.getThread)).ServeHTTP(w, r)
			return
		}
		if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/restore") {
			// POST /api/v1/posts/{id}/restore
			requireAuthCSRF(http.HandlerFunc(deps.restorePost)).ServeHTTP(w, r)
			return
		}
		if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/reactions") {
			// POST /api/v1/posts/{id}/reactions
			requireAuthCSRF(http.HandlerFunc(deps.addReactionToPost)).ServeHTTP(w, r)
			return
		}
		if r.Method == http.MethodDelete && strings.Contains(r.URL.Path, "/reactions/") {
			// DELETE /api/v1/posts/{id}/reactions/{emoji}
			requireAuthCSRF(http.HandlerFunc(deps.removeReactionFromPost)).ServeHTTP(w, r)
			return
		}
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/reactions") {
			// GET /api/v1/posts/{id}/reactions
			requireAuth(http.HandlerFunc(deps.getReactions)).ServeHTTP(w, r)
			return
		}
		if r.Method == http.MethodPatch && isPostIDPath(r.URL.Path) {
			// PATCH /api/v1/posts/{id}
			requireAuthCSRF(http.HandlerFunc(deps.updatePost)).ServeHTTP(w, r)
			return
		}
		if r.Method == http.MethodDelete && isPostIDPath(r.URL.Path) {
			// DELETE /api/v1/posts/{id}
			requireAuthCSRF(http.HandlerFunc(deps.deletePost)).ServeHTTP(w, r)
			return
		}
		if r.Method == http.MethodGet {
			requireAuth(http.HandlerFunc(deps.getPost)).ServeHTTP(w, r)
			return
		}

		writeJSONBytes(r.Context(), w, http.StatusMethodNotAllowed, []byte(`{"error":"Method not allowed","code":"METHOD_NOT_ALLOWED"}`))
	})
}

type sectionRouteDeps struct {
	listSections http.HandlerFunc
	getSection   http.HandlerFunc
	getFeed      http.HandlerFunc
}

func newSectionRouteHandler(requireAuth authMiddleware, deps sectionRouteDeps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/feed") {
			requireAuth(http.HandlerFunc(deps.getFeed)).ServeHTTP(w, r)
			return
		}
		if r.URL.Path == "/api/v1/sections/" {
			// Handle trailing slash as list sections
			requireAuth(http.HandlerFunc(deps.listSections)).ServeHTTP(w, r)
			return
		}

		requireAuth(http.HandlerFunc(deps.getSection)).ServeHTTP(w, r)
	})
}

func isPostIDPath(path string) bool {
	trimmed := strings.TrimSuffix(path, "/")
	parts := strings.Split(trimmed, "/")
	if len(parts) != 5 {
		return false
	}
	return parts[1] == "api" && parts[2] == "v1" && parts[3] == "posts" && parts[4] != ""
}

func isCommentIDPath(path string) bool {
	trimmed := strings.TrimSuffix(path, "/")
	parts := strings.Split(trimmed, "/")
	if len(parts) != 5 {
		return false
	}
	return parts[1] == "api" && parts[2] == "v1" && parts[3] == "comments" && parts[4] != ""
}
