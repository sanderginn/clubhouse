package links

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseBookMetadataGoodreadsURL(t *testing.T) {
	originalFetchTitle := fetchBookPageTitleFunc
	fetchBookPageTitleFunc = func(ctx context.Context, rawURL string) (string, error) {
		if rawURL != "https://www.goodreads.com/book/show/22328-neuromancer" {
			t.Fatalf("rawURL = %q, want goodreads URL", rawURL)
		}
		return "Neuromancer by William Gibson | Goodreads", nil
	}
	defer func() {
		fetchBookPageTitleFunc = originalFetchTitle
	}()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/search.json":
			if got := strings.TrimSpace(r.URL.Query().Get("q")); got != "Neuromancer" {
				t.Fatalf("q = %q, want Neuromancer", got)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"docs":[
					{
						"title":"Neuromancer",
						"author_name":["William Gibson"],
						"first_publish_year":1984,
						"cover_i":12345,
						"key":"/works/OL45883W"
					}
				]
			}`))
		case "/works/OL45883W.json":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"title":"Neuromancer",
				"description":{"value":"A cyberpunk classic."},
				"subjects":["Science fiction","Cyberpunk"],
				"covers":[12345]
			}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := newTestOpenLibraryClient(t, server.URL)

	metadata, err := ParseBookMetadata(context.Background(), "https://www.goodreads.com/book/show/22328-neuromancer", client)
	if err != nil {
		t.Fatalf("ParseBookMetadata error: %v", err)
	}
	if metadata == nil {
		t.Fatal("expected metadata")
	}
	if metadata.Title != "Neuromancer" {
		t.Fatalf("Title = %q, want Neuromancer", metadata.Title)
	}
	if len(metadata.Authors) != 1 || metadata.Authors[0] != "William Gibson" {
		t.Fatalf("Authors = %#v, want [William Gibson]", metadata.Authors)
	}
	if metadata.Description != "A cyberpunk classic." {
		t.Fatalf("Description = %q, want A cyberpunk classic.", metadata.Description)
	}
	if metadata.CoverURL != "https://covers.openlibrary.org/b/id/12345-L.jpg" {
		t.Fatalf("CoverURL = %q", metadata.CoverURL)
	}
	if metadata.PublishDate != "1984" {
		t.Fatalf("PublishDate = %q, want 1984", metadata.PublishDate)
	}
	if metadata.OpenLibraryKey != "/works/OL45883W" {
		t.Fatalf("OpenLibraryKey = %q, want /works/OL45883W", metadata.OpenLibraryKey)
	}
	if metadata.GoodreadsURL != "https://www.goodreads.com/book/show/22328-neuromancer" {
		t.Fatalf("GoodreadsURL = %q, want input URL", metadata.GoodreadsURL)
	}
}

func TestParseBookMetadataAmazonURL(t *testing.T) {
	originalFetchTitle := fetchBookPageTitleFunc
	fetchBookPageTitleFunc = func(ctx context.Context, rawURL string) (string, error) {
		t.Fatalf("unexpected title fetch for URL: %s", rawURL)
		return "", nil
	}
	defer func() {
		fetchBookPageTitleFunc = originalFetchTitle
	}()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/isbn/B00TEST123.json" {
			t.Fatalf("path = %q, want /isbn/B00TEST123.json", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"title":"Amazon Book",
			"publish_date":"2020",
			"number_of_pages":320,
			"isbn_13":["9780441569595"],
			"isbn_10":["0441569595"],
			"covers":[54321]
		}`))
	}))
	defer server.Close()

	client := newTestOpenLibraryClient(t, server.URL)
	metadata, err := ParseBookMetadata(context.Background(), "https://www.amazon.com/Some-Book/dp/B00TEST123", client)
	if err != nil {
		t.Fatalf("ParseBookMetadata error: %v", err)
	}
	if metadata == nil {
		t.Fatal("expected metadata")
	}
	if metadata.Title != "Amazon Book" {
		t.Fatalf("Title = %q, want Amazon Book", metadata.Title)
	}
	if metadata.PageCount != 320 {
		t.Fatalf("PageCount = %d, want 320", metadata.PageCount)
	}
	if metadata.ISBN != "9780441569595" {
		t.Fatalf("ISBN = %q, want 9780441569595", metadata.ISBN)
	}
	if metadata.CoverURL != "https://covers.openlibrary.org/b/id/54321-L.jpg" {
		t.Fatalf("CoverURL = %q", metadata.CoverURL)
	}
}

