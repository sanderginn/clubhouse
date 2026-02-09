package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/sanderginn/clubhouse/internal/models"
	"github.com/sanderginn/clubhouse/internal/observability"
	"github.com/sanderginn/clubhouse/internal/services"
)

const (
	postPrefix    = "post:%s"
	commentPrefix = "comment:%s"
)

type wsEvent struct {
	Type      string    `json:"type"`
	Data      any       `json:"data"`
	Timestamp time.Time `json:"timestamp"`
}

type postEventData struct {
	Post *models.Post `json:"post"`
}

type commentEventData struct {
	Comment *models.Comment `json:"comment"`
}

type mentionEventData struct {
	MentionedUserID  uuid.UUID           `json:"mentioned_user_id"`
	MentioningUserID uuid.UUID           `json:"mentioning_user_id"`
	MentioningUser   *models.UserSummary `json:"mentioning_user,omitempty"`
	ContentExcerpt   *string             `json:"content_excerpt,omitempty"`
	PostID           *uuid.UUID          `json:"post_id,omitempty"`
	CommentID        *uuid.UUID          `json:"comment_id,omitempty"`
}

type reactionEventData struct {
	PostID    *uuid.UUID `json:"post_id,omitempty"`
	CommentID *uuid.UUID `json:"comment_id,omitempty"`
	UserID    uuid.UUID  `json:"user_id"`
	Emoji     string     `json:"emoji"`
}

type highlightReactionEventData struct {
	PostID      uuid.UUID `json:"post_id"`
	LinkID      uuid.UUID `json:"link_id"`
	HighlightID string    `json:"highlight_id"`
	UserID      uuid.UUID `json:"user_id"`
}

type recipeSavedEventData struct {
	PostID     uuid.UUID `json:"post_id"`
	UserID     uuid.UUID `json:"user_id"`
	Username   string    `json:"username"`
	Categories []string  `json:"categories"`
}

type recipeUnsavedEventData struct {
	PostID uuid.UUID `json:"post_id"`
	UserID uuid.UUID `json:"user_id"`
}

type recipeCookedEventData struct {
	PostID   uuid.UUID `json:"post_id"`
	UserID   uuid.UUID `json:"user_id"`
	Username string    `json:"username"`
	Rating   int       `json:"rating"`
}

type recipeCookRemovedEventData struct {
	PostID uuid.UUID `json:"post_id"`
	UserID uuid.UUID `json:"user_id"`
}

type movieWatchlistedEventData struct {
	PostID     uuid.UUID `json:"post_id"`
	UserID     uuid.UUID `json:"user_id"`
	Username   string    `json:"username"`
	Categories []string  `json:"categories"`
}

type movieUnwatchlistedEventData struct {
	PostID uuid.UUID `json:"post_id"`
	UserID uuid.UUID `json:"user_id"`
}

type movieWatchedEventData struct {
	PostID   uuid.UUID `json:"post_id"`
	UserID   uuid.UUID `json:"user_id"`
	Username string    `json:"username"`
	Rating   int       `json:"rating"`
}

type movieWatchRemovedEventData struct {
	PostID uuid.UUID `json:"post_id"`
	UserID uuid.UUID `json:"user_id"`
}

func publishEvent(ctx context.Context, redisClient *redis.Client, channel string, eventType string, data any) error {
	if redisClient == nil {
		return nil
	}

	payload, err := json.Marshal(wsEvent{
		Type:      eventType,
		Data:      data,
		Timestamp: time.Now().UTC(),
	})
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	if err := publishWithRetry(ctx, redisClient, channel, payload); err != nil {
		observability.RecordWebsocketError(ctx, "publish_failed", eventType)
		return err
	}
	return nil
}

func publishWithRetry(ctx context.Context, redisClient *redis.Client, channel string, payload []byte) error {
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		err = redisClient.Publish(ctx, channel, payload).Err()
		if err == nil {
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		time.Sleep(time.Duration(attempt+1) * 50 * time.Millisecond)
	}
	return err
}

func publishContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 2*time.Second)
}

func extractMentionedUsernames(content string) []string {
	if content == "" {
		return nil
	}

	runes := []rune(content)
	seen := make(map[string]struct{})
	var usernames []string

	for i := 0; i < len(runes); i++ {
		if runes[i] != '@' {
			continue
		}

		if i > 0 && runes[i-1] == '\\' {
			continue
		}

		if i > 0 && isUsernameRune(runes[i-1]) {
			continue
		}

		start := i + 1
		if start >= len(runes) {
			continue
		}

		end := start
		for end < len(runes) && isUsernameRune(runes[end]) {
			end++
		}

		usernameLen := end - start
		if usernameLen < 3 || usernameLen > 50 {
			i = end - 1
			continue
		}

		username := string(runes[start:end])
		if _, ok := seen[username]; ok {
			i = end - 1
			continue
		}
		seen[username] = struct{}{}
		usernames = append(usernames, username)
		i = end - 1
	}

	return usernames
}

func isUsernameRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}

func normalizeMentionUsernames(usernames []string) []string {
	if len(usernames) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(usernames))
	result := make([]string, 0, len(usernames))
	for _, username := range usernames {
		trimmed := strings.TrimSpace(username)
		if trimmed == "" || len(trimmed) < 3 || len(trimmed) > 50 {
			continue
		}
		valid := true
		for _, r := range trimmed {
			if !isUsernameRune(r) {
				valid = false
				break
			}
		}
		if !valid {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, trimmed)
	}

	if len(result) == 0 {
		return nil
	}
	return result
}

func resolveMentionedUserIDs(ctx context.Context, userService *services.UserService, mentionUsernames []string, content string, authorID uuid.UUID) ([]uuid.UUID, error) {
	if userService == nil {
		return nil, nil
	}

	usernames := mentionUsernames
	if usernames == nil {
		usernames = extractMentionedUsernames(content)
	}
	usernames = normalizeMentionUsernames(usernames)
	if len(usernames) == 0 {
		return nil, nil
	}

	userIDs := make([]uuid.UUID, 0, len(usernames))
	for _, username := range usernames {
		user, err := userService.LookupUserByUsername(ctx, username)
		if err != nil {
			if err.Error() == "user not found" {
				continue
			}
			return nil, fmt.Errorf("failed to resolve mention %s: %w", username, err)
		}
		if user.ID == authorID {
			continue
		}

		userIDs = append(userIDs, user.ID)
	}

	return userIDs, nil
}

func publishMentions(ctx context.Context, redisClient *redis.Client, mentionedUserIDs []uuid.UUID, authorID uuid.UUID, postID *uuid.UUID, commentID *uuid.UUID, mentioningUser *models.UserSummary, contentExcerpt *string) error {
	if redisClient == nil {
		return nil
	}

	for _, mentionedUserID := range mentionedUserIDs {
		data := mentionEventData{
			MentionedUserID:  mentionedUserID,
			MentioningUserID: authorID,
			MentioningUser:   mentioningUser,
			ContentExcerpt:   contentExcerpt,
			PostID:           postID,
			CommentID:        commentID,
		}
		channel := formatChannel(userMentions, mentionedUserID)
		if err := publishEvent(ctx, redisClient, channel, "mention", data); err != nil {
			return err
		}
	}

	return nil
}

func userSummaryFromUser(user *models.User) *models.UserSummary {
	if user == nil {
		return nil
	}
	return &models.UserSummary{
		ID:                user.ID,
		Username:          user.Username,
		ProfilePictureURL: user.ProfilePictureURL,
	}
}

const mentionExcerptLimit = 100

func truncateMentionExcerpt(text string) *string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return nil
	}
	runes := []rune(trimmed)
	if len(runes) > mentionExcerptLimit {
		trimmed = string(runes[:mentionExcerptLimit])
	}
	return &trimmed
}
