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

func TestOpenLibraryClientSearchBooks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search.json" {
			t.Fatalf("path = %q, want /search.json", r.URL.Path)
		}
		if r.URL.Query().Get("q") != "Neuromancer" {
			t.Fatalf("q = %q, want Neuromancer", r.URL.Query().Get("q"))
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
	}))
	defer server.Close()

	client := newTestOpenLibraryClient(t, server.URL)

	results, err := client.SearchBooks(context.Background(), "Neuromancer")
	if err != nil {
		t.Fatalf("SearchBooks error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("results len = %d, want 1", len(results))
	}
	if results[0].Title != "Neuromancer" {
		t.Fatalf("title = %q, want Neuromancer", results[0].Title)
	}
	if results[0].CoverID != 12345 {
		t.Fatalf("cover id = %d, want 12345", results[0].CoverID)
	}
}

func TestOpenLibraryClientGetWork(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/works/OL45883W.json" {
			t.Fatalf("path = %q, want /works/OL45883W.json", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"title":"Neuromancer",
			"description":{"value":"A cyberpunk classic."},
			"subjects":["Science fiction","Cyberpunk"],
			"covers":[12345],
			"authors":[{"author":{"key":"/authors/OL26700A"}}]
		}`))
	}))
	defer server.Close()

	client := newTestOpenLibraryClient(t, server.URL)

	work, err := client.GetWork(context.Background(), "/works/OL45883W")
	if err != nil {
		t.Fatalf("GetWork error: %v", err)
	}
	if work.Title != "Neuromancer" {
		t.Fatalf("title = %q, want Neuromancer", work.Title)
	}
	if work.Description != "A cyberpunk classic." {
		t.Fatalf("description = %q, want A cyberpunk classic.", work.Description)
	}
	if len(work.Authors) != 1 || work.Authors[0].Author.Key != "/authors/OL26700A" {
		t.Fatalf("unexpected authors payload: %+v", work.Authors)
	}
}

func TestOpenLibraryClientGetEdition(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/books/OL7353617M.json" {
			t.Fatalf("path = %q, want /books/OL7353617M.json", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"title":"Neuromancer",
			"publishers":["Ace Books"],
			"publish_date":"1984",
			"number_of_pages":271,
			"isbn_13":["9780441569595"],
			"isbn_10":["0441569595"],
			"covers":[12345]
		}`))
	}))
	defer server.Close()

	client := newTestOpenLibraryClient(t, server.URL)

	edition, err := client.GetEdition(context.Background(), "/books/OL7353617M")
	if err != nil {
		t.Fatalf("GetEdition error: %v", err)
	}
	if edition.Title != "Neuromancer" {
		t.Fatalf("title = %q, want Neuromancer", edition.Title)
	}
	if edition.NumberOfPages != 271 {
		t.Fatalf("number_of_pages = %d, want 271", edition.NumberOfPages)
	}
}

func TestOpenLibraryClientGetByISBN(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/isbn/9780441569595.json" {
			t.Fatalf("path = %q, want /isbn/9780441569595.json", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"title":"Neuromancer",
			"publishers":["Ace Books"],
			"publish_date":"1984",
			"number_of_pages":271,
			"isbn_13":["9780441569595"],
			"isbn_10":["0441569595"],
			"covers":[12345]
		}`))
	}))
	defer server.Close()

	client := newTestOpenLibraryClient(t, server.URL)

	edition, err := client.GetByISBN(context.Background(), "9780441569595")
	if err != nil {
		t.Fatalf("GetByISBN error: %v", err)
	}
	if len(edition.ISBN13) != 1 || edition.ISBN13[0] != "9780441569595" {
		t.Fatalf("unexpected isbn_13 payload: %+v", edition.ISBN13)
	}
}

func TestOpenLibraryClientCoverURL(t *testing.T) {
	client := NewOpenLibraryClient(time.Second)

	tests := []struct {
		name    string
		coverID int
		size    string
		want    string
	}{
		{
			name:    "small",
			coverID: 12345,
			size:    "S",
			want:    "https://covers.openlibrary.org/b/id/12345-S.jpg",
		},
		{
			name:    "medium default",
			coverID: 12345,
			size:    "",
			want:    "https://covers.openlibrary.org/b/id/12345-M.jpg",
		},
		{
			name:    "large lowercase",
			coverID: 12345,
			size:    "l",
			want:    "https://covers.openlibrary.org/b/id/12345-L.jpg",
		},
		{
			name:    "invalid cover id",
			coverID: 0,
			size:    "M",
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := client.CoverURL(tt.coverID, tt.size); got != tt.want {
				t.Fatalf("CoverURL(%d, %q) = %q, want %q", tt.coverID, tt.size, got, tt.want)
			}
		})
	}
}

func TestOpenLibraryClientAPIErrors(t *testing.T) {
	t.Run("not found", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":"not found"}`))
		}))
		defer server.Close()

		client := newTestOpenLibraryClient(t, server.URL)
		_, err := client.GetWork(context.Background(), "/works/DOES_NOT_EXIST")
		if err == nil {
			t.Fatal("expected error")
		}
		if !errors.Is(err, ErrOpenLibraryNotFound) {
			t.Fatalf("expected ErrOpenLibraryNotFound, got %v", err)
		}

		var apiErr *OpenLibraryAPIError
		if !errors.As(err, &apiErr) {
			t.Fatalf("expected OpenLibraryAPIError, got %T", err)
		}
		if apiErr.StatusCode != http.StatusNotFound {
			t.Fatalf("status code = %d, want 404", apiErr.StatusCode)
		}
	})

	t.Run("malformed json", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"docs":[`))
		}))
		defer server.Close()

		client := newTestOpenLibraryClient(t, server.URL)
		_, err := client.SearchBooks(context.Background(), "Neuromancer")
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "decode open library response") {
			t.Fatalf("error = %q, want decode error", err.Error())
		}
	})

	t.Run("timeout", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(80 * time.Millisecond)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"title":"Neuromancer"}`))
		}))
		defer server.Close()

		client := newTestOpenLibraryClient(t, server.URL)
		client.httpClient.Timeout = 20 * time.Millisecond

		_, err := client.GetByISBN(context.Background(), "9780441569595")
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "open library request failed") {
			t.Fatalf("error = %q, want request failure", err.Error())
		}
	})
}

func newTestOpenLibraryClient(t *testing.T, baseURL string) *OpenLibraryClient {
	t.Helper()

	client := NewOpenLibraryClientWithHTTPClient(&http.Client{Timeout: time.Second})
	client.baseURL = baseURL

	return client
}
