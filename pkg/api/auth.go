package api

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gin-contrib/sessions"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nais/knorten/pkg/auth"
)

const (
	RedirectURICookie = "redirecturi"
	OAuthStateCookie  = "oauthstate"
	sessionCookie     = "knorten_session"
	tokenLength       = 32
	sessionLength     = 7 * time.Hour
)

func (a *API) login(c *gin.Context) string {
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

	return a.azureClient.AuthCodeURL(oauthState)
}

func (a *API) callback(c *gin.Context) (string, error) {
	host, _, err := net.SplitHostPort(c.Request.Host)
	if err != nil {
		host = c.Request.Host
	}
	loginPage := "/user"

	redirectURI, _ := c.Cookie(RedirectURICookie)
	if redirectURI != "" {
		loginPage = redirectURI
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

	tokens, err := a.azureClient.Exchange(c.Request.Context(), code)
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
	_, err = a.azureClient.Verify(c.Request.Context(), rawIDToken)
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

func (a *API) logout(c *gin.Context) (string, error) {
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
		a.log.WithError(err).Error("failed deleting session")
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

func (a *API) authMiddleware(allowedUsers []string) gin.HandlerFunc {
	if a.dryRun {
		return func(c *gin.Context) {
			user := &auth.User{
				Name:    "dummy@nav.no",
				Email:   "dummy@nav.no",
				Expires: time.Time{},
			}
			c.Set("user", user)
			c.Next()
		}
	}

	certificates, err := a.azureClient.FetchCertificates()
	if err != nil {
		a.log.Fatalf("Fetching signing certificates from IdP: %v", err)
	}

	return func(c *gin.Context) {
		sessionToken, err := c.Cookie(sessionCookie)
		if err != nil {
			c.Redirect(http.StatusFound, "/oauth2/login")
			return
		}

		session, err := a.repo.SessionGet(c, sessionToken)
		if err != nil || errors.Is(err, sql.ErrNoRows) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		user, err := a.azureClient.ValidateUser(certificates, session.AccessToken)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		if len(allowedUsers) > 0 {
			allowed := false
			for _, allowedUser := range allowedUsers {
				if user.Email == allowedUser {
					allowed = true
					break
				}
			}

			if !allowed {
				session := sessions.Default(c)
				session.AddFlash(fmt.Errorf("%v is not authorized", user.Email))
				err := session.Save()
				if err != nil {
					a.log.WithError(err).Error("problem saving session")
					c.Redirect(http.StatusSeeOther, "/")
					return
				}
				c.Redirect(http.StatusUnauthorized, "/")
				return
			}
		}

		c.Set("user", user)
		c.Next()
	}
}

func (a *API) setupAuthRoutes() {
	a.router.GET("/oauth2/login", func(c *gin.Context) {
		if a.dryRun {
			c.Redirect(http.StatusSeeOther, "http://localhost:8080/user")
		}

		consentURL := a.login(c)
		c.Redirect(http.StatusSeeOther, consentURL)
	})

	a.router.GET("/oauth2/callback", func(c *gin.Context) {
		redirectURL, err := a.callback(c)
		if err != nil {
			session := sessions.Default(c)
			session.AddFlash(err.Error())
			err := session.Save()
			if err != nil {
				a.log.WithError(err).Error("problem saving session")
				c.Redirect(http.StatusSeeOther, "/")
				return
			}
			c.Redirect(http.StatusSeeOther, "/")
			return
		}

		c.Redirect(http.StatusSeeOther, redirectURL)
	})

	a.router.GET("/oauth2/logout", func(c *gin.Context) {
		redirectURL, err := a.logout(c)
		if err != nil {
			session := sessions.Default(c)
			session.AddFlash(err.Error())
			err := session.Save()
			if err != nil {
				a.log.WithError(err).Error("problem saving session")
				c.Redirect(http.StatusSeeOther, "/")
				return
			}
			c.Redirect(http.StatusSeeOther, "/")
			return
		}
		c.Redirect(http.StatusSeeOther, redirectURL)
	})
}
