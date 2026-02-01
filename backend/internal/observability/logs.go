package observability

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"runtime/debug"
	"strings"
	"time"

	stdlog "log"

	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/trace"
)

type ErrorLog struct {
	Message    string
	Code       string
	StatusCode int
	UserID     string
	Err        error
}

var stderrLogger = stdlog.New(os.Stderr, "", 0)

func LogError(ctx context.Context, entry ErrorLog) {
	if !ShouldLog(LevelError) {
		return
	}
	logger := global.Logger("clubhouse")

	var record log.Record
	record.SetTimestamp(time.Now())
	record.SetSeverity(log.SeverityError)
	record.SetSeverityText("ERROR")
	record.SetBody(log.StringValue(entry.Message))

	attrs := []log.KeyValue{
		log.String("error.code", entry.Code),
		log.Int("http.status_code", entry.StatusCode),
	}
	if entry.UserID != "" {
		attrs = append(attrs, log.String("user_id", entry.UserID))
	}
	if entry.Err != nil {
		attrs = append(attrs, log.String("error.message", entry.Err.Error()))
	}
	stack := strings.TrimSpace(string(debug.Stack()))
	if stack != "" {
		attrs = append(attrs, log.String("error.stack", stack))
	}
	if spanCtx := trace.SpanContextFromContext(ctx); spanCtx.IsValid() {
		attrs = append(attrs,
			log.String("trace_id", spanCtx.TraceID().String()),
			log.String("span_id", spanCtx.SpanID().String()),
		)
	}

	record.AddAttributes(attrs...)
	logger.Emit(ctx, record)
	emitStderr("ERROR", entry.Message, attrs)
}

// LogInfo logs an informational message with optional key-value pairs
func LogInfo(ctx context.Context, message string, kvPairs ...string) {
	if !ShouldLog(LevelInfo) {
		return
	}
	logger := global.Logger("clubhouse")

	var record log.Record
	record.SetTimestamp(time.Now())
	record.SetSeverity(log.SeverityInfo)
	record.SetSeverityText("INFO")
	record.SetBody(log.StringValue(message))

	attrs := make([]log.KeyValue, 0)

	// Convert key-value pairs to attributes
	for i := 0; i < len(kvPairs)-1; i += 2 {
		attrs = append(attrs, log.String(kvPairs[i], kvPairs[i+1]))
	}

	if spanCtx := trace.SpanContextFromContext(ctx); spanCtx.IsValid() {
		attrs = append(attrs,
			log.String("trace_id", spanCtx.TraceID().String()),
			log.String("span_id", spanCtx.SpanID().String()),
		)
	}

	record.AddAttributes(attrs...)
	logger.Emit(ctx, record)
	emitStderr("INFO", message, attrs)
}

// LogDebug logs a debug message with optional key-value pairs.
// Debug messages are only emitted when LOG_LEVEL is set to "debug".
func LogDebug(ctx context.Context, message string, kvPairs ...string) {
	if !ShouldLog(LevelDebug) {
		return
	}
	logger := global.Logger("clubhouse")

	var record log.Record
	record.SetTimestamp(time.Now())
	record.SetSeverity(log.SeverityDebug)
	record.SetSeverityText("DEBUG")
	record.SetBody(log.StringValue(message))

	attrs := make([]log.KeyValue, 0)

	// Convert key-value pairs to attributes
	for i := 0; i < len(kvPairs)-1; i += 2 {
		attrs = append(attrs, log.String(kvPairs[i], kvPairs[i+1]))
	}

	if spanCtx := trace.SpanContextFromContext(ctx); spanCtx.IsValid() {
		attrs = append(attrs,
			log.String("trace_id", spanCtx.TraceID().String()),
			log.String("span_id", spanCtx.SpanID().String()),
		)
	}

	record.AddAttributes(attrs...)
	logger.Emit(ctx, record)
	emitStderr("DEBUG", message, attrs)
}

// LogWarn logs a warning message with optional key-value pairs.
// Warnings indicate potential issues that don't prevent operation.
func LogWarn(ctx context.Context, message string, kvPairs ...string) {
	if !ShouldLog(LevelWarn) {
		return
	}
	logger := global.Logger("clubhouse")

	var record log.Record
	record.SetTimestamp(time.Now())
	record.SetSeverity(log.SeverityWarn)
	record.SetSeverityText("WARN")
	record.SetBody(log.StringValue(message))

	attrs := make([]log.KeyValue, 0)

	// Convert key-value pairs to attributes
	for i := 0; i < len(kvPairs)-1; i += 2 {
		attrs = append(attrs, log.String(kvPairs[i], kvPairs[i+1]))
	}

	if spanCtx := trace.SpanContextFromContext(ctx); spanCtx.IsValid() {
		attrs = append(attrs,
			log.String("trace_id", spanCtx.TraceID().String()),
			log.String("span_id", spanCtx.SpanID().String()),
		)
	}

	record.AddAttributes(attrs...)
	logger.Emit(ctx, record)
	emitStderr("WARN", message, attrs)
}

