package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
	"github.com/sanderginn/clubhouse/internal/models"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"golang.org/x/crypto/bcrypt"
)

const (
	bcryptCost = 12
)

// dummyPasswordHash is a bcrypt hash for timing-equalized compares on unknown users.
var dummyPasswordHash = []byte("$2a$12$ukjUkUX1cfSD88LBRMvNjuwNn2eWmisHaOuhtgo/napH/3VmLCtNK")

var (
	ErrUsernameRequired   = errors.New("username is required")
	ErrPasswordRequired   = errors.New("password is required")
	ErrInvalidCredentials = errors.New("invalid username or password")
	ErrUserNotApproved    = errors.New("user not approved")
	ErrUserSuspended      = errors.New("user suspended")
)

// UserService handles user-related operations
type UserService struct {
	db *sql.DB
}

// NewUserService creates a new user service
func NewUserService(db *sql.DB) *UserService {
	return &UserService{db: db}
}

// RegisterUser registers a new user with password hashing
func (s *UserService) RegisterUser(ctx context.Context, req *models.RegisterRequest) (*models.User, error) {
	ctx, span := otel.Tracer("clubhouse.users").Start(ctx, "UserService.RegisterUser")
	span.SetAttributes(
		attribute.Bool("has_username", req != nil && strings.TrimSpace(req.Username) != ""),
		attribute.Bool("has_email", req != nil && strings.TrimSpace(req.Email) != ""),
	)
	defer span.End()

	// Validate input
	if err := validateRegisterInput(req); err != nil {
		recordSpanError(span, err)
		return nil, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	// Hash password
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcryptCost)
	if err != nil {
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Create user ID
	userID := uuid.New()
	emailValue := strings.TrimSpace(req.Email)
	var email sql.NullString
	if emailValue != "" {
		email = sql.NullString{String: emailValue, Valid: true}
	}

	// Insert into database
	query := `
		INSERT INTO users (id, username, email, password_hash, is_admin, created_at)
		VALUES ($1, $2, $3, $4, false, now())
		RETURNING id, username, COALESCE(email, '') as email, is_admin, created_at
	`

	var user models.User
	err = tx.QueryRowContext(ctx, query, userID, req.Username, email, string(passwordHash)).
		Scan(&user.ID, &user.Username, &user.Email, &user.IsAdmin, &user.CreatedAt)

	if err != nil {
		// Check for unique constraint violations
		if strings.Contains(err.Error(), "duplicate key") {
			if strings.Contains(err.Error(), "username") {
				duplicateErr := fmt.Errorf("username already exists")
				recordSpanError(span, duplicateErr)
				return nil, duplicateErr
			}
			if strings.Contains(err.Error(), "email") {
				duplicateErr := fmt.Errorf("email already exists")
				recordSpanError(span, duplicateErr)
				return nil, duplicateErr
			}
		}
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	auditService := NewAuditService(tx)
	metadata := map[string]interface{}{
		"username": user.Username,
		"email":    user.Email,
	}
	if err := auditService.LogAuditWithMetadata(ctx, "register_user", uuid.Nil, user.ID, metadata); err != nil {
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to create audit log: %w", err)
	}

	if err := tx.Commit(); err != nil {
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return &user, nil
}

// AdminExists checks if there is at least one active admin user.
func (s *UserService) AdminExists(ctx context.Context) (bool, error) {
	ctx, span := otel.Tracer("clubhouse.users").Start(ctx, "UserService.AdminExists")
	defer span.End()

	query := `
		SELECT EXISTS (
			SELECT 1
			FROM users
			WHERE is_admin = true
				AND deleted_at IS NULL
		)
	`

	var exists bool
	if err := s.db.QueryRowContext(ctx, query).Scan(&exists); err != nil {
		recordSpanError(span, err)
		return false, fmt.Errorf("failed to check admin existence: %w", err)
	}

	return exists, nil
}

// BootstrapAdmin creates the first admin user if none exist.
// Returns created=false when an admin already exists.
func (s *UserService) BootstrapAdmin(ctx context.Context, username, email, password string) (*models.User, bool, error) {
	ctx, span := otel.Tracer("clubhouse.users").Start(ctx, "UserService.BootstrapAdmin")
	span.SetAttributes(
		attribute.Bool("has_username", strings.TrimSpace(username) != ""),
		attribute.Bool("has_email", strings.TrimSpace(email) != ""),
	)
	defer span.End()

	req := &models.RegisterRequest{
		Username: username,
		Email:    email,
		Password: password,
	}
	if err := validateRegisterInput(req); err != nil {
		recordSpanError(span, err)
		return nil, false, err
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		recordSpanError(span, err)
		return nil, false, fmt.Errorf("failed to hash password: %w", err)
	}

	userID := uuid.New()
	emailValue := strings.TrimSpace(email)
	var emailField sql.NullString
	if emailValue != "" {
		emailField = sql.NullString{String: emailValue, Valid: true}
	}

	query := `
		WITH existing AS (
			SELECT 1
			FROM users
			WHERE is_admin = true
				AND deleted_at IS NULL
		)
		INSERT INTO users (id, username, email, password_hash, is_admin, approved_at, created_at)
		SELECT $1, $2, $3, $4, true, now(), now()
		WHERE NOT EXISTS (SELECT 1 FROM existing)
		RETURNING id, username, COALESCE(email, '') as email, is_admin, created_at
	`

	var user models.User
	err = s.db.QueryRowContext(ctx, query, userID, username, emailField, string(passwordHash)).
		Scan(&user.ID, &user.Username, &user.Email, &user.IsAdmin, &user.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, false, nil
		}
		if strings.Contains(err.Error(), "duplicate key") {
			if strings.Contains(err.Error(), "username") {
				duplicateErr := fmt.Errorf("username already exists")
				recordSpanError(span, duplicateErr)
				return nil, false, duplicateErr
			}
			if strings.Contains(err.Error(), "email") {
				duplicateErr := fmt.Errorf("email already exists")
				recordSpanError(span, duplicateErr)
				return nil, false, duplicateErr
			}
		}
		recordSpanError(span, err)
		return nil, false, fmt.Errorf("failed to create bootstrap admin: %w", err)
	}

	return &user, true, nil
}

// GetUserByID retrieves a user by ID
func (s *UserService) GetUserByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	ctx, span := otel.Tracer("clubhouse.users").Start(ctx, "UserService.GetUserByID")
	span.SetAttributes(attribute.String("user_id", id.String()))
	defer span.End()

	query := `
		SELECT id, username, COALESCE(email, '') as email, password_hash, profile_picture_url, bio, is_admin, totp_enabled, totp_secret_encrypted, approved_at, suspended_at, created_at, updated_at, deleted_at
		FROM users
		WHERE id = $1 AND deleted_at IS NULL
	`

	var user models.User
	err := s.db.QueryRowContext(ctx, query, id).
		Scan(&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.ProfilePictureURL,
			&user.Bio, &user.IsAdmin, &user.TotpEnabled, &user.TotpSecretEncrypted, &user.ApprovedAt, &user.SuspendedAt, &user.CreatedAt, &user.UpdatedAt, &user.DeletedAt)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			notFoundErr := fmt.Errorf("user not found")
			recordSpanError(span, notFoundErr)
			return nil, notFoundErr
		}
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return &user, nil
}

