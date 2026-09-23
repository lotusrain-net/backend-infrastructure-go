package e2e_test

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-chi/chi/v5"
	"github.com/lotusrain-net/backend-infrastructure-go/internal/app"
	"github.com/lotusrain-net/backend-infrastructure-go/internal/testutil"
	"github.com/lotusrain-net/backend-infrastructure-go/pkg/modules/iam"
	"github.com/lotusrain-net/backend-infrastructure-go/pkg/platform/authcache"
	"github.com/lotusrain-net/backend-infrastructure-go/pkg/platform/authcrypto"
	"github.com/lotusrain-net/backend-infrastructure-go/pkg/platform/database/dbgen"
	"github.com/lotusrain-net/backend-infrastructure-go/pkg/platform/database/iamstore"
	"github.com/lotusrain-net/backend-infrastructure-go/pkg/platform/httpserver/iamhttp"
	"github.com/redis/go-redis/v9"
)

type recordingMailer struct {
	mu    sync.Mutex
	codes map[string]string
}

func (m *recordingMailer) SendCode(_ context.Context, email string, purpose iam.EmailCodePurpose, code string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.codes[email+":"+string(purpose)] = code
	return nil
}
func (m *recordingMailer) code(email, purpose string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.codes[email+":"+purpose]
}

type refreshAdapter struct{ client *redis.Client }

func (a refreshAdapter) Get(ctx context.Context, key string) (string, error) {
	return a.client.Get(ctx, key).Result()
}
func (a refreshAdapter) Eval(ctx context.Context, script string, keys []string, args ...any) (any, error) {
	return a.client.Eval(ctx, script, keys, args...).Result()
}

type fixedCrypto struct {
	*authcrypto.Crypto
	now time.Time
}

func (c fixedCrypto) Verify(secret, code string, _ time.Time, last int64) (int64, error) {
	return c.Crypto.Verify(secret, code, c.now, last)
}
func totpAt(secret string, now time.Time) string {
	key, _ := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secret)
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], uint64(now.Unix()/30))
	m := hmac.New(sha1.New, key)
	m.Write(b[:])
	hash := m.Sum(nil)
	o := hash[len(hash)-1] & 15
	return fmt.Sprintf("%06d", (binary.BigEndian.Uint32(hash[o:o+4])&0x7fffffff)%1000000)
}

