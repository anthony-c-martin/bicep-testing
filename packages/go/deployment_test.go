package biceptesting

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armdeploymentstacks"
	biceprpcclient "github.com/anthony-c-martin/bicep-testing/packages/go/bicep-rpc-client"
)

func TestDeployCompilesDeploysAndTearsDownOnce(t *testing.T) {
	stackClient := &fakeDeploymentStackClient{}
	originalFactory := newDeploymentStackClient
	newDeploymentStackClient = func(actualSubscriptionID string, credential azcore.TokenCredential) (deploymentStackClient, error) {
		if actualSubscriptionID != subscriptionID {
			t.Errorf("subscription ID = %q, want %q", actualSubscriptionID, subscriptionID)
		}
		return stackClient, nil
	}
	t.Cleanup(func() { newDeploymentStackClient = originalFactory })

	bicep := &fakeBicepClient{compilation: biceprpcclient.CompileParamsResponse{
		Success:    true,
		Template:   `{"resources":[]}`,
		Parameters: `{"parameters":{"message":{"value":"hello"},"secret":{"reference":{"keyVault":{"id":"/subscriptions/sub/resourceGroups/rg/providers/Microsoft.KeyVault/vaults/test"},"secretName":"password","secretVersion":"v1"}}}}`,
	}}
	session := &Session{client: bicep}
	overrides := map[string]json.RawMessage{"message": json.RawMessage(`"override"`)}
	result, err := session.Deploy(context.Background(), fakeCredential{}, DeployOptions{
		FilePath:           "main.bicepparam",
		SubscriptionID:     subscriptionID,
		ResourceGroup:      resourceGroup,
		StackName:          "test-stack",
		ParameterOverrides: overrides,
	})
	if err != nil {
		t.Fatalf("Deploy returned an error: %v", err)
	}
	if !filepath.IsAbs(bicep.request.Path) {
		t.Errorf("compile path = %q, want an absolute path", bicep.request.Path)
	}
	if string(bicep.request.ParameterOverrides["message"]) != `"override"` {
		t.Errorf("parameter override = %s, want override", bicep.request.ParameterOverrides["message"])
	}
	if stackClient.resourceGroup != resourceGroup || stackClient.stackName != "test-stack" {
		t.Errorf("stack target = %s/%s, want %s/test-stack", stackClient.resourceGroup, stackClient.stackName, resourceGroup)
	}
	if actual := stackClient.stack.Properties.Parameters["message"].Value; actual != "hello" {
		t.Errorf("deployment parameter = %v, want hello", actual)
	}
	secret := stackClient.stack.Properties.Parameters["secret"].Reference
	if secret == nil || secret.KeyVault == nil || *secret.KeyVault.ID != "/subscriptions/sub/resourceGroups/rg/providers/Microsoft.KeyVault/vaults/test" || *secret.SecretName != "password" || *secret.SecretVersion != "v1" {
		t.Errorf("secret parameter = %#v, want a Key Vault reference", secret)
	}
	if actual := result.Outputs["endpoint"]; actual != "https://example.test" {
		t.Errorf("endpoint output = %v, want https://example.test", actual)
	}
	if len(result.Resources) != 1 || result.Resources[0].Type != "Microsoft.Storage/storageAccounts" {
		t.Errorf("resources = %#v, want one storage account", result.Resources)
	}

	if err := result.Teardown(context.Background()); err != nil {
		t.Fatalf("Teardown returned an error: %v", err)
	}
	if err := result.Teardown(context.Background()); err != nil {
		t.Fatalf("second Teardown returned an error: %v", err)
	}
	if stackClient.deleteCalls != 1 {
		t.Errorf("delete calls = %d, want 1", stackClient.deleteCalls)
	}
}

type fakeCredential struct{}

func (fakeCredential) GetToken(context.Context, policy.TokenRequestOptions) (azcore.AccessToken, error) {
	return azcore.AccessToken{Token: "token", ExpiresOn: time.Now().Add(time.Hour)}, nil
}

type fakeBicepClient struct {
	request     biceprpcclient.CompileParamsRequest
	compilation biceprpcclient.CompileParamsResponse
}

func (client *fakeBicepClient) CompileParams(_ context.Context, request biceprpcclient.CompileParamsRequest) (biceprpcclient.CompileParamsResponse, error) {
	client.request = request
	return client.compilation, nil
}

func (*fakeBicepClient) GetSnapshot(context.Context, biceprpcclient.GetSnapshotRequest) (biceprpcclient.GetSnapshotResponse, error) {
	return biceprpcclient.GetSnapshotResponse{}, nil
}

func (*fakeBicepClient) Close() error { return nil }

type fakeDeploymentStackClient struct {
	resourceGroup string
	stackName     string
	stack         armdeploymentstacks.DeploymentStack
	deleteCalls   int
}

func (client *fakeDeploymentStackClient) createOrUpdate(_ context.Context, resourceGroup, stackName string, stack armdeploymentstacks.DeploymentStack) (armdeploymentstacks.DeploymentStack, error) {
	client.resourceGroup = resourceGroup
	client.stackName = stackName
	client.stack = stack
	resourceID := "/subscriptions/sub/resourceGroups/rg/providers/Microsoft.Storage/storageAccounts/example"
	return armdeploymentstacks.DeploymentStack{Properties: &armdeploymentstacks.DeploymentStackProperties{
		Outputs: map[string]any{"endpoint": map[string]any{"value": "https://example.test"}},
		Resources: []*armdeploymentstacks.ManagedResourceReference{
			{ID: &resourceID},
		},
	}}, nil
}

func (client *fakeDeploymentStackClient) delete(_ context.Context, resourceGroup, stackName string) error {
	client.deleteCalls++
	return nil
}
