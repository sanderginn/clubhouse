package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/sanderginn/clubhouse/internal/middleware"
	"github.com/sanderginn/clubhouse/internal/models"
	"github.com/sanderginn/clubhouse/internal/observability"
)

const (
	defaultUploadDir      = "uploads"
	defaultUploadMaxBytes = int64(10 << 20) // 10MB
	uploadFormOverhead    = int64(1 << 20)  // 1MB for multipart overhead
)

var errUploadTooLarge = errors.New("upload exceeds max size")

// UploadHandler handles file uploads.
type UploadHandler struct {
	uploadDir    string
	maxBytes     int64
	allowedTypes map[string]string
}

// NewUploadHandler creates a new upload handler.
func NewUploadHandler() *UploadHandler {
	uploadDir := strings.TrimSpace(os.Getenv("CLUBHOUSE_UPLOAD_DIR"))
	if uploadDir == "" {
		uploadDir = defaultUploadDir
	}
	if abs, err := filepath.Abs(uploadDir); err == nil {
		uploadDir = abs
	}

	maxBytes := defaultUploadMaxBytes
	if rawMax := strings.TrimSpace(os.Getenv("CLUBHOUSE_UPLOAD_MAX_BYTES")); rawMax != "" {
		if parsed, err := strconv.ParseInt(rawMax, 10, 64); err == nil && parsed > 0 {
			maxBytes = parsed
		}
	}

	return &UploadHandler{
		uploadDir: uploadDir,
		maxBytes:  maxBytes,
		allowedTypes: map[string]string{
			"image/jpeg": ".jpg",
			"image/png":  ".png",
			"image/gif":  ".gif",
			"image/webp": ".webp",
			"image/bmp":  ".bmp",
			"image/avif": ".avif",
			"image/tiff": ".tiff",
		},
	}
}

// UploadDir returns the configured upload directory.
func (h *UploadHandler) UploadDir() string {
	return h.uploadDir
}

// UploadImage handles POST /api/v1/uploads.
func (h *UploadHandler) UploadImage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only POST requests are allowed")
		return
	}

	userID, err := middleware.GetUserIDFromContext(r.Context())
	if err != nil || userID == uuid.Nil {
		writeError(r.Context(), w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, h.maxBytes+uploadFormOverhead)
	if err := r.ParseMultipartForm(h.maxBytes); err != nil {
		if isRequestBodyTooLarge(err) {
			writeError(r.Context(), w, http.StatusRequestEntityTooLarge, "FILE_TOO_LARGE", "Image exceeds the upload size limit")
			return
		}
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid upload payload")
		return
	}
	defer func() {
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
	}()

	file, header, err := r.FormFile("file")
	if err != nil {
		if errors.Is(err, http.ErrMissingFile) {
			writeError(r.Context(), w, http.StatusBadRequest, "FILE_REQUIRED", "Select an image to upload")
			return
		}
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid upload payload")
		return
	}
	defer file.Close()

	if header.Size > h.maxBytes {
		writeError(r.Context(), w, http.StatusRequestEntityTooLarge, "FILE_TOO_LARGE", "Image exceeds the upload size limit")
		return
	}

	sniffBuffer := make([]byte, 512)
	n, err := io.ReadFull(file, sniffBuffer)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_REQUEST", "Unable to read uploaded file")
		return
	}
	if n == 0 {
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_REQUEST", "Uploaded file is empty")
		return
	}

	contentType := http.DetectContentType(sniffBuffer[:n])
	mediaType, _, _ := mime.ParseMediaType(contentType)
	resolvedExt, ok := h.allowedTypes[mediaType]
	if !ok {
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_FILE_TYPE", "Only image uploads are supported")
		return
	}

	userDir := filepath.Join(h.uploadDir, userID.String())
	if err := os.MkdirAll(userDir, 0o755); err != nil {
		writeError(r.Context(), w, http.StatusInternalServerError, "UPLOAD_FAILED", "Failed to store image")
		return
	}

	fileName := fmt.Sprintf("%s%s", uuid.New().String(), resolvedExt)
	filePath := filepath.Join(userDir, fileName)
	if err := writeUploadFile(filePath, sniffBuffer[:n], file, h.maxBytes); err != nil {
		if errors.Is(err, errUploadTooLarge) {
			writeError(r.Context(), w, http.StatusRequestEntityTooLarge, "FILE_TOO_LARGE", "Image exceeds the upload size limit")
			return
		}
		writeError(r.Context(), w, http.StatusInternalServerError, "UPLOAD_FAILED", "Failed to store image")
		return
	}

	url := fmt.Sprintf("/api/v1/uploads/%s/%s", userID.String(), fileName)
	observability.LogInfo(r.Context(), "image uploaded", "user_id", userID.String(), "path", fileName)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(models.ImageUploadResponse{URL: url}); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode upload response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusOK,
			UserID:     userID.String(),
			Err:        err,
		})
	}
}

func writeUploadFile(path string, prefix []byte, src io.Reader, maxBytes int64) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()

	written, err := file.Write(prefix)
	if err != nil {
		return err
	}
	copied, err := io.Copy(file, src)
	if err != nil {
		return err
	}
	if int64(written)+copied > maxBytes {
		return errUploadTooLarge
	}
	return nil
}
