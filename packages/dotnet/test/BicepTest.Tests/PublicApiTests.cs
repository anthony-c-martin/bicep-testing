using AnthonyCMartin.BicepTesting;
using PublicApiGenerator;

namespace BicepTest.Tests;

[TestClass]
public class PublicApiTests
{
    [TestMethod]
    public void PublicApiMatchesApprovedBaseline()
    {
        var publicApi = typeof(BicepTestSession).Assembly.GeneratePublicApi();
        var repositoryRoot = FindRepositoryRoot();
        var approvedPath = Path.Combine(repositoryRoot, "api", "dotnet", "PublicAPI.txt");

        if (string.Equals(Environment.GetEnvironmentVariable("UPDATE_PUBLIC_API"), "true", StringComparison.OrdinalIgnoreCase))
        {
            File.WriteAllText(approvedPath, publicApi);
            return;
        }

        var approvedApi = File.ReadAllText(approvedPath);
        Assert.AreEqual(approvedApi, publicApi, $"The public API has changed. Run with UPDATE_PUBLIC_API=true to update {approvedPath}.");
    }

    private static string FindRepositoryRoot()
    {
        var directory = new DirectoryInfo(AppContext.BaseDirectory);

        while (directory is not null && !File.Exists(Path.Combine(directory.FullName, "BicepTest.slnx")))
        {
            directory = directory.Parent;
        }

        return directory?.Parent?.Parent?.FullName
            ?? throw new InvalidOperationException("Could not locate the repository root.");
    }
}