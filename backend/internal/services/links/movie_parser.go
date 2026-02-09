package links

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/sanderginn/clubhouse/internal/models"
)

const (
	tmdbImageBaseURL         = "https://image.tmdb.org/t/p"
	tmdbPosterSize           = "w500"
	tmdbBackdropSize         = "w1280"
	movieMetadataCastMaxSize = 5
)

var (
	imdbIDPattern       = regexp.MustCompile(`^tt\d+$`)
	leadingDigitsRegexp = regexp.MustCompile(`^(\d+)`)
)

type MovieData = models.MovieData

func ParseMovieMetadata(ctx context.Context, rawURL string, client *TMDBClient) (*MovieData, error) {
	if ctx == nil {
		return nil, errors.New("context is required")
	}
	if client == nil {
		return nil, errors.New("tmdb client is required")
	}

	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, nil
	}

	host := strings.ToLower(strings.TrimSpace(parsedURL.Hostname()))
	segments := splitURLPath(parsedURL.Path)

	switch {
	case isIMDbHost(host):
		imdbID, ok := parseIMDbID(segments)
		if !ok {
			return nil, nil
		}
		return parseIMDbMovieMetadata(ctx, client, imdbID)
	case isTMDBHost(host):
		mediaType, tmdbID, ok := parseTMDBPath(segments)
		if !ok {
			return nil, nil
		}
		return parseTMDBMovieMetadata(ctx, client, mediaType, tmdbID)
	case isLetterboxdHost(host):
		slug, ok := parseLetterboxdSlug(segments)
		if !ok {
			return nil, nil
		}
		return parseLetterboxdMovieMetadata(ctx, client, slug)
	default:
		return nil, nil
	}
}

func isIMDbHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	return host == "imdb.com" || strings.HasSuffix(host, ".imdb.com")
}

func isTMDBHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	return host == "themoviedb.org" || strings.HasSuffix(host, ".themoviedb.org")
}

func isLetterboxdHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	return host == "letterboxd.com" || strings.HasSuffix(host, ".letterboxd.com")
}

func splitURLPath(path string) []string {
	path = strings.Trim(path, "/")
	if path == "" {
		return nil
	}

	rawSegments := strings.Split(path, "/")
	segments := make([]string, 0, len(rawSegments))
	for _, segment := range rawSegments {
		segment = strings.TrimSpace(segment)
		if segment == "" {
			continue
		}
		unescaped, err := url.PathUnescape(segment)
		if err != nil {
			unescaped = segment
		}
		segments = append(segments, unescaped)
	}
	return segments
}

func parseIMDbID(segments []string) (string, bool) {
	if len(segments) < 2 || !strings.EqualFold(segments[0], "title") {
		return "", false
	}

	imdbID := strings.ToLower(strings.TrimSpace(segments[1]))
	if !imdbIDPattern.MatchString(imdbID) {
		return "", false
	}
	return imdbID, true
}

func parseTMDBPath(segments []string) (string, int, bool) {
	if len(segments) < 2 {
		return "", 0, false
	}

	mediaType := strings.ToLower(strings.TrimSpace(segments[0]))
	if mediaType != "movie" && mediaType != "tv" {
		return "", 0, false
	}

	match := leadingDigitsRegexp.FindStringSubmatch(strings.TrimSpace(segments[1]))
	if len(match) != 2 {
		return "", 0, false
	}

	tmdbID, err := strconv.Atoi(match[1])
	if err != nil || tmdbID <= 0 {
		return "", 0, false
	}
	return mediaType, tmdbID, true
}

func parseLetterboxdSlug(segments []string) (string, bool) {
	if len(segments) < 2 || !strings.EqualFold(segments[0], "film") {
		return "", false
	}

	slug := strings.TrimSpace(segments[1])
	if slug == "" {
		return "", false
	}
	return slug, true
}