func TestAuthenticationHTTPWorkflow(t *testing.T) {
	pool := testutil.Postgres(t, 0)
	ctx := context.Background()
	q := dbgen.New(pool)
	repo := iamstore.New(q, pool)
	authRepo := iamstore.NewAuthentication(q, pool)
	passwords := iam.NewPasswordHasher(iam.DefaultArgon2Params())
	if e := app.SeedBootstrapAdmin(ctx, pool, passwords, app.AdminBootstrap{Email: "admin@example.com", Username: "admin", Password: "correct-password"}); e != nil {
		t.Fatal(e)
	}
	hash, _ := passwords.Hash("correct-password")
	if _, e := repo.Create(ctx, iam.CreateUserInput{Email: "existing@example.com", Username: "existing", Password: hash}); e != nil {
		t.Fatal(e)
	}
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { client.Close() })
	jwt, _ := iam.NewJWTManager(bytes.Repeat([]byte{1}, 32), "auth-e2e", time.Hour)
	base := iam.NewServiceWithManagement(repo, repo, repo, repo, passwords, jwt, iam.NewRefreshStore(refreshAdapter{client}, "e2e", time.Hour))
	codes := authcache.New(client, "test", bytes.Repeat([]byte{2}, 32))
	crypto, _ := authcrypto.New(bytes.Repeat([]byte{3}, 32), bytes.Repeat([]byte{2}, 32), "Test")
	now := time.Now()
	mail := &recordingMailer{codes: map[string]string{}}
	service := iam.NewAuthenticationService(base, authRepo, codes, mail, fixedCrypto{crypto, now}, codes.Challenges())
	router := chi.NewRouter()
	iamhttp.RegisterRoutes(router, base, jwt, iamhttp.HTTPConfig{Authentication: service, RefreshTTL: time.Hour})
	call := func(method, path string, body any, token string, want int) (map[string]any, *httptest.ResponseRecorder) {
		t.Helper()
		raw, _ := json.Marshal(body)
		request := httptest.NewRequest(method, path, bytes.NewReader(raw))
		request.Header.Set("Content-Type", "application/json")
		if token != "" {
			request.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, request)
		if w.Code != want {
			t.Fatalf("%s %s status=%d want=%d body=%s", method, path, w.Code, want, w.Body.String())
		}
		var envelope struct {
			Data map[string]any `json:"data"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &envelope)
		return envelope.Data, w
	}
	login := func(email string) map[string]string {
		return map[string]string{"email": email, "password": "correct-password"}
	}
	for _, email := range []string{"existing@example.com", "unknown@example.com"} {
		_, w := call("POST", "/api/v1/auth/login", login(email), "", 401)
		if strings.Contains(w.Body.String(), "initializ") {
			t.Fatal("bootstrap leak")
		}
	}
	call("POST", "/api/v1/auth/register", map[string]string{}, "", 403)
	call("POST", "/api/v1/auth/email-code", map[string]string{"email": "new@example.com", "purpose": "register"}, "", 403)
	// Concurrent first logins must initialize exactly one durable singleton.
	var firstLogins sync.WaitGroup
	outcomes := make(chan int, 4)
	for range 4 {
		firstLogins.Go(func() {
			raw, _ := json.Marshal(login("admin@example.com"))
			r := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(raw))
			w := httptest.NewRecorder()
			router.ServeHTTP(w, r)
			outcomes <- w.Code
		})
	}
	firstLogins.Wait()
	close(outcomes)
	for status := range outcomes {
		if status != 200 {
			t.Fatalf("concurrent bootstrap login returned %d", status)
		}
	}
	admin, _ := call("POST", "/api/v1/auth/login", login("admin@example.com"), "", 200)
	adminToken := admin["access_token"].(string)

	// Bootstrap admins can verify their existing mailbox without registration.
	call("POST", "/api/v1/users/me/email/verification-code", nil, "", 401)
	call("POST", "/api/v1/users/me/email/verify", map[string]string{"email_code": "123456"}, "", 401)
	_, unverifiedResponse := call("PUT", "/api/v1/users/me/security", map[string]string{"mode": "email"}, adminToken, 403)
	if !strings.Contains(unverifiedResponse.Body.String(), "email not verified") {
		t.Fatal("missing actionable error", unverifiedResponse.Body.String())
	}
	call("POST", "/api/v1/auth/email-code", map[string]string{"email": "admin@example.com", "purpose": "verify_email"}, "", 422)
	call("POST", "/api/v1/users/me/email/verification-code", nil, adminToken, 202)
	call("POST", "/api/v1/users/me/email/verification-code", nil, adminToken, 429)
	// A login-purpose code cannot verify a profile mailbox.
	call("POST", "/api/v1/auth/email-code", map[string]string{"email": "admin@example.com", "purpose": "login"}, "", 202)
	verificationCode := mail.code("admin@example.com", "verify_email")
	loginCode := mail.code("admin@example.com", "login")
	if loginCode != verificationCode {
		call("POST", "/api/v1/users/me/email/verify", map[string]string{"email_code": loginCode}, adminToken, 422)
	}
	verified, verificationResponse := call("POST", "/api/v1/users/me/email/verify", map[string]string{"email_code": verificationCode}, adminToken, 200)
	if verified["email_verified_at"] == nil || verificationResponse.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("verification not persisted", verified)
	}
	call("POST", "/api/v1/users/me/email/verify", map[string]string{"email_code": verificationCode}, adminToken, 422)
	call("PUT", "/api/v1/users/me/security", map[string]string{"mode": "email"}, adminToken, 200)
	call("POST", "/api/v1/auth/login", login("admin@example.com"), "", 422)
	adminEmailLogin := login("admin@example.com")
	adminEmailLogin["email_code"] = loginCode
	call("POST", "/api/v1/auth/login", adminEmailLogin, "", 200)
	call("PUT", "/api/v1/users/me/security", map[string]string{"mode": "default"}, adminToken, 200)
	// Initialization survives a new repository instance.
	state, e := iamstore.NewAuthentication(dbgen.New(pool), pool).Authentication(ctx)
	if e != nil || state.InitializedAt == nil {
		t.Fatal("initialization lost", e)
	}
	settings := iam.DefaultAuthenticationSettings()
	settings.RegistrationEnabled = true
	call("PUT", "/api/v1/system-settings/basic-auth", settings, adminToken, 200)
	call("POST", "/api/v1/auth/email-code", map[string]string{"email": "new@example.com", "purpose": "register"}, "", 202)
	in := map[string]string{"username": "new", "email": "new@example.com", "password": "correct-password"}
	required, w := call("POST", "/api/v1/auth/register", in, "", 422)
	if required["email_code"] != "required" || len(w.Result().Cookies()) != 0 {
		t.Fatal("missing registration verification signal", required, w.Header())
	}
	in["email_code"] = mail.code("new@example.com", "register")
	user, w := call("POST", "/api/v1/auth/register", in, "", 201)
	if user["email_verified_at"] == nil || len(w.Result().Cookies()) != 0 {
		t.Fatal("registration marker or cookies")
	}
	result, _ := call("POST", "/api/v1/auth/login", login("new@example.com"), "", 200)
	token := result["access_token"].(string)
	call("GET", "/api/v1/users/me", nil, token, 200)
	call("GET", "/api/v1/system-settings/basic-auth", nil, token, 403)
	call("PUT", "/api/v1/users/me/security", map[string]string{"mode": "email"}, token, 200)
	required, w = call("POST", "/api/v1/auth/login", login("new@example.com"), "", 422)
	if required["email_code"] != "required" || len(w.Result().Cookies()) != 0 {
		t.Fatal("missing login verification signal", required, w.Header())
	}
	call("POST", "/api/v1/auth/email-code", map[string]string{"email": "new@example.com", "purpose": "login"}, "", 202)
	emailLogin := login("new@example.com")
	emailLogin["email_code"] = mail.code("new@example.com", "login")
	result, _ = call("POST", "/api/v1/auth/login", emailLogin, "", 200)
	token = result["access_token"].(string)
	call("POST", "/api/v1/auth/login", emailLogin, "", 401)
	call("PUT", "/api/v1/users/me/security", map[string]string{"mode": "default"}, token, 200)
	enrollment, _ := call("POST", "/api/v1/users/me/security/totp/enroll", map[string]string{"password": "correct-password"}, token, 200)
	secret := enrollment["secret"].(string)
	confirmation, _ := call("POST", "/api/v1/users/me/security/totp/confirm", map[string]string{"code": totpAt(secret, now.Add(-30*time.Second))}, token, 200)
	recovery := confirmation["recovery_codes"].([]any)
	challenge, w := call("POST", "/api/v1/auth/login", login("new@example.com"), "", 202)
	if len(w.Result().Cookies()) != 0 || challenge["access_token"] != nil {
		t.Fatal("challenge issued session")
	}
	verify := map[string]string{"challenge_id": challenge["challenge_id"].(string), "code": totpAt(secret, now)}
	result, w = call("POST", "/api/v1/auth/login/totp/verify", verify, "", 200)
	if len(w.Result().Cookies()) != 2 {
		t.Fatal("verification did not set cookies")
	}
	token = result["access_token"].(string)
	call("POST", "/api/v1/auth/login/totp/verify", verify, "", 401)
	challenge, _ = call("POST", "/api/v1/auth/login", login("new@example.com"), "", 202)
	verify = map[string]string{"challenge_id": challenge["challenge_id"].(string), "recovery_code": recovery[0].(string)}
	call("POST", "/api/v1/auth/login/totp/verify", verify, "", 200)
	call("POST", "/api/v1/auth/login/totp/verify", verify, "", 401)
	// Protected settings reject a silent downgrade; disable requires a fresh proof.
	call("PUT", "/api/v1/users/me/security", map[string]string{"mode": "default"}, token, 403)
	call("POST", "/api/v1/users/me/security/totp/disable", map[string]string{"password": "correct-password", "recovery_code": recovery[1].(string)}, token, 200)
	call("POST", "/api/v1/users/me/security/totp/disable", map[string]string{"password": "correct-password"}, token, 200)
	call("POST", "/api/v1/auth/login", login("new@example.com"), "", 200)
	settings.RegistrationEmailVerificationRequired = false
	call("PUT", "/api/v1/system-settings/basic-auth", settings, adminToken, 200)
	in = map[string]string{"username": "unverified", "email": "unverified@example.com", "password": "correct-password"}
	unverified, _ := call("POST", "/api/v1/auth/register", in, "", 201)
	if unverified["email_verified_at"] != nil {
		t.Fatal("unexpected verification")
	}
	call("POST", "/api/v1/auth/login", login("unverified@example.com"), "", 200)
}
