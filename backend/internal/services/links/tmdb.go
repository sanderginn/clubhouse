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
	tmdbAPIKeyEnv            = "TMDB_API_KEY"
	tmdbDefaultBaseURL       = "https://api.themoviedb.org/3"
	tmdbUserAgent            = "ClubhouseTMDBClient/1.0"
	tmdbDefaultRateLimit     = 40
	tmdbErrorBodyMaxBytes    = 4096
	tmdbRequestTimeout       = 10 * time.Second
	tmdbDefaultRateLimitSpan = 10 * time.Second
)

var (
	ErrTMDBAPIKeyMissing = errors.New("tmdb api key is required")
	ErrTMDBNotFound      = errors.New("tmdb resource not found")
	ErrTMDBRateLimited   = errors.New("tmdb rate limited")
)

// TMDBAPIError captures non-success responses from TMDB.
type TMDBAPIError struct {
	StatusCode int
	Message    string
	RetryAfter time.Duration
}

func (e *TMDBAPIError) Error() string {
	if e == nil {
		return ""
	}

	if e.RetryAfter > 0 {
		return fmt.Sprintf("tmdb api error (%d): %s (retry after %s)", e.StatusCode, e.Message, e.RetryAfter)
	}

	return fmt.Sprintf("tmdb api error (%d): %s", e.StatusCode, e.Message)
}

func (e *TMDBAPIError) Unwrap() error {
	if e == nil {
		return nil
	}

	switch e.StatusCode {
	case http.StatusNotFound:
		return ErrTMDBNotFound
	case http.StatusTooManyRequests:
		return ErrTMDBRateLimited
	default:
		return nil
	}
}

// TMDBClient provides methods for interacting with the TMDB API.
type TMDBClient struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
	limiter    *tmdbRateLimiter
}

// MovieSearchResult is a single movie result from TMDB search.
type MovieSearchResult struct {
	ID           int     `json:"id"`
	Title        string  `json:"title"`
	Overview     string  `json:"overview"`
	PosterPath   string  `json:"poster_path"`
	BackdropPath string  `json:"backdrop_path"`
	ReleaseDate  string  `json:"release_date"`
	VoteAverage  float64 `json:"vote_average"`
}

// TVSearchResult is a single TV result from TMDB search.
type TVSearchResult struct {
	ID           int     `json:"id"`
	Name         string  `json:"name"`
	Overview     string  `json:"overview"`
	PosterPath   string  `json:"poster_path"`
	BackdropPath string  `json:"backdrop_path"`
	FirstAirDate string  `json:"first_air_date"`
	VoteAverage  float64 `json:"vote_average"`
}

// TMDBGenre represents a TMDB genre.
type TMDBGenre struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// TMDBCastMember represents a credited cast member.
type TMDBCastMember struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Character string `json:"character"`
	Order     int    `json:"order"`
}

// TMDBCrewMember represents a credited crew member.
type TMDBCrewMember struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	Job        string `json:"job"`
	Department string `json:"department"`
}

// TMDBCredits contains cast and crew data.
type TMDBCredits struct {
	Cast []TMDBCastMember `json:"cast"`
	Crew []TMDBCrewMember `json:"crew"`
}

// TMDBVideo contains movie/TV video metadata.
type TMDBVideo struct {
	ID       string `json:"id"`
	Key      string `json:"key"`
	Name     string `json:"name"`
	Site     string `json:"site"`
	Type     string `json:"type"`
	Official bool   `json:"official"`
}

// TMDBVideos is the videos payload from append_to_response.
type TMDBVideos struct {
	Results []TMDBVideo `json:"results"`
}

// MovieDetails is the detailed movie response payload.
type MovieDetails struct {
	ID           int         `json:"id"`
	Title        string      `json:"title"`
	Overview     string      `json:"overview"`
	PosterPath   string      `json:"poster_path"`
	BackdropPath string      `json:"backdrop_path"`
	Runtime      int         `json:"runtime"`
	Genres       []TMDBGenre `json:"genres"`
	ReleaseDate  string      `json:"release_date"`
	Credits      TMDBCredits `json:"credits"`
	Director     string      `json:"director"`
	VoteAverage  float64     `json:"vote_average"`
	Videos       TMDBVideos  `json:"videos"`
}

