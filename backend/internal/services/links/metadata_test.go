package links

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

type fakeResolver struct {
	addrs map[string][]net.IPAddr
	err   error
}

func (f fakeResolver) LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error) {
	if f.err != nil {
		return nil, f.err
	}
	addrs, ok := f.addrs[host]
	if !ok {
		return nil, errors.New("host not found")
	}
	return addrs, nil
}

func TestFetchMetadataHTML(t *testing.T) {
	htmlBody := `<!doctype html>
		<html>
		<head>
			<title>Fallback Title</title>
			<meta property="og:title" content="OG Title" />
			<meta name="description" content="Desc" />
			<meta property="og:image" content="/img.png" />
			<meta property="og:site_name" content="ExampleSite" />
		</head>
		</html>`

	fetcher := NewFetcher(&http.Client{
		Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     http.Header{"Content-Type": []string{"text/html; charset=utf-8"}},
				Body:       io.NopCloser(strings.NewReader(htmlBody)),
				Request:    r,
			}, nil
		}),
	})
	fetcher.resolver = fakeResolver{
		addrs: map[string][]net.IPAddr{
			"example.com": {{IP: net.ParseIP("93.184.216.34")}},
		},
	}

	metadata, err := fetcher.Fetch(context.Background(), "https://example.com/post")
	if err != nil {
		t.Fatalf("FetchMetadata error: %v", err)
	}

	if metadata["title"] != "OG Title" {
		t.Errorf("title = %v, want OG Title", metadata["title"])
	}
	if metadata["description"] != "Desc" {
		t.Errorf("description = %v, want Desc", metadata["description"])
	}
	if provider := metadata["provider"]; provider != "ExampleSite" {
		t.Errorf("provider = %v, want ExampleSite", provider)
	}
	image, _ := metadata["image"].(string)
	if !strings.HasPrefix(image, "https://example.com") {
		t.Errorf("image = %q, want prefix %q", image, "https://example.com")
	}
}

func TestFetchMetadataMovieSectionIncludesMovieMetadata(t *testing.T) {
	originalNewTMDBClientFromEnvFunc := newTMDBClientFromEnvFunc
	originalNewOMDBClientFromEnvFunc := newOMDBClientFromEnvFunc
	originalParseMovieMetadataFunc := parseMovieMetadataFunc
	resetOMDBClientFromEnvCacheForTests()
	t.Cleanup(func() {
		newTMDBClientFromEnvFunc = originalNewTMDBClientFromEnvFunc
		newOMDBClientFromEnvFunc = originalNewOMDBClientFromEnvFunc
		parseMovieMetadataFunc = originalParseMovieMetadataFunc
		resetOMDBClientFromEnvCacheForTests()
	})

	parseCalls := 0
	newTMDBClientFromEnvFunc = func() (*TMDBClient, error) {
		return &TMDBClient{}, nil
	}
	parseMovieMetadataFunc = func(ctx context.Context, rawURL string, client *TMDBClient, omdbClient *OMDBClient) (*MovieData, error) {
		parseCalls++
		if rawURL != "https://www.imdb.com/title/tt0133093/" {
			t.Fatalf("rawURL = %q, want imdb url", rawURL)
		}
		return &MovieData{Title: "The Matrix"}, nil
	}

	fetcher := NewFetcher(&http.Client{
		Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     http.Header{"Content-Type": []string{"text/html; charset=utf-8"}},
				Body:       io.NopCloser(strings.NewReader(`<!doctype html><html><head><meta property="og:title" content="Fallback Title" /></head></html>`)),
				Request:    r,
			}, nil
		}),
	})
	fetcher.resolver = fakeResolver{
		addrs: map[string][]net.IPAddr{
			"www.imdb.com": {{IP: net.ParseIP("93.184.216.34")}},
		},
	}

	ctx := WithMetadataSectionType(context.Background(), "movie")
	metadata, err := fetcher.Fetch(ctx, "https://www.imdb.com/title/tt0133093/")
	if err != nil {
		t.Fatalf("Fetch error: %v", err)
	}

	if parseCalls != 1 {
		t.Fatalf("parseCalls = %d, want 1", parseCalls)
	}

	movie, ok := metadata["movie"].(*MovieData)
	if !ok || movie == nil {
		t.Fatalf("expected movie metadata to be present")
	}
	if movie.Title != "The Matrix" {
		t.Fatalf("movie title = %q, want The Matrix", movie.Title)
	}
}

func TestFetchMetadataMovieSectionPassesOMDBClientWhenConfigured(t *testing.T) {
	originalNewTMDBClientFromEnvFunc := newTMDBClientFromEnvFunc
	originalNewOMDBClientFromEnvFunc := newOMDBClientFromEnvFunc
	originalParseMovieMetadataFunc := parseMovieMetadataFunc
	resetOMDBClientFromEnvCacheForTests()
	t.Cleanup(func() {
		newTMDBClientFromEnvFunc = originalNewTMDBClientFromEnvFunc
		newOMDBClientFromEnvFunc = originalNewOMDBClientFromEnvFunc
		parseMovieMetadataFunc = originalParseMovieMetadataFunc
		resetOMDBClientFromEnvCacheForTests()
	})

	parseCalls := 0
	newTMDBClientFromEnvFunc = func() (*TMDBClient, error) {
		return &TMDBClient{}, nil
	}
	newOMDBClientFromEnvFunc = func() (*OMDBClient, error) {
		return &OMDBClient{}, nil
	}
	parseMovieMetadataFunc = func(ctx context.Context, rawURL string, client *TMDBClient, omdbClient *OMDBClient) (*MovieData, error) {
		parseCalls++
		if omdbClient == nil {
			t.Fatal("expected omdb client to be provided")
		}
		return &MovieData{Title: "The Matrix"}, nil
	}

	fetcher := NewFetcher(&http.Client{
		Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     http.Header{"Content-Type": []string{"text/html; charset=utf-8"}},
				Body:       io.NopCloser(strings.NewReader(`<!doctype html><html><head><meta property="og:title" content="Fallback Title" /></head></html>`)),
				Request:    r,
			}, nil
		}),
	})
	fetcher.resolver = fakeResolver{
		addrs: map[string][]net.IPAddr{
			"www.imdb.com": {{IP: net.ParseIP("93.184.216.34")}},
		},
	}

	ctx := WithMetadataSectionType(context.Background(), "movie")
	metadata, err := fetcher.Fetch(ctx, "https://www.imdb.com/title/tt0133093/")
	if err != nil {
		t.Fatalf("Fetch error: %v", err)
	}

	if parseCalls != 1 {
		t.Fatalf("parseCalls = %d, want 1", parseCalls)
	}
	if _, ok := metadata["movie"]; !ok {
		t.Fatalf("expected movie metadata to be present")
	}
}

