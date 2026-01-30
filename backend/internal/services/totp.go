package services

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"golang.org/x/crypto/bcrypt"
)

const (
	totpIssuer           = "Clubhouse"
	totpCodeLength       = 6
	totpEncryptionEnv    = "CLUBHOUSE_TOTP_ENCRYPTION_KEY"
	totpEncryptionBytes  = 32
	totpBackupCodeCount  = 8
	totpBackupCodeDigits = 8
)

var (
	ErrTOTPKeyMissing       = errors.New("totp encryption key missing")
	ErrTOTPKeyInvalid       = errors.New("totp encryption key invalid")
	ErrTOTPRequired         = errors.New("totp required")
	ErrTOTPInvalid          = errors.New("invalid totp code")
	ErrTOTPNotEnrolled      = errors.New("totp not enrolled")
	ErrTOTPAlreadyEnabled   = errors.New("totp already enabled")
	ErrTOTPNotEnabled       = errors.New("totp not enabled")
	ErrTOTPUserNotFound     = errors.New("user not found")
	ErrTOTPAdminRequired    = errors.New("admin required")
	ErrTOTPSecretCorrupted  = errors.New("totp secret corrupted")
	ErrTOTPSecretEncryption = errors.New("totp encryption failed")
)

// TOTPService handles admin TOTP enrollment and verification.
type TOTPService struct {
	db     *sql.DB
	key    []byte
	keyErr error
}

type totpExecutor interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
}

// NewTOTPService creates a TOTP service with encryption key loaded from env.
func NewTOTPService(db *sql.DB) *TOTPService {
	key, err := loadTOTPKey()
	return &TOTPService{
		db:     db,
		key:    key,
		keyErr: err,
	}
}

// TOTPEnrollment represents an enrollment response payload.
type TOTPEnrollment struct {
	Secret string
	URL    string
}

// EnrollAdmin generates a new TOTP secret for an admin and stores it encrypted.
func (s *TOTPService) EnrollAdmin(ctx context.Context, userID uuid.UUID, username string) (*TOTPEnrollment, error) {
	if err := s.requireKey(); err != nil {
		return nil, err
	}

	username = strings.TrimSpace(username)
	if username == "" {
		return nil, fmt.Errorf("username is required")
	}

	var isAdmin bool
	var enabled bool
	err := s.db.QueryRowContext(ctx, `
		SELECT is_admin, totp_enabled
		FROM users
		WHERE id = $1 AND deleted_at IS NULL
	`, userID).Scan(&isAdmin, &enabled)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrTOTPUserNotFound
		}
		return nil, fmt.Errorf("failed to load admin status: %w", err)
	}
	if !isAdmin {
		return nil, ErrTOTPAdminRequired
	}
	if enabled {
		return nil, ErrTOTPAlreadyEnabled
	}

	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      totpIssuer,
		AccountName: username,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to generate totp secret: %w", err)
	}

	encryptedSecret, err := encryptTOTPSecret(s.key, key.Secret())
	if err != nil {
		return nil, err
	}

	_, err = s.db.ExecContext(ctx, `
		UPDATE users
		SET totp_secret_encrypted = $1,
			totp_enabled = false,
			updated_at = now()
		WHERE id = $2 AND deleted_at IS NULL
	`, encryptedSecret, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to store totp secret: %w", err)
	}

	return &TOTPEnrollment{
		Secret: key.Secret(),
		URL:    key.URL(),
	}, nil
}

