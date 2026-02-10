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
	case strings.HasPrefix(message, "podcast metadata is not allowed"):
		writeError(ctx, w, http.StatusBadRequest, "PODCAST_METADATA_NOT_ALLOWED", message)
		return true
	case message == "podcast kind is required":
		writeError(ctx, w, http.StatusBadRequest, "PODCAST_KIND_REQUIRED", message)
		return true
	case message == `podcast kind must be either "show" or "episode"`:
		writeError(ctx, w, http.StatusBadRequest, "PODCAST_KIND_INVALID", message)
		return true
	case message == `podcast highlight episodes are only allowed for kind "show"`:
		writeError(ctx, w, http.StatusBadRequest, "PODCAST_HIGHLIGHT_EPISODES_NOT_ALLOWED", message)
		return true
	case message == "too many podcast highlight episodes":
		writeError(ctx, w, http.StatusBadRequest, "TOO_MANY_PODCAST_HIGHLIGHT_EPISODES", message)
		return true
	case message == "podcast highlight episode title is required":
		writeError(ctx, w, http.StatusBadRequest, "PODCAST_HIGHLIGHT_EPISODE_TITLE_REQUIRED", message)
		return true
	case strings.HasPrefix(message, "podcast highlight episode title must be less than"):
		writeError(ctx, w, http.StatusBadRequest, "PODCAST_HIGHLIGHT_EPISODE_TITLE_TOO_LONG", message)
		return true
	case message == "podcast highlight episode url is required":
		writeError(ctx, w, http.StatusBadRequest, "PODCAST_HIGHLIGHT_EPISODE_URL_REQUIRED", message)
		return true
	case strings.HasPrefix(message, "podcast highlight episode url must be less than"):
		writeError(ctx, w, http.StatusBadRequest, "PODCAST_HIGHLIGHT_EPISODE_URL_TOO_LONG", message)
		return true
	case message == "podcast highlight episode url must be a valid http or https URL":
		writeError(ctx, w, http.StatusBadRequest, "PODCAST_HIGHLIGHT_EPISODE_URL_INVALID", message)
		return true
	case strings.HasPrefix(message, "podcast highlight episode note must be less than"):
		writeError(ctx, w, http.StatusBadRequest, "PODCAST_HIGHLIGHT_EPISODE_NOTE_TOO_LONG", message)
		return true
	default:
		return false
	}
}