func TestFetchMetadataMovieSectionFallbacksWhenRequestFails(t *testing.T) {
	originalNewTMDBClientFromEnvFunc := newTMDBClientFromEnvFunc
	originalNewOMDBClientFromEnvFunc := newOMDBClientFromEnvFunc
	originalParseMovieMetadataFunc := parseMovieMetadataFunc
	resetOMDBClientFromEnvCacheForTests()
	t.Cleanup(func() {
		newTMDBClientFromEnvFunc = originalNewTMDBClientFromEnvFunc
		newOMDBClientFromEnvFunc = originalNewOMDBClientFromEnvFunc
		parseMovieMetadataFunc = originalParseMovieMetadataFunc
		resetOMDBClientFromEnvCacheForTests()
	})

	newTMDBClientFromEnvFunc = func() (*TMDBClient, error) {
		return &TMDBClient{}, nil
	}
	parseMovieMetadataFunc = func(ctx context.Context, rawURL string, client *TMDBClient, omdbClient *OMDBClient) (*MovieData, error) {
		return &MovieData{
			Title:               "The Matrix",
			TMDBID:              603,
			TMDBMediaType:       "movie",
			RottenTomatoesScore: intPtr(88),
		}, nil
	}

	fetcher := NewFetcher(&http.Client{
		Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			return nil, errors.New("network unreachable")
		}),
	})
	fetcher.resolver = fakeResolver{
		addrs: map[string][]net.IPAddr{
			"www.rottentomatoes.com": {{IP: net.ParseIP("151.101.65.91")}},
		},
	}

	ctx := WithMetadataSectionType(context.Background(), "movie")
	metadata, err := fetcher.Fetch(ctx, "https://www.rottentomatoes.com/m/the_matrix")
	if err != nil {
		t.Fatalf("Fetch error: %v", err)
	}

	movie, ok := metadata["movie"].(*MovieData)
	if !ok || movie == nil {
		t.Fatalf("expected movie metadata fallback")
	}
	if movie.Title != "The Matrix" {
		t.Fatalf("movie title = %q, want The Matrix", movie.Title)
	}
	if movie.RottenTomatoesScore == nil || *movie.RottenTomatoesScore != 88 {
		t.Fatalf("rotten tomatoes score = %+v, want 88", movie.RottenTomatoesScore)
	}
	if provider, ok := metadata["provider"].(string); !ok || provider == "" {
		t.Fatalf("expected provider to be set")
	}
}

func TestFetchMetadataMovieSectionFallbacksOnHTTPStatusError(t *testing.T) {
	originalNewTMDBClientFromEnvFunc := newTMDBClientFromEnvFunc
	originalNewOMDBClientFromEnvFunc := newOMDBClientFromEnvFunc
	originalParseMovieMetadataFunc := parseMovieMetadataFunc
	resetOMDBClientFromEnvCacheForTests()
	t.Cleanup(func() {
		newTMDBClientFromEnvFunc = originalNewTMDBClientFromEnvFunc
		newOMDBClientFromEnvFunc = originalNewOMDBClientFromEnvFunc
		parseMovieMetadataFunc = originalParseMovieMetadataFunc
		resetOMDBClientFromEnvCacheForTests()
	})

	newTMDBClientFromEnvFunc = func() (*TMDBClient, error) {
		return &TMDBClient{}, nil
	}
	parseMovieMetadataFunc = func(ctx context.Context, rawURL string, client *TMDBClient, omdbClient *OMDBClient) (*MovieData, error) {
		return &MovieData{Title: "The Matrix"}, nil
	}

	fetcher := NewFetcher(&http.Client{
		Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusForbidden,
				Status:     "403 Forbidden",
				Header:     http.Header{"Content-Type": []string{"text/html; charset=utf-8"}},
				Body:       io.NopCloser(strings.NewReader("forbidden")),
				Request:    r,
			}, nil
		}),
	})
	fetcher.resolver = fakeResolver{
		addrs: map[string][]net.IPAddr{
			"www.rottentomatoes.com": {{IP: net.ParseIP("151.101.1.91")}},
		},
	}

	ctx := WithMetadataSectionType(context.Background(), "movie")
	metadata, err := fetcher.Fetch(ctx, "https://www.rottentomatoes.com/m/the_matrix")
	if err != nil {
		t.Fatalf("Fetch error: %v", err)
	}
	if _, ok := metadata["movie"]; !ok {
		t.Fatalf("expected movie metadata fallback")
	}
}

func TestFetchMetadataMovieSectionReturnsHTMLWhenMovieParsingTimesOut(t *testing.T) {
	originalNewTMDBClientFromEnvFunc := newTMDBClientFromEnvFunc
	originalNewOMDBClientFromEnvFunc := newOMDBClientFromEnvFunc
	originalParseMovieMetadataFunc := parseMovieMetadataFunc
	resetOMDBClientFromEnvCacheForTests()
	t.Cleanup(func() {
		newTMDBClientFromEnvFunc = originalNewTMDBClientFromEnvFunc
		newOMDBClientFromEnvFunc = originalNewOMDBClientFromEnvFunc
		parseMovieMetadataFunc = originalParseMovieMetadataFunc
		resetOMDBClientFromEnvCacheForTests()
	})

	newTMDBClientFromEnvFunc = func() (*TMDBClient, error) {
		return &TMDBClient{}, nil
	}
	parseMovieMetadataFunc = func(ctx context.Context, rawURL string, client *TMDBClient, omdbClient *OMDBClient) (*MovieData, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}

	fetcher := NewFetcher(&http.Client{
		Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     http.Header{"Content-Type": []string{"text/html; charset=utf-8"}},
				Body: io.NopCloser(strings.NewReader(`<!doctype html>
					<html>
					<head>
						<meta property="og:title" content="Fallback Title" />
						<meta property="og:description" content="Fallback Description" />
					</head>
					</html>`)),
				Request: r,
			}, nil
		}),
	})
	fetcher.resolver = fakeResolver{
		addrs: map[string][]net.IPAddr{
			"www.imdb.com": {{IP: net.ParseIP("93.184.216.34")}},
		},
	}

	ctx, cancel := context.WithTimeout(WithMetadataSectionType(context.Background(), "movie"), 20*time.Millisecond)
	defer cancel()

	metadata, err := fetcher.Fetch(ctx, "https://www.imdb.com/title/tt0133093/")
	if err != nil {
		t.Fatalf("Fetch error: %v", err)
	}

	if metadata["title"] != "Fallback Title" {
		t.Fatalf("title = %v, want Fallback Title", metadata["title"])
	}
	if metadata["description"] != "Fallback Description" {
		t.Fatalf("description = %v, want Fallback Description", metadata["description"])
	}
	if _, ok := metadata["movie"]; ok {
		t.Fatalf("expected movie metadata to be absent when movie parsing times out")
	}
}

