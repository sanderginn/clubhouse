package links

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewTMDBClientFromEnv(t *testing.T) {
	t.Setenv(tmdbAPIKeyEnv, "test-api-key")

	client, err := NewTMDBClientFromEnv()
	if err != nil {
		t.Fatalf("NewTMDBClientFromEnv error: %v", err)
	}
	if client == nil {
		t.Fatal("expected client")
	}
	if client.apiKey != "test-api-key" {
		t.Fatalf("api key = %q, want test-api-key", client.apiKey)
	}
}

func TestNewTMDBClientFromEnvMissingAPIKey(t *testing.T) {
	t.Setenv(tmdbAPIKeyEnv, "")

	_, err := NewTMDBClientFromEnv()
	if !errors.Is(err, ErrTMDBAPIKeyMissing) {
		t.Fatalf("expected ErrTMDBAPIKeyMissing, got %v", err)
	}
}

func TestTMDBClientSearchMovie(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search/movie" {
			t.Fatalf("path = %q, want /search/movie", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-api-key" {
			t.Fatalf("Authorization = %q, want Bearer test-api-key", got)
		}
		if r.URL.Query().Get("query") != "The Matrix" {
			t.Fatalf("query = %q", r.URL.Query().Get("query"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[{"id":603,"title":"The Matrix","overview":"A hacker learns reality is simulated.","poster_path":"/f89U3ADr1oiB1s9GkdPOEpXUk5H.jpg","backdrop_path":"/icmmSD4vTTDKOq2vvdulafOGw93.jpg","release_date":"1999-03-30","vote_average":8.2}]}`))
	}))
	defer server.Close()

	client := newTestTMDBClient(t, server.URL)

	results, err := client.SearchMovie(context.Background(), "The Matrix")
	if err != nil {
		t.Fatalf("SearchMovie error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("results len = %d, want 1", len(results))
	}
	if results[0].Title != "The Matrix" {
		t.Fatalf("title = %q, want The Matrix", results[0].Title)
	}
}

func TestTMDBClientSearchTV(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search/tv" {
			t.Fatalf("path = %q, want /search/tv", r.URL.Path)
		}
		if r.URL.Query().Get("query") != "Dark" {
			t.Fatalf("query = %q, want Dark", r.URL.Query().Get("query"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[{"id":70523,"name":"Dark","overview":"A family saga with a supernatural twist.","poster_path":"/apbrbWs8M9lyOpJYU5WXrpFbk1Z.jpg","backdrop_path":"/nrtM5uRIfho4L6ykvR4gvN2c3T7.jpg","first_air_date":"2017-12-01","vote_average":8.4}]}`))
	}))
	defer server.Close()

	client := newTestTMDBClient(t, server.URL)

	results, err := client.SearchTV(context.Background(), "Dark")
	if err != nil {
		t.Fatalf("SearchTV error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("results len = %d, want 1", len(results))
	}
	if results[0].Name != "Dark" {
		t.Fatalf("name = %q, want Dark", results[0].Name)
	}
}

func TestTMDBClientGetMovieDetails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/movie/550" {
			t.Fatalf("path = %q, want /movie/550", r.URL.Path)
		}
		if r.URL.Query().Get("append_to_response") != "credits,videos" {
			t.Fatalf("append_to_response = %q", r.URL.Query().Get("append_to_response"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id":550,
			"title":"Fight Club",
			"overview":"Mischief and mayhem.",
			"poster_path":"/a26cQPRhJPX6GbWfQbvZdrrp9j9.jpg",
			"backdrop_path":"/fCayJrkfRaCRCTh8GqN30f8oyQF.jpg",
			"runtime":139,
			"genres":[{"id":18,"name":"Drama"}],
			"release_date":"1999-10-15",
			"vote_average":8.4,
			"credits":{
				"cast":[{"id":287,"name":"Brad Pitt","character":"Tyler Durden","order":0}],
				"crew":[{"id":7467,"name":"David Fincher","job":"Director","department":"Directing"}]
			},
			"videos":{
				"results":[{"id":"abc123","key":"SUXWAEX2jlg","name":"Trailer","site":"YouTube","type":"Trailer","official":true}]
			}
		}`))
	}))
	defer server.Close()

	client := newTestTMDBClient(t, server.URL)

	details, err := client.GetMovieDetails(context.Background(), 550)
	if err != nil {
		t.Fatalf("GetMovieDetails error: %v", err)
	}
	if details.Title != "Fight Club" {
		t.Fatalf("title = %q, want Fight Club", details.Title)
	}
	if details.Director != "David Fincher" {
		t.Fatalf("director = %q, want David Fincher", details.Director)
	}
	if len(details.Videos.Results) != 1 || details.Videos.Results[0].Key != "SUXWAEX2jlg" {
		t.Fatalf("unexpected videos payload: %+v", details.Videos.Results)
	}
}

func TestTMDBClientGetTVDetails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/tv/1399" {
			t.Fatalf("path = %q, want /tv/1399", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id":1399,
			"name":"Game of Thrones",
			"overview":"Noble families fight for control.",
			"poster_path":"/1XS1oqL89opfnbLl8WnZY1O1uJx.jpg",
			"backdrop_path":"/suopoADq0k8YZr4dQXcU6pToj6s.jpg",
			"episode_run_time":[57],
			"genres":[{"id":18,"name":"Drama"}],
			"first_air_date":"2011-04-17",
			"vote_average":8.5,
			"credits":{
				"cast":[{"id":239019,"name":"Emilia Clarke","character":"Daenerys Targaryen","order":1}],
				"crew":[{"id":9813,"name":"Miguel Sapochnik","job":"Director","department":"Directing"}]
			},
			"videos":{
				"results":[{"id":"def456","key":"KPLWWIOCOOQ","name":"Teaser","site":"YouTube","type":"Teaser","official":true}]
			}
		}`))
	}))
	defer server.Close()

	client := newTestTMDBClient(t, server.URL)

	details, err := client.GetTVDetails(context.Background(), 1399)
	if err != nil {
		t.Fatalf("GetTVDetails error: %v", err)
	}
	if details.Name != "Game of Thrones" {
		t.Fatalf("name = %q, want Game of Thrones", details.Name)
	}
	if details.Runtime != 57 {
		t.Fatalf("runtime = %d, want 57", details.Runtime)
	}
	if details.Director != "Miguel Sapochnik" {
		t.Fatalf("director = %q, want Miguel Sapochnik", details.Director)
	}
}

