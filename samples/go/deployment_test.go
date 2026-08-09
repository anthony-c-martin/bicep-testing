package sample_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	biceptest "github.com/anthony-c-martin/bicep-test/packages/go"
)

func TestInfrastructureDeploysAndIsRemovedAfterward(t *testing.T) {
	subscriptionID := requireEnvironmentVariable(t, "AZURE_SUBSCRIPTION_ID")
	resourceGroup := requireEnvironmentVariable(t, "AZURE_RESOURCE_GROUP")
	stackName := requireEnvironmentVariable(t, "BICEP_TEST_STACK_NAME")
	resourcePrefix := requireEnvironmentVariable(t, "BICEP_TEST_RESOURCE_PREFIX")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	credential, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		t.Fatal(err)
	}
	session, err := biceptest.NewSession(ctx, "0.43.1")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Errorf("close session: %v", err)
		}
	})
	parametersPath, err := filepath.Abs(filepath.Join("..", "infra", "main.bicepparam"))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := session.Deploy(ctx, credential, biceptest.DeployOptions{
		FilePath:       parametersPath,
		SubscriptionID: subscriptionID,
		ResourceGroup:  resourceGroup,
		StackName:      stackName,
		ParameterOverrides: map[string]json.RawMessage{
			"env": json.RawMessage(strconv.Quote(resourcePrefix)),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := deployment.Teardown(context.Background()); err != nil {
			t.Errorf("tear down deployment: %v", err)
		}
	})

	foundStorageAccount := false
	for _, resource := range deployment.Resources {
		foundStorageAccount = foundStorageAccount || resource.Type == "Microsoft.Storage/storageAccounts"
	}
	if !foundStorageAccount {
		t.Error("deployment did not manage a storage account")
	}
	if deployment.Outputs["primaryStorageId"] == nil {
		t.Error("deployment did not return primaryStorageId")
	}
}

func requireEnvironmentVariable(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Skipf("set %s to run the live deployment sample", name)
	}
	return value
}