func TestFetchMetadataMovieSectionBackfillsRottenTomatoesScoreFromHTML(t *testing.T) {
	originalNewTMDBClientFromEnvFunc := newTMDBClientFromEnvFunc
	originalNewOMDBClientFromEnvFunc := newOMDBClientFromEnvFunc
	originalParseMovieMetadataFunc := parseMovieMetadataFunc
	resetOMDBClientFromEnvCacheForTests()
	t.Cleanup(func() {
		newTMDBClientFromEnvFunc = originalNewTMDBClientFromEnvFunc
		newOMDBClientFromEnvFunc = originalNewOMDBClientFromEnvFunc
		parseMovieMetadataFunc = originalParseMovieMetadataFunc
		resetOMDBClientFromEnvCacheForTests()
	})

	newTMDBClientFromEnvFunc = func() (*TMDBClient, error) {
		return &TMDBClient{}, nil
	}
	parseMovieMetadataFunc = func(ctx context.Context, rawURL string, client *TMDBClient, omdbClient *OMDBClient) (*MovieData, error) {
		return &MovieData{Title: "The Matrix"}, nil
	}

	fetcher := NewFetcher(&http.Client{
		Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     http.Header{"Content-Type": []string{"text/html; charset=utf-8"}},
				Body: io.NopCloser(strings.NewReader(`<!doctype html>
					<html>
					<head>
						<score-board tomatometerscore="91"></score-board>
					</head>
					</html>`)),
				Request: r,
			}, nil
		}),
	})
	fetcher.resolver = fakeResolver{
		addrs: map[string][]net.IPAddr{
			"www.rottentomatoes.com": {{IP: net.ParseIP("151.101.65.91")}},
		},
	}

	ctx := WithMetadataSectionType(context.Background(), "movie")
	metadata, err := fetcher.Fetch(ctx, "https://www.rottentomatoes.com/m/the_matrix")
	if err != nil {
		t.Fatalf("Fetch error: %v", err)
	}

	movie, ok := metadata["movie"].(*MovieData)
	if !ok || movie == nil {
		t.Fatalf("expected movie metadata to be present")
	}
	if movie.RottenTomatoesScore == nil || *movie.RottenTomatoesScore != 91 {
		t.Fatalf("rotten tomatoes score = %+v, want 91", movie.RottenTomatoesScore)
	}
}

func TestFetchMetadataMovieSectionDoesNotOverrideExistingRottenTomatoesScore(t *testing.T) {
	originalNewTMDBClientFromEnvFunc := newTMDBClientFromEnvFunc
	originalNewOMDBClientFromEnvFunc := newOMDBClientFromEnvFunc
	originalParseMovieMetadataFunc := parseMovieMetadataFunc
	resetOMDBClientFromEnvCacheForTests()
	t.Cleanup(func() {
		newTMDBClientFromEnvFunc = originalNewTMDBClientFromEnvFunc
		newOMDBClientFromEnvFunc = originalNewOMDBClientFromEnvFunc
		parseMovieMetadataFunc = originalParseMovieMetadataFunc
		resetOMDBClientFromEnvCacheForTests()
	})

	newTMDBClientFromEnvFunc = func() (*TMDBClient, error) {
		return &TMDBClient{}, nil
	}
	parseMovieMetadataFunc = func(ctx context.Context, rawURL string, client *TMDBClient, omdbClient *OMDBClient) (*MovieData, error) {
		return &MovieData{
			Title:               "The Matrix",
			RottenTomatoesScore: intPtr(88),
		}, nil
	}

	fetcher := NewFetcher(&http.Client{
		Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     http.Header{"Content-Type": []string{"text/html; charset=utf-8"}},
				Body: io.NopCloser(strings.NewReader(`<!doctype html>
					<html>
					<head>
						<score-board tomatometerscore="97"></score-board>
					</head>
					</html>`)),
				Request: r,
			}, nil
		}),
	})
	fetcher.resolver = fakeResolver{
		addrs: map[string][]net.IPAddr{
			"www.rottentomatoes.com": {{IP: net.ParseIP("151.101.65.91")}},
		},
	}

	ctx := WithMetadataSectionType(context.Background(), "movie")
	metadata, err := fetcher.Fetch(ctx, "https://www.rottentomatoes.com/m/the_matrix")
	if err != nil {
		t.Fatalf("Fetch error: %v", err)
	}

	movie, ok := metadata["movie"].(*MovieData)
	if !ok || movie == nil {
		t.Fatalf("expected movie metadata to be present")
	}
	if movie.RottenTomatoesScore == nil || *movie.RottenTomatoesScore != 88 {
		t.Fatalf("rotten tomatoes score = %+v, want 88", movie.RottenTomatoesScore)
	}
}