type stderrError struct {
	Code       string `json:"code,omitempty"`
	Message    string `json:"message,omitempty"`
	Stack      string `json:"stack,omitempty"`
	StatusCode int64  `json:"status_code,omitempty"`
}

type stderrLog struct {
	Timestamp string         `json:"timestamp"`
	Level     string         `json:"level"`
	Message   string         `json:"message"`
	TraceID   string         `json:"trace_id,omitempty"`
	SpanID    string         `json:"span_id,omitempty"`
	UserID    string         `json:"user_id,omitempty"`
	Error     *stderrError   `json:"error,omitempty"`
	Fields    map[string]any `json:"fields,omitempty"`
}

func emitStderr(severity string, message string, attrs []log.KeyValue) {
	payload := buildStderrPayload(severity, message, attrs)
	data, err := json.Marshal(payload)
	if err != nil {
		stderrLogger.Print(fallbackLogPayload(err))
		return
	}

	stderrLogger.Print(string(data))
}

func buildStderrPayload(severity string, message string, attrs []log.KeyValue) stderrLog {
	fields := keyValuesToMap(attrs)
	payload := stderrLog{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Level:     strings.ToLower(severity),
		Message:   message,
	}

	if traceID, ok := fields["trace_id"].(string); ok && traceID != "" {
		payload.TraceID = traceID
		delete(fields, "trace_id")
	}
	if spanID, ok := fields["span_id"].(string); ok && spanID != "" {
		payload.SpanID = spanID
		delete(fields, "span_id")
	}
	if userID, ok := fields["user_id"].(string); ok && userID != "" {
		payload.UserID = userID
		delete(fields, "user_id")
	}

	payload.Error = extractError(fields)

	if len(fields) > 0 {
		payload.Fields = fields
	}

	return payload
}

func extractError(fields map[string]any) *stderrError {
	if fields == nil {
		return nil
	}

	errPayload := &stderrError{}
	if value, ok := fields["error.code"].(string); ok && value != "" {
		errPayload.Code = value
		delete(fields, "error.code")
	}
	if value, ok := fields["error.message"].(string); ok && value != "" {
		errPayload.Message = value
		delete(fields, "error.message")
	}
	if value, ok := fields["error.stack"].(string); ok && value != "" {
		errPayload.Stack = value
		delete(fields, "error.stack")
	}
	if value, ok := fields["http.status_code"].(int64); ok && value != 0 {
		errPayload.StatusCode = value
		delete(fields, "http.status_code")
	}

	if errPayload.Code == "" && errPayload.Message == "" && errPayload.Stack == "" && errPayload.StatusCode == 0 {
		return nil
	}

	return errPayload
}

func fallbackLogPayload(err error) string {
	escaped, _ := json.Marshal(err.Error())
	timestamp := time.Now().UTC().Format(time.RFC3339Nano)
	return fmt.Sprintf(
		`{"timestamp":"%s","level":"error","message":"failed to marshal log payload","error":{"message":%s}}`,
		timestamp,
		string(escaped),
	)
}

func keyValuesToMap(attrs []log.KeyValue) map[string]any {
	if len(attrs) == 0 {
		return nil
	}
	out := make(map[string]any, len(attrs))
	for _, attr := range attrs {
		out[attr.Key] = logValueToInterface(attr.Value)
	}
	return out
}

func logValueToInterface(value log.Value) any {
	switch value.Kind() {
	case log.KindBool:
		return value.AsBool()
	case log.KindInt64:
		return value.AsInt64()
	case log.KindFloat64:
		return value.AsFloat64()
	case log.KindString:
		return value.AsString()
	case log.KindBytes:
		bytes := value.AsBytes()
		if len(bytes) == 0 {
			return ""
		}
		return base64.StdEncoding.EncodeToString(bytes)
	case log.KindSlice:
		values := value.AsSlice()
		out := make([]any, 0, len(values))
		for _, item := range values {
			out = append(out, logValueToInterface(item))
		}
		return out
	case log.KindMap:
		return keyValuesToMap(value.AsMap())
	default:
		return nil
	}
}
