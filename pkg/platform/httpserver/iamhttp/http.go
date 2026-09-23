package iamhttp

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	apperror "github.com/lotusrain-net/backend-infrastructure-go/pkg/apikit"
	httpserver "github.com/lotusrain-net/backend-infrastructure-go/pkg/httpkit"
	"github.com/lotusrain-net/backend-infrastructure-go/pkg/modules/iam"
	"github.com/lotusrain-net/backend-infrastructure-go/pkg/pagination"
)

const (
	AccessCookieName  = "access_token"
	RefreshCookieName = "refresh_token"
	maxJSONBodyBytes  = 1 << 20
)

type HTTPConfig struct {
	Authentication iam.AuthenticationApplication
	SecureCookies  bool
	RefreshTTL     time.Duration
}

func RegisterRoutes(router chi.Router, app iam.Application, jwt *iam.JWTManager, cfg HTTPConfig) {
	h := handler{app: app, cfg: cfg}
	router.Route("/api/v1", func(r chi.Router) {
		r.Route("/auth", func(r chi.Router) {
			r.Post("/login", h.login)
			if cfg.Authentication != nil {
				r.Post("/email-code", h.emailCode)
				r.Post("/register", h.register)
				r.Post("/login/totp/verify", h.verifyLogin)
			}
			r.Post("/refresh", h.refresh)
			r.Post("/logout", h.logout)
		})
		if jwt == nil {
			return
		}
		r.Group(func(r chi.Router) {
			r.Use(Authenticate(jwt))
			r.Get("/users/me", h.me)
			if cfg.Authentication != nil {
				r.Get("/users/me/security", h.security)
				r.Put("/users/me/security", h.putSecurity)
				r.Post("/users/me/security/totp/enroll", h.enroll)
				r.Post("/users/me/security/totp/confirm", h.confirm)
				r.Post("/users/me/security/totp/disable", h.disable)
				r.With(RequirePermission(app, "system-settings:read")).Get("/system-settings/basic-auth", h.settings)
				r.With(RequirePermission(app, "system-settings:write")).Put("/system-settings/basic-auth", h.putSettings)
			}
			r.Get("/users/me/preferences", h.preferences)
			r.Put("/users/me/preferences", h.putPreferences)
			r.With(RequirePermission(app, "users:read")).Get("/users", h.users)
			r.With(RequirePermission(app, "users:write")).Post("/users", h.createUser)
			r.With(RequirePermission(app, "users:write")).Patch("/users/{userID}", h.updateUser)
			r.With(RequirePermission(app, "users:write")).Post("/users/{userID}/password/reset", h.resetUserPassword)
			r.With(RequirePermission(app, "users:write")).Patch("/users/{userID}/active", h.setUserActive)
			r.With(RequirePermission(app, "roles:read")).Get("/roles", h.roles)
			r.With(RequirePermission(app, "roles:read")).Get("/roles/{roleID}", h.role)
			r.With(RequirePermission(app, "roles:write")).Post("/roles", h.createRole)
			r.With(RequirePermission(app, "roles:write")).Patch("/roles/{roleID}", h.updateRole)
			r.With(RequirePermission(app, "roles:write")).Delete("/roles/{roleID}", h.deleteRole)
			r.With(RequirePermission(app, "roles:read")).Get("/permissions", h.permissions)
			r.With(RequirePermission(app, "roles:write")).Post("/users/{userID}/roles/{roleID}", h.assignRole)
			r.With(RequirePermission(app, "roles:write")).Put("/users/{userID}/roles", h.replaceUserRoles)
			r.With(RequirePermission(app, "roles:write")).Post("/roles/{roleID}/permissions/{permissionID}", h.grantPermission)
			r.With(RequirePermission(app, "roles:write")).Put("/roles/{roleID}/permissions", h.replaceRolePermissions)
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
	setNoStore(w)
	var in iam.LoginInput
	if !decode(w, r, &in) {
		return
	}
	if h.cfg.Authentication != nil {
		result, err := h.cfg.Authentication.Login(r.Context(), in)
		if err != nil {
			writeIAMError(w, err)
			return
		}
		if result.Status == "totp_required" {
			apperror.Write(w, http.StatusAccepted, map[string]any{"status": result.Status, "challenge_id": result.ChallengeID, "expires_in": result.ChallengeExpiresIn})
			return
		}
		h.writeTokenPair(w, result.TokenPair)
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
	apperror.Write(w, http.StatusOK, map[string]bool{"logged_out": true})
}

func (h handler) me(w http.ResponseWriter, r *http.Request) {
	user, err := h.app.CurrentUser(r.Context(), iam.Subject(r.Context()))
	if err != nil {
		writeIAMError(w, err)
		return
	}
	apperror.Write(w, http.StatusOK, user)
}

func (h handler) preferences(w http.ResponseWriter, r *http.Request) {
	preferences, err := h.app.Preferences(r.Context(), iam.Subject(r.Context()))
	if err != nil {
		writeIAMError(w, err)
		return
	}
	apperror.Write(w, http.StatusOK, preferences)
}

func (h handler) putPreferences(w http.ResponseWriter, r *http.Request) {
	var preferences iam.Preferences
	if !decode(w, r, &preferences) {
		return
	}
	if err := h.app.PutPreferences(r.Context(), iam.Subject(r.Context()), preferences); err != nil {
		writeIAMError(w, err)
		return
	}
	apperror.Write(w, http.StatusOK, preferences)
}

func (h handler) users(w http.ResponseWriter, r *http.Request) {
	filter := iam.UserFilter{Query: strings.TrimSpace(r.URL.Query().Get("query"))}
	if value := r.URL.Query().Get("is_active"); value != "" {
		active, err := strconv.ParseBool(value)
		if err != nil {
			apperror.WriteError(w, apperror.Validation(map[string]string{"is_active": "must be true or false"}))
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
	apperror.Write(w, http.StatusOK, items)
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
	apperror.Write(w, http.StatusCreated, user)
}

func (h handler) updateUser(w http.ResponseWriter, r *http.Request) {
	var input iam.UpdateUserInput
	if !decode(w, r, &input) {
		return
	}
	user, err := h.app.UpdateUser(r.Context(), chi.URLParam(r, "userID"), input)
	if err != nil {
		writeIAMError(w, err)
		return
	}
	apperror.Write(w, http.StatusOK, user)
}

func (h handler) resetUserPassword(w http.ResponseWriter, r *http.Request) {
	var input struct {
		NewPassword string `json:"new_password"`
	}
	if !decode(w, r, &input) {
		return
	}
	if err := h.app.ResetUserPassword(r.Context(), chi.URLParam(r, "userID"), input.NewPassword); err != nil {
		writeIAMError(w, err)
		return
	}
	apperror.Write(w, http.StatusOK, map[string]bool{"reset": true})
}

func (h handler) setUserActive(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Active *bool `json:"is_active"`
	}
	if !decode(w, r, &in) {
		return
	}
	if in.Active == nil {
		apperror.WriteError(w, apperror.Validation(map[string]string{"is_active": "is required"}))
		return
	}
	if err := h.app.SetUserActive(r.Context(), chi.URLParam(r, "userID"), *in.Active); err != nil {
		writeIAMError(w, err)
		return
	}
	apperror.Write(w, http.StatusOK, map[string]bool{"is_active": *in.Active})
}

func (h handler) roles(w http.ResponseWriter, r *http.Request) {
	items, err := h.app.Roles(r.Context())
	if err != nil {
		writeIAMError(w, err)
		return
	}
	apperror.Write(w, http.StatusOK, items)
}

func (h handler) role(w http.ResponseWriter, r *http.Request) {
	role, err := h.app.Role(r.Context(), chi.URLParam(r, "roleID"))
	if err != nil {
		writeIAMError(w, err)
		return
	}
	apperror.Write(w, http.StatusOK, role)
}

func (h handler) createRole(w http.ResponseWriter, r *http.Request) {
	var input iam.RoleInput
	if !decode(w, r, &input) {
		return
	}
	role, err := h.app.CreateRole(r.Context(), input)
	if err != nil {
		writeIAMError(w, err)
		return
	}
	apperror.Write(w, http.StatusCreated, role)
}

func (h handler) updateRole(w http.ResponseWriter, r *http.Request) {
	var input iam.RoleInput
	if !decode(w, r, &input) {
		return
	}
	role, err := h.app.UpdateRole(r.Context(), chi.URLParam(r, "roleID"), input)
	if err != nil {
		writeIAMError(w, err)
		return
	}
	apperror.Write(w, http.StatusOK, role)
}

func (h handler) deleteRole(w http.ResponseWriter, r *http.Request) {
	if err := h.app.DeleteRole(r.Context(), chi.URLParam(r, "roleID")); err != nil {
		writeIAMError(w, err)
		return
	}
	apperror.Write(w, http.StatusOK, map[string]bool{"deleted": true})
}

func (h handler) permissions(w http.ResponseWriter, r *http.Request) {
	items, err := h.app.Permissions(r.Context())
	if err != nil {
		writeIAMError(w, err)
		return
	}
	apperror.Write(w, http.StatusOK, items)
}

func (h handler) assignRole(w http.ResponseWriter, r *http.Request) {
	if err := h.app.AssignRole(r.Context(), chi.URLParam(r, "userID"), chi.URLParam(r, "roleID")); err != nil {
		writeIAMError(w, err)
		return
	}
	apperror.Write(w, http.StatusOK, map[string]bool{"assigned": true})
}

func (h handler) replaceUserRoles(w http.ResponseWriter, r *http.Request) {
	var input struct {
		RoleIDs *[]string `json:"role_ids"`
	}
	if !decode(w, r, &input) {
		return
	}
	if input.RoleIDs == nil {
		apperror.WriteError(w, apperror.Validation(map[string]string{"role_ids": "is required"}))
		return
	}
	if err := h.app.ReplaceUserRoles(r.Context(), chi.URLParam(r, "userID"), *input.RoleIDs); err != nil {
		writeIAMError(w, err)
		return
	}
	apperror.Write(w, http.StatusOK, map[string]bool{"replaced": true})
}

func (h handler) grantPermission(w http.ResponseWriter, r *http.Request) {
	if err := h.app.GrantPermission(r.Context(), chi.URLParam(r, "roleID"), chi.URLParam(r, "permissionID")); err != nil {
		writeIAMError(w, err)
		return
	}
	apperror.Write(w, http.StatusOK, map[string]bool{"granted": true})
}

func (h handler) replaceRolePermissions(w http.ResponseWriter, r *http.Request) {
	var input struct {
		PermissionIDs *[]string `json:"permission_ids"`
	}
	if !decode(w, r, &input) {
		return
	}
	if input.PermissionIDs == nil {
		apperror.WriteError(w, apperror.Validation(map[string]string{"permission_ids": "is required"}))
		return
	}
	if err := h.app.ReplaceRolePermissions(r.Context(), chi.URLParam(r, "roleID"), *input.PermissionIDs); err != nil {
		writeIAMError(w, err)
		return
	}
	apperror.Write(w, http.StatusOK, map[string]bool{"replaced": true})
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
	apperror.Write(w, http.StatusOK, pair)
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
		apperror.WriteError(w, apperror.Validation(map[string]string{"body": "invalid JSON body"}))
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
	var limited *iam.RateLimitError
	if errors.As(err, &limited) {
		w.Header().Set("Retry-After", strconv.Itoa(limited.RetryAfter))
		apperror.WriteError(w, apperror.New(429, "too many requests", 429, err))
		return
	}
	switch {
	case errors.Is(err, iam.ErrEmailCodeRequired):
		apperror.WriteError(w, apperror.Validation(map[string]string{"email_code": "required"}))
	case errors.Is(err, iam.ErrInvalidCode), errors.Is(err, iam.ErrInvalidCredentials), errors.Is(err, iam.ErrInvalidRefreshToken):
		apperror.WriteError(w, apperror.New(401, "invalid credentials", 401, err))
	case errors.Is(err, iam.ErrAuthenticationForbidden):
		apperror.WriteError(w, apperror.New(403, "operation not permitted", 403, err))
	case errors.Is(err, iam.ErrInactiveUser), errors.Is(err, iam.ErrPermissionDenied):
		apperror.WriteError(w, apperror.New(403, err.Error(), 403, err))
	case errors.Is(err, iam.ErrSecurityConflict), errors.Is(err, iam.ErrDuplicateIdentity), errors.Is(err, iam.ErrCannotDeactivateSelf), errors.Is(err, iam.ErrLastActiveAdministrator), errors.Is(err, iam.ErrSystemRoleProtected):
		apperror.WriteError(w, apperror.New(409, err.Error(), 409, err))
	case errors.Is(err, iam.ErrInvalidUserInput):
		apperror.WriteError(w, apperror.Validation(map[string]string{"user": err.Error()}))
	case errors.Is(err, iam.ErrNotFound):
		apperror.WriteError(w, apperror.NotFound("resource"))
	default:
		apperror.WriteError(w, err)
	}
}