func TestTMDBClientFindByIMDBID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/find/tt0133093" {
			t.Fatalf("path = %q, want /find/tt0133093", r.URL.Path)
		}
		if r.URL.Query().Get("external_source") != "imdb_id" {
			t.Fatalf("external_source = %q", r.URL.Query().Get("external_source"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"movie_results":[{"id":603,"title":"The Matrix","overview":"A hacker learns reality is simulated."}],
			"tv_results":[{"id":99999,"name":"The Matrix Chronicles","overview":"Companion interviews."}]
		}`))
	}))
	defer server.Close()

	client := newTestTMDBClient(t, server.URL)

	result, err := client.FindByIMDBID(context.Background(), "tt0133093")
	if err != nil {
		t.Fatalf("FindByIMDBID error: %v", err)
	}
	if len(result.MovieResults) != 1 || result.MovieResults[0].ID != 603 {
		t.Fatalf("unexpected movie_results: %+v", result.MovieResults)
	}
	if len(result.TVResults) != 1 || result.TVResults[0].ID != 99999 {
		t.Fatalf("unexpected tv_results: %+v", result.TVResults)
	}
}

func TestTMDBClientAPIErrors(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		retryAfter string
		body       string
		check      func(t *testing.T, err error)
	}{
		{
			name:       "rate limited",
			statusCode: http.StatusTooManyRequests,
			retryAfter: "12",
			body:       `{"status_message":"Too many requests"}`,
			check: func(t *testing.T, err error) {
				t.Helper()
				if !errors.Is(err, ErrTMDBRateLimited) {
					t.Fatalf("expected ErrTMDBRateLimited, got %v", err)
				}
				var apiErr *TMDBAPIError
				if !errors.As(err, &apiErr) {
					t.Fatalf("expected TMDBAPIError, got %T", err)
				}
				if apiErr.RetryAfter != 12*time.Second {
					t.Fatalf("retry_after = %s, want 12s", apiErr.RetryAfter)
				}
			},
		},
		{
			name:       "not found",
			statusCode: http.StatusNotFound,
			body:       `{"status_message":"The resource you requested could not be found."}`,
			check: func(t *testing.T, err error) {
				t.Helper()
				if !errors.Is(err, ErrTMDBNotFound) {
					t.Fatalf("expected ErrTMDBNotFound, got %v", err)
				}
			},
		},
		{
			name:       "generic api error",
			statusCode: http.StatusInternalServerError,
			body:       `{"status_message":"Internal server error"}`,
			check: func(t *testing.T, err error) {
				t.Helper()
				var apiErr *TMDBAPIError
				if !errors.As(err, &apiErr) {
					t.Fatalf("expected TMDBAPIError, got %T", err)
				}
				if apiErr.StatusCode != http.StatusInternalServerError {
					t.Fatalf("status code = %d, want 500", apiErr.StatusCode)
				}
				if !strings.Contains(apiErr.Message, "Internal server error") {
					t.Fatalf("message = %q", apiErr.Message)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tt.retryAfter != "" {
					w.Header().Set("Retry-After", tt.retryAfter)
				}
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()

			client := newTestTMDBClient(t, server.URL)
			_, err := client.SearchMovie(context.Background(), "The Matrix")
			if err == nil {
				t.Fatal("expected error")
			}
			tt.check(t, err)
		})
	}
}

func TestTMDBRateLimiterWaitRespectsWindow(t *testing.T) {
	limiter := newTMDBRateLimiter(1, 50*time.Millisecond)

	start := time.Now()
	if err := limiter.Wait(context.Background()); err != nil {
		t.Fatalf("first wait error: %v", err)
	}
	if err := limiter.Wait(context.Background()); err != nil {
		t.Fatalf("second wait error: %v", err)
	}

	elapsed := time.Since(start)
	if elapsed < 40*time.Millisecond {
		t.Fatalf("elapsed = %s, want at least 40ms", elapsed)
	}
}

func TestTMDBRateLimiterWaitHonorsContextCancellation(t *testing.T) {
	limiter := newTMDBRateLimiter(1, time.Second)
	if err := limiter.Wait(context.Background()); err != nil {
		t.Fatalf("prime wait error: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	err := limiter.Wait(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected DeadlineExceeded, got %v", err)
	}
}

func newTestTMDBClient(t *testing.T, baseURL string) *TMDBClient {
	t.Helper()

	client, err := NewTMDBClient("test-api-key", &http.Client{Timeout: time.Second})
	if err != nil {
		t.Fatalf("NewTMDBClient error: %v", err)
	}

	client.baseURL = baseURL
	client.limiter = newTMDBRateLimiter(1000, time.Second)

	return client
}
