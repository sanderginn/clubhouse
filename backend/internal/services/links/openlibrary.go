package links

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/sanderginn/clubhouse/internal/observability"
)

const (
	openLibraryDefaultBaseURL    = "https://openlibrary.org"
	openLibraryCoverBaseURL      = "https://covers.openlibrary.org/b/id"
	openLibraryUserAgent         = "ClubhouseOpenLibraryClient/1.0"
	openLibraryDefaultTimeout    = 10 * time.Second
	openLibraryErrorBodyMaxBytes = 4096
)

var (
	ErrOpenLibraryNotFound = errors.New("open library resource not found")
)

// OpenLibraryAPIError captures non-success responses from Open Library.
type OpenLibraryAPIError struct {
	StatusCode int
	Message    string
}

func (e *OpenLibraryAPIError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("open library api error (%d): %s", e.StatusCode, e.Message)
}

func (e *OpenLibraryAPIError) Unwrap() error {
	if e == nil {
		return nil
	}
	if e.StatusCode == http.StatusNotFound {
		return ErrOpenLibraryNotFound
	}
	return nil
}

// OpenLibraryClient provides methods for interacting with the Open Library API.
type OpenLibraryClient struct {
	baseURL    string
	httpClient *http.Client
}

// OLSearchResult is a single result from Open Library search.
type OLSearchResult struct {
	Title            string   `json:"title"`
	AuthorName       []string `json:"author_name"`
	FirstPublishYear int      `json:"first_publish_year"`
	CoverID          int      `json:"cover_i"`
	Key              string   `json:"key"`
}

// OLWorkReference points to a related Open Library entity key.
type OLWorkReference struct {
	Key string `json:"key"`
}

// OLWorkAuthor represents work-level author references.
type OLWorkAuthor struct {
	Author OLWorkReference `json:"author"`
	Type   OLWorkReference `json:"type"`
}

// OLWork contains book work metadata.
type OLWork struct {
	Title       string         `json:"title"`
	Description string         `json:"-"`
	Subjects    []string       `json:"subjects"`
	Covers      []int          `json:"covers"`
	Authors     []OLWorkAuthor `json:"authors"`
}

func (w *OLWork) UnmarshalJSON(data []byte) error {
	type olWorkPayload struct {
		Title       string          `json:"title"`
		Description json.RawMessage `json:"description"`
		Subjects    []string        `json:"subjects"`
		Covers      []int           `json:"covers"`
		Authors     []OLWorkAuthor  `json:"authors"`
	}

	var payload olWorkPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}

	w.Title = payload.Title
	w.Description = parseOpenLibraryDescription(payload.Description)
	w.Subjects = payload.Subjects
	w.Covers = payload.Covers
	w.Authors = payload.Authors

	return nil
}

// OLEdition contains edition metadata.
type OLEdition struct {
	Title         string   `json:"title"`
	Publishers    []string `json:"publishers"`
	PublishDate   string   `json:"publish_date"`
	NumberOfPages int      `json:"number_of_pages"`
	ISBN13        []string `json:"isbn_13"`
	ISBN10        []string `json:"isbn_10"`
	Covers        []int    `json:"covers"`
}