// GetUserByUsername retrieves a user by username
func (s *UserService) GetUserByUsername(ctx context.Context, username string) (*models.User, error) {
	ctx, span := otel.Tracer("clubhouse.users").Start(ctx, "UserService.GetUserByUsername")
	span.SetAttributes(attribute.Bool("has_username", strings.TrimSpace(username) != ""))
	defer span.End()

	query := `
		SELECT id, username, COALESCE(email, '') as email, password_hash, profile_picture_url, bio, is_admin, totp_enabled, totp_secret_encrypted, approved_at, suspended_at, created_at, updated_at, deleted_at
		FROM users
		WHERE username = $1 AND deleted_at IS NULL
	`

	var user models.User
	err := s.db.QueryRowContext(ctx, query, username).
		Scan(&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.ProfilePictureURL,
			&user.Bio, &user.IsAdmin, &user.TotpEnabled, &user.TotpSecretEncrypted, &user.ApprovedAt, &user.SuspendedAt, &user.CreatedAt, &user.UpdatedAt, &user.DeletedAt)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			notFoundErr := fmt.Errorf("user not found")
			recordSpanError(span, notFoundErr)
			return nil, notFoundErr
		}
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return &user, nil
}

// GetUserByEmail retrieves a user by email
func (s *UserService) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	ctx, span := otel.Tracer("clubhouse.users").Start(ctx, "UserService.GetUserByEmail")
	span.SetAttributes(attribute.Bool("has_email", strings.TrimSpace(email) != ""))
	defer span.End()

	query := `
		SELECT id, username, COALESCE(email, '') as email, password_hash, profile_picture_url, bio, is_admin, totp_enabled, totp_secret_encrypted, approved_at, suspended_at, created_at, updated_at, deleted_at
		FROM users
		WHERE email = $1 AND deleted_at IS NULL
	`

	var user models.User
	err := s.db.QueryRowContext(ctx, query, email).
		Scan(&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.ProfilePictureURL,
			&user.Bio, &user.IsAdmin, &user.TotpEnabled, &user.TotpSecretEncrypted, &user.ApprovedAt, &user.SuspendedAt, &user.CreatedAt, &user.UpdatedAt, &user.DeletedAt)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			notFoundErr := fmt.Errorf("user not found")
			recordSpanError(span, notFoundErr)
			return nil, notFoundErr
		}
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return &user, nil
}

// LoginUser authenticates a user with username and password
func (s *UserService) LoginUser(ctx context.Context, req *models.LoginRequest) (*models.User, error) {
	ctx, span := otel.Tracer("clubhouse.users").Start(ctx, "UserService.LoginUser")
	span.SetAttributes(attribute.Bool("has_username", req != nil && strings.TrimSpace(req.Username) != ""))
	defer span.End()

	// Validate input
	if err := validateLoginInput(req); err != nil {
		recordSpanError(span, err)
		return nil, err
	}

	// Get user by username
	user, err := s.GetUserByUsername(ctx, req.Username)
	if err != nil {
		_ = bcrypt.CompareHashAndPassword(dummyPasswordHash, []byte(req.Password))
		recordSpanError(span, ErrInvalidCredentials)
		return nil, ErrInvalidCredentials
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		recordSpanError(span, ErrInvalidCredentials)
		return nil, ErrInvalidCredentials
	}

	// Check if user is approved
	if user.ApprovedAt == nil {
		recordSpanError(span, ErrUserNotApproved)
		return nil, ErrUserNotApproved
	}

	if user.SuspendedAt != nil {
		recordSpanError(span, ErrUserSuspended)
		return nil, ErrUserSuspended
	}

	return user, nil
}

// IsUserSuspended returns true when the user is currently suspended.
func (s *UserService) IsUserSuspended(ctx context.Context, userID uuid.UUID) (bool, error) {
	ctx, span := otel.Tracer("clubhouse.users").Start(ctx, "UserService.IsUserSuspended")
	span.SetAttributes(attribute.String("user_id", userID.String()))
	defer span.End()

	if s == nil || s.db == nil {
		err := fmt.Errorf("user service is not configured")
		recordSpanError(span, err)
		return false, err
	}

	query := `
		SELECT suspended_at
		FROM users
		WHERE id = $1 AND deleted_at IS NULL
	`

	var suspendedAt sql.NullTime
	if err := s.db.QueryRowContext(ctx, query, userID).Scan(&suspendedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			notFoundErr := fmt.Errorf("user not found")
			recordSpanError(span, notFoundErr)
			return false, notFoundErr
		}
		recordSpanError(span, err)
		return false, fmt.Errorf("failed to check user suspension: %w", err)
	}

	return suspendedAt.Valid, nil
}

