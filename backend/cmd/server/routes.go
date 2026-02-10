package main

import (
	"net/http"
	"strings"

	"github.com/sanderginn/clubhouse/internal/middleware"
)

type authMiddleware = middleware.Middleware

type postRouteDeps struct {
	getThread               http.HandlerFunc
	createQuote             http.HandlerFunc
	getPostQuotes           http.HandlerFunc
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
	addToBookshelf          http.HandlerFunc
	removeFromBookshelf     http.HandlerFunc
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
		if r.Method == http.MethodPost && isPostQuoteCollectionPath(r.URL.Path) {
			// POST /api/v1/posts/{id}/quotes
			requireAuthCSRF(http.HandlerFunc(deps.createQuote)).ServeHTTP(w, r)
			return
		}
		if r.Method == http.MethodGet && isPostQuoteCollectionPath(r.URL.Path) {
			// GET /api/v1/posts/{id}/quotes
			requireAuth(http.HandlerFunc(deps.getPostQuotes)).ServeHTTP(w, r)
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
		if r.Method == http.MethodPost && strings.HasSuffix(strings.TrimSuffix(r.URL.Path, "/"), "/bookshelf") {
			// POST /api/v1/posts/{id}/bookshelf
			requireAuthCSRF(http.HandlerFunc(deps.addToBookshelf)).ServeHTTP(w, r)
			return
		}
		if r.Method == http.MethodDelete && strings.HasSuffix(strings.TrimSuffix(r.URL.Path, "/"), "/bookshelf") {
			// DELETE /api/v1/posts/{id}/bookshelf
			requireAuthCSRF(http.HandlerFunc(deps.removeFromBookshelf)).ServeHTTP(w, r)
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
	listSections      http.HandlerFunc
	getSection        http.HandlerFunc
	getFeed           http.HandlerFunc
	getLinks          http.HandlerFunc
	getRecentPodcasts http.HandlerFunc
}

type bookshelfRouteDeps struct {
	getMyBookshelf    http.HandlerFunc
	getAllBookshelf   http.HandlerFunc
	listCategories    http.HandlerFunc
	createCategory    http.HandlerFunc
	reorderCategories http.HandlerFunc
	updateCategory    http.HandlerFunc
	deleteCategory    http.HandlerFunc
}

type bookQuoteRouteDeps struct {
	updateQuote http.HandlerFunc
	deleteQuote http.HandlerFunc
}

func newSectionRouteHandler(requireAuth authMiddleware, deps sectionRouteDeps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(strings.TrimSuffix(r.URL.Path, "/"), "/podcasts/recent") {
			requireAuth(http.HandlerFunc(deps.getRecentPodcasts)).ServeHTTP(w, r)
			return
		}
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

func registerBookshelfRoutes(
	mux *http.ServeMux,
	requireAuth authMiddleware,
	requireAuthCSRF authMiddleware,
	deps bookshelfRouteDeps,
) {
	mux.Handle("/api/v1/bookshelf", requireAuth(http.HandlerFunc(deps.getMyBookshelf)))
	mux.Handle("/api/v1/bookshelf/all", requireAuth(http.HandlerFunc(deps.getAllBookshelf)))
	mux.Handle("/api/v1/bookshelf/categories", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			requireAuth(http.HandlerFunc(deps.listCategories)).ServeHTTP(w, r)
			return
		}
		if r.Method == http.MethodPost {
			requireAuthCSRF(http.HandlerFunc(deps.createCategory)).ServeHTTP(w, r)
			return
		}
		writeJSONBytes(r.Context(), w, http.StatusMethodNotAllowed, []byte(`{"error":"Method not allowed","code":"METHOD_NOT_ALLOWED"}`))
	}))
	mux.Handle("/api/v1/bookshelf/categories/reorder", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			requireAuthCSRF(http.HandlerFunc(deps.reorderCategories)).ServeHTTP(w, r)
			return
		}
		writeJSONBytes(r.Context(), w, http.StatusMethodNotAllowed, []byte(`{"error":"Method not allowed","code":"METHOD_NOT_ALLOWED"}`))
	}))
	mux.Handle("/api/v1/bookshelf/categories/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			requireAuthCSRF(http.HandlerFunc(deps.updateCategory)).ServeHTTP(w, r)
			return
		}
		if r.Method == http.MethodDelete {
			requireAuthCSRF(http.HandlerFunc(deps.deleteCategory)).ServeHTTP(w, r)
			return
		}
		writeJSONBytes(r.Context(), w, http.StatusMethodNotAllowed, []byte(`{"error":"Method not allowed","code":"METHOD_NOT_ALLOWED"}`))
	}))
}

