import io
import json
from typing import Any

import pytest

from anthonycmartin.bicep_testing.rpcclient import RpcClient, RpcError


def _client_with_response(response: dict[str, Any]) -> RpcClient:
    body = json.dumps(response).encode()
    client = object.__new__(RpcClient)
    client._output = io.BytesIO(f"Content-Length: {len(body)}\r\n\r\n".encode() + body)
    return client


def test_read_message_uses_content_length_framing() -> None:
    client = _client_with_response({"jsonrpc": "2.0", "id": 1, "result": {"version": "0.43.1"}})

    assert client._read_message() == {
        "jsonrpc": "2.0",
        "id": 1,
        "result": {"version": "0.43.1"},
    }


def test_read_message_requires_content_length() -> None:
    client = object.__new__(RpcClient)
    client._output = io.BytesIO(b"Content-Type: application/json\r\n\r\n{}")

    with pytest.raises(RuntimeError, match="Content-Length"):
        client._read_message()


def test_rpc_error_is_public() -> None:
    assert issubclass(RpcError, RuntimeError)