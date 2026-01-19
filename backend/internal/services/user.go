package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/google/uuid"
	"github.com/sanderginn/clubhouse/internal/models"
	"golang.org/x/crypto/bcrypt"
)

const (
	bcryptCost = 12
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
	// Validate input
	if err := validateRegisterInput(req); err != nil {
		return nil, err
	}

	// Hash password
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcryptCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Create user ID
	userID := uuid.New()

	// Insert into database
	query := `
		INSERT INTO users (id, username, email, password_hash, is_admin, created_at)
		VALUES ($1, $2, $3, $4, false, now())
		RETURNING id, username, email, is_admin, created_at
	`

	var user models.User
	err = s.db.QueryRowContext(ctx, query, userID, req.Username, req.Email, string(passwordHash)).
		Scan(&user.ID, &user.Username, &user.Email, &user.IsAdmin, &user.CreatedAt)

	if err != nil {
		// Check for unique constraint violations
		if strings.Contains(err.Error(), "duplicate key") {
			if strings.Contains(err.Error(), "username") {
				return nil, fmt.Errorf("username already exists")
			}
			if strings.Contains(err.Error(), "email") {
				return nil, fmt.Errorf("email already exists")
			}
		}
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return &user, nil
}

// GetUserByID retrieves a user by ID
func (s *UserService) GetUserByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	query := `
		SELECT id, username, email, password_hash, profile_picture_url, bio, is_admin, approved_at, created_at, updated_at, deleted_at
		FROM users
		WHERE id = $1 AND deleted_at IS NULL
	`

	var user models.User
	err := s.db.QueryRowContext(ctx, query, id).
		Scan(&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.ProfilePictureURL,
			&user.Bio, &user.IsAdmin, &user.ApprovedAt, &user.CreatedAt, &user.UpdatedAt, &user.DeletedAt)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return &user, nil
}

// GetUserByUsername retrieves a user by username
func (s *UserService) GetUserByUsername(ctx context.Context, username string) (*models.User, error) {
	query := `
		SELECT id, username, email, password_hash, profile_picture_url, bio, is_admin, approved_at, created_at, updated_at, deleted_at
		FROM users
		WHERE username = $1 AND deleted_at IS NULL
	`

	var user models.User
	err := s.db.QueryRowContext(ctx, query, username).
		Scan(&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.ProfilePictureURL,
			&user.Bio, &user.IsAdmin, &user.ApprovedAt, &user.CreatedAt, &user.UpdatedAt, &user.DeletedAt)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return &user, nil
}

// GetUserByEmail retrieves a user by email
func (s *UserService) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	query := `
		SELECT id, username, email, password_hash, profile_picture_url, bio, is_admin, approved_at, created_at, updated_at, deleted_at
		FROM users
		WHERE email = $1 AND deleted_at IS NULL
	`

	var user models.User
	err := s.db.QueryRowContext(ctx, query, email).
		Scan(&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.ProfilePictureURL,
			&user.Bio, &user.IsAdmin, &user.ApprovedAt, &user.CreatedAt, &user.UpdatedAt, &user.DeletedAt)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return &user, nil
}

// LoginUser authenticates a user with email and password
func (s *UserService) LoginUser(ctx context.Context, req *models.LoginRequest) (*models.User, error) {
	// Validate input
	if err := validateLoginInput(req); err != nil {
		return nil, err
	}

	// Get user by email
	user, err := s.GetUserByEmail(ctx, req.Email)
	if err != nil {
		return nil, fmt.Errorf("invalid email or password")
	}

	// Check if user is approved
	if user.ApprovedAt == nil {
		return nil, fmt.Errorf("user not approved")
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, fmt.Errorf("invalid email or password")
	}

	return user, nil
}