// TVDetails is the detailed TV response payload.
type TVDetails struct {
	ID             int         `json:"id"`
	Name           string      `json:"name"`
	Overview       string      `json:"overview"`
	PosterPath     string      `json:"poster_path"`
	BackdropPath   string      `json:"backdrop_path"`
	EpisodeRunTime []int       `json:"episode_run_time"`
	Runtime        int         `json:"runtime"`
	Genres         []TMDBGenre `json:"genres"`
	FirstAirDate   string      `json:"first_air_date"`
	Credits        TMDBCredits `json:"credits"`
	Director       string      `json:"director"`
	VoteAverage    float64     `json:"vote_average"`
	Videos         TMDBVideos  `json:"videos"`
}

// FindResult is the TMDB /find response scoped for IMDB lookups.
type FindResult struct {
	MovieResults []MovieSearchResult `json:"movie_results"`
	TVResults    []TVSearchResult    `json:"tv_results"`
}

// NewTMDBClientFromEnv creates a TMDB client using TMDB_API_KEY.
func NewTMDBClientFromEnv() (*TMDBClient, error) {
	apiKey := strings.TrimSpace(os.Getenv(tmdbAPIKeyEnv))
	return NewTMDBClient(apiKey, nil)
}

// NewTMDBClient creates a TMDB client with an optional custom HTTP client.
func NewTMDBClient(apiKey string, httpClient *http.Client) (*TMDBClient, error) {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return nil, ErrTMDBAPIKeyMissing
	}

	if httpClient == nil {
		httpClient = &http.Client{Timeout: tmdbRequestTimeout}
	}

	return &TMDBClient{
		apiKey:     apiKey,
		baseURL:    tmdbDefaultBaseURL,
		httpClient: httpClient,
		limiter:    newTMDBRateLimiter(tmdbDefaultRateLimit, tmdbDefaultRateLimitSpan),
	}, nil
}

// SearchMovie searches TMDB movies by title.
func (c *TMDBClient) SearchMovie(ctx context.Context, query string) ([]MovieSearchResult, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, errors.New("movie query is required")
	}

	values := url.Values{}
	values.Set("query", query)

	var payload struct {
		Results []MovieSearchResult `json:"results"`
	}

	if err := c.get(ctx, "/search/movie", values, &payload); err != nil {
		return nil, err
	}

	return payload.Results, nil
}

// SearchTV searches TMDB TV shows by title.
func (c *TMDBClient) SearchTV(ctx context.Context, query string) ([]TVSearchResult, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, errors.New("tv query is required")
	}

	values := url.Values{}
	values.Set("query", query)

	var payload struct {
		Results []TVSearchResult `json:"results"`
	}

	if err := c.get(ctx, "/search/tv", values, &payload); err != nil {
		return nil, err
	}

	return payload.Results, nil
}

// GetMovieDetails fetches movie metadata, credits, and videos.
func (c *TMDBClient) GetMovieDetails(ctx context.Context, tmdbID int) (*MovieDetails, error) {
	if tmdbID <= 0 {
		return nil, errors.New("tmdb movie id must be positive")
	}

	values := url.Values{}
	values.Set("append_to_response", "credits,videos")

	var details MovieDetails
	if err := c.get(ctx, fmt.Sprintf("/movie/%d", tmdbID), values, &details); err != nil {
		return nil, err
	}

	details.Director = extractDirector(details.Credits.Crew)

	return &details, nil
}

// GetTVDetails fetches TV metadata, credits, and videos.
func (c *TMDBClient) GetTVDetails(ctx context.Context, tmdbID int) (*TVDetails, error) {
	if tmdbID <= 0 {
		return nil, errors.New("tmdb tv id must be positive")
	}

	values := url.Values{}
	values.Set("append_to_response", "credits,videos")

	var details TVDetails
	if err := c.get(ctx, fmt.Sprintf("/tv/%d", tmdbID), values, &details); err != nil {
		return nil, err
	}

	details.Director = extractDirector(details.Credits.Crew)
	if details.Runtime == 0 {
		details.Runtime = firstPositiveInt(details.EpisodeRunTime)
	}

	return &details, nil
}

