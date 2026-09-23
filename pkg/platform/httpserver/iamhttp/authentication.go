package iamhttp

import (
	"errors"
	"net"
	"net/http"

	apperror "github.com/lotusrain-net/backend-infrastructure-go/pkg/apikit"
	"github.com/lotusrain-net/backend-infrastructure-go/pkg/modules/audit"
	"github.com/lotusrain-net/backend-infrastructure-go/pkg/modules/iam"
)

func (h handler) emailCode(w http.ResponseWriter, r *http.Request) {
	setNoStore(w)
	var in struct {
		Email   string `json:"email"`
		Purpose string `json:"purpose"`
	}
	if !decode(w, r, &in) {
		return
	}
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	if address := audit.RequestMetadataFromContext(r.Context()).IPAddress; address != nil {
		ip = address.String()
	}
	if e := h.cfg.Authentication.RequestEmailCode(r.Context(), in.Email, in.Purpose, ip); e != nil {
		writeIAMError(w, e)
		return
	}
	apperror.Write(w, 202, map[string]any{"sent": true, "resend_after_seconds": 60})
}
func (h handler) register(w http.ResponseWriter, r *http.Request) {
	setNoStore(w)
	var in iam.RegisterInput
	if !decode(w, r, &in) {
		return
	}
	user, e := h.cfg.Authentication.Register(r.Context(), in)
	if e != nil {
		writeIAMError(w, e)
		return
	}
	apperror.Write(w, 201, user)
}
func (h handler) verifyLogin(w http.ResponseWriter, r *http.Request) {
	setNoStore(w)
	var in iam.VerifyInput
	if !decode(w, r, &in) {
		return
	}
	pair, e := h.cfg.Authentication.VerifyLogin(r.Context(), in)
	if e != nil {
		writeIAMError(w, e)
		return
	}
	h.writeTokenPair(w, pair)
}
func (h handler) settings(w http.ResponseWriter, r *http.Request) {
	setNoStore(w)
	v, e := h.cfg.Authentication.Settings(r.Context())
	if e != nil {
		writeIAMError(w, e)
		return
	}
	apperror.Write(w, 200, v)
}
func (h handler) putSettings(w http.ResponseWriter, r *http.Request) {
	setNoStore(w)
	var in struct {
		Password     *bool     `json:"password_login_enabled"`
		Registration *bool     `json:"registration_enabled"`
		Verification *bool     `json:"registration_email_verification_required"`
		Domains      *[]string `json:"allowed_email_domains"`
	}
	if !decode(w, r, &in) {
		return
	}
	if in.Password == nil || in.Registration == nil || in.Verification == nil || in.Domains == nil {
		writeIAMError(w, iam.ErrInvalidUserInput)
		return
	}
	v := iam.AuthenticationSettings{PasswordLoginEnabled: *in.Password, RegistrationEnabled: *in.Registration, RegistrationEmailVerificationRequired: *in.Verification, AllowedEmailDomains: *in.Domains}
	if e := h.cfg.Authentication.PutSettings(r.Context(), v); e != nil {
		writeIAMError(w, e)
		return
	}
	apperror.Write(w, 200, v)
}
func (h handler) security(w http.ResponseWriter, r *http.Request) {
	setNoStore(w)
	v, e := h.cfg.Authentication.Security(r.Context(), iam.Subject(r.Context()))
	if e != nil {
		writeIAMError(w, e)
		return
	}
	apperror.Write(w, 200, v)
}
func (h handler) putSecurity(w http.ResponseWriter, r *http.Request) {
	setNoStore(w)
	var in struct {
		Mode string `json:"mode"`
	}
	if !decode(w, r, &in) {
		return
	}
	v, e := h.cfg.Authentication.PutSecurity(r.Context(), iam.Subject(r.Context()), in.Mode)
	if e != nil {
		writeIAMError(w, e)
		return
	}
	apperror.Write(w, 200, v)
}
func (h handler) enroll(w http.ResponseWriter, r *http.Request) {
	setNoStore(w)
	var in iam.SecurityProof
	if !decode(w, r, &in) {
		return
	}
	v, e := h.cfg.Authentication.Enroll(r.Context(), iam.Subject(r.Context()), in)
	if e != nil {
		writeSecurityProofError(w, e)
		return
	}
	apperror.Write(w, 200, v)
}
func (h handler) confirm(w http.ResponseWriter, r *http.Request) {
	setNoStore(w)
	var in struct {
		Code string `json:"code"`
	}
	if !decode(w, r, &in) {
		return
	}
	v, e := h.cfg.Authentication.Confirm(r.Context(), iam.Subject(r.Context()), in.Code)
	if e != nil {
		writeSecurityProofError(w, e)
		return
	}
	apperror.Write(w, 200, map[string]any{"recovery_codes": v})
}
func (h handler) disable(w http.ResponseWriter, r *http.Request) {
	setNoStore(w)
	var in iam.SecurityProof
	if !decode(w, r, &in) {
		return
	}
	if e := h.cfg.Authentication.Disable(r.Context(), iam.Subject(r.Context()), in); e != nil {
		writeSecurityProofError(w, e)
		return
	}
	apperror.Write(w, 200, map[string]bool{"disabled": true})
}

// Session authentication is handled by middleware (401). Invalid step-up
// proofs on an authenticated request are validation failures, not a signal
// for clients to refresh the session or replay a one-time credential.
func writeSecurityProofError(w http.ResponseWriter, err error) {
	if errors.Is(err, iam.ErrInvalidCode) || errors.Is(err, iam.ErrInvalidCredentials) {
		apperror.WriteError(w, apperror.Validation(map[string]string{"proof": "invalid or expired credential"}))
		return
	}
	writeIAMError(w, err)
}

func (h handler) requestEmailVerification(w http.ResponseWriter, r *http.Request) {
	setNoStore(w)
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	if address := audit.RequestMetadataFromContext(r.Context()).IPAddress; address != nil {
		ip = address.String()
	}
	if err := h.cfg.Authentication.RequestEmailVerification(r.Context(), iam.Subject(r.Context()), ip); err != nil {
		writeIAMError(w, err)
		return
	}
	apperror.Write(w, http.StatusAccepted, map[string]any{"sent": true, "resend_after_seconds": 60})
}

func (h handler) verifyEmail(w http.ResponseWriter, r *http.Request) {
	setNoStore(w)
	var in struct {
		EmailCode string `json:"email_code"`
	}
	if !decode(w, r, &in) {
		return
	}
	user, err := h.cfg.Authentication.VerifyEmail(r.Context(), iam.Subject(r.Context()), in.EmailCode)
	if err != nil {
		if errors.Is(err, iam.ErrInvalidCode) {
			apperror.WriteError(w, apperror.Validation(map[string]string{"email_code": "invalid or expired credential"}))
		} else {
			writeIAMError(w, err)
		}
		return
	}
	apperror.Write(w, http.StatusOK, user)
}