// validateLoginInput validates login input
func validateLoginInput(req *models.LoginRequest) error {
	if strings.TrimSpace(req.Email) == "" {
		return fmt.Errorf("email is required")
	}

	if !isValidEmail(req.Email) {
		return fmt.Errorf("invalid email format")
	}

	if strings.TrimSpace(req.Password) == "" {
		return fmt.Errorf("password is required")
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

	if strings.TrimSpace(req.Email) == "" {
		return fmt.Errorf("email is required")
	}

	if !isValidEmail(req.Email) {
		return fmt.Errorf("invalid email format")
	}

	if len(req.Password) < 8 {
		return fmt.Errorf("password must be at least 8 characters")
	}

	if !isStrongPassword(req.Password) {
		return fmt.Errorf("password must contain uppercase, lowercase, and numeric characters")
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

// isStrongPassword checks for uppercase, lowercase, and numeric characters
func isStrongPassword(password string) bool {
	hasUpper := false
	hasLower := false
	hasDigit := false

	for _, r := range password {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasDigit = true
		}
	}

	return hasUpper && hasLower && hasDigit
}

// GetPendingUsers retrieves all users pending admin approval
func (s *UserService) GetPendingUsers(ctx context.Context) ([]*models.PendingUser, error) {
	query := `
		SELECT id, username, email, created_at
		FROM users
		WHERE approved_at IS NULL AND deleted_at IS NULL
		ORDER BY created_at ASC
	`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending users: %w", err)
	}
	defer rows.Close()

	var pendingUsers []*models.PendingUser
	for rows.Next() {
		var user models.PendingUser
		if err := rows.Scan(&user.ID, &user.Username, &user.Email, &user.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan pending user: %w", err)
		}
		pendingUsers = append(pendingUsers, &user)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating pending users: %w", err)
	}

	return pendingUsers, nil
}

// ApproveUser marks a user as approved by setting approved_at timestamp
func (s *UserService) ApproveUser(ctx context.Context, userID uuid.UUID) (*models.ApproveUserResponse, error) {
	// Get the user first to verify they exist and are pending
	query := `
		SELECT id, username, email, approved_at, deleted_at
		FROM users
		WHERE id = $1
	`

	var user models.User
	err := s.db.QueryRowContext(ctx, query, userID).
		Scan(&user.ID, &user.Username, &user.Email, &user.ApprovedAt, &user.DeletedAt)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// Check if user is already approved
	if user.ApprovedAt != nil {
		return nil, fmt.Errorf("user already approved")
	}

	// Check if user is deleted
	if user.DeletedAt != nil {
		return nil, fmt.Errorf("user has been deleted")
	}

	// Update approved_at timestamp
	updateQuery := `
		UPDATE users
		SET approved_at = now(), updated_at = now()
		WHERE id = $1
		RETURNING id, username, email
	`

	var approvedUser models.User
	err = s.db.QueryRowContext(ctx, updateQuery, userID).
		Scan(&approvedUser.ID, &approvedUser.Username, &approvedUser.Email)

	if err != nil {
		return nil, fmt.Errorf("failed to approve user: %w", err)
	}

	return &models.ApproveUserResponse{
		ID:       approvedUser.ID,
		Username: approvedUser.Username,
		Email:    approvedUser.Email,
		Message:  "User approved successfully",
	}, nil
}

// RejectUser hard-deletes a pending user (must not be approved yet)
func (s *UserService) RejectUser(ctx context.Context, userID uuid.UUID) (*models.RejectUserResponse, error) {
	// Get the user first to verify they exist and are pending
	query := `
		SELECT id, approved_at, deleted_at
		FROM users
		WHERE id = $1
	`

	var user models.User
	err := s.db.QueryRowContext(ctx, query, userID).
		Scan(&user.ID, &user.ApprovedAt, &user.DeletedAt)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// Check if user is already approved
	if user.ApprovedAt != nil {
		return nil, fmt.Errorf("cannot reject approved user")
	}

	// Hard delete the user
	deleteQuery := `
		DELETE FROM users
		WHERE id = $1
	`

	result, err := s.db.ExecContext(ctx, deleteQuery, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to reject user: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("failed to get affected rows: %w", err)
	}

	if rowsAffected == 0 {
		return nil, fmt.Errorf("user not found")
	}

	return &models.RejectUserResponse{
		ID:      userID,
		Message: "User rejected and deleted successfully",
	}, nil
}

// GetUserProfile retrieves a user profile with stats by ID
func (s *UserService) GetUserProfile(ctx context.Context, id uuid.UUID) (*models.UserProfileResponse, error) {
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
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user profile: %w", err)
	}

	return &profile, nil
}
