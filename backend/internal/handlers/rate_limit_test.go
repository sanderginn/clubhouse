package handlers

import "context"

type stubContentRateLimiter struct {
	allowed bool
	err     error
	called  bool
	key     string
}

func (s *stubContentRateLimiter) Allow(_ context.Context, key string) (bool, error) {
	s.called = true
	s.key = key
	return s.allowed, s.err
}