// EnrollUser generates a new TOTP secret for a user and stores it encrypted.
func (s *TOTPService) EnrollUser(ctx context.Context, userID uuid.UUID, username string) (*TOTPEnrollment, error) {
	if err := s.requireKey(); err != nil {
		return nil, err
	}

	username = strings.TrimSpace(username)
	if username == "" {
		return nil, fmt.Errorf("username is required")
	}

	var enabled bool
	err := s.db.QueryRowContext(ctx, `
		SELECT totp_enabled
		FROM users
		WHERE id = $1 AND deleted_at IS NULL
	`, userID).Scan(&enabled)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrTOTPUserNotFound
		}
		return nil, fmt.Errorf("failed to load totp status: %w", err)
	}
	if enabled {
		return nil, ErrTOTPAlreadyEnabled
	}

	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      totpIssuer,
		AccountName: username,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to generate totp secret: %w", err)
	}

	encryptedSecret, err := encryptTOTPSecret(s.key, key.Secret())
	if err != nil {
		return nil, err
	}

	_, err = s.db.ExecContext(ctx, `
		UPDATE users
		SET totp_secret_encrypted = $1,
			totp_enabled = false,
			updated_at = now()
		WHERE id = $2 AND deleted_at IS NULL
	`, encryptedSecret, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to store totp secret: %w", err)
	}

	return &TOTPEnrollment{
		Secret: key.Secret(),
		URL:    key.URL(),
	}, nil
}

// VerifyAdmin verifies a TOTP code and enables MFA for the admin.
func (s *TOTPService) VerifyAdmin(ctx context.Context, userID uuid.UUID, code string) error {
	code = strings.TrimSpace(code)
	if code == "" {
		return ErrTOTPRequired
	}
	if len(code) != totpCodeLength {
		return ErrTOTPInvalid
	}

	var encrypted []byte
	var enabled bool
	err := s.db.QueryRowContext(ctx, `
		SELECT totp_secret_encrypted, totp_enabled
		FROM users
		WHERE id = $1 AND is_admin = true AND deleted_at IS NULL
	`, userID).Scan(&encrypted, &enabled)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrTOTPUserNotFound
		}
		return fmt.Errorf("failed to load totp secret: %w", err)
	}
	if enabled {
		return ErrTOTPAlreadyEnabled
	}
	if len(encrypted) == 0 {
		return ErrTOTPNotEnrolled
	}

	if err := s.requireKey(); err != nil {
		return err
	}

	secret, err := decryptTOTPSecret(s.key, encrypted)
	if err != nil {
		return err
	}

	valid, err := validateTOTP(secret, code)
	if err != nil {
		return err
	}
	if !valid {
		return ErrTOTPInvalid
	}

	_, err = s.db.ExecContext(ctx, `
		UPDATE users
		SET totp_enabled = true,
			updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL
	`, userID)
	if err != nil {
		return fmt.Errorf("failed to enable totp: %w", err)
	}

	return nil
}

// EnableUserWithBackupCodes verifies the TOTP code, enables MFA, and stores backup codes.
func (s *TOTPService) EnableUserWithBackupCodes(ctx context.Context, userID uuid.UUID, code string, backupCodes []string) error {
	code = strings.TrimSpace(code)
	if code == "" {
		return ErrTOTPRequired
	}
	if len(code) != totpCodeLength {
		return ErrTOTPInvalid
	}

	var encrypted []byte
	var enabled bool
	err := s.db.QueryRowContext(ctx, `
		SELECT totp_secret_encrypted, totp_enabled
		FROM users
		WHERE id = $1 AND deleted_at IS NULL
	`, userID).Scan(&encrypted, &enabled)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrTOTPUserNotFound
		}
		return fmt.Errorf("failed to load totp secret: %w", err)
	}
	if enabled {
		return ErrTOTPAlreadyEnabled
	}
	if len(encrypted) == 0 {
		return ErrTOTPNotEnrolled
	}

	if err := s.requireKey(); err != nil {
		return err
	}

	secret, err := decryptTOTPSecret(s.key, encrypted)
	if err != nil {
		return err
	}

	valid, err := validateTOTP(secret, code)
	if err != nil {
		return err
	}
	if !valid {
		return ErrTOTPInvalid
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin totp transaction: %w", err)
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	_, err = tx.ExecContext(ctx, `
		UPDATE users
		SET totp_enabled = true,
			updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL
	`, userID)
	if err != nil {
		return fmt.Errorf("failed to enable totp: %w", err)
	}

	if err := storeBackupCodes(ctx, tx, userID, backupCodes); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit totp transaction: %w", err)
	}
	tx = nil

	return nil
}

