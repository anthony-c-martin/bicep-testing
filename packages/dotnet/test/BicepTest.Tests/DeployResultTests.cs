using Azure;
using Microsoft.VisualStudio.TestTools.UnitTesting;
using System.Text.Json;

namespace AnthonyCMartin.BicepTesting.Tests;

[TestClass]
public sealed class DeployResultTests
{
    [TestMethod]
    public void Azure_failure_preserves_the_service_error_code()
    {
        var serviceError = new RequestFailedException(
            409,
            "The stack is out of sync.",
            "DeploymentStackOutOfSync",
            innerException: null);

        var result = new DeployResult(() => Task.CompletedTask).WithError(serviceError);

        Assert.IsFalse(result.Succeeded);
        Assert.AreEqual("DeploymentStackOutOfSync", result.ErrorCode);
        Assert.AreEqual(serviceError.Message, result.ErrorMessage);
        Assert.IsNotNull(result.Error);
        using var errorDocument = JsonDocument.Parse(result.Error.RawData);
        Assert.AreEqual("DeploymentStackOutOfSync", errorDocument.RootElement.GetProperty("code").GetString());
    }

    [TestMethod]
    public async Task Failed_deployment_exposes_idempotent_cleanup()
    {
        var deleteCalls = 0;
        var failedDeployment = new DeployResult(() =>
        {
            Interlocked.Increment(ref deleteCalls);
            return Task.CompletedTask;
        });
        var serviceError = new InvalidOperationException("Azure rejected the deployment.");

        var result = await LiveBicepTestSession.CompleteDeploymentAsync(
            failedDeployment,
            () => Task.FromException<Azure.ResourceManager.Resources.DeploymentStacks.DeploymentStackResource>(serviceError));

        Assert.IsFalse(result.Succeeded);
        Assert.IsNotNull(result.Error);
        Assert.AreEqual(nameof(InvalidOperationException), result.ErrorCode);
        Assert.AreEqual(serviceError.Message, result.ErrorMessage);
        using var errorDocument = JsonDocument.Parse(result.Error.RawData);
        Assert.AreEqual(nameof(InvalidOperationException), errorDocument.RootElement.GetProperty("code").GetString());
        Assert.AreEqual(serviceError.Message, errorDocument.RootElement.GetProperty("message").GetString());
        Assert.IsEmpty(result.Outputs);
        Assert.IsEmpty(result.Resources);

        await result.TeardownAsync();
        await result.TeardownAsync();
        Assert.AreEqual(1, deleteCalls);
    }

    [TestMethod]
    public async Task TeardownAsync_shares_cleanup_and_only_cancels_the_callers_wait()
    {
        var deleteStarted = new TaskCompletionSource(TaskCreationOptions.RunContinuationsAsynchronously);
        var allowDelete = new TaskCompletionSource(TaskCreationOptions.RunContinuationsAsynchronously);
        var deleteCalls = 0;
        var result = new DeployResult(async () =>
        {
            Interlocked.Increment(ref deleteCalls);
            deleteStarted.SetResult();
            await allowDelete.Task;
        });
        Assert.IsTrue(result.Succeeded);
        Assert.IsNull(result.Error);
        Assert.IsNull(result.ErrorCode);
        Assert.IsNull(result.ErrorMessage);
        using var cancellation = new CancellationTokenSource();

        var canceledWait = result.TeardownAsync(cancellation.Token);
        await deleteStarted.Task;
        cancellation.Cancel();

        await Assert.ThrowsAsync<OperationCanceledException>(async () => await canceledWait);
        var successfulWait = result.TeardownAsync();
        Assert.AreEqual(1, deleteCalls);
        Assert.IsFalse(successfulWait.IsCompleted);

        allowDelete.SetResult();
        await successfulWait;
        await result.TeardownAsync();
        Assert.AreEqual(1, deleteCalls);
    }
}