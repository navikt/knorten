package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/nais/knorten/pkg/database"

	"github.com/golang-jwt/jwt/v4"
	"github.com/nais/knorten/pkg/api/auth"
)

type AuthService interface {
	GetLoginURL(state string) string
	CreateSession(ctx context.Context, code string) (*auth.Session, error)
	DeleteSession(ctx context.Context, token string) error
}

type authService struct {
	azureClient   *auth.Azure
	tokenLength   int
	sessionLength time.Duration
	adminGroupID  string
	repo          *database.Repo // FIXME: Should be an interface authRepo, but we don't have that yet
}

func (s *authService) DeleteSession(ctx context.Context, token string) error {
	err := s.repo.SessionDelete(ctx, token)
	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}

	return nil
}

// FIXME: Verify that we are not trying to do too much in this method
func (s *authService) CreateSession(ctx context.Context, code string) (*auth.Session, error) {
	tokens, err := s.azureClient.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("exchange authorization code for tokens: %w", err)
	}

	rawIDToken, ok := tokens.Extra("id_token").(string)
	if !ok {
		return nil, fmt.Errorf("missing id_token")
	}

	// Parse and verify ID Token payload.
	_, err = s.azureClient.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("verify ID token: %w", err)
	}

	sess := &auth.Session{
		Token:       generateSecureToken(s.tokenLength),
		Expires:     time.Now().Add(s.sessionLength),
		AccessToken: tokens.AccessToken,
	}

	b, err := base64.RawStdEncoding.DecodeString(strings.Split(tokens.AccessToken, ".")[1])
	if err != nil {
		return nil, fmt.Errorf("decode access token: %w", err)
	}

	if err := json.Unmarshal(b, sess); err != nil {
		return nil, fmt.Errorf("unmarshal access token: %w", err)
	}

	sess.IsAdmin, err = s.isUserInAdminGroup(sess.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("check if user is in admin group: %w", err)
	}

	err = s.repo.SessionCreate(ctx, sess)
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}

	return sess, nil
}

func (s *authService) isUserInAdminGroup(token string) (bool, error) {
	var claims jwt.MapClaims

	certificates, err := s.azureClient.FetchCertificates()
	if err != nil {
		return false, fmt.Errorf("fetch certificates: %w", err)
	}

	jwtValidator := auth.JWTValidator(certificates, s.azureClient.ClientID)

	_, err = jwt.ParseWithClaims(token, &claims, jwtValidator)
	if err != nil {
		return false, fmt.Errorf("parse claims: %w", err)
	}

	if claims["groups"] == nil {
		return false, nil
	}

	groups, ok := claims["groups"].([]interface{})
	if !ok {
		return false, nil
	}

	for _, group := range groups {
		if grp, ok := group.(string); ok {
			if grp == s.adminGroupID {
				return true, nil
			}
		}
	}

	return false, nil
}

func (s *authService) GetLoginURL(state string) string {
	return s.azureClient.AuthCodeURL(state)
}

func NewAuthService(repo *database.Repo, adminGroupID string, sessionLength time.Duration, tokenLength int, azureClient *auth.Azure) *authService {
	return &authService{
		azureClient:   azureClient,
		tokenLength:   tokenLength,
		sessionLength: sessionLength,
		adminGroupID:  adminGroupID,
		repo:          repo,
	}
}

// a little bit of copy is better than a little bit of dependency
func generateSecureToken(length int) string {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return hex.EncodeToString(b)
}