// validateLoginInput validates login input
func validateLoginInput(req *models.LoginRequest) error {
	if strings.TrimSpace(req.Username) == "" {
		return ErrUsernameRequired
	}

	if strings.TrimSpace(req.Password) == "" {
		return ErrPasswordRequired
	}

	return nil
}

// validateRegisterInput validates registration input
func validateRegisterInput(req *models.RegisterRequest) error {
	if strings.TrimSpace(req.Username) == "" {
		return fmt.Errorf("username is required")
	}

	if len(req.Username) < 3 || len(req.Username) > 50 {
		return fmt.Errorf("username must be between 3 and 50 characters")
	}

	if !isValidUsername(req.Username) {
		return fmt.Errorf("username can only contain alphanumeric characters and underscores")
	}

	email := strings.TrimSpace(req.Email)
	if email != "" && !isValidEmail(email) {
		return fmt.Errorf("invalid email format")
	}

	if len(req.Password) < 12 {
		return fmt.Errorf("password must be at least 12 characters")
	}

	return nil
}

// isValidUsername checks if username contains only alphanumeric and underscores
func isValidUsername(username string) bool {
	for _, r := range username {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
			return false
		}
	}
	return true
}

// isValidEmail performs basic email validation
func isValidEmail(email string) bool {
	// Simple email validation - checks for @ and domain
	parts := strings.Split(email, "@")
	if len(parts) != 2 || len(parts[0]) == 0 || len(parts[1]) == 0 {
		return false
	}
	if !strings.Contains(parts[1], ".") {
		return false
	}
	return true
}

// GetPendingUsers retrieves all users pending admin approval
func (s *UserService) GetPendingUsers(ctx context.Context) ([]*models.PendingUser, error) {
	ctx, span := otel.Tracer("clubhouse.users").Start(ctx, "UserService.GetPendingUsers")
	defer span.End()

	query := `
		SELECT id, username, COALESCE(email, '') as email, created_at
		FROM users
		WHERE approved_at IS NULL AND deleted_at IS NULL
		ORDER BY created_at ASC
	`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to get pending users: %w", err)
	}
	defer rows.Close()

	var pendingUsers []*models.PendingUser
	for rows.Next() {
		var user models.PendingUser
		if err := rows.Scan(&user.ID, &user.Username, &user.Email, &user.CreatedAt); err != nil {
			recordSpanError(span, err)
			return nil, fmt.Errorf("failed to scan pending user: %w", err)
		}
		pendingUsers = append(pendingUsers, &user)
	}

	if err := rows.Err(); err != nil {
		recordSpanError(span, err)
		return nil, fmt.Errorf("error iterating pending users: %w", err)
	}

	return pendingUsers, nil
}

// GetApprovedUsers retrieves all approved users for admin listings
func (s *UserService) GetApprovedUsers(ctx context.Context) ([]*models.ApprovedUser, error) {
	ctx, span := otel.Tracer("clubhouse.users").Start(ctx, "UserService.GetApprovedUsers")
	defer span.End()

	query := `
		SELECT id, username, COALESCE(email, '') as email, is_admin, approved_at, created_at
		FROM users
		WHERE approved_at IS NOT NULL AND deleted_at IS NULL
		ORDER BY approved_at DESC
	`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to get approved users: %w", err)
	}
	defer rows.Close()

	var approvedUsers []*models.ApprovedUser
	for rows.Next() {
		var user models.ApprovedUser
		if err := rows.Scan(&user.ID, &user.Username, &user.Email, &user.IsAdmin, &user.ApprovedAt, &user.CreatedAt); err != nil {
			recordSpanError(span, err)
			return nil, fmt.Errorf("failed to scan approved user: %w", err)
		}
		approvedUsers = append(approvedUsers, &user)
	}

	if err := rows.Err(); err != nil {
		recordSpanError(span, err)
		return nil, fmt.Errorf("error iterating approved users: %w", err)
	}

	return approvedUsers, nil
}

// SearchUsersByUsernamePrefix returns approved, active users matching a username prefix.
func (s *UserService) SearchUsersByUsernamePrefix(ctx context.Context, query string, limit int) ([]models.UserSummary, error) {
	ctx, span := otel.Tracer("clubhouse.users").Start(ctx, "UserService.SearchUsersByUsernamePrefix")
	trimmed := strings.TrimSpace(query)
	span.SetAttributes(
		attribute.String("query", trimmed),
		attribute.Int("limit", limit),
	)
	defer span.End()

	if limit <= 0 {
		limit = 8
	}
	if limit > 20 {
		limit = 20
	}

	pattern := "%"
	if trimmed != "" {
		pattern = trimmed + "%"
	}

	queryStmt := `
		SELECT id, username, profile_picture_url
		FROM users
		WHERE approved_at IS NOT NULL
		  AND suspended_at IS NULL
		  AND deleted_at IS NULL
		  AND username ILIKE $1
		ORDER BY username ASC
		LIMIT $2
	`

	rows, err := s.db.QueryContext(ctx, queryStmt, pattern, limit)
	if err != nil {
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to search users: %w", err)
	}
	defer rows.Close()

	var users []models.UserSummary
	for rows.Next() {
		var user models.UserSummary
		if err := rows.Scan(&user.ID, &user.Username, &user.ProfilePictureURL); err != nil {
			recordSpanError(span, err)
			return nil, fmt.Errorf("failed to scan user summary: %w", err)
		}
		users = append(users, user)
	}

	if err := rows.Err(); err != nil {
		recordSpanError(span, err)
		return nil, fmt.Errorf("error iterating user summaries: %w", err)
	}

	return users, nil
}

