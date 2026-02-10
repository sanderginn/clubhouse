package services

import (
	"errors"
	"net/url"
	"strings"
	"unicode"

	"github.com/sanderginn/clubhouse/internal/models"
)

var errPodcastKindSelectionRequired = errors.New("podcast kind could not be detected; explicit selection required")

func shouldDetectPodcastKinds(links []models.LinkRequest) bool {
	for _, link := range links {
		if link.Podcast == nil {
			continue
		}
		if strings.TrimSpace(link.Podcast.Kind) == "" {
			return true
		}
	}
	return false
}

func buildPodcastKindDetectionHints(requested []models.LinkRequest, existing []models.Link) []models.JSONMap {
	if len(requested) == 0 || len(existing) == 0 {
		return nil
	}

	existingByURL := make(map[string][]models.JSONMap)
	for _, link := range existing {
		if len(link.Metadata) == 0 {
			continue
		}
		existingByURL[link.URL] = append(existingByURL[link.URL], models.JSONMap(link.Metadata))
	}

	hints := make([]models.JSONMap, len(requested))
	hasHint := false
	for i, link := range requested {
		candidates := existingByURL[link.URL]
		if len(candidates) == 0 {
			continue
		}
		hints[i] = candidates[0]
		existingByURL[link.URL] = candidates[1:]
		hasHint = true
	}

	if !hasHint {
		return nil
	}
	return hints
}

func mergePodcastKindDetectionHints(primary []models.JSONMap, secondary []models.JSONMap) []models.JSONMap {
	if len(primary) == 0 {
		return secondary
	}
	if len(secondary) == 0 {
		return primary
	}

	maxLen := len(primary)
	if len(secondary) > maxLen {
		maxLen = len(secondary)
	}

	merged := make([]models.JSONMap, maxLen)
	copy(merged, primary)
	for i := 0; i < maxLen; i++ {
		if len(merged[i]) > 0 {
			continue
		}
		if i < len(secondary) && len(secondary[i]) > 0 {
			merged[i] = secondary[i]
		}
	}
	return merged
}

func resolvePodcastKinds(sectionType string, links []models.LinkRequest, metadataHints []models.JSONMap) ([]models.LinkRequest, error) {
	if sectionType != "podcast" || len(links) == 0 {
		return links, nil
	}

	resolved := make([]models.LinkRequest, len(links))
	copy(resolved, links)

	for i := range resolved {
		podcast := resolved[i].Podcast
		if podcast == nil {
			continue
		}
		if strings.TrimSpace(podcast.Kind) != "" {
			continue
		}

		if len(podcast.HighlightEpisodes) > 0 {
			updated := *podcast
			updated.Kind = "show"
			resolved[i].Podcast = &updated
			continue
		}

		var hint models.JSONMap
		if i < len(metadataHints) {
			hint = metadataHints[i]
		}

		kind, ok := detectPodcastKind(resolved[i].URL, hint)
		if !ok {
			return nil, errPodcastKindSelectionRequired
		}

		updated := *podcast
		updated.Kind = kind
		resolved[i].Podcast = &updated
	}

	return resolved, nil
}

func detectPodcastKind(linkURL string, metadata models.JSONMap) (string, bool) {
	if kind, ok := detectPodcastKindFromMetadata(metadata); ok {
		return kind, true
	}
	return detectPodcastKindFromURL(linkURL)
}

func detectPodcastKindFromMetadata(metadata models.JSONMap) (string, bool) {
	if len(metadata) == 0 {
		return "", false
	}

	for _, key := range []string{"kind", "type", "content_type", "resource_type", "og:type"} {
		if kind, ok := podcastKindFromText(metadataStringField(metadata, key)); ok {
			return kind, true
		}
	}

	for _, key := range []string{"url", "canonical_url"} {
		if kind, ok := detectPodcastKindFromURL(metadataStringField(metadata, key)); ok {
			return kind, true
		}
	}

	embed := metadataMapField(metadata, "embed")
	if len(embed) > 0 {
		for _, key := range []string{"embed_url", "url"} {
			if kind, ok := detectPodcastKindFromURL(metadataStringField(embed, key)); ok {
				return kind, true
			}
		}
		if kind, ok := podcastKindFromText(metadataStringField(embed, "type")); ok {
			return kind, true
		}
	}

	return "", false
}

