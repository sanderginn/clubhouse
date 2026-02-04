package links

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"sync"

	"github.com/Noooste/azuretls-client"
)

var (
	bandcampSession     *azuretls.Session
	bandcampSessionOnce sync.Once
)

// getBandcampSession returns a shared azuretls session for Bandcamp requests.
// The session mimics Chrome's TLS fingerprint to bypass Bandcamp's WAF.
func getBandcampSession() *azuretls.Session {
	bandcampSessionOnce.Do(func() {
		bandcampSession = azuretls.NewSession()
		bandcampSession.SetTimeout(fetchTimeout)
	})
	return bandcampSession
}

// fetchBandcampHTML fetches HTML from a Bandcamp URL using azuretls-client,
// which mimics Chrome's TLS/HTTP2 fingerprint to bypass Bandcamp's WAF.
func fetchBandcampHTML(ctx context.Context, rawURL string) ([]byte, error) {
	if ctx == nil {
		return nil, errors.New("context is required")
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parse url: %w", err)
	}
	if !isBandcampHost(parsed.Hostname()) {
		return nil, errors.New("not a bandcamp url")
	}

	session := getBandcampSession()

	// Create a channel to handle context cancellation
	type result struct {
		body []byte
		err  error
	}
	resultCh := make(chan result, 1)

	go func() {
		resp, err := session.Get(rawURL)
		if err != nil {
			resultCh <- result{nil, fmt.Errorf("bandcamp fetch: %w", err)}
			return
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 400 {
			resultCh <- result{nil, fmt.Errorf("bandcamp status: %d", resp.StatusCode)}
			return
		}
		body := resp.Body
		if len(body) > maxBodyBytes {
			body = body[:maxBodyBytes]
		}
		resultCh <- result{body, nil}
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case r := <-resultCh:
		return r.body, r.err
	}
}

// fetchBandcampMetadata fetches and extracts metadata from a Bandcamp URL.
func fetchBandcampMetadata(ctx context.Context, rawURL string) (map[string]interface{}, error) {
	if ctx == nil {
		return nil, errors.New("context is required")
	}

	ctx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parse url: %w", err)
	}

	body, err := bandcampFetchHTMLFunc(ctx, rawURL)
	if err != nil {
		return nil, err
	}

	// Extract metadata from HTML
	metaTags, title := extractHTMLMeta(body)
	title = firstNonEmpty(metaTags["og:title"], metaTags["twitter:title"], title)
	description := firstNonEmpty(metaTags["og:description"], metaTags["twitter:description"], metaTags["description"])
	image := firstNonEmpty(metaTags["og:image:secure_url"], metaTags["og:image"], metaTags["twitter:image"], metaTags["twitter:image:src"])
	siteName := firstNonEmpty(metaTags["og:site_name"], metaTags["application-name"])
	author := firstNonEmpty(metaTags["author"], metaTags["twitter:creator"])
	artist := firstNonEmpty(metaTags["music:artist"], metaTags["music:musician"], metaTags["spotify:artist"])
	ogType := metaTags["og:type"]

	metadata := make(map[string]interface{})
	if title != "" {
		metadata["title"] = title
	}
	if description != "" {
		metadata["description"] = description
	}
	if image != "" {
		metadata["image"] = resolveURL(parsed, image)
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
	metadata["provider"] = "bandcamp"

	// Extract embed data
	if embed := extractBandcampEmbedFromHTML(ctx, rawURL, metaTags); embed != nil {
		metadata["embed"] = embed
	}

	return metadata, nil
}

// extractBandcampEmbedFromHTML extracts embed data from Bandcamp HTML meta tags.
func extractBandcampEmbedFromHTML(ctx context.Context, rawURL string, metaTags map[string]string) *EmbedData {
	extractor := BandcampExtractor{}
	embed, err := extractor.ExtractFromHTML(ctx, rawURL, nil, metaTags)
	if err != nil {
		return nil
	}
	return embed
}

// CloseBandcampSession closes the shared azuretls session.
// Call this during application shutdown.
func CloseBandcampSession() {
	if bandcampSession != nil {
		bandcampSession.Close()
	}
}

// bandcampFetchHTMLFunc is the function used to fetch Bandcamp HTML.
// It can be overridden for testing.
var bandcampFetchHTMLFunc = fetchBandcampHTML

// SetBandcampFetchHTMLForTests overrides the Bandcamp HTML fetch function for testing.
func SetBandcampFetchHTMLForTests(fn func(context.Context, string) ([]byte, error)) {
	if fn == nil {
		bandcampFetchHTMLFunc = fetchBandcampHTML
		return
	}
	bandcampFetchHTMLFunc = fn
}

// ResetBandcampSessionForTests resets the Bandcamp session for testing.
// This allows tests to reinitialize the session.
func ResetBandcampSessionForTests() {
	if bandcampSession != nil {
		bandcampSession.Close()
		bandcampSession = nil
	}
	bandcampSessionOnce = sync.Once{}
}
