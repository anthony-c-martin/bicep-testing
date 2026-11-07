package sample_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	biceptesting "github.com/anthony-c-martin/bicep-testing/packages/go/bicep-testing"
)

func TestDeployments(t *testing.T) {
	settings := loadLiveSettings(t)
	credential, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		t.Fatal(err)
	}
	session := newLiveTestSession(t, credential)
	t.Run("template validates in Azure before deployment", func(t *testing.T) {
		testTemplateValidatesInAzure(t, session, settings)
	})
	t.Run("deployed storage meets the security baseline in Azure", func(t *testing.T) {
		testSecureStorageIsVerifiedInAzureAndRemoved(t, session, credential, settings)
	})
	t.Run("disabling audit storage removes the account from Azure", func(t *testing.T) {
		testDeploymentReconcilesRemovedAuditStorageAndCleansUp(t, session, credential, settings)
	})
	t.Run("identical redeployment preserves resource identity and security", func(t *testing.T) {
		testIdenticalRedeployment(t, session, credential, settings)
	})
}

func testIdenticalRedeployment(t *testing.T, session *biceptesting.LiveSession, credential azcore.TokenCredential, settings liveSettings) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	options := biceptesting.DeployOptions{
		FilePath:       filepath.Join("..", "infra", "live-storage", "main.bicepparam"),
		SubscriptionID: settings.subscriptionID, ResourceGroup: settings.resourceGroup,
		StackName: settings.stackName + "-idempotency",
		ParameterOverrides: map[string]json.RawMessage{
			"resourcePrefix":      json.RawMessage(strconv.Quote(settings.resourcePrefix)),
			"includeAuditStorage": json.RawMessage("false"),
		},
	}
	deployment, err := session.Deploy(ctx, options)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cleanupCancel()
		if err := deployment.Teardown(cleanupCtx); err != nil {
			t.Errorf("tear down deployment: %v", err)
		}
	}()
	if !deployment.Succeeded {
		t.Fatalf("initial deployment failed: %s", deployment.ErrorMessage)
	}
	primaryID, ok := deployment.Outputs["primaryStorageId"].(string)
	if !ok {
		t.Fatal("primaryStorageId must be a string")
	}
	second, err := session.Deploy(ctx, options)
	if err != nil {
		t.Fatal(err)
	}
	deployment = second
	if !deployment.Succeeded {
		t.Fatalf("identical redeployment failed: %s", deployment.ErrorMessage)
	}
	if deployment.Outputs["primaryStorageId"] != primaryID {
		t.Fatal("identical redeployment changed resource identity")
	}
	status, storage := getAzureResource(t, ctx, credential, primaryID)
	if status != http.StatusOK {
		t.Fatalf("get storage status = %d, want 200", status)
	}
	properties, ok := storage["properties"].(map[string]any)
	if !ok || properties["allowBlobPublicAccess"] != false || properties["minimumTlsVersion"] != "TLS1_2" ||
		properties["publicNetworkAccess"] != "Disabled" {
		t.Fatalf("security settings changed after redeployment: %#v", properties)
	}
}

func testTemplateValidatesInAzure(t *testing.T, session *biceptesting.LiveSession, settings liveSettings) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	validation := validateStorage(t, ctx, session, settings, settings.stackName+"-validation", false)
	if !validation.IsValid {
		t.Fatalf("validation failed: %s", validation.ErrorMessage)
	}
	found := false
	for _, resource := range validation.Resources {
		if resource.Type == "Microsoft.Storage/storageAccounts" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("validated resources did not include storage accounts: %#v", validation.Resources)
	}
}

func testSecureStorageIsVerifiedInAzureAndRemoved(t *testing.T, session *biceptesting.LiveSession, credential azcore.TokenCredential, settings liveSettings) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	deployment := deployStorage(t, ctx, session, settings, settings.stackName+"-secure", false)
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cleanupCancel()
		if err := deployment.Teardown(cleanupCtx); err != nil {
			t.Errorf("tear down deployment: %v", err)
		}
	}()

	if !deployment.Succeeded {
		t.Fatalf("deployment failed: %s", deployment.ErrorMessage)
	}
	primaryStorageID := deployment.Outputs["primaryStorageId"].(string)
	status, storage := getAzureResource(t, ctx, credential, primaryStorageID)
	if status != http.StatusOK {
		t.Fatalf("get storage account status = %d, want 200", status)
	}
	properties := storage["properties"].(map[string]any)
	if properties["allowBlobPublicAccess"] != false || properties["allowSharedKeyAccess"] != false ||
		properties["minimumTlsVersion"] != "TLS1_2" || properties["publicNetworkAccess"] != "Disabled" ||
		properties["supportsHttpsTrafficOnly"] != true {
		t.Errorf("storage security settings did not match: %#v", properties)
	}

}

