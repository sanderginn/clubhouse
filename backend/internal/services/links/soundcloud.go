package links

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/sanderginn/clubhouse/internal/observability"
	"golang.org/x/net/html"
)

const (
	soundCloudOEmbedURL = "https://soundcloud.com/oembed"
	soundCloudTimeout   = 5 * time.Second
)

type SoundCloudExtractor struct {
	client    *http.Client
	oEmbedURL string
}

type soundCloudOEmbedResponse struct {
	Type         string `json:"type"`
	Version      string `json:"version"`
	Title        string `json:"title"`
	AuthorName   string `json:"author_name"`
	HTML         string `json:"html"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
	ThumbnailURL string `json:"thumbnail_url"`
}

func NewSoundCloudExtractor(client *http.Client) *SoundCloudExtractor {
	return &SoundCloudExtractor{
		client:    client,
		oEmbedURL: soundCloudOEmbedURL,
	}
}

func (e *SoundCloudExtractor) CanExtract(rawURL string) bool {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Hostname() == "" {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	return host == "soundcloud.com" || strings.HasSuffix(host, ".soundcloud.com")
}

func (e *SoundCloudExtractor) Extract(ctx context.Context, rawURL string) (*EmbedData, error) {
	if ctx == nil {
		return nil, errors.New("context is required")
	}

	ctx, cancel := context.WithTimeout(ctx, soundCloudTimeout)
	defer cancel()

	oembedURL := fmt.Sprintf("%s?format=json&url=%s", e.oEmbedURL, url.QueryEscape(rawURL))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, oembedURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build oembed request: %w", err)
	}
	req.Header.Set("User-Agent", "ClubhouseSoundCloudEmbed/1.0")

	client := e.client
	if client == nil {
		client = &http.Client{Timeout: soundCloudTimeout}
	}

	start := time.Now()
	resp, err := client.Do(req)
	duration := time.Since(start)
	if err != nil {
		observability.LogWarn(ctx, "soundcloud oembed request failed", "duration_ms", strconv.FormatInt(duration.Milliseconds(), 10), "error", err.Error())
		return nil, fmt.Errorf("soundcloud oembed request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		observability.LogWarn(ctx, "soundcloud oembed request returned non-200", "duration_ms", strconv.FormatInt(duration.Milliseconds(), 10), "status", resp.Status)
		return nil, fmt.Errorf("soundcloud oembed status: %s", resp.Status)
	}

	var payload soundCloudOEmbedResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		observability.LogWarn(ctx, "soundcloud oembed decode failed", "duration_ms", strconv.FormatInt(duration.Milliseconds(), 10), "error", err.Error())
		return nil, fmt.Errorf("decode soundcloud oembed: %w", err)
	}

	embedURL := extractIFrameSrc(payload.HTML)
	if embedURL == "" {
		observability.LogWarn(ctx, "soundcloud oembed missing iframe src", "duration_ms", strconv.FormatInt(duration.Milliseconds(), 10))
		return nil, errors.New("soundcloud oembed missing iframe src")
	}

	observability.LogDebug(ctx, "soundcloud oembed fetched", "duration_ms", strconv.FormatInt(duration.Milliseconds(), 10), "status", strconv.Itoa(resp.StatusCode))

	return &EmbedData{
		Type:     "oembed",
		Provider: "soundcloud",
		EmbedURL: embedURL,
		Width:    payload.Width,
		Height:   payload.Height,
	}, nil
}

func extractIFrameSrc(htmlSnippet string) string {
	if strings.TrimSpace(htmlSnippet) == "" {
		return ""
	}

	parsed, err := html.Parse(strings.NewReader(htmlSnippet))
	if err != nil {
		return ""
	}

	var src string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n == nil || src != "" {
			return
		}
		if n.Type == html.ElementNode && strings.EqualFold(n.Data, "iframe") {
			for _, attr := range n.Attr {
				if strings.EqualFold(attr.Key, "src") {
					src = strings.TrimSpace(attr.Val)
					return
				}
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(parsed)

	return src
}