// DisableUser disables MFA after verifying a TOTP code.
func (s *TOTPService) DisableUser(ctx context.Context, userID uuid.UUID, code string) error {
	code = strings.TrimSpace(code)
	if code == "" {
		return ErrTOTPRequired
	}
	if len(code) != totpCodeLength {
		return ErrTOTPInvalid
	}

	var encrypted []byte
	var enabled bool
	err := s.db.QueryRowContext(ctx, `
		SELECT totp_secret_encrypted, totp_enabled
		FROM users
		WHERE id = $1 AND deleted_at IS NULL
	`, userID).Scan(&encrypted, &enabled)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrTOTPUserNotFound
		}
		return fmt.Errorf("failed to load totp settings: %w", err)
	}
	if !enabled {
		return ErrTOTPNotEnabled
	}
	if len(encrypted) == 0 {
		return ErrTOTPNotEnrolled
	}

	if err := s.requireKey(); err != nil {
		return err
	}

	secret, err := decryptTOTPSecret(s.key, encrypted)
	if err != nil {
		return err
	}

	valid, err := validateTOTP(secret, code)
	if err != nil {
		return err
	}
	if !valid {
		return ErrTOTPInvalid
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin totp transaction: %w", err)
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	_, err = tx.ExecContext(ctx, `
		UPDATE users
		SET totp_enabled = false,
			totp_secret_encrypted = NULL,
			updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL
	`, userID)
	if err != nil {
		return fmt.Errorf("failed to disable totp: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM mfa_backup_codes WHERE user_id = $1`, userID); err != nil {
		return fmt.Errorf("failed to clear backup codes: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit totp transaction: %w", err)
	}
	tx = nil

	return nil
}

// VerifyLogin checks a login TOTP code if MFA is enabled.
func (s *TOTPService) VerifyLogin(ctx context.Context, userID uuid.UUID, code string) error {
	var encrypted []byte
	var enabled bool
	err := s.db.QueryRowContext(ctx, `
		SELECT totp_secret_encrypted, totp_enabled
		FROM users
		WHERE id = $1 AND deleted_at IS NULL
	`, userID).Scan(&encrypted, &enabled)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrTOTPUserNotFound
		}
		return fmt.Errorf("failed to load totp settings: %w", err)
	}

	if !enabled {
		return nil
	}

	code = strings.TrimSpace(code)
	if code == "" {
		return ErrTOTPRequired
	}
	normalized := normalizeBackupCode(code)
	switch len(normalized) {
	case totpCodeLength:
		code = normalized
	case totpBackupCodeDigits:
		consumed, consumeErr := s.consumeBackupCode(ctx, userID, normalized)
		if consumeErr != nil {
			return consumeErr
		}
		if !consumed {
			return ErrTOTPInvalid
		}
		return nil
	default:
		return ErrTOTPInvalid
	}
	if len(encrypted) == 0 {
		return ErrTOTPNotEnrolled
	}

	if err := s.requireKey(); err != nil {
		return err
	}

	secret, err := decryptTOTPSecret(s.key, encrypted)
	if err != nil {
		return err
	}

	valid, err := validateTOTP(secret, code)
	if err != nil {
		return err
	}
	if !valid {
		return ErrTOTPInvalid
	}

	return nil
}

func (s *TOTPService) requireKey() error {
	if s == nil {
		return ErrTOTPKeyMissing
	}
	if s.keyErr != nil {
		return s.keyErr
	}
	if len(s.key) != totpEncryptionBytes {
		return ErrTOTPKeyInvalid
	}
	return nil
}

func loadTOTPKey() ([]byte, error) {
	raw := strings.TrimSpace(os.Getenv(totpEncryptionEnv))
	if raw == "" {
		return nil, ErrTOTPKeyMissing
	}

	key, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, ErrTOTPKeyInvalid
	}
	if len(key) != totpEncryptionBytes {
		return nil, ErrTOTPKeyInvalid
	}

	return key, nil
}

func encryptTOTPSecret(key []byte, secret string) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, ErrTOTPSecretEncryption
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, ErrTOTPSecretEncryption
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, ErrTOTPSecretEncryption
	}

	ciphertext := gcm.Seal(nil, nonce, []byte(secret), nil)
	return append(nonce, ciphertext...), nil
}

