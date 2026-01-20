package links

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html"
)

const (
	fetchTimeout = 5 * time.Second
	maxBodyBytes = 2 << 20 // 2MB
)

// Fetcher retrieves metadata for links.
type Fetcher struct {
	client *http.Client
}

// NewFetcher creates a new fetcher with a default client if none is provided.
func NewFetcher(client *http.Client) *Fetcher {
	if client == nil {
		client = &http.Client{Timeout: fetchTimeout}
	}
	return &Fetcher{client: client}
}

var defaultFetcher = NewFetcher(nil)

// FetchMetadata fetches metadata for a URL using the default fetcher.
func FetchMetadata(ctx context.Context, rawURL string) (map[string]interface{}, error) {
	return defaultFetcher.Fetch(ctx, rawURL)
}

// Fetch retrieves metadata for the provided URL.
func (f *Fetcher) Fetch(ctx context.Context, rawURL string) (map[string]interface{}, error) {
	if ctx == nil {
		return nil, errors.New("context is required")
	}

	ctx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()

	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parse url: %w", err)
	}
	if u.Scheme == "" {
		return nil, errors.New("missing url scheme")
	}
	if u.Host == "" {
		return nil, errors.New("missing url host")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, errors.New("unsupported url scheme")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", "ClubhouseMetadataFetcher/1.0")

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch url: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("unexpected status: %s", resp.Status)
	}

	contentType := resp.Header.Get("Content-Type")
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	metadata := make(map[string]interface{})
	provider := detectProvider(u.Hostname())

	if strings.Contains(strings.ToLower(contentType), "text/html") {
		metaTags, title := extractHTMLMeta(body)
		title = firstNonEmpty(metaTags["og:title"], metaTags["twitter:title"], title)
		description := firstNonEmpty(metaTags["og:description"], metaTags["twitter:description"], metaTags["description"])
		image := firstNonEmpty(metaTags["og:image:secure_url"], metaTags["og:image"], metaTags["twitter:image"], metaTags["twitter:image:src"])
		siteName := firstNonEmpty(metaTags["og:site_name"], metaTags["application-name"])
		author := firstNonEmpty(metaTags["author"], metaTags["twitter:creator"])
		artist := firstNonEmpty(metaTags["music:artist"], metaTags["music:musician"], metaTags["spotify:artist"])
		ogType := metaTags["og:type"]

		if title != "" {
			metadata["title"] = title
		}
		if description != "" {
			metadata["description"] = description
		}
		if image != "" {
			metadata["image"] = resolveURL(u, image)
		}
		if siteName != "" {
			metadata["site_name"] = siteName
		}
		if author != "" {
			metadata["author"] = author
		}
		if artist != "" {
			metadata["artist"] = artist
		}
		if ogType != "" {
			metadata["type"] = ogType
		}
		if provider == "" && siteName != "" {
			provider = siteName
		}
	}

	if provider == "" {
		provider = u.Hostname()
	}
	if provider != "" {
		metadata["provider"] = provider
	}

	return metadata, nil
}

func extractHTMLMeta(body []byte) (map[string]string, string) {
	metaTags := make(map[string]string)
	if len(body) == 0 {
		return metaTags, ""
	}

	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return metaTags, ""
	}

	var title string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch strings.ToLower(n.Data) {
			case "meta":
				var key, content string
				for _, attr := range n.Attr {
					switch strings.ToLower(attr.Key) {
					case "property", "name":
						key = strings.ToLower(strings.TrimSpace(attr.Val))
					case "content":
						content = strings.TrimSpace(attr.Val)
					}
				}
				if key != "" && content != "" {
					if _, exists := metaTags[key]; !exists {
						metaTags[key] = content
					}
				}
			case "title":
				if n.FirstChild != nil {
					title = strings.TrimSpace(n.FirstChild.Data)
				}
			}
		}

		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)

	return metaTags, title
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func resolveURL(base *url.URL, ref string) string {
	parsed, err := url.Parse(ref)
	if err != nil {
		return ref
	}
	if parsed.IsAbs() {
		return parsed.String()
	}
	return base.ResolveReference(parsed).String()
}

func detectProvider(host string) string {
	host = strings.ToLower(host)
	switch {
	case strings.Contains(host, "spotify.com"):
		return "spotify"
	case strings.Contains(host, "youtube.com"), strings.Contains(host, "youtu.be"):
		return "youtube"
	case strings.Contains(host, "imdb.com"):
		return "imdb"
	case strings.Contains(host, "soundcloud.com"):
		return "soundcloud"
	case strings.Contains(host, "bandcamp.com"):
		return "bandcamp"
	case strings.Contains(host, "vimeo.com"):
		return "vimeo"
	default:
		return ""
	}
}
