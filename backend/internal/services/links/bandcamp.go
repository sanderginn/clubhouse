package links

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	stdhtml "html"

	"golang.org/x/net/html"
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
	// Use azuretls-client to bypass Bandcamp's WAF
	body, err := bandcampFetchHTMLFunc(ctx, rawURL)
	if err != nil {
		return nil, fmt.Errorf("fetch bandcamp: %w", err)
	}
	metaTags, _ := extractHTMLMeta(body)
	return e.ExtractFromHTML(ctx, rawURL, body, metaTags)
}

func (e BandcampExtractor) ExtractFromHTML(
	_ context.Context,
	rawURL string,
	body []byte,
	metaTags map[string]string,
) (*EmbedData, error) {
	content, err := parseBandcampContent(rawURL, body, metaTags)
	if err != nil {
		return nil, err
	}
	embedURL, height := buildBandcampEmbedURL(content)
	if err := validateEmbedURL(embedURL); err != nil {
		return nil, err
	}
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

func parseBandcampContent(rawURL string, body []byte, metaTags map[string]string) (bandcampContent, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return bandcampContent{}, fmt.Errorf("parse url: %w", err)
	}
	host := strings.ToLower(parsed.Hostname())
	if !strings.HasSuffix(host, "bandcamp.com") {
		return bandcampContent{}, errors.New("not a bandcamp url")
	}

	content := bandcampContent{
		Type: bandcampTypeFromPath(parsed.Path),
	}

	if len(body) > 0 {
		if jsonLDContent, _, err := extractBandcampFromJSONLD(body); err == nil {
			if jsonLDContent.Type != "" {
				content.Type = jsonLDContent.Type
			}
			if jsonLDContent.ID != "" {
				content.ID = jsonLDContent.ID
			}
		}
	}

	metaType, metaID := parseBandcampMeta(metaTags)
	if metaType != "" && content.Type == "" {
		content.Type = metaType
	}
	if metaID != "" && content.ID == "" {
		content.ID = metaID
	}

	if len(body) > 0 && content.ID == "" {
		propType, propID := parseBandcampPageProperties(body)
		if propType != "" && content.Type == "" {
			content.Type = propType
		}
		if propID != "" {
			content.ID = propID
		}
	}

	if content.ID == "" {
		return bandcampContent{}, errors.New("missing bandcamp item id")
	}
	if content.Type == "" {
		return bandcampContent{}, errors.New("missing bandcamp item type")
	}

	return content, nil
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
	raw = stdhtml.UnescapeString(raw)
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return "", ""
	}
	itemType := normalizeBandcampItemType(payload["item_type"])
	itemID := normalizeBandcampItemID(payload["item_id"])
	return itemType, itemID
}

func parseBandcampPageProperties(body []byte) (string, string) {
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return "", ""
	}

	var raw string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if raw != "" {
			return
		}
		if n.Type == html.ElementNode {
			for _, attr := range n.Attr {
				if strings.EqualFold(attr.Key, "data-bc-page-properties") {
					raw = strings.TrimSpace(attr.Val)
					return
				}
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)

	if raw == "" {
		return "", ""
	}

	raw = stdhtml.UnescapeString(raw)
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return "", ""
	}
	itemType := normalizeBandcampItemType(payload["item_type"])
	itemID := normalizeBandcampItemID(payload["item_id"])
	return itemType, itemID
}

func extractBandcampFromJSONLD(body []byte) (bandcampContent, map[string]interface{}, error) {
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return bandcampContent{}, nil, fmt.Errorf("parse html: %w", err)
	}

	var scripts []string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && strings.EqualFold(n.Data, "script") {
			var scriptType string
			for _, attr := range n.Attr {
				if strings.EqualFold(attr.Key, "type") {
					scriptType = strings.TrimSpace(attr.Val)
				}
			}
			if scriptType == "application/ld+json" && n.FirstChild != nil {
				scripts = append(scripts, strings.TrimSpace(n.FirstChild.Data))
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)

	if len(scripts) == 0 {
		return bandcampContent{}, nil, errors.New("no json-ld found")
	}

	for _, script := range scripts {
		content, metadata, err := parseBandcampJSONLD([]byte(script))
		if err != nil {
			continue
		}
		if content.Type != "" && content.ID != "" {
			return content, metadata, nil
		}
	}

	return bandcampContent{}, nil, errors.New("no supported json-ld found")
}

func parseBandcampJSONLD(raw []byte) (bandcampContent, map[string]interface{}, error) {
	var payload interface{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return bandcampContent{}, nil, fmt.Errorf("parse json-ld: %w", err)
	}

	candidates := []interface{}{payload}
	if array, ok := payload.([]interface{}); ok {
		candidates = array
	}

	for _, candidate := range candidates {
		obj, ok := candidate.(map[string]interface{})
		if !ok {
			continue
		}

		ldType := normalizeJSONLDType(obj["@type"])
		switch ldType {
		case "MusicAlbum":
			itemID := findJSONLDItemID(obj["albumRelease"])
			if itemID == "" {
				itemID = findJSONLDItemID(obj["additionalProperty"])
			}
			if itemID == "" {
				return bandcampContent{}, nil, errors.New("item_id not found in json-ld")
			}
			return bandcampContent{Type: "album", ID: itemID}, bandcampJSONLDMetadata(obj), nil
		case "MusicRecording":
			itemID := findJSONLDItemID(obj["additionalProperty"])
			if itemID == "" {
				itemID = findJSONLDItemID(obj["inAlbum"])
			}
			if itemID == "" {
				return bandcampContent{}, nil, errors.New("item_id not found in json-ld")
			}
			return bandcampContent{Type: "track", ID: itemID}, bandcampJSONLDMetadata(obj), nil
		default:
			continue
		}
	}

	return bandcampContent{}, nil, errors.New("unsupported json-ld type")
}

func normalizeJSONLDType(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case []interface{}:
		for _, item := range typed {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				return strings.TrimSpace(s)
			}
		}
	}
	return ""
}

func findJSONLDItemID(value interface{}) string {
	switch typed := value.(type) {
	case []interface{}:
		for _, item := range typed {
			id := findJSONLDItemID(item)
			if id != "" {
				return id
			}
		}
	case map[string]interface{}:
		name, _ := typed["name"].(string)
		if strings.TrimSpace(name) == "item_id" {
			return normalizeBandcampItemID(typed["value"])
		}
		if additionalProperty, ok := typed["additionalProperty"]; ok {
			return findJSONLDItemID(additionalProperty)
		}
	}
	return ""
}

func bandcampJSONLDMetadata(obj map[string]interface{}) map[string]interface{} {
	metadata := map[string]interface{}{
		"site_name": "Bandcamp",
	}
	if name, ok := obj["name"].(string); ok && strings.TrimSpace(name) != "" {
		metadata["title"] = strings.TrimSpace(name)
	}
	if byArtist, ok := obj["byArtist"].(map[string]interface{}); ok {
		if artist, ok := byArtist["name"].(string); ok && strings.TrimSpace(artist) != "" {
			metadata["artist"] = strings.TrimSpace(artist)
		}
	}
	if image, ok := obj["image"].(string); ok && strings.TrimSpace(image) != "" {
		metadata["image"] = strings.TrimSpace(image)
	}
	if datePublished, ok := obj["datePublished"].(string); ok && strings.TrimSpace(datePublished) != "" {
		metadata["release_date"] = strings.TrimSpace(datePublished)
	}
	return metadata
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
