package iam

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"backend-infrastructure-go/internal/shared/apperror"
	"backend-infrastructure-go/internal/shared/response"
	"github.com/go-chi/chi/v5"
)

const (
	AccessCookieName  = "access_token"
	RefreshCookieName = "refresh_token"
)

type Application interface {
	Login(context.Context, string, string) (TokenPair, error)
	Refresh(context.Context, string) (TokenPair, error)
	Logout(context.Context, string) error
	CurrentUser(context.Context, string) (User, error)
	CreateUser(context.Context, CreateUserInput) (User, error)
	SetUserActive(context.Context, string, bool) error
	Roles(context.Context) ([]Role, error)
	Permissions(context.Context) ([]Permission, error)
	AssignRole(context.Context, string, string) error
	GrantPermission(context.Context, string, string) error
	Authorize(context.Context, string, string) error
}

type HTTPConfig struct {
	SecureCookies bool
	RefreshTTL    time.Duration
}
type subjectKey struct{}

func RegisterRoutes(router chi.Router, app Application, jwt *JWTManager, cfg HTTPConfig) {
	h := handler{app: app, cfg: cfg}
	router.Route("/api/v1", func(r chi.Router) {
		r.Route("/auth", func(r chi.Router) {
			r.Post("/login", h.login)
			r.Post("/refresh", h.refresh)
			r.Post("/logout", h.logout)
		})
		if jwt == nil {
			return
		}
		r.Group(func(r chi.Router) {
			r.Use(Authenticate(jwt))
			r.Get("/users/me", h.me)
			r.With(RequirePermission(app, "users:write")).Post("/users", h.createUser)
			r.With(RequirePermission(app, "users:write")).Patch("/users/{userID}/active", h.setUserActive)
			r.With(RequirePermission(app, "roles:read")).Get("/roles", h.roles)
			r.With(RequirePermission(app, "roles:read")).Get("/permissions", h.permissions)
			r.With(RequirePermission(app, "roles:write")).Post("/users/{userID}/roles/{roleID}", h.assignRole)
			r.With(RequirePermission(app, "roles:write")).Post("/roles/{roleID}/permissions/{permissionID}", h.grantPermission)
		})
	})
}

func ExtractAccessToken(r *http.Request) (string, error) {
	if value := r.Header.Get("Authorization"); value != "" {
		parts := strings.Fields(value)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
			return "", ErrInvalidCredentials
		}
		return parts[1], nil
	}
	cookie, err := r.Cookie(AccessCookieName)
	if err != nil || cookie.Value == "" {
		return "", ErrInvalidCredentials
	}
	return cookie.Value, nil
}

func Authenticate(jwt *JWTManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw, err := ExtractAccessToken(r)
			if err != nil {
				writeIAMError(w, ErrInvalidCredentials)
				return
			}
			claims, err := jwt.Parse(raw)
			if err != nil {
				writeIAMError(w, ErrInvalidCredentials)
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), subjectKey{}, claims.Subject)))
		})
	}
}

type authorizer interface {
	Authorize(context.Context, string, string) error
}

