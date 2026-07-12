package contract_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"backend-infrastructure-go/internal/modules/iam"
	"backend-infrastructure-go/internal/platform/httpserver"
)

const contractUserID = "00112233-4455-6677-8899-aabbccddeeff"

type contractHarness struct {
	handler     http.Handler
	accessToken string
}

type contractReadiness struct{}

func (contractReadiness) Ready(context.Context) error { return nil }

type contractApplication struct {
	pair iam.TokenPair
	user iam.User
}

func (application *contractApplication) Login(context.Context, string, string) (iam.TokenPair, error) {
	return application.pair, nil
}

func (application *contractApplication) Refresh(context.Context, string) (iam.TokenPair, error) {
	return application.pair, nil
}

func (application *contractApplication) Logout(context.Context, string) error { return nil }

func (application *contractApplication) CurrentUser(context.Context, string) (iam.User, error) {
	return application.user, nil
}

func (application *contractApplication) CreateUser(context.Context, iam.CreateUserInput) (iam.User, error) {
	return application.user, nil
}

func (application *contractApplication) SetUserActive(context.Context, string, bool) error {
	return nil
}

func (application *contractApplication) Roles(context.Context) ([]iam.Role, error) {
	return []iam.Role{}, nil
}

func (application *contractApplication) Permissions(context.Context) ([]iam.Permission, error) {
	return []iam.Permission{}, nil
}

func (application *contractApplication) AssignRole(context.Context, string, string) error {
	return nil
}

func (application *contractApplication) GrantPermission(context.Context, string, string) error {
	return nil
}

func (application *contractApplication) Authorize(context.Context, string, string) error {
	return nil
}

func newContractHarness(t *testing.T) contractHarness {
	t.Helper()
	jwt, err := iam.NewJWTManager([]byte("0123456789abcdef0123456789abcdef"), "contract-test", time.Minute)
	if err != nil {
		t.Fatalf("NewJWTManager() error = %v", err)
	}
	accessToken, err := jwt.Issue(contractUserID)
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	application := &contractApplication{
		pair: iam.TokenPair{AccessToken: accessToken, RefreshToken: "refresh-token", TokenType: "Bearer", ExpiresIn: 60},
		user: iam.User{
			ID: contractUserID, Email: "admin@example.com", Username: "admin",
			DisplayName: "Admin", Active: true,
		},
	}
	router, err := httpserver.NewRouter(httpserver.RouterOptions{
		Logger:    slog.New(slog.NewTextHandler(io.Discard, nil)),
		Readiness: contractReadiness{},
		Metrics:   http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}),
	})
	if err != nil {
		t.Fatalf("NewRouter() error = %v", err)
	}
	iam.RegisterRoutes(router, application, jwt, iam.HTTPConfig{SecureCookies: true, RefreshTTL: time.Hour})
	return contractHarness{handler: router, accessToken: accessToken}
}
