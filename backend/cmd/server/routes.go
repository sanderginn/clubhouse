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
	getPost                http.HandlerFunc
	deletePost             http.HandlerFunc
}

func newPostRouteHandler(requireAuth authMiddleware, deps postRouteDeps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if this is a thread comments request (GET /api/v1/posts/{id}/comments)
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/comments") {
			deps.getThread(w, r)
			return
		}
		if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/restore") {
			// POST /api/v1/posts/{id}/restore
			requireAuth(http.HandlerFunc(deps.restorePost)).ServeHTTP(w, r)
			return
		}
		if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/reactions") {
			// POST /api/v1/posts/{id}/reactions
			requireAuth(http.HandlerFunc(deps.addReactionToPost)).ServeHTTP(w, r)
			return
		}
		if r.Method == http.MethodDelete && strings.Contains(r.URL.Path, "/reactions/") {
			// DELETE /api/v1/posts/{id}/reactions/{emoji}
			requireAuth(http.HandlerFunc(deps.removeReactionFromPost)).ServeHTTP(w, r)
			return
		}
		if r.Method == http.MethodDelete {
			// DELETE /api/v1/posts/{id}
			requireAuth(http.HandlerFunc(deps.deletePost)).ServeHTTP(w, r)
			return
		}
		if r.Method == http.MethodGet {
			deps.getPost(w, r)
			return
		}

		writeJSONBytes(r.Context(), w, http.StatusMethodNotAllowed, []byte(`{"error":"Method not allowed","code":"METHOD_NOT_ALLOWED"}`))
	})
}
