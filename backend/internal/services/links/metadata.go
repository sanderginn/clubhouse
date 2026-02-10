package links

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sanderginn/clubhouse/internal/observability"
	"golang.org/x/net/html"
)

const (
	fetchTimeout     = 5 * time.Second
	maxBodyBytes     = 2 << 20 // 2MB
	maxRedirects     = 5
	maxFetchRetries  = 2
	retryBackoffBase = 75 * time.Millisecond
	retryBackoffMax  = 300 * time.Millisecond
	defaultUserAgent = "ClubhouseMetadataFetcher/1.0"
	imdbUserAgent    = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36"
)

type metadataContextKey string

const metadataSectionTypeContextKey metadataContextKey = "link_metadata_section_type"

var (
	newTMDBClientFromEnvFunc = NewTMDBClientFromEnv
	newOMDBClientFromEnvFunc = NewOMDBClientFromEnv
	parseMovieMetadataFunc   = ParseMovieMetadata
	parseBookMetadataFunc    = ParseBookMetadata

	rottenTomatoesScoreAttrPattern = regexp.MustCompile(`(?i)tomatometerscore\s*=\s*["'](\d{1,3})["']`)
	rottenTomatoesScoreJSONPattern = regexp.MustCompile(`(?is)"tomatometerScore"\s*:\s*\{.*?"score"\s*:\s*(\d{1,3})`)
	rottenTomatoesCriticsPattern   = regexp.MustCompile(`(?i)"criticsScore"\s*:\s*(\d{1,3})`)

	omdbClientFromEnvOnce sync.Once
	omdbClientFromEnv     *OMDBClient
	omdbClientFromEnvErr  error
)

// Fetcher retrieves metadata for links.
type Fetcher struct {
	client   *http.Client
	resolver IPResolver
}

type IPResolver interface {
	LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error)
}

// NewFetcher creates a new fetcher with a default client if none is provided.
func NewFetcher(client *http.Client) *Fetcher {
	if client == nil {
		client = &http.Client{Timeout: fetchTimeout}
	}
	return &Fetcher{
		client:   client,
		resolver: net.DefaultResolver,
	}
}

var defaultFetcher = NewFetcher(nil)

// SetDefaultFetcher overrides the default fetcher (primarily for tests).
func SetDefaultFetcher(fetcher *Fetcher) {
	if fetcher == nil {
		defaultFetcher = NewFetcher(nil)
		return
	}
	defaultFetcher = fetcher
}

// SetResolver overrides the DNS resolver used by the fetcher.
func (f *Fetcher) SetResolver(resolver IPResolver) {
	f.resolver = resolver
}

// FetchMetadata fetches metadata for a URL using the default fetcher.
func FetchMetadata(ctx context.Context, rawURL string) (map[string]interface{}, error) {
	return fetchMetadataFunc(ctx, rawURL)
}

var fetchMetadataFunc = func(ctx context.Context, rawURL string) (map[string]interface{}, error) {
	return defaultFetcher.Fetch(ctx, rawURL)
}

// SetFetchMetadataFuncForTests overrides the default fetcher. Use only in tests.
func SetFetchMetadataFuncForTests(fn func(context.Context, string) (map[string]interface{}, error)) {
	if fn == nil {
		fetchMetadataFunc = defaultFetcher.Fetch
		return
	}
	fetchMetadataFunc = fn
}

// WithMetadataSectionType stores section type metadata used by extractor-specific logic.
func WithMetadataSectionType(ctx context.Context, sectionType string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	sectionType = strings.ToLower(strings.TrimSpace(sectionType))
	return context.WithValue(ctx, metadataSectionTypeContextKey, sectionType)
}

