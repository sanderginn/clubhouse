package links

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sanderginn/clubhouse/internal/observability"
)

const (
	omdbAPIKeyEnv            = "OMDB_API_KEY"
	omdbDefaultBaseURL       = "https://www.omdbapi.com"
	omdbUserAgent            = "ClubhouseOMDBClient/1.0"
	omdbRequestTimeout       = 10 * time.Second
	omdbDefaultDailyLimit    = 1000
	omdbDefaultDailyWindow   = 24 * time.Hour
	omdbErrorBodyMaxBytes    = 4096
	omdbRatingSourceRT       = "rotten tomatoes"
	omdbRatingSourceMetaCrit = "metacritic"
)

var (
	ErrOMDBAPIKeyMissing = errors.New("omdb api key is required")
	ErrOMDBRateLimited   = errors.New("omdb rate limited")
	ErrOMDBNotFound      = errors.New("omdb resource not found")
)

// OMDBClient provides methods for interacting with the OMDb API.
type OMDBClient struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
	limiter    *omdbRateLimiter
}

// OMDBRatings contains normalized ratings from OMDb.
type OMDBRatings struct {
	RottenTomatoesScore *int
	MetacriticScore     *int
}

type omdbRating struct {
	Source string `json:"Source"`
	Value  string `json:"Value"`
}

type omdbTitleResponse struct {
	Response  string       `json:"Response"`
	Error     string       `json:"Error"`
	Ratings   []omdbRating `json:"Ratings"`
	Metascore string       `json:"Metascore"`
}

// OMDBAPIError captures non-success responses from OMDb.
type OMDBAPIError struct {
	StatusCode int
	Message    string
}

func (e *OMDBAPIError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("omdb api error (%d): %s", e.StatusCode, e.Message)
}

func (e *OMDBAPIError) Unwrap() error {
	if e == nil {
		return nil
	}

	if e.StatusCode == http.StatusTooManyRequests || strings.Contains(strings.ToLower(e.Message), "request limit reached") {
		return ErrOMDBRateLimited
	}

	message := strings.ToLower(e.Message)
	if strings.Contains(message, "movie not found") ||
		strings.Contains(message, "series not found") ||
		strings.Contains(message, "incorrect imdb id") {
		return ErrOMDBNotFound
	}

	return nil
}

// NewOMDBClientFromEnv creates an OMDb client using OMDB_API_KEY.
func NewOMDBClientFromEnv() (*OMDBClient, error) {
	apiKey := strings.TrimSpace(os.Getenv(omdbAPIKeyEnv))
	return NewOMDBClient(apiKey, nil)
}

// NewOMDBClient creates an OMDb client with an optional custom HTTP client.
func NewOMDBClient(apiKey string, httpClient *http.Client) (*OMDBClient, error) {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return nil, ErrOMDBAPIKeyMissing
	}

	if httpClient == nil {
		httpClient = &http.Client{Timeout: omdbRequestTimeout}
	}

	return &OMDBClient{
		apiKey:     apiKey,
		baseURL:    omdbDefaultBaseURL,
		httpClient: httpClient,
		limiter:    newOMDBRateLimiter(omdbDefaultDailyLimit, omdbDefaultDailyWindow),
	}, nil
}

// GetRatingsByIMDBID fetches Rotten Tomatoes and Metacritic scores for an IMDB ID.
func (c *OMDBClient) GetRatingsByIMDBID(ctx context.Context, imdbID string) (*OMDBRatings, error) {
	if ctx == nil {
		return nil, errors.New("context is required")
	}
	if c == nil {
		return nil, errors.New("omdb client is required")
	}

	imdbID = strings.ToLower(strings.TrimSpace(imdbID))
	if !imdbIDPattern.MatchString(imdbID) {
		return nil, errors.New("valid imdb id is required")
	}

	if c.limiter != nil && !c.limiter.Allow() {
		observability.LogWarn(ctx, "omdb daily quota reached", "imdb_id", imdbID)
		return nil, ErrOMDBRateLimited
	}

	values := url.Values{}
	values.Set("apikey", c.apiKey)
	values.Set("i", imdbID)

	requestURL := strings.TrimSuffix(c.baseURL, "/")
	requestURL = requestURL + "/?" + values.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build omdb request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", omdbUserAgent)

	start := time.Now()
	resp, err := c.httpClient.Do(req)
	duration := time.Since(start)
	if err != nil {
		observability.LogWarn(ctx, "omdb request failed", "duration_ms", strconv.FormatInt(duration.Milliseconds(), 10), "error", err.Error())
		return nil, fmt.Errorf("omdb request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		apiErr := parseOMDBAPIError(resp)
		observability.LogWarn(ctx, "omdb request failed", "status_code", strconv.Itoa(resp.StatusCode), "duration_ms", strconv.FormatInt(duration.Milliseconds(), 10), "error", apiErr.Error())
		return nil, apiErr
	}

	var payload omdbTitleResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		observability.LogWarn(ctx, "omdb response decode failed", "duration_ms", strconv.FormatInt(duration.Milliseconds(), 10), "error", err.Error())
		return nil, fmt.Errorf("decode omdb response: %w", err)
	}

	if !strings.EqualFold(strings.TrimSpace(payload.Response), "true") {
		message := strings.TrimSpace(payload.Error)
		if message == "" {
			message = "unknown omdb error"
		}
		return nil, &OMDBAPIError{StatusCode: http.StatusBadGateway, Message: message}
	}

	ratings := extractOMDBRatings(payload)
	if ratings.RottenTomatoesScore == nil && ratings.MetacriticScore == nil {
		return nil, nil
	}

	observability.LogDebug(ctx, "omdb request completed", "duration_ms", strconv.FormatInt(duration.Milliseconds(), 10), "status_code", strconv.Itoa(resp.StatusCode))

	return &ratings, nil
}

