using Azure;
using System.ClientModel.Primitives;

namespace AnthonyCMartin.BicepTesting;

public sealed class OperationError
{
    private OperationError(string? code, string? message, BinaryData rawData)
    {
        Code = code;
        Message = message;
        RawData = rawData;
    }

    public string? Code { get; }

    public string? Message { get; }

    public BinaryData RawData { get; }

    internal static OperationError? FromResponseError(ResponseError? error) => error is null
        ? null
        : new OperationError(
            error.Code,
            error.Message,
            ModelReaderWriter.Write(error));

    internal static OperationError FromException(Exception exception)
    {
        var code = exception is RequestFailedException requestFailedException
            ? requestFailedException.ErrorCode
            : exception.GetType().Name;
        var message = exception.Message;
        var rawData = GetResponseContent(exception)
            ?? BinaryData.FromObjectAsJson(new { code, message });
        return new OperationError(code, message, rawData);
    }

    private static BinaryData? GetResponseContent(Exception exception)
    {
        if (exception is not RequestFailedException requestFailedException)
        {
            return null;
        }

        return requestFailedException.GetRawResponse()?.Content;
    }
}