func parseIMDbMovieMetadata(ctx context.Context, client *TMDBClient, imdbID string) (*MovieData, error) {
	result, err := client.FindByIMDBID(ctx, imdbID)
	if err != nil {
		return nil, fmt.Errorf("find tmdb result by imdb id %s: %w", imdbID, err)
	}

	if len(result.MovieResults) > 0 {
		return parseTMDBMovieMetadata(ctx, client, "movie", result.MovieResults[0].ID)
	}
	if len(result.TVResults) > 0 {
		return parseTMDBMovieMetadata(ctx, client, "tv", result.TVResults[0].ID)
	}

	return nil, nil
}

func parseTMDBMovieMetadata(ctx context.Context, client *TMDBClient, mediaType string, tmdbID int) (*MovieData, error) {
	switch mediaType {
	case "movie":
		details, err := client.GetMovieDetails(ctx, tmdbID)
		if err != nil {
			return nil, fmt.Errorf("get tmdb movie details for id %d: %w", tmdbID, err)
		}
		return movieDataFromMovieDetails(details, mediaType), nil
	case "tv":
		details, err := client.GetTVDetails(ctx, tmdbID)
		if err != nil {
			return nil, fmt.Errorf("get tmdb tv details for id %d: %w", tmdbID, err)
		}
		return movieDataFromTVDetails(details, mediaType), nil
	default:
		return nil, nil
	}
}

func parseLetterboxdMovieMetadata(ctx context.Context, client *TMDBClient, slug string) (*MovieData, error) {
	titleQuery := letterboxdSlugToTitle(slug)
	if titleQuery == "" {
		return nil, nil
	}

	results, err := client.SearchMovie(ctx, titleQuery)
	if err != nil {
		return nil, fmt.Errorf("search tmdb movie for letterboxd slug %s: %w", slug, err)
	}
	if len(results) == 0 {
		return nil, nil
	}

	return parseTMDBMovieMetadata(ctx, client, "movie", results[0].ID)
}

func letterboxdSlugToTitle(slug string) string {
	slug = strings.ToLower(strings.TrimSpace(slug))
	slug = strings.Trim(slug, "/")
	if slug == "" {
		return ""
	}

	title := strings.ReplaceAll(slug, "-", " ")
	return strings.Join(strings.Fields(title), " ")
}

func movieDataFromMovieDetails(details *MovieDetails, mediaType string) *MovieData {
	if details == nil {
		return nil
	}

	return &MovieData{
		Title:         strings.TrimSpace(details.Title),
		Overview:      strings.TrimSpace(details.Overview),
		Poster:        tmdbImageURL(details.PosterPath, tmdbPosterSize),
		Backdrop:      tmdbImageURL(details.BackdropPath, tmdbBackdropSize),
		Runtime:       details.Runtime,
		Genres:        tmdbGenreNames(details.Genres),
		ReleaseDate:   strings.TrimSpace(details.ReleaseDate),
		Cast:          tmdbCastMembers(details.Credits.Cast, movieMetadataCastMaxSize),
		Director:      strings.TrimSpace(details.Director),
		TMDBRating:    details.VoteAverage,
		TrailerKey:    tmdbTrailerKey(details.Videos.Results),
		TMDBID:        details.ID,
		TMDBMediaType: strings.TrimSpace(mediaType),
	}
}

func movieDataFromTVDetails(details *TVDetails, mediaType string) *MovieData {
	if details == nil {
		return nil
	}

	return &MovieData{
		Title:         strings.TrimSpace(details.Name),
		Overview:      strings.TrimSpace(details.Overview),
		Poster:        tmdbImageURL(details.PosterPath, tmdbPosterSize),
		Backdrop:      tmdbImageURL(details.BackdropPath, tmdbBackdropSize),
		Runtime:       details.Runtime,
		Genres:        tmdbGenreNames(details.Genres),
		ReleaseDate:   strings.TrimSpace(details.FirstAirDate),
		Cast:          tmdbCastMembers(details.Credits.Cast, movieMetadataCastMaxSize),
		Seasons:       tmdbSeasons(details.Seasons),
		Director:      strings.TrimSpace(details.Director),
		TMDBRating:    details.VoteAverage,
		TrailerKey:    tmdbTrailerKey(details.Videos.Results),
		TMDBID:        details.ID,
		TMDBMediaType: strings.TrimSpace(mediaType),
	}
}