// NewOpenLibraryClient creates a client with a configurable timeout.
func NewOpenLibraryClient(timeout time.Duration) *OpenLibraryClient {
	if timeout <= 0 {
		timeout = openLibraryDefaultTimeout
	}

	return &OpenLibraryClient{
		baseURL: openLibraryDefaultBaseURL,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// NewOpenLibraryClientWithHTTPClient creates a client using a caller-provided HTTP client.
func NewOpenLibraryClientWithHTTPClient(httpClient *http.Client) *OpenLibraryClient {
	if httpClient == nil {
		return NewOpenLibraryClient(openLibraryDefaultTimeout)
	}

	return &OpenLibraryClient{
		baseURL:    openLibraryDefaultBaseURL,
		httpClient: httpClient,
	}
}

// SearchBooks searches Open Library by title or author query.
func (c *OpenLibraryClient) SearchBooks(ctx context.Context, query string) ([]OLSearchResult, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, errors.New("search query is required")
	}

	values := url.Values{}
	values.Set("q", query)

	var payload struct {
		Docs []OLSearchResult `json:"docs"`
	}
	if err := c.get(ctx, "/search.json", values, &payload); err != nil {
		return nil, err
	}

	return payload.Docs, nil
}

// GetWork fetches work metadata by key (for example /works/OL45883W).
func (c *OpenLibraryClient) GetWork(ctx context.Context, workKey string) (*OLWork, error) {
	path, err := normalizeOpenLibraryPath(workKey, "works")
	if err != nil {
		return nil, err
	}

	var work OLWork
	if err := c.get(ctx, path, nil, &work); err != nil {
		return nil, err
	}

	return &work, nil
}

// GetEdition fetches edition metadata by key.
func (c *OpenLibraryClient) GetEdition(ctx context.Context, editionKey string) (*OLEdition, error) {
	path, err := normalizeOpenLibraryPath(editionKey, "books")
	if err != nil {
		return nil, err
	}

	var edition OLEdition
	if err := c.get(ctx, path, nil, &edition); err != nil {
		return nil, err
	}

	return &edition, nil
}

// GetByISBN fetches an edition by ISBN.
func (c *OpenLibraryClient) GetByISBN(ctx context.Context, isbn string) (*OLEdition, error) {
	isbn = strings.TrimSpace(isbn)
	if isbn == "" {
		return nil, errors.New("isbn is required")
	}

	var edition OLEdition
	path := "/isbn/" + url.PathEscape(isbn) + ".json"
	if err := c.get(ctx, path, nil, &edition); err != nil {
		return nil, err
	}

	return &edition, nil
}

// CoverURL constructs a cover image URL for a cover ID and size (S, M, L).
func (c *OpenLibraryClient) CoverURL(coverID int, size string) string {
	if coverID <= 0 {
		return ""
	}

	size = strings.ToUpper(strings.TrimSpace(size))
	switch size {
	case "S", "M", "L":
	default:
		size = "M"
	}

	return fmt.Sprintf("%s/%d-%s.jpg", openLibraryCoverBaseURL, coverID, size)
}

func (c *OpenLibraryClient) get(ctx context.Context, path string, query url.Values, out interface{}) error {
	if ctx == nil {
		return errors.New("context is required")
	}
	if c == nil {
		return errors.New("open library client is required")
	}
	if c.httpClient == nil {
		return errors.New("http client is required")
	}

	base := strings.TrimSuffix(c.baseURL, "/")
	requestURL := fmt.Sprintf("%s%s", base, path)
	if encoded := query.Encode(); encoded != "" {
		requestURL = requestURL + "?" + encoded
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return fmt.Errorf("build open library request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", openLibraryUserAgent)

	start := time.Now()
	resp, err := c.httpClient.Do(req)
	duration := time.Since(start)
	if err != nil {
		observability.LogWarn(ctx, "open library request failed", "path", path, "duration_ms", strconv.FormatInt(duration.Milliseconds(), 10), "error", err.Error())
		return fmt.Errorf("open library request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		apiErr := parseOpenLibraryAPIError(resp)
		observability.LogWarn(ctx, "open library request failed", "path", path, "status_code", strconv.Itoa(resp.StatusCode), "duration_ms", strconv.FormatInt(duration.Milliseconds(), 10), "error", apiErr.Error())
		return apiErr
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		observability.LogWarn(ctx, "open library response decode failed", "path", path, "duration_ms", strconv.FormatInt(duration.Milliseconds(), 10), "error", err.Error())
		return fmt.Errorf("decode open library response: %w", err)
	}

	observability.LogDebug(ctx, "open library request completed", "path", path, "duration_ms", strconv.FormatInt(duration.Milliseconds(), 10), "status_code", strconv.Itoa(resp.StatusCode))

	return nil
}

func parseOpenLibraryAPIError(resp *http.Response) error {
	message := strings.TrimSpace(resp.Status)
	body, err := io.ReadAll(io.LimitReader(resp.Body, openLibraryErrorBodyMaxBytes))
	if err == nil && len(body) > 0 {
		message = extractOpenLibraryErrorMessage(body, message)
	}

	if message == "" {
		message = http.StatusText(resp.StatusCode)
	}

	return &OpenLibraryAPIError{
		StatusCode: resp.StatusCode,
		Message:    message,
	}
}

func extractOpenLibraryErrorMessage(body []byte, fallback string) string {
	trimmedBody := strings.TrimSpace(string(body))
	if trimmedBody == "" {
		return fallback
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err == nil {
		for _, key := range []string{"error", "message"} {
			if raw, ok := payload[key]; ok {
				if value, ok := raw.(string); ok && strings.TrimSpace(value) != "" {
					return strings.TrimSpace(value)
				}
			}
		}
	}

	return trimmedBody
}

func normalizeOpenLibraryPath(rawKey string, resource string) (string, error) {
	identifier, err := normalizeOpenLibraryIdentifier(rawKey, resource)
	if err != nil {
		return "", err
	}
	return "/" + resource + "/" + identifier + ".json", nil
}

func normalizeOpenLibraryIdentifier(rawKey, resource string) (string, error) {
	key := strings.TrimSpace(rawKey)
	if key == "" {
		return "", fmt.Errorf("%s key is required", resource)
	}

	if strings.HasPrefix(key, "http://") || strings.HasPrefix(key, "https://") {
		parsed, err := url.Parse(key)
		if err != nil {
			return "", fmt.Errorf("parse %s key: %w", resource, err)
		}
		key = parsed.Path
	}

	key = strings.TrimSpace(strings.TrimSuffix(key, ".json"))
	key = strings.TrimPrefix(key, "/")

	resourcePrefix := resource + "/"
	switch {
	case strings.HasPrefix(key, resourcePrefix):
		key = strings.TrimPrefix(key, resourcePrefix)
	case strings.Contains(key, "/"):
		return "", fmt.Errorf("%s key must use /%s/<id> format", resource, resource)
	}

	key = strings.TrimSpace(key)
	if key == "" || strings.Contains(key, "/") {
		return "", fmt.Errorf("invalid %s key", resource)
	}

	return url.PathEscape(key), nil
}

func parseOpenLibraryDescription(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}

	var description string
	if err := json.Unmarshal(raw, &description); err == nil {
		return strings.TrimSpace(description)
	}

	var object struct {
		Value string `json:"value"`
	}
	if err := json.Unmarshal(raw, &object); err == nil {
		return strings.TrimSpace(object.Value)
	}

	return ""
}
