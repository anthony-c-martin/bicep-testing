package biceptesting

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armdeploymentstacks"
	biceprpcclient "github.com/anthony-c-martin/bicep-testing/packages/go/bicep-rpc-client"
)

func TestLiveDeploySupportsAllScopes(t *testing.T) {
	tests := []struct {
		name                       string
		options                    DeployOptions
		expectedScope              deploymentScope
		expectedSubscription       string
		expectedResourceGroup      string
		expectedManagementGroup    string
		expectedLocation           string
		expectedClientSubscription string
	}{
		{
			name: "resource group",
			options: DeployOptions{
				FilePath:       "main.bicepparam",
				SubscriptionID: subscriptionID,
				ResourceGroup:  resourceGroup,
				Location:       "westus",
			},
			expectedScope:              deploymentScopeResourceGroup,
			expectedSubscription:       subscriptionID,
			expectedResourceGroup:      resourceGroup,
			expectedLocation:           "westus",
			expectedClientSubscription: subscriptionID,
		},
		{
			name: "subscription",
			options: DeployOptions{
				FilePath:       "main.bicepparam",
				SubscriptionID: subscriptionID,
				Location:       "eastus2",
			},
			expectedScope:              deploymentScopeSubscription,
			expectedSubscription:       subscriptionID,
			expectedLocation:           "eastus2",
			expectedClientSubscription: subscriptionID,
		},
		{
			name: "management group",
			options: DeployOptions{
				FilePath:          "main.bicepparam",
				ManagementGroupID: "mg-ops",
				Location:          "centralus",
			},
			expectedScope:              deploymentScopeManagementGroup,
			expectedManagementGroup:    "mg-ops",
			expectedLocation:           "centralus",
			expectedClientSubscription: "",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			stackClient := &fakeDeploymentStackClient{}
			var actualClientSubscription string
			originalFactory := newDeploymentStackClient
			newDeploymentStackClient = func(actualSubscriptionID string, credential azcore.TokenCredential) (deploymentStackClient, error) {
				actualClientSubscription = actualSubscriptionID
				return stackClient, nil
			}
			t.Cleanup(func() { newDeploymentStackClient = originalFactory })

			bicep := &fakeBicepClient{compilation: successfulCompilation()}
			session := &LiveSession{session: &Session{client: bicep}, credential: fakeCredential{}}

			result, err := session.Deploy(context.Background(), test.options)
			if err != nil {
				t.Fatalf("Deploy returned an error: %v", err)
			}
			if result == nil || !result.Succeeded {
				t.Fatalf("Deploy result = %#v, want success", result)
			}
			if actualClientSubscription != test.expectedClientSubscription {
				t.Errorf("client subscription = %q, want %q", actualClientSubscription, test.expectedClientSubscription)
			}
			if stackClient.target.scope != test.expectedScope {
				t.Errorf("scope = %v, want %v", stackClient.target.scope, test.expectedScope)
			}
			if stackClient.target.subscriptionID != test.expectedSubscription {
				t.Errorf("subscription ID = %q, want %q", stackClient.target.subscriptionID, test.expectedSubscription)
			}
			if stackClient.target.resourceGroup != test.expectedResourceGroup {
				t.Errorf("resource group = %q, want %q", stackClient.target.resourceGroup, test.expectedResourceGroup)
			}
			if stackClient.target.managementGroupID != test.expectedManagementGroup {
				t.Errorf("management group = %q, want %q", stackClient.target.managementGroupID, test.expectedManagementGroup)
			}
			if stackClient.stack.Location == nil || *stackClient.stack.Location != test.expectedLocation {
				t.Errorf("stack location = %v, want %q", stackClient.stack.Location, test.expectedLocation)
			}
			if !regexp.MustCompile(`^bicep-test-[0-9a-f]{32}$`).MatchString(stackClient.stackName) {
				t.Errorf("default stack name = %q, want bicep-test-<32 hex chars>", stackClient.stackName)
			}
		})
	}
}

