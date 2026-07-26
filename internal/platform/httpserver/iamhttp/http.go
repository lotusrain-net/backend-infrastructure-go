package iamhttp

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"backend-infrastructure-go/internal/modules/iam"
	"backend-infrastructure-go/internal/platform/httpserver"
	"backend-infrastructure-go/internal/shared/apperror"
	"backend-infrastructure-go/internal/shared/pagination"
	"backend-infrastructure-go/internal/shared/response"
	"github.com/go-chi/chi/v5"
)

const (
	AccessCookieName  = "access_token"
	RefreshCookieName = "refresh_token"
	maxJSONBodyBytes  = 1 << 20
)

type HTTPConfig struct {
	SecureCookies bool
	RefreshTTL    time.Duration
}

func RegisterRoutes(router chi.Router, app iam.Application, jwt *iam.JWTManager, cfg HTTPConfig) {
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
			r.With(RequirePermission(app, "users:read")).Get("/users", h.users)
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
			return "", iam.ErrInvalidCredentials
		}
		return parts[1], nil
	}
	cookie, err := r.Cookie(AccessCookieName)
	if err != nil || cookie.Value == "" {
		return "", iam.ErrInvalidCredentials
	}
	return cookie.Value, nil
}

func Authenticate(jwt *iam.JWTManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw, err := ExtractAccessToken(r)
			if err != nil {
				writeIAMError(w, iam.ErrInvalidCredentials)
				return
			}
			claims, err := jwt.Parse(raw)
			if err != nil {
				writeIAMError(w, iam.ErrInvalidCredentials)
				return
			}
			next.ServeHTTP(w, r.WithContext(iam.WithSubject(r.Context(), claims.Subject)))
		})
	}
}

type authorizer interface {
	Authorize(context.Context, string, string) error
}

func RequirePermission(app authorizer, permission string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			subject := iam.Subject(r.Context())
			if subject == "" {
				writeIAMError(w, iam.ErrInvalidCredentials)
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
	app iam.Application
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
	h.writeTokenPair(w, pair)
}

func (h handler) refresh(w http.ResponseWriter, r *http.Request) {
	raw, err := refreshToken(w, r)
	if err != nil {
		writeIAMError(w, err)
		return
	}
	pair, err := h.app.Refresh(r.Context(), raw)
	if err != nil {
		writeIAMError(w, err)
		return
	}
	h.writeTokenPair(w, pair)
}

func (h handler) logout(w http.ResponseWriter, r *http.Request) {
	raw, _ := refreshToken(w, r)
	if err := h.app.Logout(r.Context(), raw); err != nil {
		writeIAMError(w, err)
		return
	}
	setNoStore(w)
	h.clearAuthCookies(w)
	response.Write(w, http.StatusOK, map[string]bool{"logged_out": true})
}

func (h handler) me(w http.ResponseWriter, r *http.Request) {
	user, err := h.app.CurrentUser(r.Context(), iam.Subject(r.Context()))
	if err != nil {
		writeIAMError(w, err)
		return
	}
	response.Write(w, http.StatusOK, user)
}

func (h handler) users(w http.ResponseWriter, r *http.Request) {
	filter := iam.UserFilter{Query: strings.TrimSpace(r.URL.Query().Get("query"))}
	if value := r.URL.Query().Get("is_active"); value != "" {
		active, err := strconv.ParseBool(value)
		if err != nil {
			response.WriteError(w, apperror.Validation(map[string]string{"is_active": "must be true or false"}))
			return
		}
		filter.Active = &active
	}
	items, err := h.app.Users(r.Context(), iam.UserQuery{
		Filter: filter,
		Page:   queryInt(r, "page", 1),
		Size:   queryInt(r, "size", pagination.DefaultSize),
	})
	if err != nil {
		writeIAMError(w, err)
		return
	}
	response.Write(w, http.StatusOK, items)
}

func (h handler) createUser(w http.ResponseWriter, r *http.Request) {
	var in iam.CreateUserInput
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
		Active *bool `json:"is_active"`
	}
	if !decode(w, r, &in) {
		return
	}
	if in.Active == nil {
		response.WriteError(w, apperror.Validation(map[string]string{"is_active": "is required"}))
		return
	}
	if err := h.app.SetUserActive(r.Context(), chi.URLParam(r, "userID"), *in.Active); err != nil {
		writeIAMError(w, err)
		return
	}
	response.Write(w, http.StatusOK, map[string]bool{"is_active": *in.Active})
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

func (h handler) setAccessCookie(w http.ResponseWriter, value string, expiresIn int64) {
	http.SetCookie(w, &http.Cookie{Name: AccessCookieName, Value: value, Path: "/api/v1", HttpOnly: true, Secure: h.cfg.SecureCookies, SameSite: http.SameSiteStrictMode, MaxAge: int(expiresIn)})
}

func (h handler) clearAuthCookies(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: AccessCookieName, Path: "/api/v1", HttpOnly: true, Secure: h.cfg.SecureCookies, SameSite: http.SameSiteStrictMode, MaxAge: -1})
	http.SetCookie(w, &http.Cookie{Name: RefreshCookieName, Path: "/api/v1/auth", HttpOnly: true, Secure: h.cfg.SecureCookies, SameSite: http.SameSiteStrictMode, MaxAge: -1})
}

func (h handler) writeTokenPair(w http.ResponseWriter, pair iam.TokenPair) {
	setNoStore(w)
	h.setAccessCookie(w, pair.AccessToken, pair.ExpiresIn)
	h.setRefreshCookie(w, pair.RefreshToken)
	response.Write(w, http.StatusOK, pair)
}

func setNoStore(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
}

func refreshToken(w http.ResponseWriter, r *http.Request) (string, error) {
	cookie, err := r.Cookie(RefreshCookieName)
	if err == nil && cookie.Value != "" {
		return cookie.Value, nil
	}
	var in struct {
		RefreshToken string `json:"refresh_token"`
	}
	if r.Body != nil && httpserver.DecodeJSON(w, r, maxJSONBodyBytes, &in) == nil && in.RefreshToken != "" {
		return in.RefreshToken, nil
	}
	return "", iam.ErrInvalidRefreshToken
}

func decode(w http.ResponseWriter, r *http.Request, value any) bool {
	if err := httpserver.DecodeJSON(w, r, maxJSONBodyBytes, value); err != nil {
		response.WriteError(w, apperror.Validation(map[string]string{"body": "invalid JSON body"}))
		return false
	}
	return true
}

func queryInt(r *http.Request, name string, fallback int) int {
	value, err := strconv.Atoi(r.URL.Query().Get(name))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func writeIAMError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, iam.ErrInvalidCredentials), errors.Is(err, iam.ErrInvalidRefreshToken):
		response.WriteError(w, apperror.New(401, "invalid credentials", 401, err))
	case errors.Is(err, iam.ErrInactiveUser), errors.Is(err, iam.ErrPermissionDenied):
		response.WriteError(w, apperror.New(403, err.Error(), 403, err))
	case errors.Is(err, iam.ErrDuplicateIdentity):
		response.WriteError(w, apperror.New(409, err.Error(), 409, err))
	case errors.Is(err, iam.ErrInvalidUserInput):
		response.WriteError(w, apperror.Validation(map[string]string{"user": err.Error()}))
	case errors.Is(err, iam.ErrNotFound):
		response.WriteError(w, apperror.NotFound("resource"))
	default:
		response.WriteError(w, err)
	}
}
