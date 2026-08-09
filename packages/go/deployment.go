package biceptesting

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armdeploymentstacks"
	"github.com/anthony-c-martin/bicep-testing/packages/go/rpcclient"
)

// DeployOptions identifies a Bicep parameters file and its resource-group Deployment Stack.
type DeployOptions struct {
	FilePath           string
	SubscriptionID     string
	ResourceGroup      string
	StackName          string
	ParameterOverrides map[string]json.RawMessage
}

// DeploymentResource identifies a resource managed by a Deployment Stack.
type DeploymentResource struct {
	ID   string
	Type string
}

// DeployResult contains deployed outputs and resources and owns their cleanup.
type DeployResult struct {
	Outputs   map[string]any
	Resources []DeploymentResource

	client        deploymentStackClient
	resourceGroup string
	stackName     string
	teardownOnce  sync.Once
	teardownErr   error
}

type deploymentStackClient interface {
	createOrUpdate(context.Context, string, string, armdeploymentstacks.DeploymentStack) (armdeploymentstacks.DeploymentStack, error)
	delete(context.Context, string, string) error
}

type azureDeploymentStackClient struct {
	client *armdeploymentstacks.Client
}

var newDeploymentStackClient = func(subscriptionID string, credential azcore.TokenCredential) (deploymentStackClient, error) {
	client, err := armdeploymentstacks.NewClient(subscriptionID, credential, nil)
	if err != nil {
		return nil, err
	}
	return &azureDeploymentStackClient{client: client}, nil
}