func RequirePermission(app authorizer, permission string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			subject, _ := r.Context().Value(subjectKey{}).(string)
			if subject == "" {
				writeIAMError(w, ErrInvalidCredentials)
				return
			}
			if err := app.Authorize(r.Context(), subject, permission); err != nil {
				writeIAMError(w, err)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

type handler struct {
	app Application
	cfg HTTPConfig
}

func (h handler) login(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decode(w, r, &in) {
		return
	}
	pair, err := h.app.Login(r.Context(), in.Email, in.Password)
	if err != nil {
		writeIAMError(w, err)
		return
	}
	h.setRefreshCookie(w, pair.RefreshToken)
	response.Write(w, http.StatusOK, pair)
}
func (h handler) refresh(w http.ResponseWriter, r *http.Request) {
	raw, err := refreshToken(r)
	if err != nil {
		writeIAMError(w, err)
		return
	}
	pair, err := h.app.Refresh(r.Context(), raw)
	if err != nil {
		writeIAMError(w, err)
		return
	}
	h.setRefreshCookie(w, pair.RefreshToken)
	response.Write(w, http.StatusOK, pair)
}
func (h handler) logout(w http.ResponseWriter, r *http.Request) {
	raw, _ := refreshToken(r)
	if err := h.app.Logout(r.Context(), raw); err != nil {
		writeIAMError(w, err)
		return
	}
	h.clearRefreshCookie(w)
	response.Write(w, http.StatusOK, map[string]bool{"logged_out": true})
}
func (h handler) me(w http.ResponseWriter, r *http.Request) {
	user, err := h.app.CurrentUser(r.Context(), subject(r))
	if err != nil {
		writeIAMError(w, err)
		return
	}
	response.Write(w, http.StatusOK, user)
}
func (h handler) createUser(w http.ResponseWriter, r *http.Request) {
	var in CreateUserInput
	if !decode(w, r, &in) {
		return
	}
	user, err := h.app.CreateUser(r.Context(), in)
	if err != nil {
		writeIAMError(w, err)
		return
	}
	response.Write(w, http.StatusCreated, user)
}
func (h handler) setUserActive(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Active bool `json:"is_active"`
	}
	if !decode(w, r, &in) {
		return
	}
	if err := h.app.SetUserActive(r.Context(), chi.URLParam(r, "userID"), in.Active); err != nil {
		writeIAMError(w, err)
		return
	}
	response.Write(w, http.StatusOK, map[string]bool{"is_active": in.Active})
}
func (h handler) roles(w http.ResponseWriter, r *http.Request) {
	items, err := h.app.Roles(r.Context())
	if err != nil {
		writeIAMError(w, err)
		return
	}
	response.Write(w, http.StatusOK, items)
}
func (h handler) permissions(w http.ResponseWriter, r *http.Request) {
	items, err := h.app.Permissions(r.Context())
	if err != nil {
		writeIAMError(w, err)
		return
	}
	response.Write(w, http.StatusOK, items)
}
func (h handler) assignRole(w http.ResponseWriter, r *http.Request) {
	if err := h.app.AssignRole(r.Context(), chi.URLParam(r, "userID"), chi.URLParam(r, "roleID")); err != nil {
		writeIAMError(w, err)
		return
	}
	response.Write(w, http.StatusOK, map[string]bool{"assigned": true})
}
func (h handler) grantPermission(w http.ResponseWriter, r *http.Request) {
	if err := h.app.GrantPermission(r.Context(), chi.URLParam(r, "roleID"), chi.URLParam(r, "permissionID")); err != nil {
		writeIAMError(w, err)
		return
	}
	response.Write(w, http.StatusOK, map[string]bool{"granted": true})
}

func (h handler) setRefreshCookie(w http.ResponseWriter, value string) {
	http.SetCookie(w, &http.Cookie{Name: RefreshCookieName, Value: value, Path: "/api/v1/auth", HttpOnly: true, Secure: h.cfg.SecureCookies, SameSite: http.SameSiteStrictMode, MaxAge: int(h.cfg.RefreshTTL.Seconds())})
}
func (h handler) clearRefreshCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: RefreshCookieName, Path: "/api/v1/auth", HttpOnly: true, Secure: h.cfg.SecureCookies, SameSite: http.SameSiteStrictMode, MaxAge: -1})
}
func refreshToken(r *http.Request) (string, error) {
	c, err := r.Cookie(RefreshCookieName)
	if err == nil && c.Value != "" {
		return c.Value, nil
	}
	var in struct {
		RefreshToken string `json:"refresh_token"`
	}
	if r.Body != nil && decodeValue(r, &in) == nil && in.RefreshToken != "" {
		return in.RefreshToken, nil
	}
	return "", ErrInvalidRefreshToken
}
func subject(r *http.Request) string { v, _ := r.Context().Value(subjectKey{}).(string); return v }
func decode(w http.ResponseWriter, r *http.Request, value any) bool {
	if err := decodeValue(r, value); err != nil {
		response.WriteError(w, apperror.Validation(map[string]string{"body": "invalid JSON body"}))
		return false
	}
	return true
}
func decodeValue(r *http.Request, value any) error {
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return err
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return errors.New("multiple JSON values")
	}
	return nil
}
func writeIAMError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInvalidCredentials), errors.Is(err, ErrInvalidRefreshToken):
		response.WriteError(w, apperror.New(401, "invalid credentials", 401, err))
	case errors.Is(err, ErrInactiveUser), errors.Is(err, ErrPermissionDenied):
		response.WriteError(w, apperror.New(403, err.Error(), 403, err))
	case errors.Is(err, ErrDuplicateIdentity):
		response.WriteError(w, apperror.New(409, err.Error(), 409, err))
	case errors.Is(err, ErrNotFound):
		response.WriteError(w, apperror.NotFound("resource"))
	default:
		response.WriteError(w, err)
	}
}
