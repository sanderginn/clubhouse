package db

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"strings"

	"github.com/XSAM/otelsql"
	"github.com/sanderginn/clubhouse/internal/observability"
	"go.opentelemetry.io/otel/attribute"
)

func instrumentAttributesGetter(ctx context.Context, method otelsql.Method, query string, _ []driver.NamedValue) []attribute.KeyValue {
	attrs := make([]attribute.KeyValue, 0, 2)

	queryType := queryTypeFromSQL(query)
	if queryType != "" {
		attrs = append(attrs, attribute.String("query_type", queryType))
	}

	table := tableFromSQL(query)
	if table != "" {
		attrs = append(attrs, attribute.String("table", table))
	}

	switch method {
	case otelsql.MethodTxCommit:
		observability.RecordDBTransaction(ctx, "commit")
	case otelsql.MethodTxRollback:
		observability.RecordDBTransaction(ctx, "rollback")
	}

	return attrs
}

func instrumentErrorAttributesGetter(err error) []attribute.KeyValue {
	errorType := errorTypeFromDBError(err)
	if errorType == "" {
		errorType = "unknown"
	}
	observability.RecordDBQueryError(context.Background(), "unknown", errorType)
	return []attribute.KeyValue{
		attribute.String("error_type", errorType),
	}
}

func queryTypeFromSQL(query string) string {
	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return ""
	}
	trimmed = strings.TrimLeft(trimmed, " (")
	lower := strings.ToLower(trimmed)
	fields := strings.Fields(lower)
	if len(fields) == 0 {
		return ""
	}
	switch fields[0] {
	case "select", "insert", "update", "delete":
		return fields[0]
	case "with":
		for _, token := range fields[1:] {
			switch token {
			case "select", "insert", "update", "delete":
				return token
			}
		}
		return "with"
	default:
		return fields[0]
	}
}

func tableFromSQL(query string) string {
	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return ""
	}
	lower := strings.ToLower(trimmed)
	fields := strings.Fields(lower)
	if len(fields) == 0 {
		return ""
	}

	switch fields[0] {
	case "select", "with":
		for i, token := range fields {
			if token == "from" && i+1 < len(fields) {
				return sanitizeTableToken(fields[i+1])
			}
		}
	case "insert":
		for i, token := range fields {
			if token == "into" && i+1 < len(fields) {
				return sanitizeTableToken(fields[i+1])
			}
		}
	case "update":
		if len(fields) > 1 {
			return sanitizeTableToken(fields[1])
		}
	case "delete":
		for i, token := range fields {
			if token == "from" && i+1 < len(fields) {
				return sanitizeTableToken(fields[i+1])
			}
		}
	}

	return ""
}

func sanitizeTableToken(token string) string {
	if token == "" {
		return ""
	}
	token = strings.Trim(token, ",;()")
	token = strings.Trim(token, "\"`")
	if token == "" {
		return ""
	}
	if strings.Contains(token, ".") {
		parts := strings.Split(token, ".")
		token = parts[len(parts)-1]
	}
	return token
}

func errorTypeFromDBError(err error) string {
	if err == nil {
		return ""
	}
	switch {
	case errors.Is(err, context.Canceled):
		return "canceled"
	case errors.Is(err, context.DeadlineExceeded):
		return "deadline_exceeded"
	case errors.Is(err, sql.ErrNoRows):
		return "no_rows"
	}

	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "timeout"):
		return "timeout"
	case strings.Contains(msg, "deadlock"):
		return "deadlock"
	case strings.Contains(msg, "duplicate"):
		return "duplicate"
	case strings.Contains(msg, "connection"):
		return "connection"
	default:
		return "unknown"
	}
}
