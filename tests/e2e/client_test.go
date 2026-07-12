package e2e_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

type e2eConfig struct {
	baseURL       string
	adminEmail    string
	adminPassword string
	timeout       time.Duration
}

type workflowClient struct {
	baseURL     string
	httpClient  *http.Client
	accessToken string
}

type rawEnvelope struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

type taskExecution struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

func requireE2EConfig(t *testing.T) e2eConfig {
	t.Helper()
	if os.Getenv("E2E_RUN") != "1" {
		t.Skip("Compose E2E disabled; run go run ./scripts/verification with required E2E environment")
	}
	config := e2eConfig{
		baseURL:       strings.TrimRight(os.Getenv("E2E_BASE_URL"), "/"),
		adminEmail:    os.Getenv("E2E_ADMIN_EMAIL"),
		adminPassword: os.Getenv("E2E_ADMIN_PASSWORD"),
		timeout:       time.Minute,
	}
	for name, value := range map[string]string{
		"E2E_BASE_URL": config.baseURL, "E2E_ADMIN_EMAIL": config.adminEmail, "E2E_ADMIN_PASSWORD": config.adminPassword,
	} {
		if strings.TrimSpace(value) == "" {
			t.Fatalf("%s is required when E2E_RUN=1", name)
		}
	}
	parsed, err := url.Parse(config.baseURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		t.Fatalf("E2E_BASE_URL %q must be an absolute HTTP(S) URL", config.baseURL)
	}
	if value := os.Getenv("E2E_TIMEOUT"); value != "" {
		config.timeout, err = time.ParseDuration(value)
		if err != nil || config.timeout <= 0 {
			t.Fatalf("E2E_TIMEOUT %q must be a positive duration", value)
		}
	}
	return config
}

func newWorkflowClient(t *testing.T, config e2eConfig) *workflowClient {
	t.Helper()
	return &workflowClient{
		baseURL:    config.baseURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (client *workflowClient) expectHealthy(t *testing.T) {
	t.Helper()
	envelope := client.mustRequest(t, http.MethodGet, "/health/live", nil, http.StatusOK)
	var status struct {
		Status string `json:"status"`
	}
	decodeData(t, envelope, &status)
	if status.Status != "ok" {
		t.Fatalf("health status = %q, want ok", status.Status)
	}
}

func (client *workflowClient) login(t *testing.T, email, password string) {
	t.Helper()
	envelope := client.mustRequest(t, http.MethodPost, "/api/v1/auth/login", map[string]string{
		"email": email, "password": password,
	}, http.StatusOK)
	var pair struct {
		AccessToken string `json:"access_token"`
	}
	decodeData(t, envelope, &pair)
	if pair.AccessToken == "" {
		t.Fatal("login response does not contain access_token")
	}
	client.accessToken = pair.AccessToken
}

func (client *workflowClient) expectRBAC(t *testing.T) {
	t.Helper()
	envelope := client.mustRequest(t, http.MethodGet, "/api/v1/roles", nil, http.StatusOK)
	var roles []json.RawMessage
	decodeData(t, envelope, &roles)
	if len(roles) == 0 {
		t.Fatal("RBAC role collection is empty")
	}
}

func (client *workflowClient) submitSystemTest(t *testing.T) string {
	t.Helper()
	envelope := client.mustRequest(t, http.MethodPost, "/api/v1/task-executions", map[string]any{
		"task_type":       "system.test",
		"payload":         map[string]any{"processed_rows": 1},
		"idempotency_key": fmt.Sprintf("compose-e2e-%d", time.Now().UnixNano()),
		"max_retries":     1,
		"timeout_seconds": 30,
	}, http.StatusAccepted)
	var execution taskExecution
	decodeData(t, envelope, &execution)
	if execution.ID == "" || execution.Status != "queued" {
		t.Fatalf("submitted execution = %+v, want queued execution with ID", execution)
	}
	return execution.ID
}

func (client *workflowClient) awaitExecutionSucceeded(t *testing.T, executionID string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		envelope := client.mustRequest(t, http.MethodGet, "/api/v1/task-executions/"+url.PathEscape(executionID), nil, http.StatusOK)
		var execution taskExecution
		decodeData(t, envelope, &execution)
		switch execution.Status {
		case "succeeded":
			return
		case "failed", "cancelled":
			t.Fatalf("execution %s reached terminal status %s", executionID, execution.Status)
		}
		if time.Now().After(deadline) {
			t.Fatalf("execution %s did not succeed within %s; last status %s", executionID, timeout, execution.Status)
		}
		time.Sleep(250 * time.Millisecond)
	}
}

func (client *workflowClient) awaitAuditEvent(t *testing.T, executionID string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	path := "/api/v1/audit-logs?resource_id=" + url.QueryEscape(executionID)
	for {
		envelope := client.mustRequest(t, http.MethodGet, path, nil, http.StatusOK)
		var page struct {
			Items []struct {
				ResourceID string `json:"resource_id"`
			} `json:"items"`
		}
		decodeData(t, envelope, &page)
		for _, event := range page.Items {
			if event.ResourceID == executionID {
				return
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("audit event for execution %s not found within %s", executionID, timeout)
		}
		time.Sleep(250 * time.Millisecond)
	}
}

func (client *workflowClient) mustRequest(t *testing.T, method, path string, body any, expectedStatus int) rawEnvelope {
	t.Helper()
	var encoded io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("encode %s %s body: %v", method, path, err)
		}
		encoded = bytes.NewReader(payload)
	}
	request, err := http.NewRequestWithContext(t.Context(), method, client.baseURL+path, encoded)
	if err != nil {
		t.Fatalf("create %s %s request: %v", method, path, err)
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if client.accessToken != "" {
		request.Header.Set("Authorization", "Bearer "+client.accessToken)
	}
	response, err := client.httpClient.Do(request)
	if err != nil {
		t.Fatalf("%s %s request failed: %v", method, path, err)
	}
	defer response.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		t.Fatalf("read %s %s response: %v", method, path, err)
	}
	if response.StatusCode != expectedStatus {
		t.Fatalf("%s %s status = %d, want %d; body = %s", method, path, response.StatusCode, expectedStatus, payload)
	}
	var envelope rawEnvelope
	if err := json.Unmarshal(payload, &envelope); err != nil {
		t.Fatalf("%s %s response is not a JSON envelope: %v; body = %s", method, path, err, payload)
	}
	if envelope.Code != expectedStatus || envelope.Msg != "success" {
		t.Fatalf("%s %s envelope = %+v, want code=%d msg=success", method, path, envelope, expectedStatus)
	}
	return envelope
}

func decodeData(t *testing.T, envelope rawEnvelope, target any) {
	t.Helper()
	if err := json.Unmarshal(envelope.Data, target); err != nil {
		t.Fatalf("decode envelope data %s: %v", envelope.Data, err)
	}
}
