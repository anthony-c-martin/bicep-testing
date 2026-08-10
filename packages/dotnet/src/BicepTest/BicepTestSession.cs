using Bicep.RpcClient;
using Bicep.RpcClient.Models;
using Azure.Core;
using Azure.ResourceManager.Resources.DeploymentStacks;
using Azure.ResourceManager.Resources.DeploymentStacks.Models;
using System.Text.Json;
using System.Text.Json.Nodes;

namespace AnthonyCMartin.BicepTesting;

public sealed class BicepTestSession : IDisposable, IAsyncDisposable
{
    private static readonly JsonSerializerOptions SerializerOptions = new()
    {
        PropertyNameCaseInsensitive = true,
    };

    private readonly IBicepClient client;

    internal BicepTestSession(IBicepClient client)
    {
        this.client = client;
    }

    public static async Task<BicepTestSession> CreateAsync(
        string bicepVersion,
        CancellationToken cancellationToken = default)
    {
        ArgumentException.ThrowIfNullOrWhiteSpace(bicepVersion);

        var client = await CreateClientAsync(bicepVersion, cancellationToken);
        return new BicepTestSession(client);
    }

    public async Task<SnapshotResult> SnapshotAsync(
        SnapshotOptions options,
        CancellationToken cancellationToken = default)
    {
        ArgumentNullException.ThrowIfNull(options);
        ArgumentException.ThrowIfNullOrWhiteSpace(options.FilePath);

        var response = await client.GetSnapshot(
            new GetSnapshotRequest(
                Path.GetFullPath(options.FilePath),
                new GetSnapshotRequest.MetadataDefinition(
                    options.TenantId,
                    options.SubscriptionId,
                    options.ResourceGroup,
                    options.Location,
                    options.DeploymentName),
                ExternalInputs: null),
            cancellationToken);

        return JsonSerializer.Deserialize<SnapshotResult>(response.Snapshot, SerializerOptions)
            ?? throw new InvalidDataException("The Bicep snapshot response could not be deserialized.");
    }

    internal static DeploymentStackData BuildDeploymentStackData(
        string template,
        string parametersJson,
        string? location = null)
    {
        using var parameterDocument = JsonDocument.Parse(parametersJson);
        var stackData = new DeploymentStackData
        {
            Location = location,
            Template = BinaryData.FromString(template),
            ActionOnUnmanage = new ActionOnUnmanage(UnmanageActionResourceMode.Delete)
            {
                ResourceGroups = UnmanageActionResourceGroupMode.Delete,
                ManagementGroups = UnmanageActionManagementGroupMode.Delete,
                ResourcesWithoutDeleteSupport = ResourcesWithoutDeleteSupportAction.Fail,
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
                else if (parameter.Value.TryGetProperty("expression", out var expressionValue))
                {
                    item.Expression = expressionValue.GetRawText();
                }
                else if (parameter.Value.TryGetProperty("reference", out var reference))
                {
                    if (!reference.TryGetProperty("keyVault", out var keyVault)
                        || !keyVault.TryGetProperty("id", out var keyVaultIdElement)
                        || keyVaultIdElement.GetString() is not { } keyVaultId)
                    {
                        throw new InvalidDataException("A Key Vault parameter reference must include a vault ID.");
                    }

                    if (!reference.TryGetProperty("secretName", out var secretNameElement)
                        || secretNameElement.GetString() is not { } secretName)
                    {
                        throw new InvalidDataException("A Key Vault parameter reference must include a secret name.");
                    }

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

        return stackData;
    }

    internal async Task<(string Template, string Parameters)> CompileDeploymentAsync(
        DeployOptions options,
        CancellationToken cancellationToken)
    {
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

        return (compilation.Template, compilation.Parameters);
    }

    internal static void ValidateDeployOptions(DeployOptions options)
    {
        ArgumentNullException.ThrowIfNull(options);
        ArgumentException.ThrowIfNullOrWhiteSpace(options.FilePath);
        ArgumentException.ThrowIfNullOrWhiteSpace(options.StackName);

        _ = BuildDeploymentScope(options);

        if (options.ResourceGroup is null)
        {
            ArgumentException.ThrowIfNullOrWhiteSpace(options.Location);
        }
    }

    internal static ResourceIdentifier BuildDeploymentScope(DeployOptions options)
    {
        ArgumentNullException.ThrowIfNull(options);

        if (options.ManagementGroupId is not null)
        {
            ArgumentException.ThrowIfNullOrWhiteSpace(options.ManagementGroupId);
            if (options.SubscriptionId is not null || options.ResourceGroup is not null)
            {
                throw new ArgumentException(
                    "SubscriptionId and ResourceGroup must not be set with ManagementGroupId.",
                    nameof(options));
            }

            return new ResourceIdentifier(
                $"/providers/Microsoft.Management/managementGroups/{options.ManagementGroupId}");
        }

        ArgumentException.ThrowIfNullOrWhiteSpace(options.SubscriptionId);
        if (options.ResourceGroup is null)
        {
            return new ResourceIdentifier($"/subscriptions/{options.SubscriptionId}");
        }

        ArgumentException.ThrowIfNullOrWhiteSpace(options.ResourceGroup);
        return new ResourceIdentifier(
            $"/subscriptions/{options.SubscriptionId}/resourceGroups/{options.ResourceGroup}");
    }

    private static async Task<IBicepClient> CreateClientAsync(
        string bicepVersion,
        CancellationToken cancellationToken)
    {
        ArgumentException.ThrowIfNullOrWhiteSpace(bicepVersion);
        var factory = new PooledBicepClientFactory();
        return await factory.Initialize(
            BicepClientConfiguration.Default with { BicepVersion = bicepVersion },
            cancellationToken);
    }

    public void Dispose() => client.Dispose();

    public ValueTask DisposeAsync()
    {
        Dispose();
        return ValueTask.CompletedTask;
    }
}