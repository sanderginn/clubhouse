package db

import (
	"context"
	"errors"
	"testing"

	"github.com/XSAM/otelsql"
)

func TestQueryTypeFromSQL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		query string
		want  string
	}{
		{name: "select", query: "SELECT * FROM users", want: "select"},
		{name: "insert", query: "insert into posts (id) values ($1)", want: "insert"},
		{name: "update", query: "  UPDATE posts SET title = $1", want: "update"},
		{name: "delete", query: "delete from posts where id = $1", want: "delete"},
		{name: "with-select", query: "WITH recent AS (SELECT * FROM posts) SELECT * FROM recent", want: "select"},
		{name: "with-update", query: "WITH updated AS (UPDATE posts SET title = $1 RETURNING id) SELECT id FROM updated", want: "update"},
		{name: "empty", query: "   ", want: ""},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := queryTypeFromSQL(tt.query)
			if got != tt.want {
				t.Fatalf("queryTypeFromSQL(%q) = %q, want %q", tt.query, got, tt.want)
			}
		})
	}
}

func TestInstrumentErrorAttributesGetterRecordsQueryType(t *testing.T) {
	original := recordDBQueryError
	t.Cleanup(func() {
		recordDBQueryError = original
	})

	var gotQueryType string
	var gotErrorType string
	recordDBQueryError = func(_ context.Context, queryType, errorType string) {
		gotQueryType = queryType
		gotErrorType = errorType
	}

	instrumentAttributesGetter(context.Background(), otelsql.MethodConnQuery, "SELECT * FROM users", nil)
	attrs := instrumentErrorAttributesGetter(errors.New("timeout"))

	if gotQueryType != "select" {
		t.Fatalf("query type = %q, want %q", gotQueryType, "select")
	}
	if gotErrorType != "timeout" {
		t.Fatalf("error type = %q, want %q", gotErrorType, "timeout")
	}

	found := false
	for _, attr := range attrs {
		if string(attr.Key) == "error_type" {
			found = true
			if attr.Value.AsString() != "timeout" {
				t.Fatalf("error_type attr = %q, want %q", attr.Value.AsString(), "timeout")
			}
		}
	}
	if !found {
		t.Fatal("expected error_type attribute to be present")
	}
}
