package links

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// allowedEmbedDomains defines the only iframe domains we accept for embeds.
// This blocks malicious iframe injection and keeps CSP + embed validation aligned.
var allowedEmbedDomains = map[string]struct{}{
	"www.youtube-nocookie.com": {},
	"open.spotify.com":         {},
	"w.soundcloud.com":         {},
	"bandcamp.com":             {},
}

func validateEmbedURL(embedURL string) error {
	if strings.TrimSpace(embedURL) == "" {
		return errors.New("embed url is required")
	}

	parsed, err := url.Parse(embedURL)
	if err != nil {
		return fmt.Errorf("parse embed url: %w", err)
	}

	if parsed.Scheme != "https" {
		return errors.New("embed url must use https")
	}

	host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	if host == "" {
		return errors.New("embed url missing host")
	}

	if _, ok := allowedEmbedDomains[host]; !ok {
		return fmt.Errorf("embed domain not allowed: %s", host)
	}

	return nil
}
