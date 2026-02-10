package links

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseMovieMetadataIMDbURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/find/tt0133093":
			if got := r.URL.Query().Get("external_source"); got != "imdb_id" {
				t.Fatalf("external_source = %q, want imdb_id", got)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"movie_results":[{"id":603,"title":"The Matrix"}],"tv_results":[]}`))
		case "/movie/603":
			if got := r.URL.Query().Get("append_to_response"); got != "credits,videos,external_ids" {
				t.Fatalf("append_to_response = %q, want credits,videos,external_ids", got)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"id":603,
				"title":"The Matrix",
				"overview":"Reality is not what it seems.",
				"poster_path":"/f89U3ADr1oiB1s9GkdPOEpXUk5H.jpg",
				"backdrop_path":"/icmmSD4vTTDKOq2vvdulafOGw93.jpg",
				"runtime":136,
				"genres":[{"id":28,"name":"Action"},{"id":878,"name":"Science Fiction"}],
				"release_date":"1999-03-30",
				"vote_average":8.2,
				"imdb_id":"tt0133093",
				"credits":{
					"cast":[
						{"id":1,"name":"Keanu Reeves","character":"Neo","order":0},
						{"id":2,"name":"Carrie-Anne Moss","character":"Trinity","order":1},
						{"id":3,"name":"Laurence Fishburne","character":"Morpheus","order":2},
						{"id":4,"name":"Hugo Weaving","character":"Agent Smith","order":3},
						{"id":5,"name":"Gloria Foster","character":"Oracle","order":4},
						{"id":6,"name":"Joe Pantoliano","character":"Cypher","order":5}
					],
					"crew":[{"id":101,"name":"Lana Wachowski","job":"Director","department":"Directing"}]
				},
				"videos":{
					"results":[
						{"id":"v1","key":"teaser-key","name":"Teaser","site":"YouTube","type":"Teaser","official":true},
						{"id":"v2","key":"trailer-key","name":"Trailer","site":"YouTube","type":"Trailer","official":true}
					]
				}
			}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := newTestTMDBClient(t, server.URL)
	metadata, err := ParseMovieMetadata(context.Background(), "https://www.imdb.com/title/tt0133093/", client, nil)
	if err != nil {
		t.Fatalf("ParseMovieMetadata error: %v", err)
	}
	if metadata == nil {
		t.Fatal("expected metadata")
	}
	if metadata.Title != "The Matrix" {
		t.Fatalf("Title = %q, want The Matrix", metadata.Title)
	}
	if metadata.ReleaseDate != "1999-03-30" {
		t.Fatalf("ReleaseDate = %q, want 1999-03-30", metadata.ReleaseDate)
	}
	if metadata.Runtime != 136 {
		t.Fatalf("Runtime = %d, want 136", metadata.Runtime)
	}
	if metadata.Director != "Lana Wachowski" {
		t.Fatalf("Director = %q, want Lana Wachowski", metadata.Director)
	}
	if metadata.TMDBID != 603 {
		t.Fatalf("TMDBID = %d, want 603", metadata.TMDBID)
	}
	if metadata.TMDBMediaType != "movie" {
		t.Fatalf("TMDBMediaType = %q, want movie", metadata.TMDBMediaType)
	}
	if metadata.IMDBID != "tt0133093" {
		t.Fatalf("IMDBID = %q, want tt0133093", metadata.IMDBID)
	}
	if metadata.RottenTomatoesURL != "https://www.rottentomatoes.com/m/the_matrix" {
		t.Fatalf("RottenTomatoesURL = %q, want https://www.rottentomatoes.com/m/the_matrix", metadata.RottenTomatoesURL)
	}
	if metadata.TrailerKey != "trailer-key" {
		t.Fatalf("TrailerKey = %q, want trailer-key", metadata.TrailerKey)
	}
	if len(metadata.Cast) != 5 {
		t.Fatalf("Cast len = %d, want 5", len(metadata.Cast))
	}
	if metadata.Poster != "https://image.tmdb.org/t/p/w500/f89U3ADr1oiB1s9GkdPOEpXUk5H.jpg" {
		t.Fatalf("Poster = %q", metadata.Poster)
	}
	if metadata.Backdrop != "https://image.tmdb.org/t/p/w1280/icmmSD4vTTDKOq2vvdulafOGw93.jpg" {
		t.Fatalf("Backdrop = %q", metadata.Backdrop)
	}
}

func TestParseMovieMetadataTMDBMovieURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/movie/550" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
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
			"credits":{"cast":[],"crew":[{"id":7467,"name":"David Fincher","job":"Director","department":"Directing"}]},
			"videos":{"results":[{"id":"abc123","key":"SUXWAEX2jlg","name":"Trailer","site":"YouTube","type":"Trailer","official":true}]}
		}`))
	}))
	defer server.Close()

	client := newTestTMDBClient(t, server.URL)
	metadata, err := ParseMovieMetadata(context.Background(), "https://www.themoviedb.org/movie/550-fight-club", client, nil)
	if err != nil {
		t.Fatalf("ParseMovieMetadata error: %v", err)
	}
	if metadata == nil {
		t.Fatal("expected metadata")
	}
	if metadata.Title != "Fight Club" {
		t.Fatalf("Title = %q, want Fight Club", metadata.Title)
	}
	if metadata.Director != "David Fincher" {
		t.Fatalf("Director = %q, want David Fincher", metadata.Director)
	}
	if metadata.TMDBID != 550 {
		t.Fatalf("TMDBID = %d, want 550", metadata.TMDBID)
	}
	if metadata.TMDBMediaType != "movie" {
		t.Fatalf("TMDBMediaType = %q, want movie", metadata.TMDBMediaType)
	}
}

func TestParseMovieMetadataTMDBTVURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/tv/1399" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id":1399,
			"name":"Game of Thrones",
			"overview":"Noble families fight for control.",
			"poster_path":"/1XS1oqL89opfnbLl8WnZY1O1uJx.jpg",
			"backdrop_path":"/suopoADq0k8YZr4dQXcU6pToj6s.jpg",
			"runtime":57,
			"genres":[{"id":18,"name":"Drama"}],
			"first_air_date":"2011-04-17",
			"seasons":[
				{"season_number":2,"episode_count":10,"air_date":"2012-04-01","name":"Season 2","overview":"War begins.","poster_path":"/season2.jpg"},
				{"season_number":0,"episode_count":3,"air_date":"2010-12-05","name":"Specials","overview":"Bonus episodes.","poster_path":"/specials.jpg"},
				{"season_number":1,"episode_count":10,"air_date":"2011-04-17","name":"Season 1","overview":"Houses rise.","poster_path":"/season1.jpg"}
			],
			"vote_average":8.5,
			"external_ids":{"imdb_id":"tt0944947"},
			"credits":{"cast":[{"id":239019,"name":"Emilia Clarke","character":"Daenerys Targaryen","order":1}],"crew":[{"id":9813,"name":"Miguel Sapochnik","job":"Director","department":"Directing"}]},
			"videos":{"results":[{"id":"def456","key":"KPLWWIOCOOQ","name":"Teaser","site":"YouTube","type":"Teaser","official":true}]}
		}`))
	}))
	defer server.Close()

	client := newTestTMDBClient(t, server.URL)
	metadata, err := ParseMovieMetadata(context.Background(), "https://www.themoviedb.org/tv/1399-game-of-thrones", client, nil)
	if err != nil {
		t.Fatalf("ParseMovieMetadata error: %v", err)
	}
	if metadata == nil {
		t.Fatal("expected metadata")
	}
	if metadata.Title != "Game of Thrones" {
		t.Fatalf("Title = %q, want Game of Thrones", metadata.Title)
	}
	if metadata.ReleaseDate != "2011-04-17" {
		t.Fatalf("ReleaseDate = %q, want 2011-04-17", metadata.ReleaseDate)
	}
	if metadata.Runtime != 57 {
		t.Fatalf("Runtime = %d, want 57", metadata.Runtime)
	}
	if len(metadata.Seasons) != 3 {
		t.Fatalf("Seasons len = %d, want 3", len(metadata.Seasons))
	}
	if metadata.Seasons[0].SeasonNumber != 0 || metadata.Seasons[0].Name != "Specials" {
		t.Fatalf("season[0] = %+v, want season_number=0 name=Specials", metadata.Seasons[0])
	}
	if metadata.Seasons[0].Poster != "https://image.tmdb.org/t/p/w500/specials.jpg" {
		t.Fatalf("season[0].Poster = %q", metadata.Seasons[0].Poster)
	}
	if metadata.Seasons[1].SeasonNumber != 1 || metadata.Seasons[1].EpisodeCount != 10 {
		t.Fatalf("season[1] = %+v, want season_number=1 episode_count=10", metadata.Seasons[1])
	}
	if metadata.Seasons[2].SeasonNumber != 2 {
		t.Fatalf("season[2] = %+v, want season_number=2", metadata.Seasons[2])
	}
	if metadata.TMDBID != 1399 {
		t.Fatalf("TMDBID = %d, want 1399", metadata.TMDBID)
	}
	if metadata.TMDBMediaType != "tv" {
		t.Fatalf("TMDBMediaType = %q, want tv", metadata.TMDBMediaType)
	}
	if metadata.IMDBID != "tt0944947" {
		t.Fatalf("IMDBID = %q, want tt0944947", metadata.IMDBID)
	}
	if metadata.RottenTomatoesURL != "https://www.rottentomatoes.com/tv/game_of_thrones" {
		t.Fatalf("RottenTomatoesURL = %q, want https://www.rottentomatoes.com/tv/game_of_thrones", metadata.RottenTomatoesURL)
	}
}

func TestParseMovieMetadataTMDBTVURLWithoutSeasons(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/tv/3036" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id":3036,
			"name":"Fringe",
			"overview":"FBI fringe division investigations.",
			"poster_path":"/c4m9QfA6fFQvA4A7YxCdBxyWf6f.jpg",
			"backdrop_path":"/2ji4B4N9qI9fVf7rI8lBv9aB8eN.jpg",
			"runtime":46,
			"genres":[{"id":18,"name":"Drama"}],
			"first_air_date":"2008-09-09",
			"vote_average":8.1,
			"credits":{"cast":[],"crew":[]},
			"videos":{"results":[]}
		}`))
	}))
	defer server.Close()

	client := newTestTMDBClient(t, server.URL)
	metadata, err := ParseMovieMetadata(context.Background(), "https://www.themoviedb.org/tv/3036-fringe", client, nil)
	if err != nil {
		t.Fatalf("ParseMovieMetadata error: %v", err)
	}
	if metadata == nil {
		t.Fatal("expected metadata")
	}
	if metadata.Title != "Fringe" {
		t.Fatalf("Title = %q, want Fringe", metadata.Title)
	}
	if len(metadata.Seasons) != 0 {
		t.Fatalf("Seasons len = %d, want 0", len(metadata.Seasons))
	}
	if metadata.TMDBID != 3036 {
		t.Fatalf("TMDBID = %d, want 3036", metadata.TMDBID)
	}
	if metadata.TMDBMediaType != "tv" {
		t.Fatalf("TMDBMediaType = %q, want tv", metadata.TMDBMediaType)
	}
}

func TestParseMovieMetadataLetterboxdURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/search/movie":
			if got := strings.TrimSpace(r.URL.Query().Get("query")); got != "the matrix" {
				t.Fatalf("query = %q, want the matrix", got)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"results":[{"id":603,"title":"The Matrix"}]}`))
		case "/movie/603":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"id":603,
				"title":"The Matrix",
				"overview":"A hacker learns reality is simulated.",
				"poster_path":"/f89U3ADr1oiB1s9GkdPOEpXUk5H.jpg",
				"backdrop_path":"/icmmSD4vTTDKOq2vvdulafOGw93.jpg",
				"runtime":136,
				"genres":[{"id":28,"name":"Action"}],
				"release_date":"1999-03-30",
				"vote_average":8.2,
				"imdb_id":"tt0133093",
				"credits":{"cast":[],"crew":[]},
				"videos":{"results":[]}
			}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := newTestTMDBClient(t, server.URL)
	metadata, err := ParseMovieMetadata(context.Background(), "https://letterboxd.com/film/the-matrix/", client, nil)
	if err != nil {
		t.Fatalf("ParseMovieMetadata error: %v", err)
	}
	if metadata == nil {
		t.Fatal("expected metadata")
	}
	if metadata.Title != "The Matrix" {
		t.Fatalf("Title = %q, want The Matrix", metadata.Title)
	}
	if metadata.TMDBID != 603 {
		t.Fatalf("TMDBID = %d, want 603", metadata.TMDBID)
	}
	if metadata.TMDBMediaType != "movie" {
		t.Fatalf("TMDBMediaType = %q, want movie", metadata.TMDBMediaType)
	}
}

func TestParseMovieMetadataRottenTomatoesMovieURL(t *testing.T) {
	tmdbServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/search/movie":
			if got := strings.TrimSpace(r.URL.Query().Get("query")); got != "the matrix" {
				t.Fatalf("query = %q, want the matrix", got)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"results":[{"id":603,"title":"The Matrix","release_date":"1999-03-30","vote_average":8.2}]}`))
		case "/movie/603":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"id":603,
				"title":"The Matrix",
				"overview":"Reality is not what it seems.",
				"poster_path":"/f89U3ADr1oiB1s9GkdPOEpXUk5H.jpg",
				"backdrop_path":"/icmmSD4vTTDKOq2vvdulafOGw93.jpg",
				"runtime":136,
				"genres":[{"id":28,"name":"Action"}],
				"release_date":"1999-03-30",
				"vote_average":8.2,
				"imdb_id":"tt0133093",
				"external_ids":{"imdb_id":"tt0133093"},
				"credits":{"cast":[],"crew":[]},
				"videos":{"results":[]}
			}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer tmdbServer.Close()

	omdbServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := strings.TrimSpace(r.URL.Query().Get("i")); got != "tt0133093" {
			t.Fatalf("imdb id = %q, want tt0133093", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"Response":"True",
			"Ratings":[
				{"Source":"Rotten Tomatoes","Value":"88%"},
				{"Source":"Metacritic","Value":"73/100"}
			]
		}`))
	}))
	defer omdbServer.Close()

	client := newTestTMDBClient(t, tmdbServer.URL)
	omdbClient := newTestOMDBClient(t, omdbServer.URL)
	metadata, err := ParseMovieMetadata(context.Background(), "https://www.rottentomatoes.com/m/the_matrix", client, omdbClient)
	if err != nil {
		t.Fatalf("ParseMovieMetadata error: %v", err)
	}
	if metadata == nil {
		t.Fatal("expected metadata")
	}
	if metadata.Title != "The Matrix" {
		t.Fatalf("Title = %q, want The Matrix", metadata.Title)
	}
	if metadata.TMDBID != 603 {
		t.Fatalf("TMDBID = %d, want 603", metadata.TMDBID)
	}
	if metadata.TMDBMediaType != "movie" {
		t.Fatalf("TMDBMediaType = %q, want movie", metadata.TMDBMediaType)
	}
	if metadata.RottenTomatoesScore == nil || *metadata.RottenTomatoesScore != 88 {
		t.Fatalf("RottenTomatoesScore = %+v, want 88", metadata.RottenTomatoesScore)
	}
	if metadata.IMDBID != "tt0133093" {
		t.Fatalf("IMDBID = %q, want tt0133093", metadata.IMDBID)
	}
	if metadata.RottenTomatoesURL != "https://www.rottentomatoes.com/m/the_matrix" {
		t.Fatalf("RottenTomatoesURL = %q, want https://www.rottentomatoes.com/m/the_matrix", metadata.RottenTomatoesURL)
	}
}

func TestParseMovieMetadataRottenTomatoesTVURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/search/tv":
			if got := strings.TrimSpace(r.URL.Query().Get("query")); got != "game of thrones" {
				t.Fatalf("query = %q, want game of thrones", got)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"results":[{"id":1399,"name":"Game of Thrones","first_air_date":"2011-04-17","vote_average":8.5}]}`))
		case "/tv/1399":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"id":1399,
				"name":"Game of Thrones",
				"overview":"Noble families fight for control.",
				"poster_path":"/1XS1oqL89opfnbLl8WnZY1O1uJx.jpg",
				"backdrop_path":"/suopoADq0k8YZr4dQXcU6pToj6s.jpg",
				"runtime":57,
				"genres":[{"id":18,"name":"Drama"}],
				"first_air_date":"2011-04-17",
				"vote_average":8.5,
				"credits":{"cast":[],"crew":[]},
				"videos":{"results":[]}
			}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := newTestTMDBClient(t, server.URL)
	metadata, err := ParseMovieMetadata(context.Background(), "https://www.rottentomatoes.com/tv/game_of_thrones/s01", client, nil)
	if err != nil {
		t.Fatalf("ParseMovieMetadata error: %v", err)
	}
	if metadata == nil {
		t.Fatal("expected metadata")
	}
	if metadata.Title != "Game of Thrones" {
		t.Fatalf("Title = %q, want Game of Thrones", metadata.Title)
	}
	if metadata.TMDBID != 1399 {
		t.Fatalf("TMDBID = %d, want 1399", metadata.TMDBID)
	}
	if metadata.TMDBMediaType != "tv" {
		t.Fatalf("TMDBMediaType = %q, want tv", metadata.TMDBMediaType)
	}
	if metadata.RottenTomatoesURL != "https://www.rottentomatoes.com/tv/game_of_thrones" {
		t.Fatalf("RottenTomatoesURL = %q, want https://www.rottentomatoes.com/tv/game_of_thrones", metadata.RottenTomatoesURL)
	}
}