func TestExtractRottenTomatoesScoreFromHTML(t *testing.T) {
	tests := []struct {
		name string
		body string
		want int
		ok   bool
	}{
		{
			name: "score board attribute",
			body: `<score-board tomatometerscore="93"></score-board>`,
			want: 93,
			ok:   true,
		},
		{
			name: "json score payload",
			body: `{"tomatometerScore":{"all":{"score":87}}}`,
			want: 87,
			ok:   true,
		},
		{
			name: "critics score payload",
			body: `{"criticsScore":74}`,
			want: 74,
			ok:   true,
		},
		{
			name: "invalid score",
			body: `<score-board tomatometerscore="101"></score-board>`,
			want: 0,
			ok:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := extractRottenTomatoesScoreFromHTML([]byte(tt.body))
			if ok != tt.ok {
				t.Fatalf("ok = %v, want %v", ok, tt.ok)
			}
			if got != tt.want {
				t.Fatalf("score = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestFetchMetadataGeneralSectionSkipsMovieMetadata(t *testing.T) {
	originalNewTMDBClientFromEnvFunc := newTMDBClientFromEnvFunc
	originalNewOMDBClientFromEnvFunc := newOMDBClientFromEnvFunc
	originalParseMovieMetadataFunc := parseMovieMetadataFunc
	resetOMDBClientFromEnvCacheForTests()
	t.Cleanup(func() {
		newTMDBClientFromEnvFunc = originalNewTMDBClientFromEnvFunc
		newOMDBClientFromEnvFunc = originalNewOMDBClientFromEnvFunc
		parseMovieMetadataFunc = originalParseMovieMetadataFunc
		resetOMDBClientFromEnvCacheForTests()
	})

	parseCalls := 0
	newTMDBClientFromEnvFunc = func() (*TMDBClient, error) {
		return &TMDBClient{}, nil
	}
	parseMovieMetadataFunc = func(ctx context.Context, rawURL string, client *TMDBClient, omdbClient *OMDBClient) (*MovieData, error) {
		parseCalls++
		return &MovieData{Title: "Should Not Be Used"}, nil
	}

	fetcher := NewFetcher(&http.Client{
		Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     http.Header{"Content-Type": []string{"text/html; charset=utf-8"}},
				Body:       io.NopCloser(strings.NewReader(`<!doctype html><html><head><meta property="og:title" content="Fallback Title" /></head></html>`)),
				Request:    r,
			}, nil
		}),
	})
	fetcher.resolver = fakeResolver{
		addrs: map[string][]net.IPAddr{
			"www.imdb.com": {{IP: net.ParseIP("93.184.216.34")}},
		},
	}

	ctx := WithMetadataSectionType(context.Background(), "general")
	metadata, err := fetcher.Fetch(ctx, "https://www.imdb.com/title/tt0133093/")
	if err != nil {
		t.Fatalf("Fetch error: %v", err)
	}

	if parseCalls != 0 {
		t.Fatalf("parseCalls = %d, want 0", parseCalls)
	}
	if _, ok := metadata["movie"]; ok {
		t.Fatalf("expected movie metadata to be absent")
	}
}

func TestFetchMetadataMovieParserFailureFallsBackToHTMLMetadata(t *testing.T) {
	originalNewTMDBClientFromEnvFunc := newTMDBClientFromEnvFunc
	originalNewOMDBClientFromEnvFunc := newOMDBClientFromEnvFunc
	originalParseMovieMetadataFunc := parseMovieMetadataFunc
	resetOMDBClientFromEnvCacheForTests()
	t.Cleanup(func() {
		newTMDBClientFromEnvFunc = originalNewTMDBClientFromEnvFunc
		newOMDBClientFromEnvFunc = originalNewOMDBClientFromEnvFunc
		parseMovieMetadataFunc = originalParseMovieMetadataFunc
		resetOMDBClientFromEnvCacheForTests()
	})

	newTMDBClientFromEnvFunc = func() (*TMDBClient, error) {
		return &TMDBClient{}, nil
	}
	parseMovieMetadataFunc = func(ctx context.Context, rawURL string, client *TMDBClient, omdbClient *OMDBClient) (*MovieData, error) {
		return nil, errors.New("tmdb unavailable")
	}

	fetcher := NewFetcher(&http.Client{
		Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			if r.Header.Get("Accept-Language") == "" {
				t.Fatalf("expected Accept-Language header to be set for imdb requests")
			}

			ua := strings.TrimSpace(r.Header.Get("User-Agent"))
			body := `<!doctype html><html><head></head><body>blocked</body></html>`
			if ua == imdbUserAgent {
				body = `<!doctype html>
					<html>
					<head>
						<title>Fallback Title</title>
						<meta property="og:title" content="Fallback Title" />
						<meta property="og:description" content="Fallback Description" />
						<meta property="og:image" content="/poster.jpg" />
					</head>
					</html>`
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     http.Header{"Content-Type": []string{"text/html; charset=utf-8"}},
				Body:       io.NopCloser(strings.NewReader(body)),
				Request:    r,
			}, nil
		}),
	})
	fetcher.resolver = fakeResolver{
		addrs: map[string][]net.IPAddr{
			"www.imdb.com": {{IP: net.ParseIP("93.184.216.34")}},
		},
	}

	ctx := WithMetadataSectionType(context.Background(), "movie")
	metadata, err := fetcher.Fetch(ctx, "https://www.imdb.com/title/tt0133093/")
	if err != nil {
		t.Fatalf("Fetch error: %v", err)
	}
	if metadata["title"] != "Fallback Title" {
		t.Fatalf("title = %v, want Fallback Title", metadata["title"])
	}
	if metadata["description"] != "Fallback Description" {
		t.Fatalf("description = %v, want Fallback Description", metadata["description"])
	}
	if metadata["image"] != "https://www.imdb.com/poster.jpg" {
		t.Fatalf("image = %v, want https://www.imdb.com/poster.jpg", metadata["image"])
	}
	if metadata["provider"] != "imdb" {
		t.Fatalf("provider = %v, want imdb", metadata["provider"])
	}
	if _, ok := metadata["movie"]; ok {
		t.Fatalf("expected movie metadata to be absent when tmdb parsing fails")
	}
}

