package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
	"unicode"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/sanderginn/clubhouse/internal/models"
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
	MentionedUserID  uuid.UUID  `json:"mentioned_user_id"`
	MentioningUserID uuid.UUID  `json:"mentioning_user_id"`
	PostID           *uuid.UUID `json:"post_id,omitempty"`
	CommentID        *uuid.UUID `json:"comment_id,omitempty"`
}

type reactionEventData struct {
	PostID    *uuid.UUID `json:"post_id,omitempty"`
	CommentID *uuid.UUID `json:"comment_id,omitempty"`
	UserID    uuid.UUID  `json:"user_id"`
	Emoji     string     `json:"emoji"`
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

	return publishWithRetry(ctx, redisClient, channel, payload)
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

func publishMentions(ctx context.Context, redisClient *redis.Client, userService *services.UserService, content string, authorID uuid.UUID, postID *uuid.UUID, commentID *uuid.UUID) error {
	if redisClient == nil || userService == nil {
		return nil
	}

	usernames := extractMentionedUsernames(content)
	for _, username := range usernames {
		user, err := userService.GetUserByUsername(ctx, username)
		if err != nil {
			if err.Error() == "user not found" {
				continue
			}
			return fmt.Errorf("failed to resolve mention %s: %w", username, err)
		}
		if user.ID == authorID {
			continue
		}

		data := mentionEventData{
			MentionedUserID:  user.ID,
			MentioningUserID: authorID,
			PostID:           postID,
			CommentID:        commentID,
		}
		channel := formatChannel(userMentions, user.ID)
		if err := publishEvent(ctx, redisClient, channel, "mention", data); err != nil {
			return err
		}
	}

	return nil
}
