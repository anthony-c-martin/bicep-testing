using Bicep.RpcClient;
using Bicep.RpcClient.Models;
using Azure;
using Azure.Core;
using Azure.ResourceManager;
using Azure.ResourceManager.Resources.DeploymentStacks;
using Azure.ResourceManager.Resources.DeploymentStacks.Models;
using System.Text.Json;
using System.Text.Json.Nodes;

namespace BicepTest;

public sealed class BicepTestSession : IDisposable, IAsyncDisposable
{
    private static readonly JsonSerializerOptions SerializerOptions = new()
    {
        PropertyNameCaseInsensitive = true,
    };

    private readonly IBicepClient client;

    private BicepTestSession(IBicepClient client)
    {
        this.client = client;
    }

    public static async Task<BicepTestSession> CreateAsync(
        string bicepVersion,
        CancellationToken cancellationToken = default)
    {
        ArgumentException.ThrowIfNullOrWhiteSpace(bicepVersion);

        var factory = new BicepClientFactory();
        var client = await factory.Initialize(
            BicepClientConfiguration.Default with { BicepVersion = bicepVersion },
            cancellationToken);
        return new BicepTestSession(client);
    }

    public async Task<SnapshotResult> SnapshotAsync(
        string filePath,
        string? tenantId = null,
        string? subscriptionId = null,
        string? resourceGroup = null,
        string? location = null,
        string? deploymentName = null,
        CancellationToken cancellationToken = default)
    {
        ArgumentException.ThrowIfNullOrWhiteSpace(filePath);

        var response = await client.GetSnapshot(
            new GetSnapshotRequest(
                Path.GetFullPath(filePath),
                new GetSnapshotRequest.MetadataDefinition(
                    tenantId,
                    subscriptionId,
                    resourceGroup,
                    location,
                    deploymentName),
                ExternalInputs: null),
            cancellationToken);

        return JsonSerializer.Deserialize<SnapshotResult>(response.Snapshot, SerializerOptions)
            ?? throw new InvalidDataException("The Bicep snapshot response could not be deserialized.");
    }

    public async Task<DeployResult> DeployAsync(
        TokenCredential credential,
        DeployOptions options,
        CancellationToken cancellationToken = default)
    {
        ArgumentNullException.ThrowIfNull(credential);
        ArgumentNullException.ThrowIfNull(options);
        ArgumentException.ThrowIfNullOrWhiteSpace(options.FilePath);
        ArgumentException.ThrowIfNullOrWhiteSpace(options.SubscriptionId);
        ArgumentException.ThrowIfNullOrWhiteSpace(options.ResourceGroup);
        ArgumentException.ThrowIfNullOrWhiteSpace(options.StackName);

        var overrides = options.ParameterOverrides.ToDictionary(
            item => item.Key,
            item => JsonNode.Parse(item.Value.GetRawText())!);
        var compilation = await client.CompileParams(
            new CompileParamsRequest(Path.GetFullPath(options.FilePath), overrides),
            cancellationToken);
        if (!compilation.Success || compilation.Template is null || compilation.Parameters is null)
        {
            var diagnostics = string.Join(Environment.NewLine, compilation.Diagnostics);
            throw new InvalidDataException(
                $"Bicep parameter compilation failed{(diagnostics.Length > 0 ? $":{Environment.NewLine}{diagnostics}" : ".")}");
        }

        using var parameterDocument = JsonDocument.Parse(compilation.Parameters);
        var stackData = new DeploymentStackData
        {
            Template = BinaryData.FromString(compilation.Template),
            ActionOnUnmanage = new ActionOnUnmanage(UnmanageActionResourceMode.Delete)
            {
                ResourceGroups = UnmanageActionResourceGroupMode.Delete,
                ManagementGroups = UnmanageActionManagementGroupMode.Delete,
            },
            DenySettings = new DeploymentStackDenySettings(DeploymentStackDenySettingsMode.None),
        };
        if (parameterDocument.RootElement.TryGetProperty("parameters", out var parameters))
        {
            foreach (var parameter in parameters.EnumerateObject())
            {
                var item = new DeploymentParameterItem();
                if (parameter.Value.TryGetProperty("value", out var parameterValue))
                {
                    item.Value = BinaryData.FromString(parameterValue.GetRawText());
                }
                else if (parameter.Value.TryGetProperty("reference", out var reference))
                {
                    var keyVaultId = reference.GetProperty("keyVault").GetProperty("id").GetString()
                        ?? throw new InvalidDataException("A Key Vault parameter reference must include a vault ID.");
                    var secretName = reference.GetProperty("secretName").GetString()
                        ?? throw new InvalidDataException("A Key Vault parameter reference must include a secret name.");
                    item.Reference = new KeyVaultParameterReference(new ResourceIdentifier(keyVaultId), secretName)
                    {
                        SecretVersion = reference.TryGetProperty("secretVersion", out var secretVersion)
                            ? secretVersion.GetString()
                            : null,
                    };
                }
                stackData.Parameters.Add(parameter.Name, item);
            }
        }

        var armClient = new ArmClient(credential, options.SubscriptionId);
        var resourceGroupId = new ResourceIdentifier(
            $"/subscriptions/{options.SubscriptionId}/resourceGroups/{options.ResourceGroup}");
        var operation = await armClient.GetDeploymentStacks(resourceGroupId).CreateOrUpdateAsync(
            WaitUntil.Completed,
            options.StackName,
            stackData,
            cancellationToken);
        return DeployResult.FromStack(operation.Value);
    }

    public void Dispose() => client.Dispose();

    public ValueTask DisposeAsync()
    {
        Dispose();
        return ValueTask.CompletedTask;
    }
}