func TestFetchMetadataMovieSectionReusesCachedOMDBClient(t *testing.T) {
	originalNewTMDBClientFromEnvFunc := newTMDBClientFromEnvFunc
	originalNewOMDBClientFromEnvFunc := newOMDBClientFromEnvFunc
	originalParseMovieMetadataFunc := parseMovieMetadataFunc
	resetOMDBClientFromEnvCacheForTests()
	t.Cleanup(func() {
		newTMDBClientFromEnvFunc = originalNewTMDBClientFromEnvFunc
		newOMDBClientFromEnvFunc = originalNewOMDBClientFromEnvFunc
		parseMovieMetadataFunc = originalParseMovieMetadataFunc
		resetOMDBClientFromEnvCacheForTests()
	})

	omdbInitCalls := 0
	parsedCalls := 0
	sharedOMDBClient := &OMDBClient{}

	newTMDBClientFromEnvFunc = func() (*TMDBClient, error) {
		return &TMDBClient{}, nil
	}
	newOMDBClientFromEnvFunc = func() (*OMDBClient, error) {
		omdbInitCalls++
		return sharedOMDBClient, nil
	}
	parseMovieMetadataFunc = func(ctx context.Context, rawURL string, client *TMDBClient, omdbClient *OMDBClient) (*MovieData, error) {
		parsedCalls++
		if omdbClient != sharedOMDBClient {
			t.Fatalf("expected cached shared OMDB client instance")
		}
		return &MovieData{Title: "The Matrix"}, nil
	}

	fetcher := NewFetcher(&http.Client{
		Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     http.Header{"Content-Type": []string{"text/html; charset=utf-8"}},
				Body:       io.NopCloser(strings.NewReader(`<!doctype html><html><head><meta property="og:title" content="Fallback Title" /></head></html>`)),
				Request:    r,
			}, nil
		}),
	})
	fetcher.resolver = fakeResolver{
		addrs: map[string][]net.IPAddr{
			"www.imdb.com": {{IP: net.ParseIP("93.184.216.34")}},
		},
	}

	ctx := WithMetadataSectionType(context.Background(), "movie")
	if _, err := fetcher.Fetch(ctx, "https://www.imdb.com/title/tt0133093/"); err != nil {
		t.Fatalf("first Fetch error: %v", err)
	}
	if _, err := fetcher.Fetch(ctx, "https://www.imdb.com/title/tt0133093/"); err != nil {
		t.Fatalf("second Fetch error: %v", err)
	}

	if omdbInitCalls != 1 {
		t.Fatalf("omdbInitCalls = %d, want 1", omdbInitCalls)
	}
	if parsedCalls != 2 {
		t.Fatalf("parsedCalls = %d, want 2", parsedCalls)
	}
}

func TestShouldExtractBookMetadata(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want bool
	}{
		{
			name: "goodreads book url",
			url:  "https://www.goodreads.com/book/show/22328-neuromancer",
			want: true,
		},
		{
			name: "amazon dp url",
			url:  "https://www.amazon.com/Some-Book/dp/B00TEST123",
			want: true,
		},
		{
			name: "amazon gp product url",
			url:  "https://www.amazon.com/gp/product/0441569595",
			want: true,
		},
		{
			name: "amazon dp 13-digit isbn url",
			url:  "https://www.amazon.com/dp/9780441569595",
			want: true,
		},
		{
			name: "amazon gp product 13-digit isbn url",
			url:  "https://www.amazon.com/gp/product/9780441569595",
			want: true,
		},
		{
			name: "amazon regional host url",
			url:  "https://www.amazon.co.uk/dp/0441569595",
			want: true,
		},
		{
			name: "open library work url",
			url:  "https://openlibrary.org/works/OL45883W",
			want: true,
		},
		{
			name: "open library edition url",
			url:  "https://openlibrary.org/books/OL7353617M",
			want: true,
		},
		{
			name: "isbn in generic url",
			url:  "https://example.com/books/isbn-9780441569595/details",
			want: true,
		},
		{
			name: "non book url",
			url:  "https://example.com/posts/123",
			want: false,
		},
		{
			name: "invalid url",
			url:  "://bad-url",
			want: false,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := shouldExtractBookMetadata(tc.url); got != tc.want {
				t.Fatalf("shouldExtractBookMetadata(%q) = %v, want %v", tc.url, got, tc.want)
			}
		})
	}
}

func TestFetchMetadataBookURLIncludesBookMetadata(t *testing.T) {
	originalParseBookMetadataFunc := parseBookMetadataFunc
	t.Cleanup(func() {
		parseBookMetadataFunc = originalParseBookMetadataFunc
	})

	parseCalls := 0
	parseBookMetadataFunc = func(ctx context.Context, rawURL string, client *OpenLibraryClient) (*BookData, error) {
		parseCalls++
		if rawURL != "https://www.goodreads.com/book/show/22328-neuromancer" {
			t.Fatalf("rawURL = %q, want goodreads book url", rawURL)
		}
		if client == nil {
			t.Fatal("expected open library client to be provided")
		}
		return &BookData{Title: "Neuromancer"}, nil
	}

	fetcher := NewFetcher(&http.Client{
		Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     http.Header{"Content-Type": []string{"text/html; charset=utf-8"}},
				Body:       io.NopCloser(strings.NewReader(`<!doctype html><html><head><meta property="og:title" content="Fallback Title" /></head></html>`)),
				Request:    r,
			}, nil
		}),
	})
	fetcher.resolver = fakeResolver{
		addrs: map[string][]net.IPAddr{
			"www.goodreads.com": {{IP: net.ParseIP("93.184.216.34")}},
		},
	}

	metadata, err := fetcher.Fetch(context.Background(), "https://www.goodreads.com/book/show/22328-neuromancer")
	if err != nil {
		t.Fatalf("Fetch error: %v", err)
	}

	if parseCalls != 1 {
		t.Fatalf("parseCalls = %d, want 1", parseCalls)
	}

	bookData, ok := metadata["book_data"].(*BookData)
	if !ok || bookData == nil {
		t.Fatalf("expected book metadata to be present")
	}
	if bookData.Title != "Neuromancer" {
		t.Fatalf("book title = %q, want Neuromancer", bookData.Title)
	}
}

func TestFetchMetadataBookURLReturnsBookMetadataWhenFetchFails(t *testing.T) {
	originalParseBookMetadataFunc := parseBookMetadataFunc
	t.Cleanup(func() {
		parseBookMetadataFunc = originalParseBookMetadataFunc
	})

	parseCalls := 0
	parseBookMetadataFunc = func(ctx context.Context, rawURL string, client *OpenLibraryClient) (*BookData, error) {
		parseCalls++
		return &BookData{Title: "Neuromancer"}, nil
	}

	fetcher := NewFetcher(&http.Client{
		Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			return nil, errors.New("upstream unavailable")
		}),
	})
	fetcher.resolver = fakeResolver{
		addrs: map[string][]net.IPAddr{
			"www.goodreads.com": {{IP: net.ParseIP("93.184.216.34")}},
		},
	}

	metadata, err := fetcher.Fetch(context.Background(), "https://www.goodreads.com/book/show/22328-neuromancer")
	if err != nil {
		t.Fatalf("Fetch error: %v", err)
	}
	if parseCalls != 1 {
		t.Fatalf("parseCalls = %d, want 1", parseCalls)
	}

	bookData, ok := metadata["book_data"].(*BookData)
	if !ok || bookData == nil {
		t.Fatalf("expected book metadata to be present")
	}
	if metadata["provider"] != "www.goodreads.com" {
		t.Fatalf("provider = %v, want www.goodreads.com", metadata["provider"])
	}
}