// Fetch retrieves metadata for the provided URL.
func (f *Fetcher) Fetch(ctx context.Context, rawURL string) (map[string]interface{}, error) {
	if ctx == nil {
		return nil, errors.New("context is required")
	}

	fetchCtx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()

	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parse url: %w", err)
	}
	if err := f.validateURL(fetchCtx, u); err != nil {
		return nil, err
	}

	// Use azuretls-client for Bandcamp to bypass their WAF
	if isBandcampHost(u.Hostname()) {
		return fetchBandcampMetadata(fetchCtx, rawURL)
	}

	var movieMetadata *MovieData
	movieMetadataLoaded := false
	getMovieMetadata := func() *MovieData {
		if movieMetadataLoaded {
			return movieMetadata
		}
		movieMetadataLoaded = true

		movieCtx, movieCancel := context.WithTimeout(ctx, fetchTimeout)
		defer movieCancel()
		movieMetadata = fetchMovieMetadata(movieCtx, rawURL)
		return movieMetadata
	}

	provider := detectProvider(u.Hostname())
	bookData, bookErr := f.extractBookMetadata(ctx, rawURL)
	if bookErr != nil {
		observability.LogWarn(ctx, "book metadata extraction failed", "url", rawURL, "error", bookErr.Error())
	}

	client := f.client
	if client == nil {
		client = &http.Client{Timeout: fetchTimeout}
	}
	clientCopy := *client
	clientCopy.CheckRedirect = f.redirectValidator(fetchCtx, client.CheckRedirect)

	resp, err := f.doRequestWithRetry(fetchCtx, &clientCopy, u)
	if err != nil {
		if bookData != nil {
			return buildBookMetadataOnlyResponse(u, provider, bookData), nil
		}
		if fallback := fallbackMetadataForMovieURL(ctx, u, getMovieMetadata()); fallback != nil {
			return fallback, nil
		}
		return nil, fmt.Errorf("fetch url: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusBadRequest {
		if bookData != nil {
			return buildBookMetadataOnlyResponse(u, provider, bookData), nil
		}
		if fallback := fallbackMetadataForMovieURL(ctx, u, getMovieMetadata()); fallback != nil {
			return fallback, nil
		}
		return nil, fmt.Errorf("unexpected status: %s", resp.Status)
	}

	contentType := resp.Header.Get("Content-Type")
	contentTypeLower := strings.ToLower(contentType)
	isHTML := strings.Contains(contentTypeLower, "text/html")
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		if fallback := fallbackMetadataForMovieURL(ctx, u, getMovieMetadata()); fallback != nil {
			return fallback, nil
		}
		return nil, fmt.Errorf("read response: %w", err)
	}

	metadata := make(map[string]interface{})

	// Treat SVGs as images here; frontend renders via <img> to avoid inline SVG execution.
	if strings.HasPrefix(contentTypeLower, "image/") {
		metadata["image"] = u.String()
		metadata["type"] = "image"
	}

	if isHTML {
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
		if recipe := parseRecipeIfPresent(body, u.Hostname()); recipe != nil {
			if recipe.Image != "" {
				recipe.Image = resolveURL(u, recipe.Image)
			}
			metadata["recipe"] = recipe
		}
		if embed := extractEmbed(fetchCtx, rawURL, body, metaTags); embed != nil {
			metadata["embed"] = embed
		}
	}
	if movie := getMovieMetadata(); movie != nil {
		enrichMovieDataWithRottenTomatoesScoreFromHTML(movie, u, body)
		metadata["movie"] = movie
	}
	if bookData != nil {
		metadata["book_data"] = bookData
	}

	if _, ok := metadata["image"]; !ok && !isHTML && looksLikeImageURL(u) {
		metadata["image"] = u.String()
		metadata["type"] = "image"
	}

	if provider == "" {
		provider = u.Hostname()
	}
	if provider != "" {
		metadata["provider"] = provider
	}

	return metadata, nil
}

func (f *Fetcher) extractBookMetadata(ctx context.Context, rawURL string) (*BookData, error) {
	if !shouldExtractBookMetadata(rawURL) {
		return nil, nil
	}

	bookClient := NewOpenLibraryClient(openLibraryDefaultTimeout)
	return parseBookMetadataFunc(ctx, rawURL, bookClient)
}

func buildBookMetadataOnlyResponse(u *url.URL, provider string, bookData *BookData) map[string]interface{} {
	metadata := map[string]interface{}{
		"book_data": bookData,
	}

	if provider == "" && u != nil {
		provider = u.Hostname()
	}
	if provider != "" {
		metadata["provider"] = provider
	}

	return metadata
}

func getOMDBClientFromEnv() (*OMDBClient, error) {
	omdbClientFromEnvOnce.Do(func() {
		omdbClientFromEnv, omdbClientFromEnvErr = newOMDBClientFromEnvFunc()
	})
	return omdbClientFromEnv, omdbClientFromEnvErr
}

// resetOMDBClientFromEnvCacheForTests resets cached OMDb client state. Use only in tests.
func resetOMDBClientFromEnvCacheForTests() {
	omdbClientFromEnvOnce = sync.Once{}
	omdbClientFromEnv = nil
	omdbClientFromEnvErr = nil
}

func shouldExtractMovieMetadata(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	sectionType, _ := ctx.Value(metadataSectionTypeContextKey).(string)
	return sectionType == "movie" || sectionType == "series"
}

func fetchMovieMetadata(ctx context.Context, rawURL string) *MovieData {
	if !shouldExtractMovieMetadata(ctx) {
		return nil
	}

	tmdbClient, err := newTMDBClientFromEnvFunc()
	if err != nil {
		return nil
	}

	var omdbClient *OMDBClient
	if omdb, omdbErr := getOMDBClientFromEnv(); omdbErr == nil {
		omdbClient = omdb
	}

	movie, movieErr := parseMovieMetadataFunc(ctx, rawURL, tmdbClient, omdbClient)
	if movieErr != nil || movie == nil {
		return nil
	}

	return movie
}