func TestParseMovieMetadataRottenTomatoesMovieURLPrefersYearMatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/search/movie":
			if got := strings.TrimSpace(r.URL.Query().Get("query")); got != "it" {
				t.Fatalf("query = %q, want it", got)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"results":[
					{"id":1562,"title":"It","release_date":"1990-11-18","vote_average":6.8},
					{"id":346364,"title":"It","release_date":"2017-09-06","vote_average":7.2}
				]
			}`))
		case "/movie/346364":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"id":346364,
				"title":"It",
				"overview":"You'll float too.",
				"poster_path":"/9E2y5Q7WlCVNEhP5GiVTjhEhx1o.jpg",
				"backdrop_path":"/tcheoA2nPATCm2vvXw2hVQoaEFD.jpg",
				"runtime":135,
				"genres":[{"id":27,"name":"Horror"}],
				"release_date":"2017-09-06",
				"vote_average":7.2,
				"credits":{"cast":[],"crew":[]},
				"videos":{"results":[]}
			}`))
		case "/movie/1562":
			t.Fatalf("selected wrong movie id from ambiguous Rotten Tomatoes slug")
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := newTestTMDBClient(t, server.URL)
	metadata, err := ParseMovieMetadata(context.Background(), "https://www.rottentomatoes.com/m/it_2017", client, nil)
	if err != nil {
		t.Fatalf("ParseMovieMetadata error: %v", err)
	}
	if metadata == nil {
		t.Fatal("expected metadata")
	}
	if metadata.TMDBID != 346364 {
		t.Fatalf("TMDBID = %d, want 346364", metadata.TMDBID)
	}
}

