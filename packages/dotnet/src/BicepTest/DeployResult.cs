using Azure;
using Azure.ResourceManager.Resources.DeploymentStacks;
using Azure.ResourceManager.Resources.DeploymentStacks.Models;
using System.Text.Json;

namespace AnthonyCMartin.BicepTesting;

public sealed class DeployResult : IAsyncDisposable
{
    private readonly DeploymentStackResource stack;
    private Task? teardownTask;

    private DeployResult(
        DeploymentStackResource stack,
        IReadOnlyDictionary<string, JsonElement> outputs,
        IReadOnlyList<DeploymentResource> resources)
    {
        this.stack = stack;
        Outputs = outputs;
        Resources = resources;
    }

    public IReadOnlyDictionary<string, JsonElement> Outputs { get; }

    public IReadOnlyList<DeploymentResource> Resources { get; }

    internal static DeployResult FromStack(DeploymentStackResource stack)
    {
        var outputs = new Dictionary<string, JsonElement>();
        if (stack.Data.Outputs is not null)
        {
            using var outputDocument = JsonDocument.Parse(stack.Data.Outputs);
            foreach (var output in outputDocument.RootElement.EnumerateObject())
            {
                outputs[output.Name] = output.Value.TryGetProperty("value", out var value)
                    ? value.Clone()
                    : output.Value.Clone();
            }
        }

        var resources = stack.Data.Resources
            .Where(resource => resource.Id is not null)
            .Select(resource => new DeploymentResource(resource.Id!.ToString(), resource.Type?.ToString()))
            .ToArray();
        return new DeployResult(stack, outputs, resources);
    }

    public Task TeardownAsync(CancellationToken cancellationToken = default)
    {
        teardownTask ??= DeleteAsync(cancellationToken);
        return teardownTask;
    }

    public async ValueTask DisposeAsync() => await TeardownAsync();

    private async Task DeleteAsync(CancellationToken cancellationToken)
    {
        await stack.DeleteAsync(
            WaitUntil.Completed,
            UnmanageActionResourceMode.Delete,
            UnmanageActionResourceGroupMode.Delete,
            UnmanageActionManagementGroupMode.Delete,
            ResourcesWithoutDeleteSupportAction.Fail,
            bypassStackOutOfSyncError: false,
            cancellationToken);
    }
}

public sealed record DeploymentResource(string Id, string? Type);