func fallbackMetadataForMovieURL(ctx context.Context, u *url.URL, movie *MovieData) map[string]interface{} {
	if !shouldExtractMovieMetadata(ctx) || movie == nil || u == nil {
		return nil
	}

	provider := detectProvider(u.Hostname())
	if provider == "" {
		provider = u.Hostname()
	}

	return map[string]interface{}{
		"movie":    movie,
		"provider": provider,
	}
}

func enrichMovieDataWithRottenTomatoesScoreFromHTML(movie *MovieData, u *url.URL, body []byte) {
	if movie == nil || movie.RottenTomatoesScore != nil || u == nil || len(body) == 0 {
		return
	}
	if !isRottenTomatoesHost(u.Hostname()) {
		return
	}

	score, ok := extractRottenTomatoesScoreFromHTML(body)
	if !ok {
		return
	}
	movie.RottenTomatoesScore = intPtr(score)
}

func extractRottenTomatoesScoreFromHTML(body []byte) (int, bool) {
	if len(body) == 0 {
		return 0, false
	}

	patterns := []*regexp.Regexp{
		rottenTomatoesScoreAttrPattern,
		rottenTomatoesScoreJSONPattern,
		rottenTomatoesCriticsPattern,
	}
	for _, pattern := range patterns {
		match := pattern.FindSubmatch(body)
		if len(match) < 2 {
			continue
		}

		score, err := strconv.Atoi(string(match[1]))
		if err != nil || score < 0 || score > 100 {
			continue
		}
		return score, true
	}

	return 0, false
}

func shouldExtractBookMetadata(rawURL string) bool {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return false
	}

	host := strings.ToLower(strings.TrimSpace(parsedURL.Hostname()))
	segments := splitURLPath(parsedURL.Path)

	switch {
	case isGoodreadsHost(host):
		_, ok := parseGoodreadsBookID(segments)
		return ok
	case isAmazonHost(host):
		if _, ok := parseAmazonASIN(segments); ok {
			return true
		}
		_, ok := extractISBNFromURL(parsedURL, segments)
		return ok
	case isOpenLibraryHost(host):
		if _, ok := parseOpenLibraryWorkKey(segments); ok {
			return true
		}
		if _, ok := parseOpenLibraryEditionKey(segments); ok {
			return true
		}
		_, ok := extractISBNFromURL(parsedURL, segments)
		return ok
	default:
		_, ok := extractISBNFromURL(parsedURL, segments)
		return ok
	}
}