func detectPodcastKindFromURL(rawURL string) (string, bool) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Host == "" {
		return "", false
	}

	host := strings.ToLower(strings.TrimSuffix(parsed.Hostname(), "."))
	pathSegments := splitPathSegments(parsed.Path)

	if host == "open.spotify.com" {
		if kind, ok := detectSpotifyPodcastKind(host, pathSegments); ok {
			return kind, true
		}
		if segmentsContainAny(pathSegments, "show", "episode") {
			return "", false
		}
	}

	query := parsed.Query()
	if strings.HasSuffix(host, "podcasts.apple.com") {
		if strings.TrimSpace(query.Get("i")) != "" {
			return "episode", true
		}
		if segmentsContainAny(pathSegments, "podcast", "podcasts") {
			return "show", true
		}
	}

	if hasEpisodeQueryParam(query) {
		return "episode", true
	}

	if segmentsContainAny(pathSegments, "episode", "episodes") {
		return "episode", true
	}
	if segmentsContainAny(pathSegments, "show", "shows") {
		return "show", true
	}
	if segmentsContainAny(pathSegments, "podcast", "podcasts") {
		return "show", true
	}

	return "", false
}

func detectSpotifyPodcastKind(host string, pathSegments []string) (string, bool) {
	if host != "open.spotify.com" || len(pathSegments) == 0 {
		return "", false
	}

	index := 0
	for index < len(pathSegments) {
		segment := pathSegments[index]
		if strings.HasPrefix(segment, "intl-") || segment == "embed" {
			index++
			continue
		}
		break
	}

	if len(pathSegments) <= index+1 {
		return "", false
	}

	switch pathSegments[index] {
	case "show":
		return "show", true
	case "episode":
		return "episode", true
	default:
		return "", false
	}
}

func hasEpisodeQueryParam(query url.Values) bool {
	for _, key := range []string{"i", "episode", "episode_id", "episodeid"} {
		if strings.TrimSpace(query.Get(key)) != "" {
			return true
		}
	}
	return false
}

func splitPathSegments(path string) []string {
	trimmed := strings.Trim(path, "/")
	if trimmed == "" {
		return nil
	}
	parts := strings.Split(trimmed, "/")
	segments := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.ToLower(strings.TrimSpace(part))
		if part == "" {
			continue
		}
		segments = append(segments, part)
	}
	return segments
}

func segmentsContainAny(segments []string, tokens ...string) bool {
	if len(segments) == 0 {
		return false
	}
	tokenSet := make(map[string]struct{}, len(tokens))
	for _, token := range tokens {
		token = strings.ToLower(strings.TrimSpace(token))
		if token == "" {
			continue
		}
		tokenSet[token] = struct{}{}
	}
	for _, segment := range segments {
		for _, part := range splitTextTokens(segment) {
			if _, ok := tokenSet[part]; ok {
				return true
			}
		}
	}
	return false
}

func podcastKindFromText(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", false
	}

	for _, token := range splitTextTokens(value) {
		switch token {
		case "episode", "episodes":
			return "episode", true
		case "show", "shows", "podcast", "podcasts":
			return "show", true
		}
	}

	return "", false
}

func splitTextTokens(value string) []string {
	fields := strings.FieldsFunc(strings.ToLower(value), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})
	if len(fields) == 0 {
		return nil
	}
	return fields
}

func metadataStringField(metadata map[string]interface{}, key string) string {
	raw, ok := metadata[key]
	if !ok {
		return ""
	}
	value, ok := raw.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}

func metadataMapField(metadata map[string]interface{}, key string) map[string]interface{} {
	raw, ok := metadata[key]
	if !ok || raw == nil {
		return nil
	}
	switch typed := raw.(type) {
	case map[string]interface{}:
		return typed
	case models.JSONMap:
		return map[string]interface{}(typed)
	default:
		return nil
	}
}
