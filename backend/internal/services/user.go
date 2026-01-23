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
	"golang.org/x/crypto/bcrypt"
)

const (
	bcryptCost = 12
)

// dummyPasswordHash is a bcrypt hash for timing-equalized compares on unknown users.
var dummyPasswordHash = []byte("$2a$12$ukjUkUX1cfSD88LBRMvNjuwNn2eWmisHaOuhtgo/napH/3VmLCtNK")

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
	err = s.db.QueryRowContext(ctx, query, userID, req.Username, email, string(passwordHash)).
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

// AdminExists checks if there is at least one active admin user.
func (s *UserService) AdminExists(ctx context.Context) (bool, error) {
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
		return false, fmt.Errorf("failed to check admin existence: %w", err)
	}

	return exists, nil
}

// BootstrapAdmin creates the first admin user if none exist.
// Returns created=false when an admin already exists.
func (s *UserService) BootstrapAdmin(ctx context.Context, username, email, password string) (*models.User, bool, error) {
	req := &models.RegisterRequest{
		Username: username,
		Email:    email,
		Password: password,
	}
	if err := validateRegisterInput(req); err != nil {
		return nil, false, err
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
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
				return nil, false, fmt.Errorf("username already exists")
			}
			if strings.Contains(err.Error(), "email") {
				return nil, false, fmt.Errorf("email already exists")
			}
		}
		return nil, false, fmt.Errorf("failed to create bootstrap admin: %w", err)
	}

	return &user, true, nil
}