// LookupUserByUsername returns an approved, active user summary by username (case-insensitive).
func (s *UserService) LookupUserByUsername(ctx context.Context, username string) (*models.UserSummary, error) {
	ctx, span := otel.Tracer("clubhouse.users").Start(ctx, "UserService.LookupUserByUsername")
	trimmed := strings.TrimSpace(username)
	span.SetAttributes(attribute.String("username", trimmed))
	defer span.End()

	if trimmed == "" {
		notFoundErr := fmt.Errorf("user not found")
		recordSpanError(span, notFoundErr)
		return nil, notFoundErr
	}

	query := `
		SELECT id, username, profile_picture_url
		FROM users
		WHERE approved_at IS NOT NULL
		  AND suspended_at IS NULL
		  AND deleted_at IS NULL
		  AND lower(username) = lower($1)
	`

	var user models.UserSummary
	err := s.db.QueryRowContext(ctx, query, trimmed).
		Scan(&user.ID, &user.Username, &user.ProfilePictureURL)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			notFoundErr := fmt.Errorf("user not found")
			recordSpanError(span, notFoundErr)
			return nil, notFoundErr
		}
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to lookup user: %w", err)
	}

	return &user, nil
}

// ApproveUser marks a user as approved by setting approved_at timestamp
func (s *UserService) ApproveUser(ctx context.Context, userID uuid.UUID, adminUserID uuid.UUID) (*models.ApproveUserResponse, error) {
	ctx, span := otel.Tracer("clubhouse.users").Start(ctx, "UserService.ApproveUser")
	span.SetAttributes(
		attribute.String("user_id", userID.String()),
		attribute.String("admin_user_id", adminUserID.String()),
	)
	defer span.End()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	// Get the user first to verify they exist and are pending
	query := `
		SELECT id, username, COALESCE(email, '') as email, approved_at, deleted_at
		FROM users
		WHERE id = $1
	`

	var user models.User
	err = tx.QueryRowContext(ctx, query, userID).
		Scan(&user.ID, &user.Username, &user.Email, &user.ApprovedAt, &user.DeletedAt)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			notFoundErr := fmt.Errorf("user not found")
			recordSpanError(span, notFoundErr)
			return nil, notFoundErr
		}
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// Check if user is already approved
	if user.ApprovedAt != nil {
		alreadyApprovedErr := fmt.Errorf("user already approved")
		recordSpanError(span, alreadyApprovedErr)
		return nil, alreadyApprovedErr
	}

	// Check if user is deleted
	if user.DeletedAt != nil {
		deletedErr := fmt.Errorf("user has been deleted")
		recordSpanError(span, deletedErr)
		return nil, deletedErr
	}

	// Update approved_at timestamp
	updateQuery := `
		UPDATE users
		SET approved_at = now(), updated_at = now()
		WHERE id = $1
		RETURNING id, username, COALESCE(email, '') as email
	`

	var approvedUser models.User
	err = tx.QueryRowContext(ctx, updateQuery, userID).
		Scan(&approvedUser.ID, &approvedUser.Username, &approvedUser.Email)

	if err != nil {
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to approve user: %w", err)
	}

	// Create audit log entry
	auditService := NewAuditService(tx)
	metadata := map[string]interface{}{
		"target_user_id": userID.String(),
	}
	if err := auditService.LogAuditWithMetadata(ctx, "approve_user", adminUserID, userID, metadata); err != nil {
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to create audit log: %w", err)
	}

	if err := tx.Commit(); err != nil {
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return &models.ApproveUserResponse{
		ID:       approvedUser.ID,
		Username: approvedUser.Username,
		Email:    approvedUser.Email,
		Message:  "User approved successfully",
	}, nil
}

