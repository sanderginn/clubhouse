package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"time"

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

var mentionPattern = regexp.MustCompile(`(^|[^A-Za-z0-9_])@([A-Za-z0-9_]{3,50})`)

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
	matches := mentionPattern.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(matches))
	var usernames []string
	for _, match := range matches {
		if len(match) < 3 {
			continue
		}
		username := match[2]
		if _, ok := seen[username]; ok {
			continue
		}
		seen[username] = struct{}{}
		usernames = append(usernames, username)
	}

	return usernames
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