func TestFetchMetadataBookURLReturnsBookMetadataWhenStatusIsNonSuccess(t *testing.T) {
	originalParseBookMetadataFunc := parseBookMetadataFunc
	t.Cleanup(func() {
		parseBookMetadataFunc = originalParseBookMetadataFunc
	})

	parseBookMetadataFunc = func(ctx context.Context, rawURL string, client *OpenLibraryClient) (*BookData, error) {
		return &BookData{Title: "Neuromancer"}, nil
	}

	fetcher := NewFetcher(&http.Client{
		Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusServiceUnavailable,
				Status:     "503 Service Unavailable",
				Header:     http.Header{"Content-Type": []string{"text/html"}},
				Body:       io.NopCloser(strings.NewReader("down")),
				Request:    r,
			}, nil
		}),
	})
	fetcher.resolver = fakeResolver{
		addrs: map[string][]net.IPAddr{
			"www.goodreads.com": {{IP: net.ParseIP("93.184.216.34")}},
		},
	}

	metadata, err := fetcher.Fetch(context.Background(), "https://www.goodreads.com/book/show/22328-neuromancer")
	if err != nil {
		t.Fatalf("Fetch error: %v", err)
	}
	if _, ok := metadata["book_data"].(*BookData); !ok {
		t.Fatalf("expected book metadata to be present")
	}
}

func TestFetchMetadataNonBookURLSkipsBookMetadata(t *testing.T) {
	originalParseBookMetadataFunc := parseBookMetadataFunc
	t.Cleanup(func() {
		parseBookMetadataFunc = originalParseBookMetadataFunc
	})

	parseCalls := 0
	parseBookMetadataFunc = func(ctx context.Context, rawURL string, client *OpenLibraryClient) (*BookData, error) {
		parseCalls++
		return &BookData{Title: "Should Not Be Used"}, nil
	}

	fetcher := NewFetcher(&http.Client{
		Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     http.Header{"Content-Type": []string{"text/html; charset=utf-8"}},
				Body:       io.NopCloser(strings.NewReader(`<!doctype html><html><head><meta property="og:title" content="Fallback Title" /></head></html>`)),
				Request:    r,
			}, nil
		}),
	})
	fetcher.resolver = fakeResolver{
		addrs: map[string][]net.IPAddr{
			"example.com": {{IP: net.ParseIP("93.184.216.34")}},
		},
	}

	metadata, err := fetcher.Fetch(context.Background(), "https://example.com/posts/123")
	if err != nil {
		t.Fatalf("Fetch error: %v", err)
	}

	if parseCalls != 0 {
		t.Fatalf("parseCalls = %d, want 0", parseCalls)
	}
	if _, ok := metadata["book_data"]; ok {
		t.Fatalf("expected book metadata to be absent")
	}
}

func TestFetchMetadataBookParserFailureFallsBackToHTMLMetadata(t *testing.T) {
	originalParseBookMetadataFunc := parseBookMetadataFunc
	t.Cleanup(func() {
		parseBookMetadataFunc = originalParseBookMetadataFunc
	})

	parseBookMetadataFunc = func(ctx context.Context, rawURL string, client *OpenLibraryClient) (*BookData, error) {
		return nil, errors.New("open library unavailable")
	}

	fetcher := NewFetcher(&http.Client{
		Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     http.Header{"Content-Type": []string{"text/html; charset=utf-8"}},
				Body: io.NopCloser(strings.NewReader(`<!doctype html>
					<html>
					<head>
						<title>Fallback Title</title>
						<meta property="og:title" content="Fallback Title" />
						<meta property="og:description" content="Fallback Description" />
					</head>
					</html>`)),
				Request: r,
			}, nil
		}),
	})
	fetcher.resolver = fakeResolver{
		addrs: map[string][]net.IPAddr{
			"www.goodreads.com": {{IP: net.ParseIP("93.184.216.34")}},
		},
	}

	metadata, err := fetcher.Fetch(context.Background(), "https://www.goodreads.com/book/show/22328-neuromancer")
	if err != nil {
		t.Fatalf("Fetch error: %v", err)
	}
	if metadata["title"] != "Fallback Title" {
		t.Fatalf("title = %v, want Fallback Title", metadata["title"])
	}
	if metadata["description"] != "Fallback Description" {
		t.Fatalf("description = %v, want Fallback Description", metadata["description"])
	}
	if _, ok := metadata["book_data"]; ok {
		t.Fatalf("expected book metadata to be absent when parsing fails")
	}
}

func TestFetchMetadataTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	fetcher := NewFetcher(&http.Client{
		Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			<-r.Context().Done()
			return nil, r.Context().Err()
		}),
	})
	fetcher.resolver = fakeResolver{
		addrs: map[string][]net.IPAddr{
			"example.com": {{IP: net.ParseIP("93.184.216.34")}},
		},
	}

	_, err := fetcher.Fetch(ctx, "https://example.com/slow")
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got %v", err)
	}
}

func TestFetchMetadataImageContent(t *testing.T) {
	fetcher := NewFetcher(&http.Client{
		Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     http.Header{"Content-Type": []string{"image/png"}},
				Body:       io.NopCloser(strings.NewReader("pngbytes")),
				Request:    r,
			}, nil
		}),
	})
	fetcher.resolver = fakeResolver{
		addrs: map[string][]net.IPAddr{
			"example.com": {{IP: net.ParseIP("93.184.216.34")}},
		},
	}

	metadata, err := fetcher.Fetch(context.Background(), "https://example.com/image.png")
	if err != nil {
		t.Fatalf("FetchMetadata error: %v", err)
	}

	if metadata["image"] != "https://example.com/image.png" {
		t.Errorf("image = %v, want %v", metadata["image"], "https://example.com/image.png")
	}
	if metadata["type"] != "image" {
		t.Errorf("type = %v, want image", metadata["type"])
	}
	if metadata["provider"] != "example.com" {
		t.Errorf("provider = %v, want example.com", metadata["provider"])
	}
}