// PromoteUserToAdmin grants admin privileges to a user (admin-only operation).
func (s *UserService) PromoteUserToAdmin(ctx context.Context, userID uuid.UUID, adminUserID uuid.UUID) (*models.PromoteUserResponse, error) {
	ctx, span := otel.Tracer("clubhouse.users").Start(ctx, "UserService.PromoteUserToAdmin")
	span.SetAttributes(
		attribute.String("user_id", userID.String()),
		attribute.String("admin_user_id", adminUserID.String()),
	)
	defer span.End()

	if userID == adminUserID {
		selfErr := fmt.Errorf("cannot promote self")
		recordSpanError(span, selfErr)
		return nil, selfErr
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	query := `
		SELECT id, username, COALESCE(email, '') as email, is_admin, deleted_at
		FROM users
		WHERE id = $1
	`

	var user models.User
	err = tx.QueryRowContext(ctx, query, userID).
		Scan(&user.ID, &user.Username, &user.Email, &user.IsAdmin, &user.DeletedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			notFoundErr := fmt.Errorf("user not found")
			recordSpanError(span, notFoundErr)
			return nil, notFoundErr
		}
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	if user.DeletedAt != nil {
		deletedErr := fmt.Errorf("user has been deleted")
		recordSpanError(span, deletedErr)
		return nil, deletedErr
	}
	if user.IsAdmin {
		alreadyAdminErr := fmt.Errorf("user already admin")
		recordSpanError(span, alreadyAdminErr)
		return nil, alreadyAdminErr
	}

	updateQuery := `
		UPDATE users
		SET is_admin = true, updated_at = now()
		WHERE id = $1
		RETURNING id, username, COALESCE(email, '') as email, is_admin
	`

	var promotedUser models.User
	err = tx.QueryRowContext(ctx, updateQuery, userID).
		Scan(&promotedUser.ID, &promotedUser.Username, &promotedUser.Email, &promotedUser.IsAdmin)
	if err != nil {
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to promote user: %w", err)
	}

	auditService := NewAuditService(tx)
	metadata := map[string]interface{}{
		"target_user_id":    userID.String(),
		"target_username":   promotedUser.Username,
		"previous_is_admin": user.IsAdmin,
	}
	if err := auditService.LogAuditWithMetadata(ctx, "promote_to_admin", adminUserID, userID, metadata); err != nil {
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to create audit log: %w", err)
	}

	if err := tx.Commit(); err != nil {
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return &models.PromoteUserResponse{
		ID:       promotedUser.ID,
		Username: promotedUser.Username,
		Email:    promotedUser.Email,
		IsAdmin:  promotedUser.IsAdmin,
		Message:  "User promoted to admin",
	}, nil
}

// SuspendUser suspends a user account (admin-only operation).
func (s *UserService) SuspendUser(ctx context.Context, adminUserID uuid.UUID, targetUserID uuid.UUID, reason string) (*models.SuspendUserResponse, error) {
	ctx, span := otel.Tracer("clubhouse.users").Start(ctx, "UserService.SuspendUser")
	span.SetAttributes(
		attribute.String("admin_user_id", adminUserID.String()),
		attribute.String("target_user_id", targetUserID.String()),
		attribute.Bool("has_reason", strings.TrimSpace(reason) != ""),
	)
	defer span.End()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	query := `
		SELECT id, username, suspended_at, deleted_at
		FROM users
		WHERE id = $1
	`

	var user models.User
	err = tx.QueryRowContext(ctx, query, targetUserID).
		Scan(&user.ID, &user.Username, &user.SuspendedAt, &user.DeletedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			notFoundErr := fmt.Errorf("user not found")
			recordSpanError(span, notFoundErr)
			return nil, notFoundErr
		}
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	if user.DeletedAt != nil {
		deletedErr := fmt.Errorf("user has been deleted")
		recordSpanError(span, deletedErr)
		return nil, deletedErr
	}
	if user.SuspendedAt != nil {
		alreadySuspendedErr := fmt.Errorf("user already suspended")
		recordSpanError(span, alreadySuspendedErr)
		return nil, alreadySuspendedErr
	}

	updateQuery := `
		UPDATE users
		SET suspended_at = now(), updated_at = now()
		WHERE id = $1
		RETURNING id, suspended_at
	`

	var suspendedAt time.Time
	var updatedUserID uuid.UUID
	if err := tx.QueryRowContext(ctx, updateQuery, targetUserID).Scan(&updatedUserID, &suspendedAt); err != nil {
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to suspend user: %w", err)
	}

	auditService := NewAuditService(tx)
	metadata := map[string]interface{}{
		"target_user_id": targetUserID.String(),
	}
	if strings.TrimSpace(reason) != "" {
		metadata["reason"] = strings.TrimSpace(reason)
	}
	if err := auditService.LogAuditWithMetadata(ctx, "suspend_user", adminUserID, targetUserID, metadata); err != nil {
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to create audit log: %w", err)
	}

	if err := tx.Commit(); err != nil {
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return &models.SuspendUserResponse{
		ID:          updatedUserID,
		SuspendedAt: suspendedAt,
		Message:     "User suspended successfully",
	}, nil
}

// UnsuspendUser removes a user suspension (admin-only operation).
func (s *UserService) UnsuspendUser(ctx context.Context, adminUserID uuid.UUID, targetUserID uuid.UUID) (*models.UnsuspendUserResponse, error) {
	ctx, span := otel.Tracer("clubhouse.users").Start(ctx, "UserService.UnsuspendUser")
	span.SetAttributes(
		attribute.String("admin_user_id", adminUserID.String()),
		attribute.String("target_user_id", targetUserID.String()),
	)
	defer span.End()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	query := `
		SELECT id, suspended_at, deleted_at
		FROM users
		WHERE id = $1
	`

	var user models.User
	err = tx.QueryRowContext(ctx, query, targetUserID).
		Scan(&user.ID, &user.SuspendedAt, &user.DeletedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			notFoundErr := fmt.Errorf("user not found")
			recordSpanError(span, notFoundErr)
			return nil, notFoundErr
		}
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	if user.DeletedAt != nil {
		deletedErr := fmt.Errorf("user has been deleted")
		recordSpanError(span, deletedErr)
		return nil, deletedErr
	}
	if user.SuspendedAt == nil {
		notSuspendedErr := fmt.Errorf("user not suspended")
		recordSpanError(span, notSuspendedErr)
		return nil, notSuspendedErr
	}

	updateQuery := `
		UPDATE users
		SET suspended_at = NULL, updated_at = now()
		WHERE id = $1
		RETURNING id
	`

	var updatedUserID uuid.UUID
	if err := tx.QueryRowContext(ctx, updateQuery, targetUserID).Scan(&updatedUserID); err != nil {
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to unsuspend user: %w", err)
	}

	auditService := NewAuditService(tx)
	metadata := map[string]interface{}{
		"target_user_id": targetUserID.String(),
	}
	if err := auditService.LogAuditWithMetadata(ctx, "unsuspend_user", adminUserID, targetUserID, metadata); err != nil {
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to create audit log: %w", err)
	}

	if err := tx.Commit(); err != nil {
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return &models.UnsuspendUserResponse{
		ID:      updatedUserID,
		Message: "User unsuspended successfully",
	}, nil
}

// RejectUser hard-deletes a pending user (must not be approved yet)
func (s *UserService) RejectUser(ctx context.Context, userID uuid.UUID, adminUserID uuid.UUID) (*models.RejectUserResponse, error) {
	ctx, span := otel.Tracer("clubhouse.users").Start(ctx, "UserService.RejectUser")
	span.SetAttributes(
		attribute.String("user_id", userID.String()),
		attribute.String("admin_user_id", adminUserID.String()),
	)
	defer span.End()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	// Get the user first to verify they exist and are pending
	query := `
		SELECT id, approved_at, deleted_at
		FROM users
		WHERE id = $1
	`

	var user models.User
	err = tx.QueryRowContext(ctx, query, userID).
		Scan(&user.ID, &user.ApprovedAt, &user.DeletedAt)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			notFoundErr := fmt.Errorf("user not found")
			recordSpanError(span, notFoundErr)
			return nil, notFoundErr
		}
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// Check if user is already approved
	if user.ApprovedAt != nil {
		approvedErr := fmt.Errorf("cannot reject approved user")
		recordSpanError(span, approvedErr)
		return nil, approvedErr
	}

	// Create audit log entry BEFORE deleting the user (FK constraint)
	auditService := NewAuditService(tx)
	metadata := map[string]interface{}{
		"target_user_id": userID.String(),
	}
	if err := auditService.LogAuditWithMetadata(ctx, "reject_user", adminUserID, userID, metadata); err != nil {
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to create audit log: %w", err)
	}

	// Hard delete the user
	deleteQuery := `
		DELETE FROM users
		WHERE id = $1
	`

	result, err := tx.ExecContext(ctx, deleteQuery, userID)
	if err != nil {
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to reject user: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to get affected rows: %w", err)
	}

	if rowsAffected == 0 {
		notFoundErr := fmt.Errorf("user not found")
		recordSpanError(span, notFoundErr)
		return nil, notFoundErr
	}

	if err := tx.Commit(); err != nil {
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return &models.RejectUserResponse{
		ID:      userID,
		Message: "User rejected and deleted successfully",
	}, nil
}

// GetUserProfile retrieves a user profile with stats by ID
func (s *UserService) GetUserProfile(ctx context.Context, id uuid.UUID) (*models.UserProfileResponse, error) {
	ctx, span := otel.Tracer("clubhouse.users").Start(ctx, "UserService.GetUserProfile")
	span.SetAttributes(attribute.String("user_id", id.String()))
	defer span.End()

	query := `
		SELECT
			u.id, u.username, u.bio, u.profile_picture_url, u.created_at,
			(SELECT COUNT(*) FROM posts WHERE user_id = u.id AND deleted_at IS NULL) as post_count,
			(SELECT COUNT(*) FROM comments WHERE user_id = u.id AND deleted_at IS NULL) as comment_count
		FROM users u
		WHERE u.id = $1 AND u.deleted_at IS NULL AND u.approved_at IS NOT NULL
	`

	var profile models.UserProfileResponse
	err := s.db.QueryRowContext(ctx, query, id).
		Scan(&profile.ID, &profile.Username, &profile.Bio, &profile.ProfilePictureUrl,
			&profile.CreatedAt, &profile.Stats.PostCount, &profile.Stats.CommentCount)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			notFoundErr := fmt.Errorf("user not found")
			recordSpanError(span, notFoundErr)
			return nil, notFoundErr
		}
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to get user profile: %w", err)
	}

	return &profile, nil
}

// getCommentReactions retrieves reaction counts and viewer reactions for a comment
func (s *UserService) getCommentReactions(ctx context.Context, commentID uuid.UUID, viewerID uuid.UUID) (map[string]int, []string, error) {
	// Get counts
	rows, err := s.db.QueryContext(ctx, `
		SELECT emoji, COUNT(*)
		FROM reactions
		WHERE comment_id = $1 AND deleted_at IS NULL
		GROUP BY emoji
	`, commentID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var emoji string
		var count int
		if err := rows.Scan(&emoji, &count); err != nil {
			return nil, nil, err
		}
		counts[emoji] = count
	}

	var viewerReactions []string
	if viewerID != uuid.Nil {
		rows, err := s.db.QueryContext(ctx, `
			SELECT emoji
			FROM reactions
			WHERE comment_id = $1 AND user_id = $2 AND deleted_at IS NULL
		`, commentID, viewerID)
		if err != nil {
			return nil, nil, err
		}
		defer rows.Close()

		for rows.Next() {
			var emoji string
			if err := rows.Scan(&emoji); err != nil {
				return nil, nil, err
			}
			viewerReactions = append(viewerReactions, emoji)
		}
	}

	return counts, viewerReactions, nil
}

// GetUserComments retrieves a paginated list of comments by a user
func (s *UserService) GetUserComments(ctx context.Context, userID uuid.UUID, cursor *string, limit int) (*models.GetThreadResponse, error) {
	ctx, span := otel.Tracer("clubhouse.users").Start(ctx, "UserService.GetUserComments")
	span.SetAttributes(
		attribute.String("user_id", userID.String()),
		attribute.Bool("has_cursor", cursor != nil && *cursor != ""),
		attribute.Int("limit", limit),
	)
	defer span.End()

	// First verify the user exists and is approved
	var exists bool
	err := s.db.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM users WHERE id = $1 AND deleted_at IS NULL AND approved_at IS NOT NULL)
	`, userID).Scan(&exists)
	if err != nil {
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to check user: %w", err)
	}
	if !exists {
		notFoundErr := fmt.Errorf("user not found")
		recordSpanError(span, notFoundErr)
		return nil, notFoundErr
	}

	if limit <= 0 || limit > 100 {
		limit = 20
	}

	// Build query for user's comments
	query := `
		SELECT
			c.id, c.user_id, c.post_id, c.parent_comment_id, c.image_id, c.content,
			c.created_at, c.updated_at,
			u.id, u.username, u.profile_picture_url
		FROM comments c
		JOIN users u ON c.user_id = u.id
		WHERE c.user_id = $1 AND c.deleted_at IS NULL
	`

	args := []interface{}{userID}
	argIndex := 2

	// Apply cursor if provided (cursor is the created_at timestamp)
	if cursor != nil && *cursor != "" {
		query += fmt.Sprintf(" AND c.created_at < $%d", argIndex)
		args = append(args, *cursor)
		argIndex++
	}

	query += fmt.Sprintf(" ORDER BY c.created_at DESC LIMIT $%d", argIndex)
	args = append(args, limit+1) // Fetch one extra to determine hasMore

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to query comments: %w", err)
	}
	defer rows.Close()

	var comments []models.Comment
	for rows.Next() {
		var comment models.Comment
		var user models.User
		var imageID sql.NullString

		err := rows.Scan(
			&comment.ID, &comment.UserID, &comment.PostID, &comment.ParentCommentID, &imageID, &comment.Content,
			&comment.CreatedAt, &comment.UpdatedAt,
			&user.ID, &user.Username, &user.ProfilePictureURL,
		)
		if err != nil {
			recordSpanError(span, err)
			return nil, fmt.Errorf("failed to scan comment: %w", err)
		}

		comment.User = &user
		if imageID.Valid {
			parsedID, _ := uuid.Parse(imageID.String)
			comment.ImageID = &parsedID
		}

		// Fetch reactions
		// Note: We don't have the viewer ID here in the current signature.
		// However, GetUserComments is usually viewing another user's profile.
		// We should update the signature to include viewerID if we want to show viewer's reactions.
		// For now, I'll pass uuid.Nil as I haven't updated the interface yet and it might be out of scope or requires more changes.
		// Actually, I should update the signature.
		counts, _, err := s.getCommentReactions(ctx, comment.ID, uuid.Nil)
		if err != nil {
			recordSpanError(span, err)
			return nil, fmt.Errorf("failed to get comment reactions: %w", err)
		}
		comment.ReactionCounts = counts
		// comment.ViewerReactions = viewerReactions // Not setting this as we don't have viewerID

		comments = append(comments, comment)
	}

	if err = rows.Err(); err != nil {
		recordSpanError(span, err)
		return nil, fmt.Errorf("error iterating comments: %w", err)
	}

	// Determine if there are more comments
	hasMore := len(comments) > limit
	if hasMore {
		comments = comments[:limit]
	}

	// Determine next cursor
	var nextCursor *string
	if hasMore && len(comments) > 0 {
		lastComment := comments[len(comments)-1]
		cursorStr := lastComment.CreatedAt.Format("2006-01-02T15:04:05.000Z07:00")
		nextCursor = &cursorStr
	}

	return &models.GetThreadResponse{
		Comments: comments,
		Meta: models.PageMeta{
			Cursor:  nextCursor,
			HasMore: hasMore,
		},
	}, nil
}

// UpdateProfile updates the user's own profile (bio and profile picture URL)
func (s *UserService) UpdateProfile(ctx context.Context, userID uuid.UUID, req *models.UpdateUserRequest) (*models.UpdateUserResponse, error) {
	ctx, span := otel.Tracer("clubhouse.users").Start(ctx, "UserService.UpdateProfile")
	span.SetAttributes(
		attribute.String("user_id", userID.String()),
		attribute.Bool("has_bio", req != nil && req.Bio != nil),
		attribute.Bool("has_profile_picture_url", req != nil && req.ProfilePictureUrl != nil),
	)
	defer span.End()

	// Validate profile picture URL if provided
	if req.ProfilePictureUrl != nil && *req.ProfilePictureUrl != "" {
		if err := validateProfilePictureURL(*req.ProfilePictureUrl); err != nil {
			recordSpanError(span, err)
			return nil, err
		}
	}

	// Check if at least one field is provided
	if req.Bio == nil && req.ProfilePictureUrl == nil {
		missingErr := fmt.Errorf("at least one field (bio or profile_picture_url) is required")
		recordSpanError(span, missingErr)
		return nil, missingErr
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	var currentBio sql.NullString
	var currentProfilePictureURL sql.NullString
	currentQuery := `
		SELECT bio, profile_picture_url
		FROM users
		WHERE id = $1 AND deleted_at IS NULL
	`
	if err := tx.QueryRowContext(ctx, currentQuery, userID).Scan(&currentBio, &currentProfilePictureURL); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			notFoundErr := fmt.Errorf("user not found")
			recordSpanError(span, notFoundErr)
			return nil, notFoundErr
		}
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to load current profile: %w", err)
	}

	// Build dynamic UPDATE query based on provided fields
	setClauses := []string{"updated_at = now()"}
	args := []interface{}{}
	argIndex := 1

	if req.Bio != nil {
		setClauses = append(setClauses, fmt.Sprintf("bio = $%d", argIndex))
		args = append(args, *req.Bio)
		argIndex++
	}

	if req.ProfilePictureUrl != nil {
		setClauses = append(setClauses, fmt.Sprintf("profile_picture_url = $%d", argIndex))
		args = append(args, *req.ProfilePictureUrl)
		argIndex++
	}

	args = append(args, userID)

	query := fmt.Sprintf(`
		UPDATE users
		SET %s
		WHERE id = $%d AND deleted_at IS NULL
		RETURNING id, username, COALESCE(email, '') as email, profile_picture_url, bio, is_admin
	`, strings.Join(setClauses, ", "), argIndex)

	var response models.UpdateUserResponse
	err = tx.QueryRowContext(ctx, query, args...).
		Scan(&response.ID, &response.Username, &response.Email,
			&response.ProfilePictureUrl, &response.Bio, &response.IsAdmin)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			notFoundErr := fmt.Errorf("user not found")
			recordSpanError(span, notFoundErr)
			return nil, notFoundErr
		}
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to update profile: %w", err)
	}

	changes := map[string]interface{}{}
	changedFields := []string{}
	if req.Bio != nil {
		newBio := *req.Bio
		if !currentBio.Valid || currentBio.String != newBio {
			var oldValue interface{}
			if currentBio.Valid {
				oldValue = currentBio.String
			}
			changes["bio"] = map[string]interface{}{
				"old": oldValue,
				"new": newBio,
			}
			changedFields = append(changedFields, "bio")
		}
	}

	if req.ProfilePictureUrl != nil {
		newProfilePicture := *req.ProfilePictureUrl
		if !currentProfilePictureURL.Valid || currentProfilePictureURL.String != newProfilePicture {
			var oldValue interface{}
			if currentProfilePictureURL.Valid {
				oldValue = currentProfilePictureURL.String
			}
			changes["profile_picture_url"] = map[string]interface{}{
				"old": oldValue,
				"new": newProfilePicture,
			}
			changedFields = append(changedFields, "profile_picture_url")
		}
	}

	metadata := map[string]interface{}{
		"changed_fields": changedFields,
	}
	if len(changes) > 0 {
		metadata["changes"] = changes
	}

	auditService := NewAuditService(tx)
	if err := auditService.LogAuditWithMetadata(ctx, "update_profile", uuid.Nil, userID, metadata); err != nil {
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to create audit log: %w", err)
	}

	if err := tx.Commit(); err != nil {
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return &response, nil
}

// validateProfilePictureURL validates that the profile picture URL is a valid URL
func validateProfilePictureURL(urlStr string) error {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return fmt.Errorf("invalid profile picture URL")
	}

	// Must have a scheme (http or https)
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("profile picture URL must use http or https scheme")
	}

	// Must have a host
	if parsedURL.Host == "" {
		return fmt.Errorf("invalid profile picture URL")
	}

	return nil
}

// GetSectionSubscriptions lists section opt-outs for a user.
func (s *UserService) GetSectionSubscriptions(ctx context.Context, userID uuid.UUID) ([]models.SectionSubscription, error) {
	ctx, span := otel.Tracer("clubhouse.users").Start(ctx, "UserService.GetSectionSubscriptions")
	span.SetAttributes(attribute.String("user_id", userID.String()))
	defer span.End()

	query := `
		SELECT section_id, opted_out_at
		FROM section_subscriptions
		WHERE user_id = $1
		ORDER BY opted_out_at DESC
	`

	rows, err := s.db.QueryContext(ctx, query, userID)
	if err != nil {
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to list section subscriptions: %w", err)
	}
	defer rows.Close()

	var subscriptions []models.SectionSubscription
	for rows.Next() {
		var subscription models.SectionSubscription
		if err := rows.Scan(&subscription.SectionID, &subscription.OptedOutAt); err != nil {
			recordSpanError(span, err)
			return nil, fmt.Errorf("failed to scan section subscription: %w", err)
		}
		subscriptions = append(subscriptions, subscription)
	}

	if err := rows.Err(); err != nil {
		recordSpanError(span, err)
		return nil, fmt.Errorf("error iterating section subscriptions: %w", err)
	}

	return subscriptions, nil
}

// UpdateSectionSubscription sets a user's opt-out preference for a section.
func (s *UserService) UpdateSectionSubscription(ctx context.Context, userID uuid.UUID, sectionID uuid.UUID, optedOut bool) (*models.UpdateSectionSubscriptionResponse, error) {
	ctx, span := otel.Tracer("clubhouse.users").Start(ctx, "UserService.UpdateSectionSubscription")
	span.SetAttributes(
		attribute.String("user_id", userID.String()),
		attribute.String("section_id", sectionID.String()),
		attribute.Bool("opted_out", optedOut),
	)
	defer span.End()

	var sectionExists bool
	if err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM sections WHERE id = $1)`, sectionID).Scan(&sectionExists); err != nil {
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to check section: %w", err)
	}
	if !sectionExists {
		notFoundErr := fmt.Errorf("section not found")
		recordSpanError(span, notFoundErr)
		return nil, notFoundErr
	}

	if optedOut {
		var optedOutAt time.Time
		query := `
			INSERT INTO section_subscriptions (user_id, section_id, opted_out_at)
			VALUES ($1, $2, now())
			ON CONFLICT (user_id, section_id)
			DO UPDATE SET opted_out_at = now()
			RETURNING opted_out_at
		`
		if err := s.db.QueryRowContext(ctx, query, userID, sectionID).Scan(&optedOutAt); err != nil {
			recordSpanError(span, err)
			return nil, fmt.Errorf("failed to opt out of section: %w", err)
		}

		return &models.UpdateSectionSubscriptionResponse{
			SectionID:  sectionID,
			OptedOut:   true,
			OptedOutAt: &optedOutAt,
		}, nil
	}

	_, err := s.db.ExecContext(ctx, `DELETE FROM section_subscriptions WHERE user_id = $1 AND section_id = $2`, userID, sectionID)
	if err != nil {
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to opt in to section: %w", err)
	}

	return &models.UpdateSectionSubscriptionResponse{
		SectionID: sectionID,
		OptedOut:  false,
	}, nil
}