func registerReadHistoryRoute(mux *http.ServeMux, requireAuth authMiddleware, getReadHistory http.HandlerFunc) {
	mux.Handle("/api/v1/read-history", requireAuth(http.HandlerFunc(getReadHistory)))
}

func registerBookQuoteRoutes(
	mux *http.ServeMux,
	requireAuthCSRF authMiddleware,
	deps bookQuoteRouteDeps,
) {
	mux.Handle("/api/v1/quotes/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isQuoteIDPath(r.URL.Path) {
			writeJSONBytes(r.Context(), w, http.StatusNotFound, []byte(`{"error":"Not found","code":"NOT_FOUND"}`))
			return
		}
		if r.Method == http.MethodPut {
			requireAuthCSRF(http.HandlerFunc(deps.updateQuote)).ServeHTTP(w, r)
			return
		}
		if r.Method == http.MethodDelete {
			requireAuthCSRF(http.HandlerFunc(deps.deleteQuote)).ServeHTTP(w, r)
			return
		}
		writeJSONBytes(r.Context(), w, http.StatusMethodNotAllowed, []byte(`{"error":"Method not allowed","code":"METHOD_NOT_ALLOWED"}`))
	}))
}

func isPostIDPath(path string) bool {
	trimmed := strings.TrimSuffix(path, "/")
	parts := strings.Split(trimmed, "/")
	if len(parts) != 5 {
		return false
	}
	return parts[1] == "api" && parts[2] == "v1" && parts[3] == "posts" && parts[4] != ""
}

func isPostQuoteCollectionPath(path string) bool {
	trimmed := strings.TrimSuffix(path, "/")
	parts := strings.Split(trimmed, "/")
	if len(parts) != 6 {
		return false
	}
	return parts[1] == "api" && parts[2] == "v1" && parts[3] == "posts" && parts[4] != "" && parts[5] == "quotes"
}

func isHighlightReactionPath(path string) bool {
	trimmed := strings.TrimSuffix(path, "/")
	return strings.Contains(trimmed, "/highlights/") && strings.HasSuffix(trimmed, "/reactions")
}

func isQuoteIDPath(path string) bool {
	trimmed := strings.TrimSuffix(path, "/")
	parts := strings.Split(trimmed, "/")
	if len(parts) != 5 {
		return false
	}
	return parts[1] == "api" && parts[2] == "v1" && parts[3] == "quotes" && parts[4] != ""
}

func isUserQuoteCollectionPath(path string) bool {
	trimmed := strings.TrimSuffix(path, "/")
	parts := strings.Split(trimmed, "/")
	if len(parts) != 6 {
		return false
	}
	return parts[1] == "api" && parts[2] == "v1" && parts[3] == "users" && parts[4] != "" && parts[4] != "me" && parts[5] == "quotes"
}

func isCommentIDPath(path string) bool {
	trimmed := strings.TrimSuffix(path, "/")
	parts := strings.Split(trimmed, "/")
	if len(parts) != 5 {
		return false
	}
	return parts[1] == "api" && parts[2] == "v1" && parts[3] == "comments" && parts[4] != ""
}
