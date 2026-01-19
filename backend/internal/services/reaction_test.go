package services

import (
	"testing"
)

func TestAddReactionToPost(t *testing.T) {
	t.Skip("requires test database setup")
}

func TestRemoveReaction(t *testing.T) {
	t.Skip("requires test database setup")
}

func TestValidateEmoji(t *testing.T) {
	tests := []struct {
		name    string
		emoji   string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid emoji",
			emoji:   "👍",
			wantErr: false,
		},
		{
			name:    "valid text emoji",
			emoji:   ":thumbsup:",
			wantErr: false,
		},
		{
			name:    "empty emoji",
			emoji:   "",
			wantErr: true,
			errMsg:  "emoji is required",
		},
		{
			name:    "whitespace only",
			emoji:   "   ",
			wantErr: true,
			errMsg:  "emoji is required",
		},
		{
			name:    "emoji too long",
			emoji:   "12345678901",
			wantErr: true,
			errMsg:  "emoji must be 10 characters or less",
		},
		{
			name:    "emoji at max length",
			emoji:   "1234567890",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateEmoji(tt.emoji)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateEmoji() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && err.Error() != tt.errMsg {
				t.Errorf("validateEmoji() error message = %q, want %q", err.Error(), tt.errMsg)
			}
		})
	}
}
