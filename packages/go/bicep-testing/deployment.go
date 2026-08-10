package biceptesting

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armdeploymentstacks"
	biceprpcclient "github.com/anthony-c-martin/bicep-testing/packages/go/bicep-rpc-client"
)

// DeployOptions identifies a Bicep parameters file and its Deployment Stack scope.
type DeployOptions struct {
	FilePath           string
	ManagementGroupID  string
	SubscriptionID     string
	ResourceGroup      string
	Location           string
	StackName          string
	ParameterOverrides map[string]json.RawMessage
}

// DeploymentResource identifies a resource managed by a Deployment Stack.
type DeploymentResource struct {
	ID   string
	Type string
}

// OperationError captures an Azure operation failure.
type OperationError struct {
	Code    string
	Message string
	RawData json.RawMessage
}

// ValidateResult describes a Deployment Stack validation response.
type ValidateResult struct {
	IsValid       bool
	Resources     []DeploymentResource
	CorrelationID string
	Error         *OperationError
	ErrorCode     string
	ErrorMessage  string
}

// DeployResult contains deployment status, outputs, resources, and cleanup ownership.
type DeployResult struct {
	Succeeded    bool
	Error        *OperationError
	ErrorCode    string
	ErrorMessage string
	Outputs      map[string]any
	Resources    []DeploymentResource

	client       deploymentStackClient
	target       deploymentTarget
	stackName    string
	teardownMu   sync.Mutex
	teardownWait chan struct{}
	teardownDone bool
}