func extractOMDBRatings(payload omdbTitleResponse) OMDBRatings {
	var ratings OMDBRatings

	for _, entry := range payload.Ratings {
		source := strings.ToLower(strings.TrimSpace(entry.Source))
		switch source {
		case omdbRatingSourceRT:
			if score, ok := parseOMDBPercent(entry.Value); ok {
				ratings.RottenTomatoesScore = intPtr(score)
			}
		case omdbRatingSourceMetaCrit:
			if score, ok := parseOMDBScoreOutOf100(entry.Value); ok {
				ratings.MetacriticScore = intPtr(score)
			}
		}
	}

	if ratings.MetacriticScore == nil {
		if score, ok := parseOMDBInteger(payload.Metascore); ok {
			ratings.MetacriticScore = intPtr(score)
		}
	}

	return ratings
}

func parseOMDBAPIError(resp *http.Response) error {
	message := strings.TrimSpace(resp.Status)

	body, err := io.ReadAll(io.LimitReader(resp.Body, omdbErrorBodyMaxBytes))
	if err == nil && len(body) > 0 {
		var payload omdbTitleResponse
		if decodeErr := json.Unmarshal(body, &payload); decodeErr == nil {
			if strings.TrimSpace(payload.Error) != "" {
				message = strings.TrimSpace(payload.Error)
			}
		}
	}

	if message == "" {
		message = http.StatusText(resp.StatusCode)
	}

	return &OMDBAPIError{
		StatusCode: resp.StatusCode,
		Message:    message,
	}
}

func parseOMDBPercent(raw string) (int, bool) {
	value := strings.TrimSpace(raw)
	value = strings.TrimSuffix(value, "%")
	score, err := strconv.Atoi(value)
	if err != nil || score < 0 || score > 100 {
		return 0, false
	}
	return score, true
}

func parseOMDBScoreOutOf100(raw string) (int, bool) {
	value := strings.TrimSpace(raw)
	parts := strings.SplitN(value, "/", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[1]) != "100" {
		return 0, false
	}

	score, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil || score < 0 || score > 100 {
		return 0, false
	}
	return score, true
}

func parseOMDBInteger(raw string) (int, bool) {
	value := strings.TrimSpace(raw)
	if value == "" || strings.EqualFold(value, "N/A") {
		return 0, false
	}
	score, err := strconv.Atoi(value)
	if err != nil || score < 0 || score > 100 {
		return 0, false
	}
	return score, true
}

func intPtr(value int) *int {
	v := value
	return &v
}

type omdbRateLimiter struct {
	limit      int
	window     time.Duration
	mu         sync.Mutex
	requestLog []time.Time
}

func newOMDBRateLimiter(limit int, window time.Duration) *omdbRateLimiter {
	return &omdbRateLimiter{limit: limit, window: window}
}

func (r *omdbRateLimiter) Allow() bool {
	if r == nil || r.limit <= 0 || r.window <= 0 {
		return true
	}

	now := time.Now()
	r.mu.Lock()
	defer r.mu.Unlock()

	r.prune(now)
	if len(r.requestLog) >= r.limit {
		return false
	}

	r.requestLog = append(r.requestLog, now)
	return true
}

func (r *omdbRateLimiter) prune(now time.Time) {
	if len(r.requestLog) == 0 {
		return
	}

	cutoff := now.Add(-r.window)
	firstKept := 0
	for ; firstKept < len(r.requestLog); firstKept++ {
		if r.requestLog[firstKept].After(cutoff) {
			break
		}
	}

	if firstKept == 0 {
		return
	}

	copy(r.requestLog, r.requestLog[firstKept:])
	r.requestLog = r.requestLog[:len(r.requestLog)-firstKept]
}
