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
