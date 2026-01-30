package services

import (
	"context"
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pquerna/otp/totp"
	"github.com/sanderginn/clubhouse/internal/testutil"
)

func TestVerifyLoginAcceptsBackupCode(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	keyBytes := make([]byte, 32)
	for i := range keyBytes {
		keyBytes[i] = byte(i + 1)
	}
	t.Setenv("CLUBHOUSE_TOTP_ENCRYPTION_KEY", base64.StdEncoding.EncodeToString(keyBytes))

	userID := uuid.MustParse(testutil.CreateTestUser(t, db, "backupuser", "backupuser@example.com", false, true))
	service := NewTOTPService(db)

	enrollment, err := service.EnrollUser(context.Background(), userID, "backupuser")
	if err != nil {
		t.Fatalf("EnrollUser failed: %v", err)
	}

	code, err := totp.GenerateCode(enrollment.Secret, time.Now().UTC())
	if err != nil {
		t.Fatalf("failed to generate totp code: %v", err)
	}

	backupCodes, err := GenerateBackupCodes()
	if err != nil {
		t.Fatalf("GenerateBackupCodes failed: %v", err)
	}

	if err := service.EnableUserWithBackupCodes(context.Background(), userID, code, backupCodes); err != nil {
		t.Fatalf("EnableUserWithBackupCodes failed: %v", err)
	}

	if err := service.VerifyLogin(context.Background(), userID, backupCodes[0]); err != nil {
		t.Fatalf("VerifyLogin with backup code failed: %v", err)
	}

	if err := service.VerifyLogin(context.Background(), userID, backupCodes[0]); !errors.Is(err, ErrTOTPInvalid) {
		t.Fatalf("expected backup code reuse to return ErrTOTPInvalid, got %v", err)
	}
}
