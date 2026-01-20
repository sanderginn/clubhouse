package services

import "errors"

// Sentinel errors for service layer
var (
	ErrPostNotFound    = errors.New("post not found")
	ErrCommentNotFound = errors.New("comment not found")
)