func TestFetchMetadataImageFallbackByExtension(t *testing.T) {
	fetcher := NewFetcher(&http.Client{
		Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     http.Header{"Content-Type": []string{"text/plain"}},
				Body:       io.NopCloser(strings.NewReader("ok")),
				Request:    r,
			}, nil
		}),
	})
	fetcher.resolver = fakeResolver{
		addrs: map[string][]net.IPAddr{
			"example.com": {{IP: net.ParseIP("93.184.216.34")}},
		},
	}

	metadata, err := fetcher.Fetch(context.Background(), "https://example.com/photo.jpg")
	if err != nil {
		t.Fatalf("FetchMetadata error: %v", err)
	}

	if metadata["image"] != "https://example.com/photo.jpg" {
		t.Errorf("image = %v, want %v", metadata["image"], "https://example.com/photo.jpg")
	}
}

func TestFetchMetadataImageFallbackByQuery(t *testing.T) {
	fetcher := NewFetcher(&http.Client{
		Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     http.Header{"Content-Type": []string{"application/octet-stream"}},
				Body:       io.NopCloser(strings.NewReader("ok")),
				Request:    r,
			}, nil
		}),
	})
	fetcher.resolver = fakeResolver{
		addrs: map[string][]net.IPAddr{
			"example.com": {{IP: net.ParseIP("93.184.216.34")}},
		},
	}

	metadata, err := fetcher.Fetch(context.Background(), "https://example.com/asset?id=123&format=jpg")
	if err != nil {
		t.Fatalf("FetchMetadata error: %v", err)
	}

	if metadata["image"] != "https://example.com/asset?id=123&format=jpg" {
		t.Errorf("image = %v, want %v", metadata["image"], "https://example.com/asset?id=123&format=jpg")
	}
}

func TestFetchMetadataImageFallbackByQueryValue(t *testing.T) {
	fetcher := NewFetcher(&http.Client{
		Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     http.Header{"Content-Type": []string{"application/octet-stream"}},
				Body:       io.NopCloser(strings.NewReader("ok")),
				Request:    r,
			}, nil
		}),
	})
	fetcher.resolver = fakeResolver{
		addrs: map[string][]net.IPAddr{
			"example.com": {{IP: net.ParseIP("93.184.216.34")}},
		},
	}

	metadata, err := fetcher.Fetch(context.Background(), "https://example.com/asset?url=https://cdn.example.com/photo.png")
	if err != nil {
		t.Fatalf("FetchMetadata error: %v", err)
	}

	if metadata["image"] != "https://example.com/asset?url=https://cdn.example.com/photo.png" {
		t.Errorf("image = %v, want %v", metadata["image"], "https://example.com/asset?url=https://cdn.example.com/photo.png")
	}
}

func TestFetchMetadataBandcampEmbed(t *testing.T) {
	htmlBody := []byte(`<!doctype html>
		<html>
		<head>
			<title>Test Album | Test Artist</title>
			<meta property="og:title" content="Test Album" />
			<meta property="og:image" content="https://f4.bcbits.com/img/a12345.jpg" />
			<meta name="bc-page-properties" content="{&quot;item_type&quot;:&quot;a&quot;,&quot;item_id&quot;:12345}" />
		</head>
		</html>`)

	// Mock the Bandcamp fetch function
	SetBandcampFetchHTMLForTests(func(ctx context.Context, rawURL string) ([]byte, error) {
		if !strings.Contains(rawURL, "bandcamp.com") {
			t.Fatalf("expected bandcamp URL, got %q", rawURL)
		}
		return htmlBody, nil
	})
	defer SetBandcampFetchHTMLForTests(nil)

	fetcher := NewFetcher(nil)
	fetcher.resolver = fakeResolver{
		addrs: map[string][]net.IPAddr{
			"artist.bandcamp.com": {{IP: net.ParseIP("93.184.216.34")}},
		},
	}

	metadata, err := fetcher.Fetch(context.Background(), "https://artist.bandcamp.com/album/test")
	if err != nil {
		t.Fatalf("FetchMetadata error: %v", err)
	}

	if metadata["title"] != "Test Album" {
		t.Fatalf("title = %v, want Test Album", metadata["title"])
	}
	if metadata["provider"] != "bandcamp" {
		t.Fatalf("provider = %v, want bandcamp", metadata["provider"])
	}

	embed, ok := metadata["embed"].(*EmbedData)
	if !ok || embed == nil {
		t.Fatalf("expected bandcamp embed to be present")
	}
	if embed.Provider != "bandcamp" {
		t.Fatalf("embed provider = %v, want bandcamp", embed.Provider)
	}
	if embed.Height != bandcampAlbumHeight {
		t.Fatalf("height = %v, want %d", embed.Height, bandcampAlbumHeight)
	}
}

func TestFetchMetadataBandcampTrack(t *testing.T) {
	htmlBody := []byte(`<!doctype html>
		<html>
		<head>
			<title>Test Track | Test Artist</title>
			<meta property="og:title" content="Test Track" />
			<meta property="og:description" content="A test track description" />
			<meta property="og:image" content="https://f4.bcbits.com/img/a12345.jpg" />
			<meta name="bc-page-properties" content="{&quot;item_type&quot;:&quot;t&quot;,&quot;item_id&quot;:67890}" />
		</head>
		</html>`)

	// Mock the Bandcamp fetch function
	SetBandcampFetchHTMLForTests(func(ctx context.Context, rawURL string) ([]byte, error) {
		return htmlBody, nil
	})
	defer SetBandcampFetchHTMLForTests(nil)

	fetcher := NewFetcher(nil)
	fetcher.resolver = fakeResolver{
		addrs: map[string][]net.IPAddr{
			"artist.bandcamp.com": {{IP: net.ParseIP("93.184.216.34")}},
		},
	}

	metadata, err := fetcher.Fetch(context.Background(), "https://artist.bandcamp.com/track/test")
	if err != nil {
		t.Fatalf("FetchMetadata error: %v", err)
	}

	if metadata["title"] != "Test Track" {
		t.Fatalf("title = %v, want Test Track", metadata["title"])
	}
	if metadata["description"] != "A test track description" {
		t.Fatalf("description = %v, want A test track description", metadata["description"])
	}

	embed, ok := metadata["embed"].(*EmbedData)
	if !ok || embed == nil {
		t.Fatalf("expected bandcamp embed to be present")
	}
	if embed.Height != bandcampTrackHeight {
		t.Fatalf("height = %v, want %d", embed.Height, bandcampTrackHeight)
	}
}

