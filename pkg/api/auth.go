package api

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nais/knorten/pkg/auth"
)

const (
	RedirectURICookie               = "redirecturi"
	OAuthStateCookie                = "oauthstate"
	sessionCookie                   = "knorten_session"
	tokenLength                     = 32
	sessionLength     time.Duration = 7 * time.Hour
)

func (a *API) Login(c *gin.Context) string {
	host, _, err := net.SplitHostPort(c.Request.Host)
	if err != nil {
		host = c.Request.Host
	}

	redirectURI := c.Request.URL.Query().Get("redirect_uri")
	c.SetCookie(
		RedirectURICookie,
		redirectURI,
		time.Now().Add(30*time.Minute).Second(),
		"/",
		host,
		true,
		true,
	)

	oauthState := uuid.New().String()
	c.SetCookie(
		OAuthStateCookie,
		oauthState,
		time.Now().Add(30*time.Minute).Second(),
		"/",
		host,
		true,
		true,
	)

	return a.oauth2.AuthCodeURL(oauthState)
}

func (a *API) Callback(c *gin.Context) (string, error) {
	host, _, err := net.SplitHostPort(c.Request.Host)
	if err != nil {
		host = c.Request.Host
	}
	loginPage := "/user"

	redirectURI, err := c.Cookie(RedirectURICookie)
	if err == nil {
		loginPage = loginPage + strings.TrimPrefix(redirectURI, "/")
	}

	if strings.HasPrefix(c.Request.Host, "localhost") {
		loginPage = "http://localhost:8080" + loginPage
	}

	deleteCookie(c, RedirectURICookie, host)
	code := c.Request.URL.Query().Get("code")
	if len(code) == 0 {
		return loginPage + "?error=unauthenticated", errors.New("unauthenticated")
	}

	oauthCookie, err := c.Cookie(OAuthStateCookie)
	if err != nil {
		a.log.Errorf("Missing oauth state cookie: %v", err)
		return loginPage + "?error=invalid-state", errors.New("invalid state")
	}

	deleteCookie(c, OAuthStateCookie, host)

	state := c.Request.URL.Query().Get("state")
	if state != oauthCookie {
		a.log.Info("Incoming state does not match local state")
		return loginPage + "?error=invalid-state", errors.New("invalid state")
	}

	tokens, err := a.oauth2.Exchange(c.Request.Context(), code)
	if err != nil {
		a.log.Errorf("Exchanging authorization code for tokens: %v", err)
		return loginPage + "?error=invalid-state", errors.New("forbidden")
	}

	rawIDToken, ok := tokens.Extra("id_token").(string)
	if !ok {
		a.log.Info("Missing id_token")
		return loginPage + "?error=unauthenticated", errors.New("unauthenticated")
	}

	// Parse and verify ID Token payload.
	_, err = a.oauth2.Verify(c.Request.Context(), rawIDToken)
	if err != nil {
		a.log.Info("Invalid id_token")
		return loginPage + "?error=unauthenticated", errors.New("unauthenticated")
	}

	session := &auth.Session{
		Token:       generateSecureToken(tokenLength),
		Expires:     time.Now().Add(sessionLength),
		AccessToken: tokens.AccessToken,
	}

	b, err := base64.RawStdEncoding.DecodeString(strings.Split(tokens.AccessToken, ".")[1])
	if err != nil {
		a.log.WithError(err).Error("unable decode access token")
		return loginPage + "?error=unauthenticated", errors.New("unauthenticated")
	}

	if err := json.Unmarshal(b, session); err != nil {
		a.log.WithError(err).Error("unable unmarshalling token")
		return loginPage + "?error=unauthenticated", errors.New("unauthenticated")
	}

	if err := a.repo.SessionCreate(c, session); err != nil {
		a.log.WithError(err).Error("unable to create session")
		return loginPage + "?error=internal-server-error", errors.New("unable to create session")
	}

	c.SetCookie(
		sessionCookie,
		session.Token,
		86400,
		"/",
		host,
		true,
		true,
	)

	return loginPage, nil
}

func (a *API) Logout(c *gin.Context) (string, error) {
	host, _, err := net.SplitHostPort(c.Request.Host)
	if err != nil {
		host = c.Request.Host
	}

	deleteCookie(c, sessionCookie, host)

	var loginPage string
	if strings.HasPrefix(c.Request.Host, "localhost") {
		loginPage = "http://localhost:8080/"
	} else {
		loginPage = "/"
	}

	err = a.repo.SessionDelete(c, sessionCookie)
	if err != nil {
		fmt.Println(err)
		return loginPage, err
	}

	return loginPage, nil
}

func generateSecureToken(length int) string {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return hex.EncodeToString(b)
}

func deleteCookie(c *gin.Context, name, host string) {
	c.SetCookie(
		name,
		"",
		time.Unix(0, 0).Second(),
		"/",
		host,
		true,
		true,
	)
}

func (a *API) authMiddleware() gin.HandlerFunc {
	certificates, err := a.oauth2.FetchCertificates()
	if err != nil {
		a.log.Fatalf("Fetching signing certificates from IdP: %v", err)
	}

	return func(c *gin.Context) {
		sessionToken, err := c.Cookie(sessionCookie)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}

		session, err := a.repo.SessionGet(c, sessionToken)
		if err != nil || errors.Is(err, sql.ErrNoRows) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		user, err := a.oauth2.ValidateUser(certificates, session.AccessToken)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		c.Set("user", user)
		c.Next()
	}
}
