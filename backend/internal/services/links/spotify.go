package links

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// SpotifyExtractor extracts embed metadata for Spotify URLs.
type SpotifyExtractor struct{}

// SpotifyContent represents the parsed Spotify content type and ID.
type SpotifyContent struct {
	Type string
	ID   string
}

var spotifyContentTypes = map[string]struct{}{
	"track":    {},
	"album":    {},
	"playlist": {},
	"artist":   {},
	"show":     {},
	"episode":  {},
}

// CanExtract returns true when the URL is a supported Spotify URL.
func (SpotifyExtractor) CanExtract(rawURL string) bool {
	_, err := parseSpotifyURL(rawURL)
	return err == nil
}

// Extract parses the Spotify URL and returns an iframe embed payload.
func (SpotifyExtractor) Extract(ctx context.Context, rawURL string) (*EmbedData, error) {
	_ = ctx
	content, err := parseSpotifyURL(rawURL)
	if err != nil {
		return nil, err
	}

	embedURL := fmt.Sprintf("https://open.spotify.com/embed/%s/%s", content.Type, content.ID)
	return &EmbedData{
		Type:     "iframe",
		Provider: "spotify",
		EmbedURL: embedURL,
		Height:   spotifyEmbedHeight(content.Type),
	}, nil
}

func parseSpotifyURL(rawURL string) (*SpotifyContent, error) {
	if strings.TrimSpace(rawURL) == "" {
		return nil, errors.New("spotify url is required")
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parse spotify url: %w", err)
	}

	host := strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
	if host != "open.spotify.com" {
		return nil, errors.New("unsupported spotify host")
	}

	segments := strings.FieldsFunc(strings.Trim(u.Path, "/"), func(r rune) bool {
		return r == '/'
	})
	if len(segments) == 0 {
		return nil, errors.New("spotify path is missing")
	}

	index := 0
	if strings.HasPrefix(segments[index], "intl-") && len(segments) > 1 {
		index++
	}
	if index < len(segments) && segments[index] == "embed" && len(segments) > index+2 {
		index++
	}

	if len(segments) <= index+1 {
		return nil, errors.New("spotify content type or id missing")
	}

	contentType := strings.ToLower(segments[index])
	contentID := segments[index+1]
	if _, ok := spotifyContentTypes[contentType]; !ok {
		return nil, errors.New("unsupported spotify content type")
	}
	if strings.TrimSpace(contentID) == "" {
		return nil, errors.New("spotify content id missing")
	}

	return &SpotifyContent{Type: contentType, ID: contentID}, nil
}

func spotifyEmbedHeight(contentType string) int {
	switch contentType {
	case "track":
		return 152
	case "show", "episode":
		return 232
	case "album", "playlist", "artist":
		return 380
	default:
		return 380
	}
}