// FindByIMDBID resolves an IMDB ID to TMDB entities.
func (c *TMDBClient) FindByIMDBID(ctx context.Context, imdbID string) (*FindResult, error) {
	imdbID = strings.TrimSpace(imdbID)
	if imdbID == "" {
		return nil, errors.New("imdb id is required")
	}

	values := url.Values{}
	values.Set("external_source", "imdb_id")

	var result FindResult
	if err := c.get(ctx, "/find/"+url.PathEscape(imdbID), values, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

func (c *TMDBClient) get(ctx context.Context, path string, query url.Values, out interface{}) error {
	if ctx == nil {
		return errors.New("context is required")
	}
	if c == nil {
		return errors.New("tmdb client is required")
	}

	if err := c.limiter.Wait(ctx); err != nil {
		return fmt.Errorf("tmdb rate limiter wait: %w", err)
	}

	if query == nil {
		query = make(url.Values)
	}
	query.Set("api_key", c.apiKey)

	base := strings.TrimSuffix(c.baseURL, "/")
	requestURL := fmt.Sprintf("%s%s", base, path)
	encoded := query.Encode()
	if encoded != "" {
		requestURL = requestURL + "?" + encoded
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return fmt.Errorf("build tmdb request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", tmdbUserAgent)

	start := time.Now()
	resp, err := c.httpClient.Do(req)
	duration := time.Since(start)
	if err != nil {
		observability.LogWarn(ctx, "tmdb request failed", "path", path, "duration_ms", strconv.FormatInt(duration.Milliseconds(), 10), "error", err.Error())
		return fmt.Errorf("tmdb request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		apiErr := parseTMDBAPIError(resp)
		observability.LogWarn(ctx, "tmdb request failed", "path", path, "status_code", strconv.Itoa(resp.StatusCode), "duration_ms", strconv.FormatInt(duration.Milliseconds(), 10), "error", apiErr.Error())
		return apiErr
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		observability.LogWarn(ctx, "tmdb response decode failed", "path", path, "duration_ms", strconv.FormatInt(duration.Milliseconds(), 10), "error", err.Error())
		return fmt.Errorf("decode tmdb response: %w", err)
	}

	observability.LogDebug(ctx, "tmdb request completed", "path", path, "duration_ms", strconv.FormatInt(duration.Milliseconds(), 10), "status_code", strconv.Itoa(resp.StatusCode))

	return nil
}

func parseTMDBAPIError(resp *http.Response) error {
	message := strings.TrimSpace(resp.Status)
	retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"))

	body, err := io.ReadAll(io.LimitReader(resp.Body, tmdbErrorBodyMaxBytes))
	if err == nil && len(body) > 0 {
		var payload struct {
			StatusMessage string `json:"status_message"`
		}
		if decodeErr := json.Unmarshal(body, &payload); decodeErr == nil {
			if strings.TrimSpace(payload.StatusMessage) != "" {
				message = strings.TrimSpace(payload.StatusMessage)
			}
		}
	}

	if message == "" {
		message = http.StatusText(resp.StatusCode)
	}

	return &TMDBAPIError{
		StatusCode: resp.StatusCode,
		Message:    message,
		RetryAfter: retryAfter,
	}
}

func parseRetryAfter(raw string) time.Duration {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}

	seconds, err := strconv.Atoi(raw)
	if err == nil {
		if seconds <= 0 {
			return 0
		}
		return time.Duration(seconds) * time.Second
	}

	retryAt, err := http.ParseTime(raw)
	if err != nil {
		return 0
	}

	d := time.Until(retryAt)
	if d < 0 {
		return 0
	}
	return d
}

func extractDirector(crew []TMDBCrewMember) string {
	for _, member := range crew {
		if strings.EqualFold(strings.TrimSpace(member.Job), "director") {
			return strings.TrimSpace(member.Name)
		}
	}
	return ""
}

func firstPositiveInt(values []int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

type tmdbRateLimiter struct {
	limit      int
	window     time.Duration
	mu         sync.Mutex
	requestLog []time.Time
}

func newTMDBRateLimiter(limit int, window time.Duration) *tmdbRateLimiter {
	return &tmdbRateLimiter{limit: limit, window: window}
}

func (r *tmdbRateLimiter) Wait(ctx context.Context) error {
	if r == nil || r.limit <= 0 || r.window <= 0 {
		return nil
	}

	for {
		now := time.Now()

		r.mu.Lock()
		r.prune(now)
		if len(r.requestLog) < r.limit {
			r.requestLog = append(r.requestLog, now)
			r.mu.Unlock()
			return nil
		}

		waitFor := r.requestLog[0].Add(r.window).Sub(now)
		r.mu.Unlock()

		if waitFor <= 0 {
			continue
		}

		timer := time.NewTimer(waitFor)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func (r *tmdbRateLimiter) prune(now time.Time) {
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
