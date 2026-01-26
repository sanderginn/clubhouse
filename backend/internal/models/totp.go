package models

// TOTPEnrollResponse represents the response from starting TOTP enrollment.
type TOTPEnrollResponse struct {
	Secret     string `json:"secret"`
	OtpAuthURL string `json:"otpauth_url"`
	Message    string `json:"message"`
}

// TOTPVerifyRequest represents the request to verify a TOTP code.
type TOTPVerifyRequest struct {
	Code string `json:"code"`
}

// TOTPVerifyResponse represents the response from verifying TOTP.
type TOTPVerifyResponse struct {
	Message string `json:"message"`
}
