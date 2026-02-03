package links

import "testing"

func TestValidateEmbedURLAllowsWhitelistedHTTPS(t *testing.T) {
	cases := []string{
		"https://www.youtube-nocookie.com/embed/abc123",
		"https://open.spotify.com/embed/track/xyz",
		"https://w.soundcloud.com/player/?url=https%3A//api.soundcloud.com/tracks/123",
		"https://bandcamp.com/EmbeddedPlayer/album=123/size=large/",
	}

	for _, tc := range cases {
		t.Run(tc, func(t *testing.T) {
			if err := validateEmbedURL(tc); err != nil {
				t.Fatalf("expected url %q to be valid, got %v", tc, err)
			}
		})
	}
}

func TestValidateEmbedURLRejectsInvalid(t *testing.T) {
	cases := []struct {
		name string
		url  string
	}{
		{name: "empty", url: ""},
		{name: "invalid", url: "://bad-url"},
		{name: "http", url: "http://open.spotify.com/embed/track/xyz"},
		{name: "non-whitelist", url: "https://evil.example.com/embed/track/xyz"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateEmbedURL(tc.url); err == nil {
				t.Fatalf("expected url %q to be rejected", tc.url)
			}
		})
	}
}