func TestParseMovieMetadataRottenTomatoesMovieURLNumericTitleSlug(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/search/movie":
			if got := strings.TrimSpace(r.URL.Query().Get("query")); got != "2012" {
				t.Fatalf("query = %q, want 2012", got)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"results":[
					{"id":14161,"title":"2012","release_date":"2009-10-10","vote_average":5.8},
					{"id":530915,"title":"1917","release_date":"2019-12-25","vote_average":8.0}
				]
			}`))
		case "/movie/14161":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"id":14161,
				"title":"2012",
				"overview":"A global cataclysm threatens humanity.",
				"poster_path":"/zaqam2RNscH5ooYFWInV6hjx6y5.jpg",
				"backdrop_path":"/fQ5s7xBvYjQ2iF0Q6w8X9Kf3xB1.jpg",
				"runtime":158,
				"genres":[{"id":28,"name":"Action"}],
				"release_date":"2009-10-10",
				"vote_average":5.8,
				"credits":{"cast":[],"crew":[]},
				"videos":{"results":[]}
			}`))
		case "/movie/530915":
			t.Fatalf("selected wrong movie id for numeric title slug")
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := newTestTMDBClient(t, server.URL)
	metadata, err := ParseMovieMetadata(context.Background(), "https://www.rottentomatoes.com/m/2012", client, nil)
	if err != nil {
		t.Fatalf("ParseMovieMetadata error: %v", err)
	}
	if metadata == nil {
		t.Fatal("expected metadata")
	}
	if metadata.Title != "2012" {
		t.Fatalf("Title = %q, want 2012", metadata.Title)
	}
	if metadata.TMDBID != 14161 {
		t.Fatalf("TMDBID = %d, want 14161", metadata.TMDBID)
	}
	if metadata.TMDBMediaType != "movie" {
		t.Fatalf("TMDBMediaType = %q, want movie", metadata.TMDBMediaType)
	}
}

