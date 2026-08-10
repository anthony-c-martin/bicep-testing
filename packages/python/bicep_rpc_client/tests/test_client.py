from io import BytesIO
from pathlib import Path
import subprocess
from typing import Any
from unittest.mock import Mock, patch

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


@pytest.mark.parametrize("already_exited", [False, True])
def test_close_closes_process_streams(already_exited: bool) -> None:
    process = Mock()
    process.stdin = BytesIO()
    process.stdout = BytesIO()
    process.poll.return_value = 0 if already_exited else None
    process.wait.return_value = 0
    with patch(
        "anthonycmartin.bicep_rpc_client.client.subprocess.Popen",
        return_value=process,
    ):
        client = BicepClient("bicep")

    client.close()

    assert process.stdin.closed
    assert process.stdout.closed
    assert process.terminate.call_count == (0 if already_exited else 1)


def test_close_kills_process_that_does_not_terminate() -> None:
    process = Mock()
    process.stdin = BytesIO()
    process.stdout = BytesIO()
    process.poll.return_value = None
    process.wait.side_effect = [subprocess.TimeoutExpired("bicep", 5), 0]
    with patch(
        "anthonycmartin.bicep_rpc_client.client.subprocess.Popen",
        return_value=process,
    ):
        client = BicepClient("bicep")

    client.close()

    process.kill.assert_called_once_with()
    assert process.stdin.closed
    assert process.stdout.closed


def test_initialization_failure_closes_available_process_stream() -> None:
    process = Mock()
    process.stdin = None
    process.stdout = BytesIO()
    process.poll.return_value = None
    process.wait.return_value = 0
    with patch(
        "anthonycmartin.bicep_rpc_client.client.subprocess.Popen",
        return_value=process,
    ):
        with pytest.raises(RuntimeError, match="did not expose stdin and stdout"):
            BicepClient("bicep")

    assert process.stdout.closed