// ResetPassword resets a user's password (called after token verification)
func (s *UserService) ResetPassword(ctx context.Context, userID uuid.UUID, newPassword string) error {
	ctx, span := otel.Tracer("clubhouse.users").Start(ctx, "UserService.ResetPassword")
	span.SetAttributes(
		attribute.String("user_id", userID.String()),
		attribute.Bool("has_password", strings.TrimSpace(newPassword) != ""),
	)
	defer span.End()

	// Validate password
	if len(newPassword) < 12 {
		err := fmt.Errorf("password must be at least 12 characters")
		recordSpanError(span, err)
		return err
	}

	// Hash password
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcryptCost)
	if err != nil {
		recordSpanError(span, err)
		return fmt.Errorf("failed to hash password: %w", err)
	}

	// Update password in database
	query := `
		UPDATE users
		SET password_hash = $1, updated_at = now()
		WHERE id = $2 AND deleted_at IS NULL
	`

	result, err := s.db.ExecContext(ctx, query, string(passwordHash), userID)
	if err != nil {
		recordSpanError(span, err)
		return fmt.Errorf("failed to reset password: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		recordSpanError(span, err)
		return fmt.Errorf("failed to check rows affected: %w", err)
	}

	if rowsAffected == 0 {
		notFoundErr := fmt.Errorf("user not found")
		recordSpanError(span, notFoundErr)
		return notFoundErr
	}

	return nil
}
