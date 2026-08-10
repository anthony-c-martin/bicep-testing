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
	t.Run("secure storage is verified in Azure and removed", func(t *testing.T) {
		testSecureStorageIsVerifiedInAzureAndRemoved(t, session, credential, settings)
	})
	t.Run("removed audit storage is reconciled and remaining resources are cleaned up", func(t *testing.T) {
		testDeploymentReconcilesRemovedAuditStorageAndCleansUp(t, session, credential, settings)
	})
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
	primaryStorageID := deployment.Outputs["primaryStorageId"].(string)
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cleanupCancel()
		if err := deployment.Teardown(cleanupCtx); err != nil {
			t.Errorf("tear down deployment: %v", err)
		}
	}()

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

	if err := deployment.Teardown(ctx); err != nil {
		t.Fatal(err)
	}
	status, _ = getAzureResource(t, ctx, credential, primaryStorageID)
	if status != http.StatusNotFound {
		t.Errorf("storage account status after teardown = %d, want 404", status)
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
	primaryStorageID := deployment.Outputs["primaryStorageId"].(string)
	auditStorageID := deployment.Outputs["auditStorageId"].(string)
	if len(deployment.Resources) != 2 {
		t.Fatalf("initial resources = %d, want 2", len(deployment.Resources))
	}

	deployment = deployStorage(t, ctx, session, settings, settings.stackName, false)
	if len(deployment.Resources) != 1 || deployment.Resources[0].ID != primaryStorageID {
		t.Errorf("unexpected reconciled resources: %#v", deployment.Resources)
	}
	if status, _ := getAzureResource(t, ctx, credential, auditStorageID); status != http.StatusNotFound {
		t.Errorf("audit storage status after reconciliation = %d, want 404", status)
	}
	if err := deployment.Teardown(ctx); err != nil {
		t.Fatal(err)
	}
	if status, _ := getAzureResource(t, ctx, credential, primaryStorageID); status != http.StatusNotFound {
		t.Errorf("primary storage status after teardown = %d, want 404", status)
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
	if !deployment.Succeeded {
		t.Fatalf("deployment failed: %s", deployment.ErrorMessage)
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
