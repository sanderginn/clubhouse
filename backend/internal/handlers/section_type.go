package handlers

import (
	"errors"
	"strings"
)

var errInvalidSectionType = errors.New("invalid section type")

func parseMovieOrSeriesSectionType(raw string) (*string, error) {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	if normalized == "" {
		return nil, nil
	}
	if normalized != "movie" && normalized != "series" {
		return nil, errInvalidSectionType
	}
	return &normalized, nil
}