func TestParseMovieMetadataRottenTomatoesMovieURLNoCloseMatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/search/movie":
			if got := strings.TrimSpace(r.URL.Query().Get("query")); got != "zzzzzz" {
				t.Fatalf("query = %q, want zzzzzz", got)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"results":[{"id":603,"title":"The Matrix","release_date":"1999-03-30","vote_average":8.2}]}`))
		case "/movie/603":
			t.Fatalf("expected no details fetch when Rotten Tomatoes search has no close match")
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := newTestTMDBClient(t, server.URL)
	metadata, err := ParseMovieMetadata(context.Background(), "https://www.rottentomatoes.com/m/zzzzzz", client, nil)
	if err != nil {
		t.Fatalf("ParseMovieMetadata error: %v", err)
	}
	if metadata != nil {
		t.Fatalf("expected nil metadata, got %+v", metadata)
	}
}

func TestParseMovieMetadataNoMatchingURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("unexpected request: %s", r.URL.String())
	}))
	defer server.Close()

	client := newTestTMDBClient(t, server.URL)
	metadata, err := ParseMovieMetadata(context.Background(), "https://example.com/not-a-movie-link", client, nil)
	if err != nil {
		t.Fatalf("ParseMovieMetadata error: %v", err)
	}
	if metadata != nil {
		t.Fatalf("expected nil metadata, got %+v", metadata)
	}
}

func TestParseMovieMetadataTMDBAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"status_message":"Too many requests"}`))
	}))
	defer server.Close()

	client := newTestTMDBClient(t, server.URL)
	metadata, err := ParseMovieMetadata(context.Background(), "https://www.themoviedb.org/movie/550", client, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if metadata != nil {
		t.Fatalf("expected nil metadata, got %+v", metadata)
	}
	if !errors.Is(err, ErrTMDBRateLimited) {
		t.Fatalf("expected ErrTMDBRateLimited, got %v", err)
	}
}

