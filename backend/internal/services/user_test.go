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
				Password: "LongPassword123",
			},
			wantErr: false,
		},
		{
			name: "empty username",
			req: &models.RegisterRequest{
				Username: "",
				Email:    "test@example.com",
				Password: "LongPassword123",
			},
			wantErr: true,
			errMsg:  "username is required",
		},
		{
			name: "username too short",
			req: &models.RegisterRequest{
				Username: "ab",
				Email:    "test@example.com",
				Password: "LongPassword123",
			},
			wantErr: true,
			errMsg:  "username must be between 3 and 50 characters",
		},
		{
			name: "invalid username characters",
			req: &models.RegisterRequest{
				Username: "test user",
				Email:    "test@example.com",
				Password: "LongPassword123",
			},
			wantErr: true,
			errMsg:  "username can only contain alphanumeric characters and underscores",
		},
		{
			name: "empty email allowed",
			req: &models.RegisterRequest{
				Username: "testuser",
				Email:    "",
				Password: "LongPassword123",
			},
			wantErr: false,
		},
		{
			name: "invalid email",
			req: &models.RegisterRequest{
				Username: "testuser",
				Email:    "notanemail",
				Password: "LongPassword123",
			},
			wantErr: true,
			errMsg:  "invalid email format",
		},
		{
			name: "password too short",
			req: &models.RegisterRequest{
				Username: "testuser",
				Email:    "test@example.com",
				Password: "ShortPass1",
			},
			wantErr: true,
			errMsg:  "password must be at least 12 characters",
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

func TestValidateLoginInput(t *testing.T) {
	tests := []struct {
		name    string
		req     *models.LoginRequest
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid login",
			req: &models.LoginRequest{
				Username: "testuser",
				Password: "LongPassword123",
			},
			wantErr: false,
		},
		{
			name: "empty username",
			req: &models.LoginRequest{
				Username: "",
				Password: "LongPassword123",
			},
			wantErr: true,
			errMsg:  "username is required",
		},
		{
			name: "empty password",
			req: &models.LoginRequest{
				Username: "testuser",
				Password: "",
			},
			wantErr: true,
			errMsg:  "password is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateLoginInput(tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateLoginInput() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && tt.errMsg != "" && err.Error() != tt.errMsg {
				t.Errorf("validateLoginInput() error = %v, want %v", err.Error(), tt.errMsg)
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

func TestValidateProfilePictureURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid https URL",
			url:     "https://example.com/image.png",
			wantErr: false,
		},
		{
			name:    "valid http URL",
			url:     "http://example.com/image.jpg",
			wantErr: false,
		},
		{
			name:    "valid URL with path and query",
			url:     "https://cdn.example.com/images/avatar.png?size=256",
			wantErr: false,
		},
		{
			name:    "invalid - no scheme",
			url:     "example.com/image.png",
			wantErr: true,
			errMsg:  "profile picture URL must use http or https scheme",
		},
		{
			name:    "invalid - ftp scheme",
			url:     "ftp://example.com/image.png",
			wantErr: true,
			errMsg:  "profile picture URL must use http or https scheme",
		},
		{
			name:    "invalid - file scheme",
			url:     "file:///etc/passwd",
			wantErr: true,
			errMsg:  "profile picture URL must use http or https scheme",
		},
		{
			name:    "invalid - javascript scheme",
			url:     "javascript:alert(1)",
			wantErr: true,
			errMsg:  "profile picture URL must use http or https scheme",
		},
		{
			name:    "invalid - no host",
			url:     "https:///path",
			wantErr: true,
			errMsg:  "invalid profile picture URL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateProfilePictureURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateProfilePictureURL() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && tt.errMsg != "" && err.Error() != tt.errMsg {
				t.Errorf("validateProfilePictureURL() error = %v, want %v", err.Error(), tt.errMsg)
			}
		})
	}
}
