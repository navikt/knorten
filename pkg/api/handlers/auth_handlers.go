package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/nais/knorten/pkg/api/service"

	"github.com/gin-contrib/sessions"

	"github.com/google/uuid"

	"github.com/nais/knorten/pkg/database"
	"github.com/sirupsen/logrus"

	"github.com/nais/knorten/pkg/api/auth"

	"github.com/gin-gonic/gin"
	"github.com/nais/knorten/pkg/config"
)

const (
	tokenLength   = 32
	sessionLength = 1 * time.Hour
)

type AuthHandler struct {
	authService service.AuthService
	cookies     config.Cookies
	log         *logrus.Entry
	repo        *database.Repo // FIXME: This should not be here
	loginPage   string
}

func NewAuthHandler(authService service.AuthService, loginPage string, cookies config.Cookies, log *logrus.Entry, repo *database.Repo) *AuthHandler {
	return &AuthHandler{
		authService: authService,
		cookies:     cookies,
		log:         log,
		loginPage:   loginPage,
		repo:        repo,
	}
}

func (h *AuthHandler) LogoutHandler() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		deleteCookie(ctx, h.cookies.Session.Name, h.cookies.Session.Domain, h.cookies.Session.Path)

		// FIXME: Something seems wrong here. I don't think this h.cookies.Session.Name usage is correct
		// FIXME: Seems like we should delete the token here, so we should get the token from the cookie first?
		// Looking at the old code, we should be deleting the session from the database based on the retrieved
		// cookie
		err := h.authService.DeleteSession(ctx.Request.Context(), h.cookies.Session.Name)
		if err != nil {
			session := sessions.Default(ctx)
			session.AddFlash(err.Error())
			err := session.Save()
			if err != nil {
				h.log.WithError(err).Error("problem saving session")
				ctx.Redirect(http.StatusSeeOther, "/")
				return
			}
			ctx.Redirect(http.StatusSeeOther, "/")

			return
		}
		ctx.Redirect(http.StatusSeeOther, h.loginPage)
	}
}

func (h *AuthHandler) CallbackHandler() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		redirectURL, err := h.callback(ctx)
		if err != nil {
			session := sessions.Default(ctx)
			session.AddFlash(err.Error())
			err := session.Save()
			if err != nil {
				h.log.WithError(err).Error("problem saving session")
				ctx.Redirect(http.StatusSeeOther, "/")
				return
			}
			ctx.Redirect(http.StatusSeeOther, "/")
			return
		}

		ctx.Redirect(http.StatusSeeOther, redirectURL)
	}
}

func (h *AuthHandler) callback(ctx *gin.Context) (string, error) {
	// FIXME: How is loginPage meant to be used here?
	loginPage := "/oversikt"

	redirectURI, _ := ctx.Cookie(h.cookies.Redirect.Name)
	if redirectURI != "" {
		loginPage = redirectURI
	}

	if strings.HasPrefix(ctx.Request.Host, "localhost") {
		loginPage = "http://localhost:8080" + loginPage
	}

	deleteCookie(ctx, h.cookies.Redirect.Name, h.cookies.Redirect.Path, h.cookies.Redirect.Domain)

	code := ctx.Request.URL.Query().Get("code")
	if len(code) == 0 {
		return loginPage + "?error=unauthenticated", errors.New("unauthenticated")
	}

	oauthCookie, err := ctx.Cookie(h.cookies.OauthState.Name)
	if err != nil {
		h.log.Infof("Missing oauth state cookie: %v", err)
		return loginPage + "?error=invalid-state", errors.New("invalid state")
	}

	deleteCookie(ctx, h.cookies.OauthState.Name, h.cookies.OauthState.Path, h.cookies.OauthState.Domain)

	state := ctx.Request.URL.Query().Get("state")
	if state != oauthCookie {
		h.log.Info("Incoming state does not match local state")
		return loginPage + "?error=invalid-state", errors.New("invalid state")
	}

	sess, err := h.authService.CreateSession(ctx.Request.Context(), code)
	if err != nil {
		h.log.WithError(err).Error("unable to create session")

		return loginPage + "?error=unauthenticated", fmt.Errorf("unable to create session: %w", err)
	}

	ctx.SetCookie(
		h.cookies.Session.Name,
		sess.Token,
		h.cookies.Session.MaxAge,
		h.cookies.Session.Path,
		h.cookies.Session.Domain,
		h.cookies.Session.Secure,
		h.cookies.Session.HttpOnly,
	)

	return loginPage, nil
}

func deleteCookie(ctx *gin.Context, name, host, path string) {
	ctx.SetCookie(name, "", -1, path, host, true, true)
}

func (h *AuthHandler) LoginHandler(dryRun bool) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		// FIXME: We should not propagate dryRun, but rather do this by injecting a
		//  different service, e.g., a mock service, and by passing in a different
		// cookie configuration, perhaps?
		if dryRun {
			if err := h.createDryRunSession(ctx); err != nil {
				h.log.Error("creating dryrun session")
			}

			ctx.Redirect(http.StatusSeeOther, "http://localhost:8080/oversikt")

			return
		}

		ctx.SetSameSite(h.cookies.Redirect.GetSameSite())
		ctx.SetCookie(
			h.cookies.Redirect.Name,
			ctx.Request.URL.Query().Get("redirect_uri"),
			h.cookies.Redirect.MaxAge,
			h.cookies.Redirect.Path,
			h.cookies.Redirect.Domain,
			h.cookies.Redirect.Secure,
			h.cookies.Redirect.HttpOnly,
		)

		oauthState := uuid.New().String()
		ctx.SetSameSite(h.cookies.OauthState.GetSameSite())
		ctx.SetCookie(
			h.cookies.OauthState.Name,
			oauthState,
			h.cookies.OauthState.MaxAge,
			h.cookies.OauthState.Path,
			h.cookies.OauthState.Domain,
			h.cookies.OauthState.Secure,
			h.cookies.OauthState.HttpOnly,
		)

		ctx.Redirect(http.StatusSeeOther, h.authService.GetLoginURL(oauthState))
	}
}

// FIXME: Need to get rid of this
func (h *AuthHandler) createDryRunSession(ctx *gin.Context) error {
	session := &auth.Session{
		Token:       generateSecureToken(tokenLength),
		Expires:     time.Now().Add(sessionLength),
		AccessToken: "",
		IsAdmin:     true,
	}

	if err := h.repo.SessionCreate(ctx, session); err != nil {
		h.log.WithError(err).Error("unable to create session")
		return errors.New("unable to create session")
	}

	ctx.SetCookie(
		h.cookies.Session.Name,
		session.Token,
		h.cookies.Session.MaxAge,
		h.cookies.Session.Path,
		h.cookies.Session.Domain,
		h.cookies.Session.Secure,
		h.cookies.Session.HttpOnly,
	)

	return nil
}

// a little bit of copy is better than a little bit of depdency
func generateSecureToken(length int) string {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return hex.EncodeToString(b)
}
