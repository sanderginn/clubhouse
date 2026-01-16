package services

import (
	"testing"

	"github.com/sanderginn/clubhouse/internal/models"
)

func TestValidateRegisterInput(t *testing.T) {
	tests := []struct {
		name    string
		req     *models.RegisterRequest
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid registration",
			req: &models.RegisterRequest{
				Username: "testuser",
				Email:    "test@example.com",
				Password: "Password123",
			},
			wantErr: false,
		},
		{
			name: "empty username",
			req: &models.RegisterRequest{
				Username: "",
				Email:    "test@example.com",
				Password: "Password123",
			},
			wantErr: true,
			errMsg:  "username is required",
		},
		{
			name: "username too short",
			req: &models.RegisterRequest{
				Username: "ab",
				Email:    "test@example.com",
				Password: "Password123",
			},
			wantErr: true,
			errMsg:  "username must be between 3 and 50 characters",
		},
		{
			name: "invalid username characters",
			req: &models.RegisterRequest{
				Username: "test user",
				Email:    "test@example.com",
				Password: "Password123",
			},
			wantErr: true,
			errMsg:  "username can only contain alphanumeric characters and underscores",
		},
		{
			name: "empty email",
			req: &models.RegisterRequest{
				Username: "testuser",
				Email:    "",
				Password: "Password123",
			},
			wantErr: true,
			errMsg:  "email is required",
		},
		{
			name: "invalid email",
			req: &models.RegisterRequest{
				Username: "testuser",
				Email:    "notanemail",
				Password: "Password123",
			},
			wantErr: true,
			errMsg:  "invalid email format",
		},
		{
			name: "password too short",
			req: &models.RegisterRequest{
				Username: "testuser",
				Email:    "test@example.com",
				Password: "Pass12",
			},
			wantErr: true,
			errMsg:  "password must be at least 8 characters",
		},
		{
			name: "weak password - no uppercase",
			req: &models.RegisterRequest{
				Username: "testuser",
				Email:    "test@example.com",
				Password: "password123",
			},
			wantErr: true,
			errMsg:  "password must contain uppercase, lowercase, and numeric characters",
		},
		{
			name: "weak password - no lowercase",
			req: &models.RegisterRequest{
				Username: "testuser",
				Email:    "test@example.com",
				Password: "PASSWORD123",
			},
			wantErr: true,
			errMsg:  "password must contain uppercase, lowercase, and numeric characters",
		},
		{
			name: "weak password - no digits",
			req: &models.RegisterRequest{
				Username: "testuser",
				Email:    "test@example.com",
				Password: "PasswordABC",
			},
			wantErr: true,
			errMsg:  "password must contain uppercase, lowercase, and numeric characters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRegisterInput(tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateRegisterInput() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && tt.errMsg != "" && err.Error() != tt.errMsg {
				t.Errorf("validateRegisterInput() error = %v, want %v", err.Error(), tt.errMsg)
			}
		})
	}
}

func TestIsValidUsername(t *testing.T) {
	tests := []struct {
		name     string
		username string
		want     bool
	}{
		{"valid alphanumeric", "user123", true},
		{"valid with underscore", "user_name", true},
		{"valid mixed", "User_Name_123", true},
		{"invalid space", "user name", false},
		{"invalid dash", "user-name", false},
		{"invalid special char", "user@name", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isValidUsername(tt.username); got != tt.want {
				t.Errorf("isValidUsername(%s) = %v, want %v", tt.username, got, tt.want)
			}
		})
	}
}

func TestIsValidEmail(t *testing.T) {
	tests := []struct {
		name  string
		email string
		want  bool
	}{
		{"valid email", "test@example.com", true},
		{"valid email complex", "user.name+tag@example.co.uk", true},
		{"invalid no at", "testexample.com", false},
		{"invalid no domain", "test@", false},
		{"invalid no local", "@example.com", false},
		{"invalid no tld", "test@example", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isValidEmail(tt.email); got != tt.want {
				t.Errorf("isValidEmail(%s) = %v, want %v", tt.email, got, tt.want)
			}
		})
	}
}

func TestIsStrongPassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		want     bool
	}{
		{"strong password", "Password123", true},
		{"no uppercase", "password123", false},
		{"no lowercase", "PASSWORD123", false},
		{"no digit", "PasswordAbc", false},
		{"all requirements met long", "StrongPass123", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isStrongPassword(tt.password); got != tt.want {
				t.Errorf("isStrongPassword(%s) = %v, want %v", tt.password, got, tt.want)
			}
		})
	}
}
