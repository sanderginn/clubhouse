package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sanderginn/clubhouse/internal/models"
)

func TestWriteHighlightValidationError_PodcastErrors(t *testing.T) {
	tests := []struct {
		name    string
		message string
		code    string
	}{
		{
			name:    "metadata not allowed",
			message: `podcast metadata is not allowed for section type "general"`,
			code:    "PODCAST_METADATA_NOT_ALLOWED",
		},
		{
			name:    "kind required",
			message: "podcast kind is required",
			code:    "PODCAST_KIND_REQUIRED",
		},
		{
			name:    "kind selection required",
			message: "podcast kind could not be detected; explicit selection required",
			code:    "PODCAST_KIND_SELECTION_REQUIRED",
		},
		{
			name:    "episode highlights not allowed",
			message: `podcast highlight episodes are only allowed for kind "show"`,
			code:    "PODCAST_HIGHLIGHT_EPISODES_NOT_ALLOWED",
		},
		{
			name:    "url invalid",
			message: "podcast highlight episode url must be a valid http or https URL",
			code:    "PODCAST_HIGHLIGHT_EPISODE_URL_INVALID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			handled := writeHighlightValidationError(context.Background(), recorder, errors.New(tt.message))
			if !handled {
				t.Fatalf("expected error to be handled")
			}
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("expected status 400, got %d", recorder.Code)
			}

			var response models.ErrorResponse
			if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}
			if response.Code != tt.code {
				t.Fatalf("expected code %q, got %q", tt.code, response.Code)
			}
		})
	}
}
