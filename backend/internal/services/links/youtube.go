package links

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
)

const youtubeEmbedBaseURL = "https://www.youtube-nocookie.com/embed/"

type YouTubeExtractor struct{}

func (YouTubeExtractor) CanExtract(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	return isYouTubeHost(u.Hostname())
}

func (YouTubeExtractor) Extract(_ context.Context, rawURL string) (*EmbedData, error) {
	videoID, err := parseYouTubeVideoID(rawURL)
	if err != nil {
		return nil, err
	}

	embedURL := fmt.Sprintf("%s%s", youtubeEmbedBaseURL, videoID)
	if err := validateEmbedURL(embedURL); err != nil {
		return nil, err
	}

	return &EmbedData{
		Type:     "iframe",
		Provider: "youtube",
		EmbedURL: embedURL,
	}, nil
}

func parseYouTubeVideoID(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("parse youtube url: %w", err)
	}

	host := strings.ToLower(u.Hostname())
	if !isYouTubeHost(host) {
		return "", errors.New("not a youtube url")
	}

	pathValue := strings.Trim(u.Path, "/")
	switch {
	case strings.HasSuffix(host, "youtu.be"):
		id := firstPathSegment(pathValue)
		if id == "" {
			return "", errors.New("missing youtube video id")
		}
		return id, nil
	case strings.Contains(host, "youtube.com"):
		if strings.HasPrefix(pathValue, "watch") {
			id := strings.TrimSpace(u.Query().Get("v"))
			if id == "" {
				return "", errors.New("missing youtube video id")
			}
			return id, nil
		}
		if strings.HasPrefix(pathValue, "embed/") {
			id := firstPathSegment(strings.TrimPrefix(pathValue, "embed/"))
			if id == "" {
				return "", errors.New("missing youtube video id")
			}
			return id, nil
		}
		if strings.HasPrefix(pathValue, "shorts/") {
			id := firstPathSegment(strings.TrimPrefix(pathValue, "shorts/"))
			if id == "" {
				return "", errors.New("missing youtube video id")
			}
			return id, nil
		}
		if strings.HasPrefix(pathValue, "v/") {
			id := firstPathSegment(strings.TrimPrefix(pathValue, "v/"))
			if id == "" {
				return "", errors.New("missing youtube video id")
			}
			return id, nil
		}
	}

	return "", errors.New("missing youtube video id")
}

func isYouTubeHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	return host == "youtube.com" ||
		strings.HasSuffix(host, ".youtube.com") ||
		host == "youtu.be" ||
		strings.HasSuffix(host, ".youtu.be")
}

func firstPathSegment(value string) string {
	value = strings.Trim(value, "/")
	if value == "" {
		return ""
	}
	parts := strings.Split(value, "/")
	return parts[0]
}