type deploymentStackClient interface {
	createOrUpdate(context.Context, deploymentTarget, string, armdeploymentstacks.DeploymentStack) (armdeploymentstacks.DeploymentStack, error)
	validate(context.Context, deploymentTarget, string, armdeploymentstacks.DeploymentStack) (armdeploymentstacks.DeploymentStackValidateResult, error)
	delete(context.Context, deploymentTarget, string) error
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

// LiveSession owns a credential and an offline session for snapshot, validation, and deployment.
type LiveSession struct {
	session    *Session
	credential azcore.TokenCredential
}

// NewLiveSession creates a credential-owning live session.
func NewLiveSession(ctx context.Context, bicepVersion string, credential azcore.TokenCredential) (*LiveSession, error) {
	if credential == nil {
		return nil, errors.New("credential must not be nil")
	}
	session, err := NewSession(ctx, bicepVersion)
	if err != nil {
		return nil, err
	}
	return &LiveSession{session: session, credential: credential}, nil
}

// Snapshot forwards to the owned offline session.
func (session *LiveSession) Snapshot(ctx context.Context, filePath string, metadata SnapshotMetadata) (*SnapshotResult, error) {
	if session == nil || session.session == nil {
		return nil, errors.New("live session must not be nil")
	}
	return session.session.Snapshot(ctx, filePath, metadata)
}

// Validate compiles and validates a Bicep parameters file as a Deployment Stack.
func (session *LiveSession) Validate(ctx context.Context, options DeployOptions) (*ValidateResult, error) {
	if session == nil || session.session == nil {
		return nil, errors.New("live session must not be nil")
	}

	normalized, err := normalizeDeployOptions(options)
	if err != nil {
		return nil, err
	}

	stack, err := session.session.compileDeploymentStack(ctx, normalized)
	if err != nil {
		return nil, err
	}

	client, err := newDeploymentStackClient(clientSubscriptionID(normalized.target), session.credential)
	if err != nil {
		return nil, fmt.Errorf("create Deployment Stacks client: %w", err)
	}

	validation, err := client.validate(ctx, normalized.target, normalized.stackName, stack)
	if err != nil {
		return nil, fmt.Errorf("validate stack: %w", err)
	}

	result := &ValidateResult{
		IsValid:       validation.Error == nil,
		Resources:     validationResources(validation.Properties),
		CorrelationID: validationCorrelationID(validation.Properties),
	}
	if validation.Error != nil {
		operationError := operationErrorFromErrorDetail(validation.Error)
		result.Error = &operationError
		result.ErrorCode = operationError.Code
		result.ErrorMessage = operationError.Message
	}

	return result, nil
}

// Deploy compiles and deploys a Bicep parameters file as a Deployment Stack.
// Pre-submission failures return an error. Post-submission Azure failures return a failed DeployResult.
func (session *LiveSession) Deploy(ctx context.Context, options DeployOptions) (*DeployResult, error) {
	if session == nil || session.session == nil {
		return nil, errors.New("live session must not be nil")
	}

	normalized, err := normalizeDeployOptions(options)
	if err != nil {
		return nil, err
	}

	stack, err := session.session.compileDeploymentStack(ctx, normalized)
	if err != nil {
		return nil, err
	}

	client, err := newDeploymentStackClient(clientSubscriptionID(normalized.target), session.credential)
	if err != nil {
		return nil, fmt.Errorf("create Deployment Stacks client: %w", err)
	}

	result := &DeployResult{
		Succeeded: false,
		Outputs:   map[string]any{},
		Resources: []DeploymentResource{},
		client:    client,
		target:    normalized.target,
		stackName: normalized.stackName,
	}

	stackResult, err := client.createOrUpdate(ctx, normalized.target, normalized.stackName, stack)
	if err != nil {
		operationError := operationErrorFromError(err)
		result.Error = &operationError
		result.ErrorCode = operationError.Code
		result.ErrorMessage = operationError.Message
		return result, nil
	}

	if stackResult.Properties == nil {
		operationError := operationErrorFromValues("InvalidResponse", "deployment stack response did not include properties", nil)
		result.Error = &operationError
		result.ErrorCode = operationError.Code
		result.ErrorMessage = operationError.Message
		return result, nil
	}

	result.Succeeded = true
	result.Outputs = deploymentOutputs(stackResult.Properties.Outputs)
	result.Resources = stackResources(stackResult.Properties)
	return result, nil
}

// Close disconnects from the Bicep CLI and terminates its process.
func (session *LiveSession) Close() error {
	if session == nil || session.session == nil {
		return nil
	}
	return session.session.Close()
}

type deploymentScope int

const (
	deploymentScopeResourceGroup deploymentScope = iota
	deploymentScopeSubscription
	deploymentScopeManagementGroup
)

type deploymentTarget struct {
	scope             deploymentScope
	managementGroupID string
	subscriptionID    string
	resourceGroup     string
	location          string
}

type normalizedDeployOptions struct {
	filePath           string
	stackName          string
	parameterOverrides map[string]json.RawMessage
	target             deploymentTarget
}

func normalizeDeployOptions(options DeployOptions) (normalizedDeployOptions, error) {
	if strings.TrimSpace(options.FilePath) == "" {
		return normalizedDeployOptions{}, errors.New("file path is required")
	}
	if options.StackName != "" && strings.TrimSpace(options.StackName) == "" {
		return normalizedDeployOptions{}, errors.New("stack name must not be empty")
	}

	normalized := normalizedDeployOptions{
		filePath:           options.FilePath,
		stackName:          options.StackName,
		parameterOverrides: options.ParameterOverrides,
	}
	if normalized.stackName == "" {
		normalized.stackName = defaultStackName()
	}
	if normalized.parameterOverrides == nil {
		normalized.parameterOverrides = map[string]json.RawMessage{}
	}

	managementGroupID := strings.TrimSpace(options.ManagementGroupID)
	subscriptionID := strings.TrimSpace(options.SubscriptionID)
	resourceGroup := strings.TrimSpace(options.ResourceGroup)
	location := strings.TrimSpace(options.Location)

	if managementGroupID != "" {
		if subscriptionID != "" || resourceGroup != "" {
			return normalizedDeployOptions{}, errors.New("subscription ID and resource group must not be set with management group ID")
		}
		if location == "" {
			return normalizedDeployOptions{}, errors.New("location is required for management-group deployments")
		}
		normalized.target = deploymentTarget{
			scope:             deploymentScopeManagementGroup,
			managementGroupID: managementGroupID,
			location:          location,
		}
		return normalized, nil
	}

	if subscriptionID == "" {
		return normalizedDeployOptions{}, errors.New("subscription ID is required")
	}
	if resourceGroup != "" {
		normalized.target = deploymentTarget{
			scope:          deploymentScopeResourceGroup,
			subscriptionID: subscriptionID,
			resourceGroup:  resourceGroup,
			location:       location,
		}
		return normalized, nil
	}

	if location == "" {
		return normalizedDeployOptions{}, errors.New("location is required for subscription deployments")
	}
	normalized.target = deploymentTarget{
		scope:          deploymentScopeSubscription,
		subscriptionID: subscriptionID,
		location:       location,
	}
	return normalized, nil
}

func clientSubscriptionID(target deploymentTarget) string {
	if target.scope == deploymentScopeManagementGroup {
		return ""
	}
	return target.subscriptionID
}

func defaultStackName() string {
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		return "bicep-test-fallback"
	}
	return "bicep-test-" + hex.EncodeToString(random)
}