func tmdbImageURL(pathValue, size string) string {
	pathValue = strings.TrimSpace(pathValue)
	if pathValue == "" {
		return ""
	}
	if strings.HasPrefix(pathValue, "https://") || strings.HasPrefix(pathValue, "http://") {
		return pathValue
	}

	if !strings.HasPrefix(pathValue, "/") {
		pathValue = "/" + pathValue
	}

	return fmt.Sprintf("%s/%s%s", tmdbImageBaseURL, size, pathValue)
}

func tmdbGenreNames(genres []TMDBGenre) []string {
	if len(genres) == 0 {
		return nil
	}

	names := make([]string, 0, len(genres))
	for _, genre := range genres {
		name := strings.TrimSpace(genre.Name)
		if name == "" {
			continue
		}
		names = append(names, name)
	}
	if len(names) == 0 {
		return nil
	}
	return names
}

func tmdbCastMembers(cast []TMDBCastMember, limit int) []models.CastMember {
	if len(cast) == 0 || limit <= 0 {
		return nil
	}

	orderedCast := make([]TMDBCastMember, 0, len(cast))
	for _, member := range cast {
		if strings.TrimSpace(member.Name) == "" {
			continue
		}
		orderedCast = append(orderedCast, member)
	}
	sort.SliceStable(orderedCast, func(i, j int) bool {
		return orderedCast[i].Order < orderedCast[j].Order
	})

	if len(orderedCast) > limit {
		orderedCast = orderedCast[:limit]
	}

	result := make([]models.CastMember, 0, len(orderedCast))
	for _, member := range orderedCast {
		result = append(result, models.CastMember{
			Name:      strings.TrimSpace(member.Name),
			Character: strings.TrimSpace(member.Character),
		})
	}

	if len(result) == 0 {
		return nil
	}
	return result
}

func tmdbSeasons(seasons []TMDBSeason) []models.Season {
	if len(seasons) == 0 {
		return nil
	}

	orderedSeasons := make([]TMDBSeason, len(seasons))
	copy(orderedSeasons, seasons)
	sort.SliceStable(orderedSeasons, func(i, j int) bool {
		return orderedSeasons[i].SeasonNumber < orderedSeasons[j].SeasonNumber
	})

	result := make([]models.Season, 0, len(orderedSeasons))
	for _, season := range orderedSeasons {
		result = append(result, models.Season{
			SeasonNumber: season.SeasonNumber,
			EpisodeCount: season.EpisodeCount,
			AirDate:      strings.TrimSpace(season.AirDate),
			Name:         strings.TrimSpace(season.Name),
			Overview:     strings.TrimSpace(season.Overview),
			Poster:       tmdbImageURL(season.PosterPath, tmdbPosterSize),
		})
	}

	if len(result) == 0 {
		return nil
	}

	return result
}

func tmdbTrailerKey(videos []TMDBVideo) string {
	if len(videos) == 0 {
		return ""
	}

	type trailerMatcher struct {
		official bool
		kind     string
	}

	matchers := []trailerMatcher{
		{official: true, kind: "trailer"},
		{official: false, kind: "trailer"},
		{official: true, kind: "teaser"},
		{official: false, kind: ""},
	}

	for _, matcher := range matchers {
		for _, video := range videos {
			if !strings.EqualFold(strings.TrimSpace(video.Site), "youtube") {
				continue
			}

			videoType := strings.ToLower(strings.TrimSpace(video.Type))
			if matcher.kind != "" && videoType != matcher.kind {
				continue
			}
			if matcher.official && !video.Official {
				continue
			}

			key := strings.TrimSpace(video.Key)
			if key != "" {
				return key
			}
		}
	}

	return ""
}