func TestParseBookMetadataAmazonCanonicalDPURL(t *testing.T) {
	originalFetchTitle := fetchBookPageTitleFunc
	fetchBookPageTitleFunc = func(ctx context.Context, rawURL string) (string, error) {
		t.Fatalf("unexpected title fetch for URL: %s", rawURL)
		return "", nil
	}
	defer func() {
		fetchBookPageTitleFunc = originalFetchTitle
	}()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/isbn/B00TEST123.json" {
			t.Fatalf("path = %q, want /isbn/B00TEST123.json", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"title":"Amazon Book",
			"publish_date":"2020",
			"number_of_pages":320,
			"isbn_13":["9780441569595"],
			"covers":[54321]
		}`))
	}))
	defer server.Close()

	client := newTestOpenLibraryClient(t, server.URL)
	metadata, err := ParseBookMetadata(context.Background(), "https://www.amazon.com/dp/B00TEST123", client)
	if err != nil {
		t.Fatalf("ParseBookMetadata error: %v", err)
	}
	if metadata == nil {
		t.Fatal("expected metadata")
	}
	if metadata.Title != "Amazon Book" {
		t.Fatalf("Title = %q, want Amazon Book", metadata.Title)
	}
}

func TestParseBookMetadataOpenLibraryWorkURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/works/OL45883W.json" {
			t.Fatalf("path = %q, want /works/OL45883W.json", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"title":"Neuromancer",
			"description":"A cyberpunk classic.",
			"subjects":["Science fiction","Cyberpunk"],
			"covers":[12345]
		}`))
	}))
	defer server.Close()

	client := newTestOpenLibraryClient(t, server.URL)
	metadata, err := ParseBookMetadata(context.Background(), "https://openlibrary.org/works/OL45883W", client)
	if err != nil {
		t.Fatalf("ParseBookMetadata error: %v", err)
	}
	if metadata == nil {
		t.Fatal("expected metadata")
	}
	if metadata.Title != "Neuromancer" {
		t.Fatalf("Title = %q, want Neuromancer", metadata.Title)
	}
	if metadata.OpenLibraryKey != "/works/OL45883W" {
		t.Fatalf("OpenLibraryKey = %q, want /works/OL45883W", metadata.OpenLibraryKey)
	}
}

func TestParseBookMetadataOpenLibraryEditionURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/books/OL7353617M.json" {
			t.Fatalf("path = %q, want /books/OL7353617M.json", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"title":"Neuromancer",
			"publish_date":"1984",
			"number_of_pages":271,
			"isbn_13":["9780441569595"],
			"isbn_10":["0441569595"],
			"covers":[12345]
		}`))
	}))
	defer server.Close()

	client := newTestOpenLibraryClient(t, server.URL)
	metadata, err := ParseBookMetadata(context.Background(), "https://openlibrary.org/books/OL7353617M", client)
	if err != nil {
		t.Fatalf("ParseBookMetadata error: %v", err)
	}
	if metadata == nil {
		t.Fatal("expected metadata")
	}
	if metadata.Title != "Neuromancer" {
		t.Fatalf("Title = %q, want Neuromancer", metadata.Title)
	}
	if metadata.OpenLibraryKey != "/books/OL7353617M" {
		t.Fatalf("OpenLibraryKey = %q, want /books/OL7353617M", metadata.OpenLibraryKey)
	}
	if metadata.PageCount != 271 {
		t.Fatalf("PageCount = %d, want 271", metadata.PageCount)
	}
	if metadata.ISBN != "9780441569595" {
		t.Fatalf("ISBN = %q, want 9780441569595", metadata.ISBN)
	}
}

func TestParseBookMetadataISBNURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/isbn/9780441569595.json" {
			t.Fatalf("path = %q, want /isbn/9780441569595.json", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"title":"Neuromancer",
			"publish_date":"1984",
			"number_of_pages":271,
			"isbn_13":["9780441569595"],
			"covers":[12345]
		}`))
	}))
	defer server.Close()

	client := newTestOpenLibraryClient(t, server.URL)
	metadata, err := ParseBookMetadata(context.Background(), "https://example.com/books/isbn-9780441569595/details", client)
	if err != nil {
		t.Fatalf("ParseBookMetadata error: %v", err)
	}
	if metadata == nil {
		t.Fatal("expected metadata")
	}
	if metadata.ISBN != "9780441569595" {
		t.Fatalf("ISBN = %q, want 9780441569595", metadata.ISBN)
	}
	if metadata.Title != "Neuromancer" {
		t.Fatalf("Title = %q, want Neuromancer", metadata.Title)
	}
}

func TestParseBookMetadataNoMatchingURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("unexpected request: %s", r.URL.String())
	}))
	defer server.Close()

	client := newTestOpenLibraryClient(t, server.URL)
	metadata, err := ParseBookMetadata(context.Background(), "https://example.com/not-a-book-link", client)
	if err != nil {
		t.Fatalf("ParseBookMetadata error: %v", err)
	}
	if metadata != nil {
		t.Fatalf("metadata = %+v, want nil", metadata)
	}
}

func TestParseBookMetadataOpenLibraryErrorsHandledGracefully(t *testing.T) {
	t.Run("not found returns nil", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":"not found"}`))
		}))
		defer server.Close()

		client := newTestOpenLibraryClient(t, server.URL)
		metadata, err := ParseBookMetadata(context.Background(), "https://openlibrary.org/works/OL404W", client)
		if err != nil {
			t.Fatalf("ParseBookMetadata error: %v", err)
		}
		if metadata != nil {
			t.Fatalf("metadata = %+v, want nil", metadata)
		}
	})

	t.Run("api error returns error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"internal server error"}`))
		}))
		defer server.Close()

		client := newTestOpenLibraryClient(t, server.URL)
		metadata, err := ParseBookMetadata(context.Background(), "https://openlibrary.org/works/OL500W", client)
		if err == nil {
			t.Fatal("expected error")
		}
		if metadata != nil {
			t.Fatalf("metadata = %+v, want nil", metadata)
		}
	})
}
