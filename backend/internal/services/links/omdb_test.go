package links

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewOMDBClientFromEnv(t *testing.T) {
	t.Setenv(omdbAPIKeyEnv, "test-omdb-key")

	client, err := NewOMDBClientFromEnv()
	if err != nil {
		t.Fatalf("NewOMDBClientFromEnv error: %v", err)
	}
	if client == nil {
		t.Fatal("expected client")
	}
	if client.apiKey != "test-omdb-key" {
		t.Fatalf("api key = %q, want test-omdb-key", client.apiKey)
	}
}

func TestNewOMDBClientFromEnvMissingAPIKey(t *testing.T) {
	t.Setenv(omdbAPIKeyEnv, "")

	_, err := NewOMDBClientFromEnv()
	if !errors.Is(err, ErrOMDBAPIKeyMissing) {
		t.Fatalf("expected ErrOMDBAPIKeyMissing, got %v", err)
	}
}

func TestOMDBClientGetRatingsByIMDBID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			t.Fatalf("path = %q, want /", r.URL.Path)
		}
		if got := r.URL.Query().Get("apikey"); got != "test-omdb-key" {
			t.Fatalf("apikey = %q, want test-omdb-key", got)
		}
		if got := r.URL.Query().Get("i"); got != "tt0133093" {
			t.Fatalf("imdb id = %q, want tt0133093", got)
		}
		if got := r.Header.Get("User-Agent"); got != omdbUserAgent {
			t.Fatalf("user-agent = %q, want %q", got, omdbUserAgent)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"Response":"True",
			"Ratings":[
				{"Source":"Internet Movie Database","Value":"8.7/10"},
				{"Source":"Rotten Tomatoes","Value":"83%"},
				{"Source":"Metacritic","Value":"73/100"}
			]
		}`))
	}))
	defer server.Close()

	client := newTestOMDBClient(t, server.URL)

	ratings, err := client.GetRatingsByIMDBID(context.Background(), "tt0133093")
	if err != nil {
		t.Fatalf("GetRatingsByIMDBID error: %v", err)
	}
	if ratings == nil {
		t.Fatal("expected ratings")
	}
	if ratings.RottenTomatoesScore == nil || *ratings.RottenTomatoesScore != 83 {
		t.Fatalf("rotten tomatoes = %+v, want 83", ratings.RottenTomatoesScore)
	}
	if ratings.MetacriticScore == nil || *ratings.MetacriticScore != 73 {
		t.Fatalf("metacritic = %+v, want 73", ratings.MetacriticScore)
	}
}

func TestOMDBClientGetRatingsByIMDBIDUsesMetascoreFallback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"Response":"True",
			"Metascore":"64",
			"Ratings":[
				{"Source":"Rotten Tomatoes","Value":"91%"}
			]
		}`))
	}))
	defer server.Close()

	client := newTestOMDBClient(t, server.URL)

	ratings, err := client.GetRatingsByIMDBID(context.Background(), "tt0133093")
	if err != nil {
		t.Fatalf("GetRatingsByIMDBID error: %v", err)
	}
	if ratings == nil {
		t.Fatal("expected ratings")
	}
	if ratings.RottenTomatoesScore == nil || *ratings.RottenTomatoesScore != 91 {
		t.Fatalf("rotten tomatoes = %+v, want 91", ratings.RottenTomatoesScore)
	}
	if ratings.MetacriticScore == nil || *ratings.MetacriticScore != 64 {
		t.Fatalf("metacritic = %+v, want 64", ratings.MetacriticScore)
	}
}

func TestOMDBClientGetRatingsByIMDBIDNoSupportedRatings(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"Response":"True",
			"Metascore":"N/A",
			"Ratings":[{"Source":"Internet Movie Database","Value":"8.8/10"}]
		}`))
	}))
	defer server.Close()

	client := newTestOMDBClient(t, server.URL)

	ratings, err := client.GetRatingsByIMDBID(context.Background(), "tt0133093")
	if err != nil {
		t.Fatalf("GetRatingsByIMDBID error: %v", err)
	}
	if ratings != nil {
		t.Fatalf("expected nil ratings, got %+v", ratings)
	}
}

func TestOMDBClientAPIErrors(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		check      func(t *testing.T, err error)
	}{
		{
			name:       "http rate limit",
			statusCode: http.StatusTooManyRequests,
			body:       `{"Error":"Request limit reached!"}`,
			check: func(t *testing.T, err error) {
				t.Helper()
				if !errors.Is(err, ErrOMDBRateLimited) {
					t.Fatalf("expected ErrOMDBRateLimited, got %v", err)
				}
			},
		},
		{
			name:       "not found",
			statusCode: http.StatusOK,
			body:       `{"Response":"False","Error":"Movie not found!"}`,
			check: func(t *testing.T, err error) {
				t.Helper()
				if !errors.Is(err, ErrOMDBNotFound) {
					t.Fatalf("expected ErrOMDBNotFound, got %v", err)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()

			client := newTestOMDBClient(t, server.URL)
			_, err := client.GetRatingsByIMDBID(context.Background(), "tt0133093")
			if err == nil {
				t.Fatal("expected error")
			}
			tt.check(t, err)
		})
	}
}

func TestOMDBClientGetRatingsByIMDBIDRespectsDailyLimit(t *testing.T) {
	var requestCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"Response":"True","Ratings":[{"Source":"Rotten Tomatoes","Value":"90%"}]}`))
	}))
	defer server.Close()

	client := newTestOMDBClient(t, server.URL)
	client.limiter = newOMDBRateLimiter(1, time.Hour)

	if _, err := client.GetRatingsByIMDBID(context.Background(), "tt0133093"); err != nil {
		t.Fatalf("first request error: %v", err)
	}

	_, err := client.GetRatingsByIMDBID(context.Background(), "tt0133093")
	if !errors.Is(err, ErrOMDBRateLimited) {
		t.Fatalf("expected ErrOMDBRateLimited, got %v", err)
	}

	if got := atomic.LoadInt32(&requestCount); got != 1 {
		t.Fatalf("request count = %d, want 1", got)
	}
}

func TestOMDBClientGetRatingsByIMDBIDInvalidIMDBID(t *testing.T) {
	client := newTestOMDBClient(t, "https://example.invalid")
	_, err := client.GetRatingsByIMDBID(context.Background(), "tt-bad")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "valid imdb id is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func newTestOMDBClient(t *testing.T, baseURL string) *OMDBClient {
	t.Helper()

	client, err := NewOMDBClient("test-omdb-key", &http.Client{Timeout: time.Second})
	if err != nil {
		t.Fatalf("NewOMDBClient error: %v", err)
	}

	client.baseURL = baseURL
	client.limiter = newOMDBRateLimiter(1000, time.Second)

	return client
}