func (session *Session) compileDeploymentStack(ctx context.Context, options normalizedDeployOptions) (armdeploymentstacks.DeploymentStack, error) {
	absolutePath, err := filepath.Abs(options.filePath)
	if err != nil {
		return armdeploymentstacks.DeploymentStack{}, fmt.Errorf("resolve Bicep parameters file path: %w", err)
	}
	compilation, err := session.client.CompileParams(ctx, biceprpcclient.CompileParamsRequest{
		Path:               absolutePath,
		ParameterOverrides: options.parameterOverrides,
	})
	if err != nil {
		return armdeploymentstacks.DeploymentStack{}, err
	}
	if !compilation.Success || compilation.Template == "" || compilation.Parameters == "" {
		diagnostics := formatDiagnostics(compilation.Diagnostics)
		if diagnostics == "" {
			return armdeploymentstacks.DeploymentStack{}, errors.New("Bicep parameter compilation failed")
		}
		return armdeploymentstacks.DeploymentStack{}, fmt.Errorf("Bicep parameter compilation failed: %s", diagnostics)
	}

	var template any
	if err := json.Unmarshal([]byte(compilation.Template), &template); err != nil {
		return armdeploymentstacks.DeploymentStack{}, fmt.Errorf("decode compiled template: %w", err)
	}
	var parameterFile struct {
		Parameters map[string]struct {
			Value     json.RawMessage `json:"value"`
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
		return armdeploymentstacks.DeploymentStack{}, fmt.Errorf("decode compiled parameters: %w", err)
	}
	parameters := make(map[string]*armdeploymentstacks.DeploymentParameter, len(parameterFile.Parameters))
	for name, parameter := range parameterFile.Parameters {
		deploymentParameter := &armdeploymentstacks.DeploymentParameter{}
		if parameter.Value != nil {
			if bytes.Equal(bytes.TrimSpace(parameter.Value), []byte("null")) {
				deploymentParameter.Value = azcore.NullValue[*any]()
			} else if err := json.Unmarshal(parameter.Value, &deploymentParameter.Value); err != nil {
				return armdeploymentstacks.DeploymentStack{}, fmt.Errorf("decode deployment parameter %q: %w", name, err)
			}
		}
		if parameter.Reference != nil {
			deploymentParameter.Reference = &armdeploymentstacks.KeyVaultParameterReference{
				KeyVault:      &armdeploymentstacks.KeyVaultReference{ID: &parameter.Reference.KeyVault.ID},
				SecretName:    &parameter.Reference.SecretName,
				SecretVersion: parameter.Reference.SecretVersion,
			}
		}
		parameters[name] = deploymentParameter
	}

	deleteAction := armdeploymentstacks.DeploymentStacksDeleteDetachEnumDelete
	denyMode := armdeploymentstacks.DenySettingsModeNone
	stack := armdeploymentstacks.DeploymentStack{
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
	}
	if options.target.location != "" {
		stack.Location = &options.target.location
	}

	return stack, nil
}

// Teardown deletes the Deployment Stack and all resources it manages.
// Concurrent calls share an active deletion, and a failed deletion can be retried.
func (result *DeployResult) Teardown(ctx context.Context) error {
	for {
		result.teardownMu.Lock()
		if result.teardownDone {
			result.teardownMu.Unlock()
			return nil
		}
		if wait := result.teardownWait; wait != nil {
			result.teardownMu.Unlock()
			select {
			case <-wait:
				continue
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		wait := make(chan struct{})
		result.teardownWait = wait
		result.teardownMu.Unlock()

		err := result.client.delete(ctx, result.target, result.stackName)
		if isNotFound(err) {
			err = nil
		}

		result.teardownMu.Lock()
		result.teardownDone = err == nil
		result.teardownWait = nil
		close(wait)
		result.teardownMu.Unlock()
		return err
	}
}

func (client *azureDeploymentStackClient) createOrUpdate(ctx context.Context, target deploymentTarget, stackName string, stack armdeploymentstacks.DeploymentStack) (armdeploymentstacks.DeploymentStack, error) {
	switch target.scope {
	case deploymentScopeResourceGroup:
		poller, err := client.client.BeginCreateOrUpdateAtResourceGroup(ctx, target.resourceGroup, stackName, stack, nil)
		if err != nil {
			return armdeploymentstacks.DeploymentStack{}, err
		}
		response, err := poller.PollUntilDone(ctx, &runtime.PollUntilDoneOptions{})
		return response.DeploymentStack, err
	case deploymentScopeSubscription:
		poller, err := client.client.BeginCreateOrUpdateAtSubscription(ctx, stackName, stack, nil)
		if err != nil {
			return armdeploymentstacks.DeploymentStack{}, err
		}
		response, err := poller.PollUntilDone(ctx, &runtime.PollUntilDoneOptions{})
		return response.DeploymentStack, err
	case deploymentScopeManagementGroup:
		poller, err := client.client.BeginCreateOrUpdateAtManagementGroup(ctx, target.managementGroupID, stackName, stack, nil)
		if err != nil {
			return armdeploymentstacks.DeploymentStack{}, err
		}
		response, err := poller.PollUntilDone(ctx, &runtime.PollUntilDoneOptions{})
		return response.DeploymentStack, err
	default:
		return armdeploymentstacks.DeploymentStack{}, errors.New("unsupported deployment scope")
	}
}

func (client *azureDeploymentStackClient) validate(ctx context.Context, target deploymentTarget, stackName string, stack armdeploymentstacks.DeploymentStack) (armdeploymentstacks.DeploymentStackValidateResult, error) {
	switch target.scope {
	case deploymentScopeResourceGroup:
		poller, err := client.client.BeginValidateStackAtResourceGroup(ctx, target.resourceGroup, stackName, stack, nil)
		if err != nil {
			return armdeploymentstacks.DeploymentStackValidateResult{}, err
		}
		response, err := poller.PollUntilDone(ctx, &runtime.PollUntilDoneOptions{})
		return response.DeploymentStackValidateResult, err
	case deploymentScopeSubscription:
		poller, err := client.client.BeginValidateStackAtSubscription(ctx, stackName, stack, nil)
		if err != nil {
			return armdeploymentstacks.DeploymentStackValidateResult{}, err
		}
		response, err := poller.PollUntilDone(ctx, &runtime.PollUntilDoneOptions{})
		return response.DeploymentStackValidateResult, err
	case deploymentScopeManagementGroup:
		poller, err := client.client.BeginValidateStackAtManagementGroup(ctx, target.managementGroupID, stackName, stack, nil)
		if err != nil {
			return armdeploymentstacks.DeploymentStackValidateResult{}, err
		}
		response, err := poller.PollUntilDone(ctx, &runtime.PollUntilDoneOptions{})
		return response.DeploymentStackValidateResult, err
	default:
		return armdeploymentstacks.DeploymentStackValidateResult{}, errors.New("unsupported deployment scope")
	}
}

func (client *azureDeploymentStackClient) delete(ctx context.Context, target deploymentTarget, stackName string) error {
	deleteManagementGroups := armdeploymentstacks.UnmanageActionManagementGroupModeDelete
	deleteResourceGroups := armdeploymentstacks.UnmanageActionResourceGroupModeDelete
	deleteResources := armdeploymentstacks.UnmanageActionResourceModeDelete

	switch target.scope {
	case deploymentScopeResourceGroup:
		poller, err := client.client.BeginDeleteAtResourceGroup(ctx, target.resourceGroup, stackName, &armdeploymentstacks.ClientBeginDeleteAtResourceGroupOptions{
			UnmanageActionManagementGroups: &deleteManagementGroups,
			UnmanageActionResourceGroups:   &deleteResourceGroups,
			UnmanageActionResources:        &deleteResources,
		})
		if err != nil {
			return err
		}
		_, err = poller.PollUntilDone(ctx, &runtime.PollUntilDoneOptions{})
		return err
	case deploymentScopeSubscription:
		poller, err := client.client.BeginDeleteAtSubscription(ctx, stackName, &armdeploymentstacks.ClientBeginDeleteAtSubscriptionOptions{
			UnmanageActionManagementGroups: &deleteManagementGroups,
			UnmanageActionResourceGroups:   &deleteResourceGroups,
			UnmanageActionResources:        &deleteResources,
		})
		if err != nil {
			return err
		}
		_, err = poller.PollUntilDone(ctx, &runtime.PollUntilDoneOptions{})
		return err
	case deploymentScopeManagementGroup:
		poller, err := client.client.BeginDeleteAtManagementGroup(ctx, target.managementGroupID, stackName, &armdeploymentstacks.ClientBeginDeleteAtManagementGroupOptions{
			UnmanageActionManagementGroups: &deleteManagementGroups,
			UnmanageActionResourceGroups:   &deleteResourceGroups,
			UnmanageActionResources:        &deleteResources,
		})
		if err != nil {
			return err
		}
		_, err = poller.PollUntilDone(ctx, &runtime.PollUntilDoneOptions{})
		return err
	default:
		return errors.New("unsupported deployment scope")
	}
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

func stackResources(properties *armdeploymentstacks.DeploymentStackProperties) []DeploymentResource {
	if properties == nil {
		return []DeploymentResource{}
	}
	result := make([]DeploymentResource, 0, len(properties.Resources))
	for _, resource := range properties.Resources {
		if resource == nil || resource.ID == nil {
			continue
		}
		resourceTypeName := resourceType(*resource.ID)
		result = append(result, DeploymentResource{ID: *resource.ID, Type: resourceTypeName})
	}
	return result
}

func validationResources(properties *armdeploymentstacks.DeploymentStackValidateProperties) []DeploymentResource {
	if properties == nil {
		return []DeploymentResource{}
	}
	result := make([]DeploymentResource, 0, len(properties.ValidatedResources))
	for _, resource := range properties.ValidatedResources {
		if resource == nil || resource.ID == nil {
			continue
		}
		result = append(result, DeploymentResource{ID: *resource.ID, Type: resourceType(*resource.ID)})
	}
	return result
}

func validationCorrelationID(properties *armdeploymentstacks.DeploymentStackValidateProperties) string {
	if properties == nil {
		return ""
	}
	return stringValue(properties.CorrelationID)
}

func formatDiagnostics(diagnostics []map[string]any) string {
	parts := make([]string, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		level := stringMapValue(diagnostic, "level")
		code := stringMapValue(diagnostic, "code")
		message := stringMapValue(diagnostic, "message")
		part := strings.TrimSpace(strings.Join([]string{level, code}, " "))
		if part != "" && message != "" {
			part += ": " + message
		} else if part == "" {
			part = message
		}
		part = strings.TrimSpace(part)
		if part != "" {
			parts = append(parts, part)
		}
	}
	return strings.Join(parts, "\n")
}

func operationErrorFromErrorDetail(detail *armdeploymentstacks.ErrorDetail) OperationError {
	if detail == nil {
		return operationErrorFromValues("", "", nil)
	}
	rawData, _ := json.Marshal(detail)
	return operationErrorFromValues(stringValue(detail.Code), stringValue(detail.Message), rawData)
}

func operationErrorFromError(err error) OperationError {
	code := "Error"
	message := ""
	if err != nil {
		message = err.Error()
	}

	var responseError *azcore.ResponseError
	if errors.As(err, &responseError) {
		if responseError.ErrorCode != "" {
			code = responseError.ErrorCode
		}
		if rawData := responseErrorRawData(responseError); rawData != nil {
			if parsedCode, parsedMessage := parseOperationError(rawData); parsedCode != "" {
				code = parsedCode
				if parsedMessage != "" {
					message = parsedMessage
				}
			}
			return operationErrorFromValues(code, message, rawData)
		}
	}

	if message == "" && err != nil {
		message = err.Error()
	}
	return operationErrorFromValues(code, message, nil)
}

func responseErrorRawData(responseError *azcore.ResponseError) []byte {
	if responseError == nil || responseError.RawResponse == nil || responseError.RawResponse.Body == nil {
		return nil
	}
	body, err := io.ReadAll(responseError.RawResponse.Body)
	if err != nil {
		return nil
	}
	responseError.RawResponse.Body = io.NopCloser(bytes.NewReader(body))
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 || !json.Valid(trimmed) {
		return nil
	}
	return append([]byte(nil), trimmed...)
}

func parseOperationError(rawData []byte) (string, string) {
	var document map[string]any
	if err := json.Unmarshal(rawData, &document); err != nil {
		return "", ""
	}

	errorObject := document
	if nested, ok := document["error"].(map[string]any); ok {
		errorObject = nested
	}

	return stringMapValue(errorObject, "code"), stringMapValue(errorObject, "message")
}

func operationErrorFromValues(code, message string, rawData []byte) OperationError {
	if rawData == nil {
		fallback, _ := json.Marshal(map[string]string{"code": code, "message": message})
		rawData = fallback
	}
	return OperationError{Code: code, Message: message, RawData: json.RawMessage(rawData)}
}

func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	var responseError *azcore.ResponseError
	return errors.As(err, &responseError) && responseError.StatusCode == http.StatusNotFound
}

func stringMapValue(values map[string]any, key string) string {
	if values == nil {
		return ""
	}
	value, ok := values[key]
	if !ok {
		return ""
	}
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return text
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
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