func TestLiveDeployCompilesAndPreservesParameters(t *testing.T) {
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
		Parameters: `{"parameters":{"message":{"value":"hello"},"optionalValue":{"value":null},"secret":{"reference":{"keyVault":{"id":"/subscriptions/sub/resourceGroups/rg/providers/Microsoft.KeyVault/vaults/test"},"secretName":"password","secretVersion":"v1"}}}}`,
	}}
	session := &LiveSession{session: &Session{client: bicep}, credential: fakeCredential{}}
	overrides := map[string]json.RawMessage{"message": json.RawMessage(`"override"`)}
	result, err := session.Deploy(context.Background(), DeployOptions{
		FilePath:           "main.bicepparam",
		SubscriptionID:     subscriptionID,
		ResourceGroup:      resourceGroup,
		StackName:          "test-stack-parameters",
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
	if stackClient.target.resourceGroup != resourceGroup || stackClient.stackName != "test-stack-parameters" {
		t.Errorf("stack target = %s/%s, want %s/test-stack-parameters", stackClient.target.resourceGroup, stackClient.stackName, resourceGroup)
	}
	if actual := stackClient.stack.Properties.Parameters["message"].Value; actual != "hello" {
		t.Errorf("deployment parameter = %v, want hello", actual)
	}
	if actual := stackClient.stack.Properties.Parameters["optionalValue"].Value; !azcore.IsNullValue(actual) {
		t.Errorf("optional deployment parameter = %#v, want explicit null", actual)
	}
	encodedNull, err := json.Marshal(stackClient.stack.Properties.Parameters["optionalValue"])
	if err != nil {
		t.Fatalf("marshal null deployment parameter: %v", err)
	}
	if string(encodedNull) != `{"value":null}` {
		t.Errorf("null deployment parameter = %s, want explicit null value", encodedNull)
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
}

func TestValidateReturnsRichErrorResult(t *testing.T) {
	stackClient := &fakeDeploymentStackClient{
		validation: armdeploymentstacks.DeploymentStackValidateResult{
			Error: &armdeploymentstacks.ErrorDetail{
				Code:    stringPointer("InvalidTemplate"),
				Message: stringPointer("The template is invalid."),
			},
			Properties: &armdeploymentstacks.DeploymentStackValidateProperties{
				CorrelationID: stringPointer("00000000-0000-0000-0000-000000000001"),
			},
		},
	}
	originalFactory := newDeploymentStackClient
	newDeploymentStackClient = func(string, azcore.TokenCredential) (deploymentStackClient, error) {
		return stackClient, nil
	}
	t.Cleanup(func() { newDeploymentStackClient = originalFactory })

	session := &LiveSession{session: &Session{client: &fakeBicepClient{compilation: successfulCompilation()}}, credential: fakeCredential{}}

	result, err := session.Validate(context.Background(), DeployOptions{
		FilePath:       "main.bicepparam",
		SubscriptionID: subscriptionID,
		ResourceGroup:  resourceGroup,
		StackName:      "validate-stack",
	})
	if err != nil {
		t.Fatalf("Validate returned an error: %v", err)
	}
	if result.IsValid {
		t.Fatalf("IsValid = true, want false")
	}
	if result.CorrelationID != "00000000-0000-0000-0000-000000000001" {
		t.Errorf("correlation ID = %q", result.CorrelationID)
	}
	if result.Error == nil || result.Error.Code != "InvalidTemplate" || result.Error.Message != "The template is invalid." {
		t.Fatalf("error = %#v, want code/message from validation", result.Error)
	}
	if !strings.Contains(string(result.Error.RawData), "InvalidTemplate") {
		t.Errorf("raw data = %s, want serialized service error", result.Error.RawData)
	}
}

func TestDeployReturnsFailedResultAfterSubmissionAndKeepsCleanup(t *testing.T) {
	body := `{"error":{"code":"DeploymentStackOutOfSync","message":"The stack is out of sync.","details":[{"code":"ManagedResourceFailure","message":"A managed resource failed."}]}}`
	stackClient := &fakeDeploymentStackClient{
		createError: &azcore.ResponseError{
			ErrorCode:  "DeploymentStackOutOfSync",
			StatusCode: http.StatusConflict,
			RawResponse: &http.Response{
				StatusCode: http.StatusConflict,
				Body:       io.NopCloser(strings.NewReader(body)),
			},
		},
	}
	originalFactory := newDeploymentStackClient
	newDeploymentStackClient = func(string, azcore.TokenCredential) (deploymentStackClient, error) {
		return stackClient, nil
	}
	t.Cleanup(func() { newDeploymentStackClient = originalFactory })

	session := &LiveSession{session: &Session{client: &fakeBicepClient{compilation: successfulCompilation()}}, credential: fakeCredential{}}
	result, err := session.Deploy(context.Background(), DeployOptions{
		FilePath:       "main.bicepparam",
		SubscriptionID: subscriptionID,
		ResourceGroup:  resourceGroup,
		StackName:      "failed-stack",
	})
	if err != nil {
		t.Fatalf("Deploy returned an error: %v", err)
	}
	if result.Succeeded {
		t.Fatalf("Succeeded = true, want false")
	}
	if result.ErrorCode != "DeploymentStackOutOfSync" || result.ErrorMessage != "The stack is out of sync." {
		t.Fatalf("error fields = %#v", result)
	}
	if result.Error == nil || !strings.Contains(string(result.Error.RawData), "ManagedResourceFailure") {
		t.Fatalf("error = %#v, want raw service body", result.Error)
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

func TestDeployRejectsBadOptionsAndPresubmissionFailures(t *testing.T) {
	t.Run("bad options", func(t *testing.T) {
		bicep := &fakeBicepClient{compilation: successfulCompilation()}
		session := &LiveSession{session: &Session{client: bicep}, credential: fakeCredential{}}

		_, err := session.Deploy(context.Background(), DeployOptions{FilePath: "main.bicepparam", SubscriptionID: subscriptionID})
		if err == nil || !strings.Contains(err.Error(), "location is required") {
			t.Fatalf("error = %v, want location validation", err)
		}
		if bicep.compileCalls != 0 {
			t.Errorf("compile calls = %d, want 0", bicep.compileCalls)
		}
	})

	t.Run("compile failure", func(t *testing.T) {
		bicep := &fakeBicepClient{compilation: biceprpcclient.CompileParamsResponse{Success: false, Diagnostics: []map[string]any{{"level": "Error", "code": "BCP001", "message": "Invalid Bicep."}}}}
		session := &LiveSession{session: &Session{client: bicep}, credential: fakeCredential{}}

		_, err := session.Deploy(context.Background(), DeployOptions{FilePath: "main.bicepparam", SubscriptionID: subscriptionID, ResourceGroup: resourceGroup, StackName: "x"})
		if err == nil || !strings.Contains(err.Error(), "BCP001") {
			t.Fatalf("error = %v, want compile diagnostics", err)
		}
	})

	t.Run("client construction failure", func(t *testing.T) {
		originalFactory := newDeploymentStackClient
		newDeploymentStackClient = func(string, azcore.TokenCredential) (deploymentStackClient, error) {
			return nil, errors.New("factory failure")
		}
		t.Cleanup(func() { newDeploymentStackClient = originalFactory })

		session := &LiveSession{session: &Session{client: &fakeBicepClient{compilation: successfulCompilation()}}, credential: fakeCredential{}}
		_, err := session.Deploy(context.Background(), DeployOptions{FilePath: "main.bicepparam", SubscriptionID: subscriptionID, ResourceGroup: resourceGroup, StackName: "x"})
		if err == nil || !strings.Contains(err.Error(), "factory failure") {
			t.Fatalf("error = %v, want factory failure", err)
		}
	})
}

func TestLiveSessionForwardsSnapshotAndClose(t *testing.T) {
	t.Parallel()

	bicep := &fakeBicepClient{snapshot: biceprpcclient.GetSnapshotResponse{Snapshot: `{"predictedResources":[],"diagnostics":[],"outputs":{}}`}}
	live := &LiveSession{session: &Session{client: bicep}, credential: fakeCredential{}}

	_, err := live.Snapshot(context.Background(), "main.bicepparam", SnapshotMetadata{TenantID: tenantID})
	if err != nil {
		t.Fatalf("Snapshot returned an error: %v", err)
	}
	if err := live.Close(); err != nil {
		t.Fatalf("Close returned an error: %v", err)
	}
	if bicep.closed != 1 {
		t.Errorf("close calls = %d, want 1", bicep.closed)
	}
}

func TestTeardownRetriesAfterFailure(t *testing.T) {
	t.Parallel()

	stackClient := &fakeDeploymentStackClient{deleteErrors: []error{context.Canceled, nil}}
	result := &DeployResult{client: stackClient, target: deploymentTarget{scope: deploymentScopeResourceGroup, resourceGroup: resourceGroup}, stackName: "test-stack"}

	if err := result.Teardown(context.Background()); !errors.Is(err, context.Canceled) {
		t.Fatalf("first Teardown error = %v, want context canceled", err)
	}
	if err := result.Teardown(context.Background()); err != nil {
		t.Fatalf("second Teardown returned an error: %v", err)
	}
	if err := result.Teardown(context.Background()); err != nil {
		t.Fatalf("third Teardown returned an error: %v", err)
	}
	if stackClient.deleteCalls != 2 {
		t.Errorf("delete calls = %d, want 2", stackClient.deleteCalls)
	}
}

func TestTeardownSharesActiveDeletionAndAllowsWaiterCancellation(t *testing.T) {
	t.Parallel()

	stackClient := &blockingDeploymentStackClient{
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	result := &DeployResult{client: stackClient, target: deploymentTarget{scope: deploymentScopeResourceGroup, resourceGroup: resourceGroup}, stackName: "test-stack"}
	firstDone := make(chan error, 1)
	go func() {
		firstDone <- result.Teardown(context.Background())
	}()
	<-stackClient.started

	waiterCtx, cancelWaiter := context.WithCancel(context.Background())
	waiterDone := make(chan error, 1)
	go func() {
		waiterDone <- result.Teardown(waiterCtx)
	}()
	cancelWaiter()
	if err := <-waiterDone; !errors.Is(err, context.Canceled) {
		t.Fatalf("waiting Teardown error = %v, want context canceled", err)
	}
	if calls := stackClient.deleteCalls.Load(); calls != 1 {
		t.Fatalf("delete calls while active = %d, want 1", calls)
	}

	close(stackClient.release)
	if err := <-firstDone; err != nil {
		t.Fatalf("active Teardown returned an error: %v", err)
	}
	if err := result.Teardown(context.Background()); err != nil {
		t.Fatalf("completed Teardown returned an error: %v", err)
	}
	if calls := stackClient.deleteCalls.Load(); calls != 1 {
		t.Errorf("delete calls = %d, want 1", calls)
	}
}

func TestTeardownTreatsNotFoundAsSuccess(t *testing.T) {
	t.Parallel()

	stackClient := &fakeDeploymentStackClient{deleteErrors: []error{&azcore.ResponseError{StatusCode: http.StatusNotFound}}}
	result := &DeployResult{client: stackClient, target: deploymentTarget{scope: deploymentScopeResourceGroup, resourceGroup: resourceGroup}, stackName: "missing-stack"}

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

func TestNewLiveSessionRejectsNilCredential(t *testing.T) {
	t.Parallel()

	_, err := NewLiveSession(context.Background(), "0.46.1", nil)
	if err == nil || !strings.Contains(err.Error(), "credential") {
		t.Fatalf("error = %v, want credential validation", err)
	}
}

type fakeCredential struct{}

func (fakeCredential) GetToken(context.Context, policy.TokenRequestOptions) (azcore.AccessToken, error) {
	return azcore.AccessToken{Token: "token", ExpiresOn: time.Now().Add(time.Hour)}, nil
}

type fakeBicepClient struct {
	request      biceprpcclient.CompileParamsRequest
	compilation  biceprpcclient.CompileParamsResponse
	compileCalls int
	snapshot     biceprpcclient.GetSnapshotResponse
	closed       int
}

func (client *fakeBicepClient) CompileParams(_ context.Context, request biceprpcclient.CompileParamsRequest) (biceprpcclient.CompileParamsResponse, error) {
	client.request = request
	client.compileCalls++
	return client.compilation, nil
}

func (client *fakeBicepClient) GetSnapshot(context.Context, biceprpcclient.GetSnapshotRequest) (biceprpcclient.GetSnapshotResponse, error) {
	return client.snapshot, nil
}

func (client *fakeBicepClient) Close() error {
	client.closed++
	return nil
}

type fakeDeploymentStackClient struct {
	target       deploymentTarget
	stackName    string
	stack        armdeploymentstacks.DeploymentStack
	validation   armdeploymentstacks.DeploymentStackValidateResult
	createError  error
	deleteCalls  int
	deleteErrors []error
}

func (client *fakeDeploymentStackClient) createOrUpdate(_ context.Context, target deploymentTarget, stackName string, stack armdeploymentstacks.DeploymentStack) (armdeploymentstacks.DeploymentStack, error) {
	client.target = target
	client.stackName = stackName
	client.stack = stack
	if client.createError != nil {
		return armdeploymentstacks.DeploymentStack{}, client.createError
	}
	resourceID := "/subscriptions/sub/resourceGroups/rg/providers/Microsoft.Storage/storageAccounts/example"
	return armdeploymentstacks.DeploymentStack{Properties: &armdeploymentstacks.DeploymentStackProperties{
		Outputs: map[string]any{"endpoint": map[string]any{"value": "https://example.test"}},
		Resources: []*armdeploymentstacks.ManagedResourceReference{
			{ID: &resourceID},
		},
	}}, nil
}

func (client *fakeDeploymentStackClient) validate(_ context.Context, target deploymentTarget, stackName string, stack armdeploymentstacks.DeploymentStack) (armdeploymentstacks.DeploymentStackValidateResult, error) {
	client.target = target
	client.stackName = stackName
	client.stack = stack
	if client.validation.Properties == nil {
		client.validation.Properties = &armdeploymentstacks.DeploymentStackValidateProperties{}
	}
	return client.validation, nil
}

func (client *fakeDeploymentStackClient) delete(_ context.Context, target deploymentTarget, stackName string) error {
	client.target = target
	client.stackName = stackName
	client.deleteCalls++
	if len(client.deleteErrors) > 0 {
		err := client.deleteErrors[0]
		client.deleteErrors = client.deleteErrors[1:]
		return err
	}
	return nil
}

type blockingDeploymentStackClient struct {
	started     chan struct{}
	release     chan struct{}
	deleteCalls atomic.Int32
}

func (*blockingDeploymentStackClient) createOrUpdate(context.Context, deploymentTarget, string, armdeploymentstacks.DeploymentStack) (armdeploymentstacks.DeploymentStack, error) {
	return armdeploymentstacks.DeploymentStack{}, nil
}

func (*blockingDeploymentStackClient) validate(context.Context, deploymentTarget, string, armdeploymentstacks.DeploymentStack) (armdeploymentstacks.DeploymentStackValidateResult, error) {
	return armdeploymentstacks.DeploymentStackValidateResult{}, nil
}

func (client *blockingDeploymentStackClient) delete(ctx context.Context, _ deploymentTarget, _ string) error {
	client.deleteCalls.Add(1)
	close(client.started)
	select {
	case <-client.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func successfulCompilation() biceprpcclient.CompileParamsResponse {
	return biceprpcclient.CompileParamsResponse{
		Success:    true,
		Template:   `{"resources":[]}`,
		Parameters: `{"parameters":{"message":{"value":"hello"}}}`,
	}
}

func stringPointer(value string) *string {
	return &value
}