// Deploy compiles and deploys a Bicep parameters file as a resource-group Deployment Stack.
func (session *Session) Deploy(ctx context.Context, credential azcore.TokenCredential, options DeployOptions) (*DeployResult, error) {
	if credential == nil {
		return nil, errors.New("credential must not be nil")
	}
	if options.FilePath == "" || options.SubscriptionID == "" || options.ResourceGroup == "" || options.StackName == "" {
		return nil, errors.New("file path, subscription ID, resource group, and stack name must not be empty")
	}
	absolutePath, err := filepath.Abs(options.FilePath)
	if err != nil {
		return nil, fmt.Errorf("resolve Bicep parameters file path: %w", err)
	}
	compilation, err := session.client.CompileParams(ctx, rpcclient.CompileParamsRequest{
		Path:               absolutePath,
		ParameterOverrides: options.ParameterOverrides,
	})
	if err != nil {
		return nil, err
	}
	if !compilation.Success || compilation.Template == "" || compilation.Parameters == "" {
		diagnostics, _ := json.Marshal(compilation.Diagnostics)
		return nil, fmt.Errorf("Bicep parameter compilation failed: %s", diagnostics)
	}

	var template any
	if err := json.Unmarshal([]byte(compilation.Template), &template); err != nil {
		return nil, fmt.Errorf("decode compiled template: %w", err)
	}
	var parameterFile struct {
		Parameters map[string]struct {
			Value     any `json:"value"`
			Reference *struct {
				KeyVault struct {
					ID string `json:"id"`
				} `json:"keyVault"`
				SecretName    string  `json:"secretName"`
				SecretVersion *string `json:"secretVersion"`
			} `json:"reference"`
		} `json:"parameters"`
	}
	if err := json.Unmarshal([]byte(compilation.Parameters), &parameterFile); err != nil {
		return nil, fmt.Errorf("decode compiled parameters: %w", err)
	}
	parameters := make(map[string]*armdeploymentstacks.DeploymentParameter, len(parameterFile.Parameters))
	for name, parameter := range parameterFile.Parameters {
		deploymentParameter := &armdeploymentstacks.DeploymentParameter{Value: parameter.Value}
		if parameter.Reference != nil {
			deploymentParameter.Reference = &armdeploymentstacks.KeyVaultParameterReference{
				KeyVault:      &armdeploymentstacks.KeyVaultReference{ID: &parameter.Reference.KeyVault.ID},
				SecretName:    &parameter.Reference.SecretName,
				SecretVersion: parameter.Reference.SecretVersion,
			}
		}
		parameters[name] = deploymentParameter
	}

	client, err := newDeploymentStackClient(options.SubscriptionID, credential)
	if err != nil {
		return nil, fmt.Errorf("create Deployment Stacks client: %w", err)
	}
	deleteAction := armdeploymentstacks.DeploymentStacksDeleteDetachEnumDelete
	denyMode := armdeploymentstacks.DenySettingsModeNone
	stack, err := client.createOrUpdate(ctx, options.ResourceGroup, options.StackName, armdeploymentstacks.DeploymentStack{
		Properties: &armdeploymentstacks.DeploymentStackProperties{
			ActionOnUnmanage: &armdeploymentstacks.ActionOnUnmanage{
				Resources:        &deleteAction,
				ResourceGroups:   &deleteAction,
				ManagementGroups: &deleteAction,
			},
			DenySettings: &armdeploymentstacks.DenySettings{Mode: &denyMode},
			Parameters:   parameters,
			Template:     template,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("deploy stack: %w", err)
	}
	if stack.Properties == nil {
		return nil, errors.New("Deployment Stack response did not include properties")
	}

	result := &DeployResult{
		Outputs:       deploymentOutputs(stack.Properties.Outputs),
		client:        client,
		resourceGroup: options.ResourceGroup,
		stackName:     options.StackName,
	}
	for _, resource := range stack.Properties.Resources {
		if resource != nil && resource.ID != nil {
			result.Resources = append(result.Resources, DeploymentResource{ID: *resource.ID, Type: resourceType(*resource.ID)})
		}
	}
	return result, nil
}

// Teardown deletes the Deployment Stack and all resources it manages. Repeated calls return the first result.
func (result *DeployResult) Teardown(ctx context.Context) error {
	result.teardownOnce.Do(func() {
		result.teardownErr = result.client.delete(ctx, result.resourceGroup, result.stackName)
	})
	return result.teardownErr
}

func (client *azureDeploymentStackClient) createOrUpdate(ctx context.Context, resourceGroup, stackName string, stack armdeploymentstacks.DeploymentStack) (armdeploymentstacks.DeploymentStack, error) {
	poller, err := client.client.BeginCreateOrUpdateAtResourceGroup(ctx, resourceGroup, stackName, stack, nil)
	if err != nil {
		return armdeploymentstacks.DeploymentStack{}, err
	}
	response, err := poller.PollUntilDone(ctx, &runtime.PollUntilDoneOptions{})
	return response.DeploymentStack, err
}

func (client *azureDeploymentStackClient) delete(ctx context.Context, resourceGroup, stackName string) error {
	deleteManagementGroups := armdeploymentstacks.UnmanageActionManagementGroupModeDelete
	deleteResourceGroups := armdeploymentstacks.UnmanageActionResourceGroupModeDelete
	deleteResources := armdeploymentstacks.UnmanageActionResourceModeDelete
	poller, err := client.client.BeginDeleteAtResourceGroup(ctx, resourceGroup, stackName, &armdeploymentstacks.ClientBeginDeleteAtResourceGroupOptions{
		UnmanageActionManagementGroups: &deleteManagementGroups,
		UnmanageActionResourceGroups:   &deleteResourceGroups,
		UnmanageActionResources:        &deleteResources,
	})
	if err != nil {
		return err
	}
	_, err = poller.PollUntilDone(ctx, &runtime.PollUntilDoneOptions{})
	return err
}

func deploymentOutputs(value any) map[string]any {
	outputs, ok := value.(map[string]any)
	if !ok {
		return map[string]any{}
	}
	result := make(map[string]any, len(outputs))
	for name, output := range outputs {
		if object, ok := output.(map[string]any); ok {
			if outputValue, found := object["value"]; found {
				result[name] = outputValue
				continue
			}
		}
		result[name] = output
	}
	return result
}

func resourceType(id string) string {
	parts := strings.Split(strings.Trim(id, "/"), "/")
	for index, part := range parts {
		if strings.EqualFold(part, "providers") && index+2 < len(parts) {
			typeParts := []string{parts[index+1]}
			for resourceIndex := index + 2; resourceIndex < len(parts); resourceIndex += 2 {
				typeParts = append(typeParts, parts[resourceIndex])
			}
			return strings.Join(typeParts, "/")
		}
	}
	return ""
}
