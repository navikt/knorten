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

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"github.com/nais/knorten/pkg/api/auth"
	"k8s.io/utils/strings/slices"
)

const (
	RedirectURICookie = "redirecturi"
	OAuthStateCookie  = "oauthstate"
	sessionCookie     = "knorten_session"
	tokenLength       = 32
	sessionLength     = 1 * time.Hour
)

func (c *client) login(ctx *gin.Context) string {
	host, _, err := net.SplitHostPort(ctx.Request.Host)
	if err != nil {
		host = ctx.Request.Host
	}

	redirectURI := ctx.Request.URL.Query().Get("redirect_uri")
	ctx.SetCookie(
		RedirectURICookie,
		redirectURI,
		time.Now().Add(30*time.Minute).Second(),
		"/",
		host,
		true,
		true,
	)

	oauthState := uuid.New().String()
	ctx.SetCookie(
		OAuthStateCookie,
		oauthState,
		time.Now().Add(30*time.Minute).Second(),
		"/",
		host,
		true,
		true,
	)

	return c.azureClient.AuthCodeURL(oauthState)
}

func (c *client) callback(ctx *gin.Context) (string, error) {
	host, _, err := net.SplitHostPort(ctx.Request.Host)
	if err != nil {
		host = ctx.Request.Host
	}
	loginPage := "/oversikt"

	redirectURI, _ := ctx.Cookie(RedirectURICookie)
	if redirectURI != "" {
		loginPage = redirectURI
	}

	if strings.HasPrefix(ctx.Request.Host, "localhost") {
		loginPage = "http://localhost:8080" + loginPage
	}

	deleteCookie(ctx, RedirectURICookie, host)
	code := ctx.Request.URL.Query().Get("code")
	if len(code) == 0 {
		return loginPage + "?error=unauthenticated", errors.New("unauthenticated")
	}

	oauthCookie, err := ctx.Cookie(OAuthStateCookie)
	if err != nil {
		c.log.Errorf("Missing oauth state cookie: %v", err)
		return loginPage + "?error=invalid-state", errors.New("invalid state")
	}

	deleteCookie(ctx, OAuthStateCookie, host)

	state := ctx.Request.URL.Query().Get("state")
	if state != oauthCookie {
		c.log.Info("Incoming state does not match local state")
		return loginPage + "?error=invalid-state", errors.New("invalid state")
	}

	tokens, err := c.azureClient.Exchange(ctx.Request.Context(), code)
	if err != nil {
		c.log.Errorf("Exchanging authorization code for tokens: %v", err)
		return loginPage + "?error=invalid-state", errors.New("forbidden")
	}

	rawIDToken, ok := tokens.Extra("id_token").(string)
	if !ok {
		c.log.Info("Missing id_token")
		return loginPage + "?error=unauthenticated", errors.New("unauthenticated")
	}

	// Parse and verify ID Token payload.
	_, err = c.azureClient.Verify(ctx.Request.Context(), rawIDToken)
	if err != nil {
		c.log.Info("Invalid id_token")
		return loginPage + "?error=unauthenticated", errors.New("unauthenticated")
	}

	session := &auth.Session{
		Token:       generateSecureToken(tokenLength),
		Expires:     time.Now().Add(sessionLength),
		AccessToken: tokens.AccessToken,
	}

	b, err := base64.RawStdEncoding.DecodeString(strings.Split(tokens.AccessToken, ".")[1])
	if err != nil {
		c.log.WithError(err).Error("unable decode access token")
		return loginPage + "?error=unauthenticated", errors.New("unauthenticated")
	}

	if err := json.Unmarshal(b, session); err != nil {
		c.log.WithError(err).Error("unable unmarshalling token")
		return loginPage + "?error=unauthenticated", errors.New("unauthenticated")
	}

	session.IsAdmin = c.isUserInAdminGroup(session.AccessToken)

	if err := c.repo.SessionCreate(ctx, session); err != nil {
		c.log.WithError(err).Error("unable to create session")
		return loginPage + "?error=internal-server-error", errors.New("unable to create session")
	}

	ctx.SetCookie(
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

func (c *client) logout(ctx *gin.Context) (string, error) {
	host, _, err := net.SplitHostPort(ctx.Request.Host)
	if err != nil {
		host = ctx.Request.Host
	}

	deleteCookie(ctx, sessionCookie, host)

	var loginPage string
	if strings.HasPrefix(ctx.Request.Host, "localhost") {
		loginPage = "http://localhost:8080/"
	} else {
		loginPage = "/"
	}

	err = c.repo.SessionDelete(ctx, sessionCookie)
	if err != nil {
		c.log.WithError(err).Error("failed deleting session")
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

func deleteCookie(ctx *gin.Context, name, host string) {
	ctx.SetCookie(
		name,
		"",
		time.Unix(0, 0).Second(),
		"/",
		host,
		true,
		true,
	)
}

func (c *client) authMiddleware() gin.HandlerFunc {
	if c.dryRun {
		return func(ctx *gin.Context) {
			user := &auth.User{
				Name:    "Dum My",
				Email:   "dummy@nav.no",
				Expires: time.Time{},
			}
			ctx.Set("user", user)
			ctx.Next()
		}
	}

	certificates, err := c.azureClient.FetchCertificates()
	if err != nil {
		c.log.Fatalf("Fetching signing certificates from IdP: %v", err)
	}

	return func(ctx *gin.Context) {
		sessionToken, err := ctx.Cookie(sessionCookie)
		if err != nil {
			ctx.Redirect(http.StatusSeeOther, "/oauth2/login")
			return
		}

		session, err := c.repo.SessionGet(ctx, sessionToken)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				ctx.Redirect(http.StatusSeeOther, "/oauth2/login")
				return
			}
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		user, err := c.azureClient.ValidateUser(certificates, session.AccessToken)
		if err != nil {
			if errors.Is(err, auth.ErrAzureTokenExpired) {
				ctx.Redirect(http.StatusSeeOther, "/oauth2/login")
				return
			}
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized validate user"})
			return
		}

		teamSlug := ctx.Param("slug")
		if teamSlug != "" {
			team, err := c.repo.TeamBySlugGet(ctx, teamSlug)
			if err != nil {
				c.log.WithError(err).Errorf("problem checking for authorization %v", user.Email)
				ctx.Redirect(http.StatusSeeOther, "/")
				return
			}

			if !slices.Contains(team.Users, strings.ToLower(user.Email)) {
				sess := sessions.Default(ctx)
				sess.AddFlash(fmt.Sprintf("%v is not authorized", user.Email))
				err = sess.Save()
				if err != nil {
					c.log.WithError(err).Error("problem saving session")
					ctx.Redirect(http.StatusSeeOther, "/")
					return
				}
				ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": fmt.Sprintf("%v is not part of team %v", user.Email, teamSlug)})
				return
			}
		}

		ctx.Set("user", user)
		ctx.Next()
	}
}

func (c *client) adminAuthMiddleware() gin.HandlerFunc {
	if c.dryRun {
		return func(ctx *gin.Context) {
			user := &auth.User{
				Name:    "Dum My",
				Email:   "dummy@nav.no",
				Expires: time.Time{},
			}
			ctx.Set("user", user)
			ctx.Next()
		}
	}
	return func(ctx *gin.Context) {
		if !c.isAdmin(ctx) {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		}

		ctx.Next()
	}
}

func (c *client) isUserInAdminGroup(token string) bool {
	var claims jwt.MapClaims

	certificates, err := c.azureClient.FetchCertificates()
	if err != nil {
		c.log.WithError(err).Error("fetch certificates")
		return false
	}

	jwtValidator := auth.JWTValidator(certificates, c.azureClient.ClientID)

	_, err = jwt.ParseWithClaims(token, &claims, jwtValidator)

	if err != nil {
		c.log.WithError(err).Error("Parse token")
		return false
	}

	if claims["groups"] != nil {
		groups, ok := claims["groups"].([]interface{})
		if !ok {
			c.log.Logger.Error("User does not have groups in claims")
			return false
		}
		for _, group := range groups {
			grp, ok := group.(string)
			if ok {
				if grp == c.adminGroupID {
					return true
				}
			}
		}
	}
	return false
}

func (c *client) setupAuthRoutes() {
	c.router.GET("/oauth2/login", func(ctx *gin.Context) {
		if c.dryRun {
			if err := c.createDryRunSession(ctx); err != nil {
				c.log.Error("creating dryrun session")
			}
			ctx.Redirect(http.StatusSeeOther, "http://localhost:8080/oversikt")
			return
		}

		consentURL := c.login(ctx)
		ctx.Redirect(http.StatusSeeOther, consentURL)
	})

	c.router.GET("/oauth2/callback", func(ctx *gin.Context) {
		redirectURL, err := c.callback(ctx)
		if err != nil {
			session := sessions.Default(ctx)
			session.AddFlash(err.Error())
			err := session.Save()
			if err != nil {
				c.log.WithError(err).Error("problem saving session")
				ctx.Redirect(http.StatusSeeOther, "/")
				return
			}
			ctx.Redirect(http.StatusSeeOther, "/")
			return
		}

		ctx.Redirect(http.StatusSeeOther, redirectURL)
	})

	c.router.GET("/oauth2/logout", func(ctx *gin.Context) {
		redirectURL, err := c.logout(ctx)
		if err != nil {
			session := sessions.Default(ctx)
			session.AddFlash(err.Error())
			err := session.Save()
			if err != nil {
				c.log.WithError(err).Error("problem saving session")
				ctx.Redirect(http.StatusSeeOther, "/")
				return
			}
			ctx.Redirect(http.StatusSeeOther, "/")
			return
		}
		ctx.Redirect(http.StatusSeeOther, redirectURL)
	})
}

func (c *client) createDryRunSession(ctx *gin.Context) error {
	session := &auth.Session{
		Token:       generateSecureToken(tokenLength),
		Expires:     time.Now().Add(sessionLength),
		AccessToken: "",
		IsAdmin:     true,
	}

	if err := c.repo.SessionCreate(ctx, session); err != nil {
		c.log.WithError(err).Error("unable to create session")
		return errors.New("unable to create session")
	}

	ctx.SetCookie(
		sessionCookie,
		session.Token,
		86400,
		"/",
		"localhost",
		true,
		true,
	)

	return nil
}
