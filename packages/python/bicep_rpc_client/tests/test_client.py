from pathlib import Path
from typing import Any

import pytest

from anthonycmartin.bicep_rpc_client import (
    BicepClient,
    CompileParamsRequest,
    GetSnapshotRequest,
    SnapshotMetadata,
)


def _client(responses: dict[str, Any]) -> BicepClient:
    client = object.__new__(BicepClient)
    client._version = None
    client._call = lambda method, params: responses[method]
    return client


def test_get_version_is_cached() -> None:
    calls = 0
    client = object.__new__(BicepClient)
    client._version = None

    def call(method: str, params: dict[str, Any]) -> dict[str, str]:
        nonlocal calls
        calls += 1
        return {"version": "0.46.1"}

    client._call = call

    assert client.get_version() == "0.46.1"
    assert client.get_version() == "0.46.1"
    assert calls == 1


def test_compile_params_returns_typed_response() -> None:
    client = _client(
        {
            "bicep/compileParams": {
                "success": True,
                "diagnostics": [],
                "parameters": "{}",
                "template": "{}",
                "templateSpecId": None,
            }
        }
    )

    result = client.compile_params(CompileParamsRequest(Path("main.bicepparam")))

    assert result.success
    assert result.parameters == "{}"
    assert result.template == "{}"


def test_get_snapshot_maps_metadata_to_rpc_casing() -> None:
    captured: dict[str, Any] = {}
    client = object.__new__(BicepClient)
    client._version = "0.46.1"

    def call(method: str, params: dict[str, Any]) -> dict[str, str]:
        captured.update(params)
        return {"snapshot": "{}"}

    client._call = call
    client.get_snapshot(
        GetSnapshotRequest(
            "main.bicepparam",
            SnapshotMetadata(subscription_id="subscription", resource_group="group"),
        )
    )

    assert captured["metadata"] == {
        "subscriptionId": "subscription",
        "resourceGroup": "group",
    }


def test_version_gate_rejects_old_cli() -> None:
    client = _client({"bicep/version": {"version": "0.35.0"}})

    with pytest.raises(RuntimeError, match="0.36.1 or later"):
        client.get_snapshot(GetSnapshotRequest("main.bicepparam"))