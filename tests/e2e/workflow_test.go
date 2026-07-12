package e2e_test

import "testing"

func TestComposeWorkflow(t *testing.T) {
	config := requireE2EConfig(t)
	client := newWorkflowClient(t, config)

	client.expectHealthy(t)
	client.login(t, config.adminEmail, config.adminPassword)
	client.expectRBAC(t)
	executionID := client.submitSystemTest(t)
	client.awaitExecutionSucceeded(t, executionID, config.timeout)
	client.awaitAuditEvent(t, executionID, config.timeout)
}