func testDeploymentReconcilesRemovedAuditStorageAndCleansUp(t *testing.T, session *biceptesting.LiveSession, credential azcore.TokenCredential, settings liveSettings) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	deployment := deployStorage(t, ctx, session, settings, settings.stackName, true)
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cleanupCancel()
		if err := deployment.Teardown(cleanupCtx); err != nil {
			t.Errorf("tear down deployment: %v", err)
		}
	}()
	if !deployment.Succeeded {
		t.Fatalf("initial deployment failed: %s", deployment.ErrorMessage)
	}
	primaryStorageID := deployment.Outputs["primaryStorageId"].(string)
	auditStorageID := deployment.Outputs["auditStorageId"].(string)

	deployment = deployStorage(t, ctx, session, settings, settings.stackName, false)
	if !deployment.Succeeded {
		t.Fatalf("updated deployment failed: %s", deployment.ErrorMessage)
	}
	if deployment.Outputs["primaryStorageId"] != primaryStorageID {
		t.Error("disabling audit storage must preserve the primary account")
	}
	if status, _ := getAzureResource(t, ctx, credential, primaryStorageID); status != http.StatusOK {
		t.Errorf("primary storage status = %d, want 200", status)
	}
	if status, _ := getAzureResource(t, ctx, credential, auditStorageID); status != http.StatusNotFound {
		t.Errorf("audit storage status after reconciliation = %d, want 404", status)
	}
}

type liveSettings struct {
	subscriptionID string
	resourceGroup  string
	stackName      string
	resourcePrefix string
}

func newLiveTestSession(t *testing.T, credential azcore.TokenCredential) *biceptesting.LiveSession {
	t.Helper()
	session, err := biceptesting.NewLiveSession(context.Background(), "0.46.1", credential)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Errorf("close live session: %v", err)
		}
	})
	return session
}

func loadLiveSettings(t *testing.T) liveSettings {
	t.Helper()
	return liveSettings{
		subscriptionID: requireEnvironmentVariable(t, "AZURE_SUBSCRIPTION_ID"),
		resourceGroup:  requireEnvironmentVariable(t, "AZURE_RESOURCE_GROUP"),
		stackName:      requireEnvironmentVariable(t, "BICEP_TEST_STACK_NAME"),
		resourcePrefix: requireEnvironmentVariable(t, "BICEP_TEST_RESOURCE_PREFIX"),
	}
}

func validateStorage(t *testing.T, ctx context.Context, session *biceptesting.LiveSession, settings liveSettings, stackName string, includeAudit bool) *biceptesting.ValidateResult {
	t.Helper()
	parametersPath, err := filepath.Abs(filepath.Join("..", "infra", "live-storage", "main.bicepparam"))
	if err != nil {
		t.Fatal(err)
	}
	validation, err := session.Validate(ctx, biceptesting.DeployOptions{
		FilePath: parametersPath, SubscriptionID: settings.subscriptionID, ResourceGroup: settings.resourceGroup, StackName: stackName,
		ParameterOverrides: map[string]json.RawMessage{
			"resourcePrefix":      json.RawMessage(strconv.Quote(settings.resourcePrefix)),
			"includeAuditStorage": json.RawMessage(strconv.FormatBool(includeAudit)),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return validation
}

func deployStorage(t *testing.T, ctx context.Context, session *biceptesting.LiveSession, settings liveSettings, stackName string, includeAudit bool) *biceptesting.DeployResult {
	t.Helper()
	validation := validateStorage(t, ctx, session, settings, stackName+"-validate", includeAudit)
	if !validation.IsValid {
		t.Fatalf("validation failed: %s", validation.ErrorMessage)
	}

	parametersPath, err := filepath.Abs(filepath.Join("..", "infra", "live-storage", "main.bicepparam"))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := session.Deploy(ctx, biceptesting.DeployOptions{
		FilePath: parametersPath, SubscriptionID: settings.subscriptionID, ResourceGroup: settings.resourceGroup, StackName: stackName,
		ParameterOverrides: map[string]json.RawMessage{
			"resourcePrefix":      json.RawMessage(strconv.Quote(settings.resourcePrefix)),
			"includeAuditStorage": json.RawMessage(strconv.FormatBool(includeAudit)),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return deployment
}

func getAzureResource(t *testing.T, ctx context.Context, credential azcore.TokenCredential, resourceID string) (int, map[string]any) {
	t.Helper()
	token, err := credential.GetToken(ctx, policy.TokenRequestOptions{Scopes: []string{"https://management.azure.com/.default"}})
	if err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("https://management.azure.com%s?api-version=2023-05-01", resourceID), nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+token.Token)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var body map[string]any
	if response.StatusCode == http.StatusOK {
		if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
	}
	return response.StatusCode, body
}

func requireEnvironmentVariable(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Skipf("set %s to run the live deployment samples", name)
	}
	return value
}