func TestFetchMetadataBandcampFetchError(t *testing.T) {
	// Mock the Bandcamp fetch function to return an error
	SetBandcampFetchHTMLForTests(func(ctx context.Context, rawURL string) ([]byte, error) {
		return nil, errors.New("connection refused")
	})
	defer SetBandcampFetchHTMLForTests(nil)

	fetcher := NewFetcher(nil)
	fetcher.resolver = fakeResolver{
		addrs: map[string][]net.IPAddr{
			"artist.bandcamp.com": {{IP: net.ParseIP("93.184.216.34")}},
		},
	}

	_, err := fetcher.Fetch(context.Background(), "https://artist.bandcamp.com/track/test")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "connection refused") {
		t.Fatalf("expected connection refused error, got %v", err)
	}
}

func TestFetchMetadataRetriesOnServerError(t *testing.T) {
	htmlBody := `<!doctype html><html><head><title>Retry Title</title></head></html>`
	attempts := 0

	fetcher := NewFetcher(&http.Client{
		Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			attempts++
			if attempts == 1 {
				return &http.Response{
					StatusCode: http.StatusServiceUnavailable,
					Status:     "503 Service Unavailable",
					Header:     http.Header{"Content-Type": []string{"text/html; charset=utf-8"}},
					Body:       io.NopCloser(strings.NewReader("unavailable")),
					Request:    r,
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     http.Header{"Content-Type": []string{"text/html; charset=utf-8"}},
				Body:       io.NopCloser(strings.NewReader(htmlBody)),
				Request:    r,
			}, nil
		}),
	})
	fetcher.resolver = fakeResolver{
		addrs: map[string][]net.IPAddr{
			"example.com": {{IP: net.ParseIP("93.184.216.34")}},
		},
	}

	metadata, err := fetcher.Fetch(context.Background(), "https://example.com/retry")
	if err != nil {
		t.Fatalf("FetchMetadata error: %v", err)
	}
	if attempts < 2 {
		t.Fatalf("expected retry attempts, got %d", attempts)
	}
	if metadata["title"] != "Retry Title" {
		t.Fatalf("expected title metadata after retry, got %v", metadata["title"])
	}
}

func TestFetchMetadataImageFallbackSkipsHTML(t *testing.T) {
	fetcher := NewFetcher(&http.Client{
		Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     http.Header{"Content-Type": []string{"text/html; charset=utf-8"}},
				Body:       io.NopCloser(strings.NewReader("<html>login</html>")),
				Request:    r,
			}, nil
		}),
	})
	fetcher.resolver = fakeResolver{
		addrs: map[string][]net.IPAddr{
			"example.com": {{IP: net.ParseIP("93.184.216.34")}},
		},
	}

	metadata, err := fetcher.Fetch(context.Background(), "https://example.com/photo.jpg")
	if err != nil {
		t.Fatalf("FetchMetadata error: %v", err)
	}

	if _, ok := metadata["image"]; ok {
		t.Fatalf("expected no image metadata for HTML response")
	}
}

func TestIsInternalUploadURL(t *testing.T) {
	tests := []struct {
		rawURL string
		want   bool
	}{
		{rawURL: "/api/v1/uploads/128620aa-7f7e-47d6-9400-91699dc61e1a/photo.png", want: true},
		{rawURL: "/api/v1/uploads", want: true},
		{rawURL: "https://clubhouse.example/api/v1/uploads/128620aa-7f7e-47d6-9400-91699dc61e1a/photo.png", want: true},
		{rawURL: "https://clubhouse.example/api/v1/uploads", want: true},
		{rawURL: "https://example.com/api/v1/uploading/photo.png", want: false},
		{rawURL: "/api/v1/upload/photo.png", want: false},
		{rawURL: "https://example.com/photo.png", want: false},
		{rawURL: "", want: false},
	}

	for _, tt := range tests {
		if got := IsInternalUploadURL(tt.rawURL); got != tt.want {
			t.Errorf("IsInternalUploadURL(%q) = %v, want %v", tt.rawURL, got, tt.want)
		}
	}
}

func TestValidateURLBlocksHosts(t *testing.T) {
	fetcher := NewFetcher(&http.Client{})
	fetcher.resolver = fakeResolver{
		addrs: map[string][]net.IPAddr{
			"private.example": {{IP: net.ParseIP("10.0.0.10")}},
			"public.example":  {{IP: net.ParseIP("93.184.216.34")}},
		},
	}

	tests := []struct {
		rawURL  string
		allowed bool
	}{
		{rawURL: "http://localhost", allowed: false},
		{rawURL: "http://127.0.0.1", allowed: false},
		{rawURL: "http://169.254.169.254", allowed: false},
		{rawURL: "http://private.example", allowed: false},
		{rawURL: "https://public.example", allowed: true},
	}

	for _, tt := range tests {
		u, err := url.Parse(tt.rawURL)
		if err != nil {
			t.Fatalf("parse url: %v", err)
		}
		err = fetcher.validateURL(context.Background(), u)
		if tt.allowed && err != nil {
			t.Errorf("validateURL(%q) error = %v, want nil", tt.rawURL, err)
		}
		if !tt.allowed && err == nil {
			t.Errorf("validateURL(%q) = nil, want error", tt.rawURL)
		}
	}
}

func TestRedirectValidator(t *testing.T) {
	fetcher := NewFetcher(&http.Client{})
	fetcher.resolver = fakeResolver{
		addrs: map[string][]net.IPAddr{
			"example.com": {{IP: net.ParseIP("93.184.216.34")}},
		},
	}
	check := fetcher.redirectValidator(context.Background(), nil)

	req := &http.Request{URL: mustParseURL(t, "https://example.com")}
	via := make([]*http.Request, maxRedirects)
	if err := check(req, via); err == nil {
		t.Fatalf("expected redirect limit error")
	}

	reqBlocked := &http.Request{URL: mustParseURL(t, "http://localhost")}
	if err := check(reqBlocked, nil); err == nil {
		t.Fatalf("expected blocked redirect error")
	}
}

func TestDetectProvider(t *testing.T) {
	tests := []struct {
		host string
		want string
	}{
		{host: "open.spotify.com", want: "spotify"},
		{host: "www.youtube.com", want: "youtube"},
		{host: "youtu.be", want: "youtube"},
		{host: "imdb.com", want: "imdb"},
	}

	for _, tt := range tests {
		if got := detectProvider(tt.host); got != tt.want {
			t.Errorf("detectProvider(%q) = %q, want %q", tt.host, got, tt.want)
		}
	}
}

func mustParseURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}
	return parsed
}
