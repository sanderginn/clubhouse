package main

import (
	"net/http"
	"strings"

	"github.com/sanderginn/clubhouse/internal/middleware"
)

type authMiddleware = middleware.Middleware

type postRouteDeps struct {
	getThread               http.HandlerFunc
	restorePost             http.HandlerFunc
	addHighlightReaction    http.HandlerFunc
	getHighlightReactions   http.HandlerFunc
	removeHighlightReaction http.HandlerFunc
	addReactionToPost       http.HandlerFunc
	removeReactionFromPost  http.HandlerFunc
	getReactions            http.HandlerFunc
	saveRecipe              http.HandlerFunc
	unsaveRecipe            http.HandlerFunc
	getPostSaves            http.HandlerFunc
	addToWatchlist          http.HandlerFunc
	removeFromWatchlist     http.HandlerFunc
	getPostWatchlistInfo    http.HandlerFunc
	logCook                 http.HandlerFunc
	updateCookLog           http.HandlerFunc
	removeCookLog           http.HandlerFunc
	getCookLogs             http.HandlerFunc
	logWatch                http.HandlerFunc
	updateWatchLog          http.HandlerFunc
	removeWatchLog          http.HandlerFunc
	getWatchLogs            http.HandlerFunc
	logRead                 http.HandlerFunc
	updateReadLog           http.HandlerFunc
	removeReadLog           http.HandlerFunc
	getReadLogs             http.HandlerFunc
	getPost                 http.HandlerFunc
	updatePost              http.HandlerFunc
	deletePost              http.HandlerFunc
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
		if r.Method == http.MethodPost && isHighlightReactionPath(r.URL.Path) {
			// POST /api/v1/posts/{id}/highlights/{highlightId}/reactions
			requireAuthCSRF(http.HandlerFunc(deps.addHighlightReaction)).ServeHTTP(w, r)
			return
		}
		if r.Method == http.MethodGet && isHighlightReactionPath(r.URL.Path) {
			// GET /api/v1/posts/{id}/highlights/{highlightId}/reactions
			requireAuth(http.HandlerFunc(deps.getHighlightReactions)).ServeHTTP(w, r)
			return
		}
		if r.Method == http.MethodDelete && isHighlightReactionPath(r.URL.Path) {
			// DELETE /api/v1/posts/{id}/highlights/{highlightId}/reactions
			requireAuthCSRF(http.HandlerFunc(deps.removeHighlightReaction)).ServeHTTP(w, r)
			return
		}
		if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/reactions") {
			// POST /api/v1/posts/{id}/reactions
			requireAuthCSRF(http.HandlerFunc(deps.addReactionToPost)).ServeHTTP(w, r)
			return
		}
		if r.Method == http.MethodPost && strings.HasSuffix(strings.TrimSuffix(r.URL.Path, "/"), "/save") {
			// POST /api/v1/posts/{id}/save
			requireAuthCSRF(http.HandlerFunc(deps.saveRecipe)).ServeHTTP(w, r)
			return
		}
		if r.Method == http.MethodDelete && strings.HasSuffix(strings.TrimSuffix(r.URL.Path, "/"), "/save") {
			// DELETE /api/v1/posts/{id}/save
			requireAuthCSRF(http.HandlerFunc(deps.unsaveRecipe)).ServeHTTP(w, r)
			return
		}
		if r.Method == http.MethodGet && strings.HasSuffix(strings.TrimSuffix(r.URL.Path, "/"), "/saves") {
			// GET /api/v1/posts/{id}/saves
			requireAuth(http.HandlerFunc(deps.getPostSaves)).ServeHTTP(w, r)
			return
		}
		if r.Method == http.MethodPost && strings.HasSuffix(strings.TrimSuffix(r.URL.Path, "/"), "/watchlist") {
			// POST /api/v1/posts/{id}/watchlist
			requireAuthCSRF(http.HandlerFunc(deps.addToWatchlist)).ServeHTTP(w, r)
			return
		}
		if r.Method == http.MethodDelete && strings.HasSuffix(strings.TrimSuffix(r.URL.Path, "/"), "/watchlist") {
			// DELETE /api/v1/posts/{id}/watchlist
			requireAuthCSRF(http.HandlerFunc(deps.removeFromWatchlist)).ServeHTTP(w, r)
			return
		}
		if r.Method == http.MethodGet && strings.HasSuffix(strings.TrimSuffix(r.URL.Path, "/"), "/watchlist-info") {
			// GET /api/v1/posts/{id}/watchlist-info
			requireAuth(http.HandlerFunc(deps.getPostWatchlistInfo)).ServeHTTP(w, r)
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
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/cook-logs") {
			// GET /api/v1/posts/{id}/cook-logs
			requireAuth(http.HandlerFunc(deps.getCookLogs)).ServeHTTP(w, r)
			return
		}
		if r.Method == http.MethodGet && strings.HasSuffix(strings.TrimSuffix(r.URL.Path, "/"), "/watch-logs") {
			// GET /api/v1/posts/{id}/watch-logs
			requireAuth(http.HandlerFunc(deps.getWatchLogs)).ServeHTTP(w, r)
			return
		}
		if r.Method == http.MethodGet && strings.HasSuffix(strings.TrimSuffix(r.URL.Path, "/"), "/read") {
			// GET /api/v1/posts/{id}/read
			requireAuth(http.HandlerFunc(deps.getReadLogs)).ServeHTTP(w, r)
			return
		}
		if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/cook-log") {
			// POST /api/v1/posts/{id}/cook-log
			requireAuthCSRF(http.HandlerFunc(deps.logCook)).ServeHTTP(w, r)
			return
		}
		if r.Method == http.MethodPost && strings.HasSuffix(strings.TrimSuffix(r.URL.Path, "/"), "/watch-log") {
			// POST /api/v1/posts/{id}/watch-log
			requireAuthCSRF(http.HandlerFunc(deps.logWatch)).ServeHTTP(w, r)
			return
		}
		if r.Method == http.MethodPost && strings.HasSuffix(strings.TrimSuffix(r.URL.Path, "/"), "/read") {
			// POST /api/v1/posts/{id}/read
			requireAuthCSRF(http.HandlerFunc(deps.logRead)).ServeHTTP(w, r)
			return
		}
		if r.Method == http.MethodPut && strings.Contains(r.URL.Path, "/cook-log") {
			// PUT /api/v1/posts/{id}/cook-log
			requireAuthCSRF(http.HandlerFunc(deps.updateCookLog)).ServeHTTP(w, r)
			return
		}
		if r.Method == http.MethodPut && strings.HasSuffix(strings.TrimSuffix(r.URL.Path, "/"), "/watch-log") {
			// PUT /api/v1/posts/{id}/watch-log
			requireAuthCSRF(http.HandlerFunc(deps.updateWatchLog)).ServeHTTP(w, r)
			return
		}
		if r.Method == http.MethodPut && strings.HasSuffix(strings.TrimSuffix(r.URL.Path, "/"), "/read") {
			// PUT /api/v1/posts/{id}/read
			requireAuthCSRF(http.HandlerFunc(deps.updateReadLog)).ServeHTTP(w, r)
			return
		}
		if r.Method == http.MethodDelete && strings.Contains(r.URL.Path, "/cook-log") {
			// DELETE /api/v1/posts/{id}/cook-log
			requireAuthCSRF(http.HandlerFunc(deps.removeCookLog)).ServeHTTP(w, r)
			return
		}
		if r.Method == http.MethodDelete && strings.HasSuffix(strings.TrimSuffix(r.URL.Path, "/"), "/watch-log") {
			// DELETE /api/v1/posts/{id}/watch-log
			requireAuthCSRF(http.HandlerFunc(deps.removeWatchLog)).ServeHTTP(w, r)
			return
		}
		if r.Method == http.MethodDelete && strings.HasSuffix(strings.TrimSuffix(r.URL.Path, "/"), "/read") {
			// DELETE /api/v1/posts/{id}/read
			requireAuthCSRF(http.HandlerFunc(deps.removeReadLog)).ServeHTTP(w, r)
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
	getLinks     http.HandlerFunc
}

func newSectionRouteHandler(requireAuth authMiddleware, deps sectionRouteDeps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/links") {
			requireAuth(http.HandlerFunc(deps.getLinks)).ServeHTTP(w, r)
			return
		}
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

func isHighlightReactionPath(path string) bool {
	trimmed := strings.TrimSuffix(path, "/")
	return strings.Contains(trimmed, "/highlights/") && strings.HasSuffix(trimmed, "/reactions")
}

func isCommentIDPath(path string) bool {
	trimmed := strings.TrimSuffix(path, "/")
	parts := strings.Split(trimmed, "/")
	if len(parts) != 5 {
		return false
	}
	return parts[1] == "api" && parts[2] == "v1" && parts[3] == "comments" && parts[4] != ""
}
