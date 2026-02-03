package links

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/url"
	"strconv"
	"strings"
)

const (
	bandcampAlbumHeight = 470
	bandcampTrackHeight = 120
)

type BandcampExtractor struct{}

func (e BandcampExtractor) CanExtract(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	return strings.HasSuffix(host, "bandcamp.com")
}

func (e BandcampExtractor) Extract(ctx context.Context, rawURL string) (*EmbedData, error) {
	body, metaTags, err := defaultFetcher.fetchHTML(ctx, rawURL)
	if err != nil {
		return nil, err
	}
	return e.ExtractFromHTML(ctx, rawURL, body, metaTags)
}

func (e BandcampExtractor) ExtractFromHTML(
	_ context.Context,
	rawURL string,
	_ []byte,
	metaTags map[string]string,
) (*EmbedData, error) {
	content, err := parseBandcampContent(rawURL, metaTags)
	if err != nil {
		return nil, err
	}
	embedURL, height := buildBandcampEmbedURL(content)
	return &EmbedData{
		Type:     "iframe",
		Provider: "bandcamp",
		EmbedURL: embedURL,
		Height:   height,
	}, nil
}

type bandcampContent struct {
	Type string
	ID   string
}

func parseBandcampContent(rawURL string, metaTags map[string]string) (bandcampContent, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return bandcampContent{}, fmt.Errorf("parse url: %w", err)
	}
	host := strings.ToLower(parsed.Hostname())
	if !strings.HasSuffix(host, "bandcamp.com") {
		return bandcampContent{}, errors.New("not a bandcamp url")
	}

	itemType := bandcampTypeFromPath(parsed.Path)
	metaType, metaID := parseBandcampMeta(metaTags)
	if metaType != "" {
		itemType = metaType
	}
	if metaID == "" {
		return bandcampContent{}, errors.New("missing bandcamp item id")
	}
	if itemType == "" {
		return bandcampContent{}, errors.New("missing bandcamp item type")
	}

	return bandcampContent{
		Type: itemType,
		ID:   metaID,
	}, nil
}

func bandcampTypeFromPath(path string) string {
	lower := strings.ToLower(path)
	switch {
	case strings.Contains(lower, "/album/"):
		return "album"
	case strings.Contains(lower, "/track/"):
		return "track"
	default:
		return ""
	}
}

func parseBandcampMeta(metaTags map[string]string) (string, string) {
	raw := strings.TrimSpace(metaTags["bc-page-properties"])
	if raw == "" {
		return "", ""
	}
	raw = html.UnescapeString(raw)
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return "", ""
	}
	itemType := normalizeBandcampItemType(payload["item_type"])
	itemID := normalizeBandcampItemID(payload["item_id"])
	return itemType, itemID
}

func normalizeBandcampItemType(value interface{}) string {
	raw, ok := value.(string)
	if !ok {
		return ""
	}
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "a", "album":
		return "album"
	case "t", "track":
		return "track"
	default:
		return ""
	}
}

func normalizeBandcampItemID(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case json.Number:
		return strings.TrimSpace(typed.String())
	case float64:
		if typed == 0 {
			return ""
		}
		return strconv.FormatInt(int64(typed), 10)
	default:
		return ""
	}
}

func buildBandcampEmbedURL(content bandcampContent) (string, int) {
	tracklist := "true"
	height := bandcampAlbumHeight
	if content.Type == "track" {
		tracklist = "false"
		height = bandcampTrackHeight
	}
	embedURL := fmt.Sprintf(
		"https://bandcamp.com/EmbeddedPlayer/%s=%s/size=large/bgcol=ffffff/linkcol=0687f5/tracklist=%s/artwork=small/transparent=true/",
		content.Type,
		content.ID,
		tracklist,
	)
	return embedURL, height
}
