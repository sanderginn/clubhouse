package handlers

import (
	"context"
	"net/http"
	"strings"
)

func writeHighlightValidationError(ctx context.Context, w http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}

	message := err.Error()
	switch {
	case strings.HasPrefix(message, "highlights are not allowed"):
		writeError(ctx, w, http.StatusBadRequest, "HIGHLIGHTS_NOT_ALLOWED", message)
		return true
	case message == "too many highlights":
		writeError(ctx, w, http.StatusBadRequest, "TOO_MANY_HIGHLIGHTS", message)
		return true
	case message == "highlight timestamp must be non-negative":
		writeError(ctx, w, http.StatusBadRequest, "HIGHLIGHT_TIMESTAMP_INVALID", message)
		return true
	case strings.HasPrefix(message, "highlight label must be less than"):
		writeError(ctx, w, http.StatusBadRequest, "HIGHLIGHT_LABEL_TOO_LONG", message)
		return true
	default:
		return false
	}
}