func (f *Fetcher) doRequestWithRetry(ctx context.Context, client *http.Client, u *url.URL) (*http.Response, error) {
	if client == nil {
		client = &http.Client{Timeout: fetchTimeout}
	}
	var resp *http.Response
	var err error
	for attempt := 0; attempt <= maxFetchRetries; attempt++ {
		req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		if reqErr != nil {
			return nil, fmt.Errorf("build request: %w", reqErr)
		}
		applyRequestHeaders(req, u)

		resp, err = client.Do(req)
		if !shouldRetryFetch(ctx, err, resp) || attempt == maxFetchRetries {
			return resp, err
		}
		if resp != nil && resp.Body != nil {
			if _, copyErr := io.Copy(io.Discard, resp.Body); copyErr != nil {
				return nil, copyErr
			}
			resp.Body.Close()
		}
		backoff := retryBackoff(attempt)
		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
	return resp, err
}

func applyRequestHeaders(req *http.Request, u *url.URL) {
	if req == nil || u == nil {
		return
	}
	req.Header.Set("User-Agent", defaultUserAgent)
	if isIMDbHost(u.Hostname()) {
		req.Header.Set("User-Agent", imdbUserAgent)
		req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	}
}

func isBandcampHost(host string) bool {
	normalized := strings.ToLower(strings.TrimSpace(host))
	return normalized == "bandcamp.com" || strings.HasSuffix(normalized, ".bandcamp.com")
}

func shouldRetryFetch(ctx context.Context, err error, resp *http.Response) bool {
	if ctx == nil {
		return false
	}
	if ctx.Err() != nil {
		return false
	}
	if err != nil {
		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			return true
		}
		return false
	}
	if resp == nil {
		return false
	}
	switch resp.StatusCode {
	case http.StatusRequestTimeout, http.StatusTooManyRequests, http.StatusInternalServerError,
		http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

func retryBackoff(attempt int) time.Duration {
	if attempt < 0 {
		attempt = 0
	}
	backoff := retryBackoffBase * time.Duration(1<<attempt)
	if backoff > retryBackoffMax {
		return retryBackoffMax
	}
	return backoff
}

func (f *Fetcher) redirectValidator(ctx context.Context, existing func(req *http.Request, via []*http.Request) error) func(req *http.Request, via []*http.Request) error {
	return func(req *http.Request, via []*http.Request) error {
		if len(via) >= maxRedirects {
			return errors.New("too many redirects")
		}
		if err := f.validateURL(ctx, req.URL); err != nil {
			return err
		}
		if existing != nil {
			return existing(req, via)
		}
		return nil
	}
}

func (f *Fetcher) validateURL(ctx context.Context, u *url.URL) error {
	if u == nil {
		return errors.New("url is required")
	}
	if u.Scheme == "" {
		return errors.New("missing url scheme")
	}
	if u.Host == "" {
		return errors.New("missing url host")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return errors.New("unsupported url scheme")
	}

	host := strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
	if host == "" {
		return errors.New("missing url host")
	}
	if isBlockedHostname(host) {
		return fmt.Errorf("blocked host: %s", host)
	}

	if ip := net.ParseIP(host); ip != nil {
		if isBlockedIP(ip) {
			return fmt.Errorf("blocked ip: %s", host)
		}
		return nil
	}

	resolver := f.resolver
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	addrs, err := resolver.LookupIPAddr(ctx, host)
	if err != nil {
		return fmt.Errorf("resolve host: %w", err)
	}
	if len(addrs) == 0 {
		return errors.New("resolve host: no addresses")
	}
	for _, addr := range addrs {
		if isBlockedIP(addr.IP) {
			return fmt.Errorf("blocked ip: %s", addr.IP.String())
		}
	}

	return nil
}

// ClassifyFetchError returns a short error type for link metadata fetch failures.
func ClassifyFetchError(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return "timeout"
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return "timeout"
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "missing url"),
		strings.Contains(msg, "unsupported url scheme"),
		strings.Contains(msg, "parse url"):
		return "invalid_url"
	case strings.Contains(msg, "blocked host"),
		strings.Contains(msg, "blocked ip"):
		return "blocked"
	case strings.Contains(msg, "unexpected status"):
		return "http_status"
	case strings.Contains(msg, "resolve host"):
		return "dns"
	case strings.Contains(msg, "too many redirects"):
		return "redirect"
	default:
		return "fetch_error"
	}
}

// ExtractDomain returns a lowercased hostname for the provided URL string.
func ExtractDomain(rawURL string) string {
	if strings.TrimSpace(rawURL) == "" {
		return ""
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	if host == "" {
		return ""
	}
	return host
}

// IsInternalUploadURL reports whether rawURL points to the internal uploads endpoint.
func IsInternalUploadURL(rawURL string) bool {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return false
	}
	if strings.HasPrefix(trimmed, "/api/v1/uploads/") || trimmed == "/api/v1/uploads" {
		return true
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return false
	}
	path := strings.TrimSpace(parsed.Path)
	if path == "" {
		return false
	}
	if strings.HasPrefix(path, "/api/v1/uploads/") || path == "/api/v1/uploads" {
		return true
	}
	return false
}

func isBlockedHostname(host string) bool {
	switch host {
	case "localhost", "metadata.google.internal":
		return true
	}
	return strings.HasSuffix(host, ".localhost")
}

func isBlockedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsPrivate() || ip.IsUnspecified() {
		return true
	}
	return false
}

func looksLikeImageURL(u *url.URL) bool {
	if u == nil {
		return false
	}
	imageExtensions := []string{"jpg", "jpeg", "png", "gif", "webp", "bmp", "svg", "avif", "tif", "tiff"}
	hasImageExtension := func(value string) bool {
		if value == "" {
			return false
		}
		lower := strings.ToLower(value)
		for _, ext := range imageExtensions {
			needle := "." + ext
			idx := strings.LastIndex(lower, needle)
			if idx == -1 {
				continue
			}
			end := idx + len(needle)
			if end == len(lower) {
				return true
			}
			switch lower[end] {
			case '?', '#', '&':
				return true
			}
		}
		return false
	}

	if hasImageExtension(u.Path) {
		return true
	}

	if u.RawQuery == "" {
		return false
	}

	query := u.Query()
	for _, key := range []string{"format", "fm", "ext", "type"} {
		value := strings.ToLower(query.Get(key))
		switch value {
		case "jpg", "jpeg", "png", "gif", "webp", "bmp", "svg", "avif", "tif", "tiff", "image":
			return true
		}
	}

	for _, values := range query {
		for _, value := range values {
			if hasImageExtension(value) {
				return true
			}
		}
	}

	return false
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