func TestParseMovieMetadataEnrichesWithOMDBRatings(t *testing.T) {
	tmdbServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/movie/603" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id":603,
			"title":"The Matrix",
			"overview":"A hacker learns reality is simulated.",
			"poster_path":"/f89U3ADr1oiB1s9GkdPOEpXUk5H.jpg",
			"backdrop_path":"/icmmSD4vTTDKOq2vvdulafOGw93.jpg",
			"runtime":136,
			"genres":[{"id":28,"name":"Action"}],
			"release_date":"1999-03-30",
			"vote_average":8.2,
			"imdb_id":"tt0133093",
			"credits":{"cast":[],"crew":[]},
			"videos":{"results":[]}
		}`))
	}))
	defer tmdbServer.Close()

	omdbServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := strings.TrimSpace(r.URL.Query().Get("i")); got != "tt0133093" {
			t.Fatalf("imdb id = %q, want tt0133093", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"Response":"True",
			"Ratings":[
				{"Source":"Rotten Tomatoes","Value":"88%"},
				{"Source":"Metacritic","Value":"73/100"}
			]
		}`))
	}))
	defer omdbServer.Close()

	tmdbClient := newTestTMDBClient(t, tmdbServer.URL)
	omdbClient := newTestOMDBClient(t, omdbServer.URL)
	metadata, err := ParseMovieMetadata(context.Background(), "https://www.themoviedb.org/movie/603-the-matrix", tmdbClient, omdbClient)
	if err != nil {
		t.Fatalf("ParseMovieMetadata error: %v", err)
	}
	if metadata == nil {
		t.Fatal("expected metadata")
	}
	if metadata.RottenTomatoesScore == nil || *metadata.RottenTomatoesScore != 88 {
		t.Fatalf("RottenTomatoesScore = %+v, want 88", metadata.RottenTomatoesScore)
	}
	if metadata.MetacriticScore == nil || *metadata.MetacriticScore != 73 {
		t.Fatalf("MetacriticScore = %+v, want 73", metadata.MetacriticScore)
	}
	if metadata.IMDBID != "tt0133093" {
		t.Fatalf("IMDBID = %q, want tt0133093", metadata.IMDBID)
	}
	if metadata.RottenTomatoesURL != "https://www.rottentomatoes.com/m/the_matrix" {
		t.Fatalf("RottenTomatoesURL = %q, want https://www.rottentomatoes.com/m/the_matrix", metadata.RottenTomatoesURL)
	}
}

func TestParseMovieMetadataOMDBFailureFallsBackToTMDB(t *testing.T) {
	tmdbServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/tv/1399" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id":1399,
			"name":"Game of Thrones",
			"overview":"Noble families fight for control.",
			"poster_path":"/1XS1oqL89opfnbLl8WnZY1O1uJx.jpg",
			"backdrop_path":"/suopoADq0k8YZr4dQXcU6pToj6s.jpg",
			"runtime":57,
			"genres":[{"id":18,"name":"Drama"}],
			"first_air_date":"2011-04-17",
			"vote_average":8.5,
			"external_ids":{"imdb_id":"tt0944947"},
			"credits":{"cast":[],"crew":[]},
			"videos":{"results":[]}
		}`))
	}))
	defer tmdbServer.Close()

	omdbServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"Error":"Request limit reached!"}`))
	}))
	defer omdbServer.Close()

	tmdbClient := newTestTMDBClient(t, tmdbServer.URL)
	omdbClient := newTestOMDBClient(t, omdbServer.URL)
	metadata, err := ParseMovieMetadata(context.Background(), "https://www.themoviedb.org/tv/1399", tmdbClient, omdbClient)
	if err != nil {
		t.Fatalf("ParseMovieMetadata error: %v", err)
	}
	if metadata == nil {
		t.Fatal("expected metadata")
	}
	if metadata.Title != "Game of Thrones" {
		t.Fatalf("Title = %q, want Game of Thrones", metadata.Title)
	}
	if metadata.IMDBID != "tt0944947" {
		t.Fatalf("IMDBID = %q, want tt0944947", metadata.IMDBID)
	}
	if metadata.RottenTomatoesURL != "https://www.rottentomatoes.com/tv/game_of_thrones" {
		t.Fatalf("RottenTomatoesURL = %q, want https://www.rottentomatoes.com/tv/game_of_thrones", metadata.RottenTomatoesURL)
	}
	if metadata.RottenTomatoesScore != nil {
		t.Fatalf("expected RottenTomatoesScore to be nil, got %v", *metadata.RottenTomatoesScore)
	}
	if metadata.MetacriticScore != nil {
		t.Fatalf("expected MetacriticScore to be nil, got %v", *metadata.MetacriticScore)
	}
}
