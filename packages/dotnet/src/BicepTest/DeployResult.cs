using Azure;
using Azure.ResourceManager.Resources.DeploymentStacks;
using Azure.ResourceManager.Resources.DeploymentStacks.Models;
using System.Text.Json;

namespace AnthonyCMartin.BicepTesting;

public sealed class DeployResult : IAsyncDisposable
{
    private readonly Func<Task> deleteAsync;
    private readonly object teardownLock = new();
    private Task? teardownTask;

    private DeployResult(
        IReadOnlyDictionary<string, JsonElement> outputs,
        IReadOnlyList<DeploymentResource> resources,
        OperationError? error,
        Func<Task> deleteAsync)
    {
        this.deleteAsync = deleteAsync;
        Outputs = outputs;
        Resources = resources;
        Error = error;
    }

    internal DeployResult(Func<Task> deleteAsync, OperationError? error = null)
        : this(
            new Dictionary<string, JsonElement>(),
            Array.Empty<DeploymentResource>(),
            error,
            deleteAsync)
    {
    }

    public IReadOnlyDictionary<string, JsonElement> Outputs { get; }

    public IReadOnlyList<DeploymentResource> Resources { get; }

    public bool Succeeded => Error is null;

    public OperationError? Error { get; }

    public string? ErrorCode => Error?.Code;

    public string? ErrorMessage => Error?.Message;

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
        return new DeployResult(outputs, resources, error: null, () => DeleteAsync(stack));
    }

    internal static DeployResult FromStackReference(DeploymentStackResource stack) =>
        new(() => DeleteAsync(stack));

    internal DeployResult WithError(Exception error) => new(deleteAsync, OperationError.FromException(error));

    public Task TeardownAsync(CancellationToken cancellationToken = default)
    {
        Task task;
        lock (teardownLock)
        {
            teardownTask ??= deleteAsync();
            task = teardownTask;
        }

        return cancellationToken.CanBeCanceled
            ? task.WaitAsync(cancellationToken)
            : task;
    }

    public async ValueTask DisposeAsync() => await TeardownAsync();

    private static async Task DeleteAsync(DeploymentStackResource stack)
    {
        try
        {
            await stack.DeleteAsync(
                WaitUntil.Completed,
                UnmanageActionResourceMode.Delete,
                UnmanageActionResourceGroupMode.Delete,
                UnmanageActionManagementGroupMode.Delete,
                ResourcesWithoutDeleteSupportAction.Fail,
                bypassStackOutOfSyncError: false,
                CancellationToken.None);
        }
        catch (RequestFailedException exception) when (exception.Status == 404)
        {
        }
    }
}

public sealed record DeploymentResource(string Id, string? Type);