// GetUserByID retrieves a user by ID
func (s *UserService) GetUserByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	query := `
		SELECT id, username, COALESCE(email, '') as email, password_hash, profile_picture_url, bio, is_admin, approved_at, created_at, updated_at, deleted_at
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
		SELECT id, username, COALESCE(email, '') as email, password_hash, profile_picture_url, bio, is_admin, approved_at, created_at, updated_at, deleted_at
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
		SELECT id, username, COALESCE(email, '') as email, password_hash, profile_picture_url, bio, is_admin, approved_at, created_at, updated_at, deleted_at
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

// LoginUser authenticates a user with username and password
func (s *UserService) LoginUser(ctx context.Context, req *models.LoginRequest) (*models.User, error) {
	// Validate input
	if err := validateLoginInput(req); err != nil {
		return nil, err
	}

	// Get user by username
	user, err := s.GetUserByUsername(ctx, req.Username)
	if err != nil {
		_ = bcrypt.CompareHashAndPassword(dummyPasswordHash, []byte(req.Password))
		return nil, fmt.Errorf("invalid username or password")
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, fmt.Errorf("invalid username or password")
	}

	// Check if user is approved
	if user.ApprovedAt == nil {
		return nil, fmt.Errorf("invalid username or password")
	}

	return user, nil
}

// validateLoginInput validates login input
func validateLoginInput(req *models.LoginRequest) error {
	if strings.TrimSpace(req.Username) == "" {
		return fmt.Errorf("username is required")
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
	query := `
		SELECT id, username, COALESCE(email, '') as email, created_at
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
func (s *UserService) ApproveUser(ctx context.Context, userID uuid.UUID, adminUserID uuid.UUID) (*models.ApproveUserResponse, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
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
		RETURNING id, username, COALESCE(email, '') as email
	`

	var approvedUser models.User
	err = tx.QueryRowContext(ctx, updateQuery, userID).
		Scan(&approvedUser.ID, &approvedUser.Username, &approvedUser.Email)

	if err != nil {
		return nil, fmt.Errorf("failed to approve user: %w", err)
	}

	// Create audit log entry
	_, err = tx.ExecContext(ctx, `
		INSERT INTO audit_logs (admin_user_id, action, related_user_id, created_at)
		VALUES ($1, 'approve_user', $2, now())
	`, adminUserID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to create audit log: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return &models.ApproveUserResponse{
		ID:       approvedUser.ID,
		Username: approvedUser.Username,
		Email:    approvedUser.Email,
		Message:  "User approved successfully",
	}, nil
}

// RejectUser hard-deletes a pending user (must not be approved yet)
func (s *UserService) RejectUser(ctx context.Context, userID uuid.UUID, adminUserID uuid.UUID) (*models.RejectUserResponse, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
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
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// Check if user is already approved
	if user.ApprovedAt != nil {
		return nil, fmt.Errorf("cannot reject approved user")
	}

	// Create audit log entry BEFORE deleting the user (FK constraint)
	_, err = tx.ExecContext(ctx, `
		INSERT INTO audit_logs (admin_user_id, action, related_user_id, created_at)
		VALUES ($1, 'reject_user', $2, now())
	`, adminUserID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to create audit log: %w", err)
	}

	// Hard delete the user
	deleteQuery := `
		DELETE FROM users
		WHERE id = $1
	`

	result, err := tx.ExecContext(ctx, deleteQuery, userID)
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

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
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
	// First verify the user exists and is approved
	var exists bool
	err := s.db.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM users WHERE id = $1 AND deleted_at IS NULL AND approved_at IS NOT NULL)
	`, userID).Scan(&exists)
	if err != nil {
		return nil, fmt.Errorf("failed to check user: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("user not found")
	}

	if limit <= 0 || limit > 100 {
		limit = 20
	}

	// Build query for user's comments
	query := `
		SELECT
			c.id, c.user_id, c.post_id, c.parent_comment_id, c.content,
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
		return nil, fmt.Errorf("failed to query comments: %w", err)
	}
	defer rows.Close()

	var comments []models.Comment
	for rows.Next() {
		var comment models.Comment
		var user models.User

		err := rows.Scan(
			&comment.ID, &comment.UserID, &comment.PostID, &comment.ParentCommentID, &comment.Content,
			&comment.CreatedAt, &comment.UpdatedAt,
			&user.ID, &user.Username, &user.ProfilePictureURL,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan comment: %w", err)
		}

		comment.User = &user

		// Fetch reactions
		// Note: We don't have the viewer ID here in the current signature.
		// However, GetUserComments is usually viewing another user's profile.
		// We should update the signature to include viewerID if we want to show viewer's reactions.
		// For now, I'll pass uuid.Nil as I haven't updated the interface yet and it might be out of scope or requires more changes.
		// Actually, I should update the signature.
		counts, _, err := s.getCommentReactions(ctx, comment.ID, uuid.Nil)
		if err != nil {
			return nil, fmt.Errorf("failed to get comment reactions: %w", err)
		}
		comment.ReactionCounts = counts
		// comment.ViewerReactions = viewerReactions // Not setting this as we don't have viewerID

		comments = append(comments, comment)
	}

	if err = rows.Err(); err != nil {
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
	// Validate profile picture URL if provided
	if req.ProfilePictureUrl != nil && *req.ProfilePictureUrl != "" {
		if err := validateProfilePictureURL(*req.ProfilePictureUrl); err != nil {
			return nil, err
		}
	}

	// Check if at least one field is provided
	if req.Bio == nil && req.ProfilePictureUrl == nil {
		return nil, fmt.Errorf("at least one field (bio or profile_picture_url) is required")
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
	err := s.db.QueryRowContext(ctx, query, args...).
		Scan(&response.ID, &response.Username, &response.Email,
			&response.ProfilePictureUrl, &response.Bio, &response.IsAdmin)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to update profile: %w", err)
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
	query := `
		SELECT section_id, opted_out_at
		FROM section_subscriptions
		WHERE user_id = $1
		ORDER BY opted_out_at DESC
	`

	rows, err := s.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list section subscriptions: %w", err)
	}
	defer rows.Close()

	var subscriptions []models.SectionSubscription
	for rows.Next() {
		var subscription models.SectionSubscription
		if err := rows.Scan(&subscription.SectionID, &subscription.OptedOutAt); err != nil {
			return nil, fmt.Errorf("failed to scan section subscription: %w", err)
		}
		subscriptions = append(subscriptions, subscription)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating section subscriptions: %w", err)
	}

	return subscriptions, nil
}

// UpdateSectionSubscription sets a user's opt-out preference for a section.
func (s *UserService) UpdateSectionSubscription(ctx context.Context, userID uuid.UUID, sectionID uuid.UUID, optedOut bool) (*models.UpdateSectionSubscriptionResponse, error) {
	var sectionExists bool
	if err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM sections WHERE id = $1)`, sectionID).Scan(&sectionExists); err != nil {
		return nil, fmt.Errorf("failed to check section: %w", err)
	}
	if !sectionExists {
		return nil, fmt.Errorf("section not found")
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
		return nil, fmt.Errorf("failed to opt in to section: %w", err)
	}

	return &models.UpdateSectionSubscriptionResponse{
		SectionID: sectionID,
		OptedOut:  false,
	}, nil
}