func decryptTOTPSecret(key []byte, encrypted []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", ErrTOTPSecretCorrupted
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", ErrTOTPSecretCorrupted
	}

	if len(encrypted) < gcm.NonceSize() {
		return "", ErrTOTPSecretCorrupted
	}

	nonce := encrypted[:gcm.NonceSize()]
	ciphertext := encrypted[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", ErrTOTPSecretCorrupted
	}

	return string(plaintext), nil
}

func validateTOTP(secret, code string) (bool, error) {
	return totp.ValidateCustom(code, secret, time.Now().UTC(), totp.ValidateOpts{
		Period:    30,
		Skew:      1,
		Digits:    otp.DigitsSix,
		Algorithm: otp.AlgorithmSHA1,
	})
}

// GenerateBackupCodes creates a set of backup codes for MFA enrollment.
func GenerateBackupCodes() ([]string, error) {
	codes := make([]string, 0, totpBackupCodeCount)
	max := int64(1)
	for i := 0; i < totpBackupCodeDigits; i++ {
		max *= 10
	}
	for i := 0; i < totpBackupCodeCount; i++ {
		n, err := rand.Int(rand.Reader, big.NewInt(max))
		if err != nil {
			return nil, fmt.Errorf("failed to generate backup code: %w", err)
		}
		raw := fmt.Sprintf("%0*d", totpBackupCodeDigits, n.Int64())
		code := raw
		if len(raw) > 4 {
			code = fmt.Sprintf("%s-%s", raw[:4], raw[4:])
		}
		codes = append(codes, code)
	}
	return codes, nil
}

func storeBackupCodes(ctx context.Context, execer totpExecutor, userID uuid.UUID, codes []string) error {
	if len(codes) == 0 {
		return fmt.Errorf("backup codes are required")
	}

	if _, err := execer.ExecContext(ctx, `DELETE FROM mfa_backup_codes WHERE user_id = $1`, userID); err != nil {
		return fmt.Errorf("failed to clear backup codes: %w", err)
	}

	for _, code := range codes {
		normalized := normalizeBackupCode(code)
		if normalized == "" {
			continue
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(normalized), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("failed to hash backup code: %w", err)
		}
		if _, err := execer.ExecContext(ctx, `
			INSERT INTO mfa_backup_codes (id, user_id, code_hash, created_at)
			VALUES ($1, $2, $3, now())
		`, uuid.New(), userID, string(hash)); err != nil {
			return fmt.Errorf("failed to store backup code: %w", err)
		}
	}

	return nil
}

func (s *TOTPService) consumeBackupCode(ctx context.Context, userID uuid.UUID, code string) (bool, error) {
	normalized := normalizeBackupCode(code)
	if normalized == "" {
		return false, nil
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, code_hash
		FROM mfa_backup_codes
		WHERE user_id = $1 AND used_at IS NULL
	`, userID)
	if err != nil {
		return false, fmt.Errorf("failed to load backup codes: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var id uuid.UUID
		var hash string
		if err := rows.Scan(&id, &hash); err != nil {
			return false, fmt.Errorf("failed to parse backup code: %w", err)
		}
		if bcrypt.CompareHashAndPassword([]byte(hash), []byte(normalized)) == nil {
			if _, err := s.db.ExecContext(ctx, `
				UPDATE mfa_backup_codes
				SET used_at = now()
				WHERE id = $1 AND used_at IS NULL
			`, id); err != nil {
				return false, fmt.Errorf("failed to mark backup code used: %w", err)
			}
			return true, nil
		}
	}

	if err := rows.Err(); err != nil {
		return false, fmt.Errorf("failed to scan backup codes: %w", err)
	}

	return false, nil
}

func normalizeBackupCode(code string) string {
	trimmed := strings.TrimSpace(code)
	trimmed = strings.ReplaceAll(trimmed, "-", "")
	trimmed = strings.ReplaceAll(trimmed, " ", "")
	return trimmed
}
