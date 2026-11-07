using Azure.Core;
using System.Globalization;
using System.Text.Json;

namespace AnthonyCMartin.BicepTesting;

public sealed class ResourceScope
{
    internal ResourceScope(string? subscriptionId, string? resourceGroup, string? managementGroupId)
    {
        SubscriptionId = subscriptionId;
        ResourceGroup = resourceGroup;
        ManagementGroupId = managementGroupId;
    }

    public string? SubscriptionId { get; }
    public string? ResourceGroup { get; }
    public string? ManagementGroupId { get; }
}

public static class SnapshotHelpers
{
    public static SnapshotResource[] ResourcesOfType(this SnapshotResult snapshot, string type)
    {
        ArgumentNullException.ThrowIfNull(snapshot);
        ArgumentException.ThrowIfNullOrWhiteSpace(type);
        return snapshot.PredictedResources
            .Where(resource => string.Equals(resource.Type, type, StringComparison.OrdinalIgnoreCase))
            .ToArray();
    }

    public static SnapshotResource RequireResource(this SnapshotResult snapshot, string id)
    {
        ArgumentNullException.ThrowIfNull(snapshot);
        ArgumentException.ThrowIfNullOrWhiteSpace(id);
        return RequireSingle(snapshot.PredictedResources.Where(resource =>
            string.Equals(resource.Id, id, StringComparison.OrdinalIgnoreCase)), id);
    }

    public static SnapshotResource RequireResource(this SnapshotResult snapshot, string type, string name)
    {
        ArgumentException.ThrowIfNullOrWhiteSpace(name);
        return RequireSingle(snapshot.ResourcesOfType(type).Where(resource => resource.Name == name), $"{type}/{name}");
    }

    private static SnapshotResource RequireSingle(IEnumerable<SnapshotResource> resources, string selector)
    {
        var matches = resources.ToArray();
        if (matches.Length != 1)
        {
            throw new InvalidOperationException(
                $"Expected exactly one resource matching {selector}; found {matches.Length}.");
        }
        return matches[0];
    }

    public static ResourceScope GetResourceScope(string id)
    {
        ArgumentException.ThrowIfNullOrWhiteSpace(id);
        var parts = id.Split('/').Skip(1).ToArray();
        ArgumentException Invalid() => new($"Invalid Azure resource ID: {id}", nameof(id));
        if (!id.StartsWith('/') || id.IndexOfAny(['?', '#']) >= 0 || parts.Any(string.IsNullOrWhiteSpace))
        {
            throw Invalid();
        }
        var index = 0;
        if (parts[0].Equals("subscriptions", StringComparison.OrdinalIgnoreCase))
        {
            if (parts.Length < 2 || !Guid.TryParseExact(parts[1], "D", out _))
            {
                throw Invalid();
            }
            index = 2;
            if (index < parts.Length && parts[index].Equals("resourceGroups", StringComparison.OrdinalIgnoreCase))
            {
                if (parts.Length < index + 2)
                {
                    throw Invalid();
                }
                index += 2;
            }
        }
        else if (!parts[0].Equals("providers", StringComparison.OrdinalIgnoreCase))
        {
            throw Invalid();
        }
        while (index < parts.Length)
        {
            if (!parts[index].Equals("providers", StringComparison.OrdinalIgnoreCase) || index + 1 >= parts.Length)
            {
                throw Invalid();
            }
            index += 2;
            var start = index;
            while (index < parts.Length && !parts[index].Equals("providers", StringComparison.OrdinalIgnoreCase))
            {
                index += 2;
            }
            if (index == start || index > parts.Length)
            {
                throw Invalid();
            }
        }
        var identifier = new ResourceIdentifier(id);
        var managementGroupId = parts.Length >= 4
            && parts[0].Equals("providers", StringComparison.OrdinalIgnoreCase)
            && parts[1].Equals("Microsoft.Management", StringComparison.OrdinalIgnoreCase)
            && parts[2].Equals("managementGroups", StringComparison.OrdinalIgnoreCase)
                ? parts[3] : null;
        return new ResourceScope(identifier.SubscriptionId, identifier.ResourceGroupName, managementGroupId);
    }

    public static JsonElement GetSnapshotValue(JsonElement value, string pointer)
    {
        ArgumentNullException.ThrowIfNull(pointer);
        if (pointer == "")
        {
            return value;
        }
        if (!pointer.StartsWith('/'))
        {
            throw new ArgumentException("A JSON Pointer must be empty or start with /.", nameof(pointer));
        }
        foreach (var encoded in pointer[1..].Split('/'))
        {
            for (var index = 0; index < encoded.Length; index++)
            {
                if (encoded[index] == '~' && (++index >= encoded.Length || encoded[index] is not ('0' or '1')))
                {
                    throw new ArgumentException($"Invalid JSON Pointer escape: {pointer}", nameof(pointer));
                }
            }
            var key = encoded.Replace("~1", "/").Replace("~0", "~");
            if (value.ValueKind == JsonValueKind.Object && value.TryGetProperty(key, out var property))
            {
                value = property;
            }
            else if (value.ValueKind == JsonValueKind.Array
                && (key == "0" || (key.Length > 0 && key[0] is >= '1' and <= '9'))
                && int.TryParse(key, NumberStyles.None, CultureInfo.InvariantCulture, out var index)
                && index < value.GetArrayLength())
            {
                value = value[index];
            }
            else
            {
                throw new KeyNotFoundException($"JSON Pointer not found: {pointer}");
            }
        }
        return value;
    }
}
