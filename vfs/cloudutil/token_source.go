package cloudutil

import (
	"context"
	"fmt"
	"strings"
)

// AccessTokenSource returns a valid bearer token for cloud-backed VFS adapters.
type AccessTokenSource interface {
	AccessToken(context.Context) (string, error)
}

// StaticAccessTokenSource always returns the same access token.
type StaticAccessTokenSource struct {
	token string
}

// NewStaticAccessTokenSource validates and wraps a fixed bearer token.
func NewStaticAccessTokenSource(token string) (*StaticAccessTokenSource, error) {
	trimmed := strings.TrimSpace(token)
	if trimmed == "" {
		return nil, fmt.Errorf("cloudutil: access token is required")
	}
	return &StaticAccessTokenSource{token: trimmed}, nil
}

// AccessToken implements AccessTokenSource.
func (s *StaticAccessTokenSource) AccessToken(context.Context) (string, error) {
	if s == nil || strings.TrimSpace(s.token) == "" {
		return "", fmt.Errorf("cloudutil: access token is required")
	}
	return s.token, nil
}
