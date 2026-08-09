from __future__ import annotations

import json
import os
import platform
import stat
import urllib.request
from pathlib import Path
from typing import Any, Protocol, Self

from .models import DeployResult, DeploymentResource, SnapshotMetadata, SnapshotResult
from .rpcclient import RpcClient


class _DeploymentStacksOperations(Protocol):
    def begin_create_or_update_at_resource_group(
        self, resource_group: str, stack_name: str, deployment_stack: dict[str, Any]
    ) -> Any: ...

    def begin_delete_at_resource_group(
        self, resource_group: str, stack_name: str, **kwargs: Any
    ) -> Any: ...


def _create_deployment_stacks_operations(credential: Any, subscription_id: str) -> _DeploymentStacksOperations:
    from azure.mgmt.resource.deploymentstacks import DeploymentStacksClient

    return DeploymentStacksClient(credential, subscription_id).deployment_stacks


class BicepTestSession:
    """Installs and invokes a pinned Bicep CLI for infrastructure tests."""

    def __init__(self, client: RpcClient) -> None:
        self._client = client

    @classmethod
    def create(cls, bicep_version: str) -> Self:
        """Install a Bicep CLI version if needed and start its RPC client."""
        if not bicep_version.strip():
            raise ValueError("bicep_version must not be empty")
        executable = _install_bicep(bicep_version)
        client = RpcClient(executable)
        version = client.call("bicep/version", {})["version"]
        if _version_tuple(version) < (0, 36, 1):
            client.close()
            raise RuntimeError(f"Bicep CLI 0.36.1 or later is required; detected {version}")
        return cls(client)

    def snapshot(
        self,
        file_path: str | os.PathLike[str],
        metadata: SnapshotMetadata | None = None,
    ) -> SnapshotResult:
        """Evaluate a Bicep parameters file without deploying it."""
        path = Path(file_path).resolve()
        response = self._client.call(
            "bicep/getSnapshot",
            {
                "path": str(path),
                "metadata": (metadata or SnapshotMetadata())._as_rpc_dict(),
            },
        )
        return SnapshotResult._from_dict(json.loads(response["snapshot"]))

    def deploy(
        self,
        credential: Any,
        file_path: str | os.PathLike[str],
        subscription_id: str,
        resource_group: str,
        stack_name: str,
        parameter_overrides: dict[str, Any] | None = None,
    ) -> DeployResult:
        """Compile and deploy a Bicep parameters file as a resource-group Deployment Stack."""
        if credential is None:
            raise ValueError("credential must not be None")
        if not subscription_id or not resource_group or not stack_name:
            raise ValueError("subscription_id, resource_group, and stack_name must not be empty")
        compilation = self._client.call(
            "bicep/compileParams",
            {
                "path": str(Path(file_path).resolve()),
                "parameterOverrides": parameter_overrides or {},
            },
        )
        if not compilation.get("success") or not compilation.get("template") or not compilation.get("parameters"):
            diagnostics = json.dumps(compilation.get("diagnostics", []))
            raise RuntimeError(f"Bicep parameter compilation failed: {diagnostics}")

        template = json.loads(compilation["template"])
        parameter_file = json.loads(compilation["parameters"])
        operations = _create_deployment_stacks_operations(credential, subscription_id)
        stack = operations.begin_create_or_update_at_resource_group(
            resource_group,
            stack_name,
            {
                "properties": {
                    "template": template,
                    "parameters": parameter_file.get("parameters", {}),
                    "actionOnUnmanage": {
                        "resources": "delete",
                        "resourceGroups": "delete",
                        "managementGroups": "delete",
                        "resourcesWithoutDeleteSupport": "fail",
                    },
                    "denySettings": {"mode": "none"},
                }
            },
        ).result()

        outputs = {
            name: value.get("value", value) if isinstance(value, dict) else value
            for name, value in (getattr(stack, "outputs", None) or {}).items()
        }
        resources = tuple(
            DeploymentResource(id=resource.id, type=_resource_type(resource.id))
            for resource in (getattr(stack, "resources", None) or ())
            if getattr(resource, "id", None)
        )

        def teardown() -> None:
            operations.begin_delete_at_resource_group(
                resource_group,
                stack_name,
                unmanage_action_resources="delete",
                unmanage_action_resource_groups="delete",
                unmanage_action_management_groups="delete",
                unmanage_action_resources_without_delete_support="fail",
            ).result()

        return DeployResult._create(outputs, resources, teardown)

    def close(self) -> None:
        """Disconnect from the Bicep CLI and terminate its process."""
        self._client.close()

    def __enter__(self) -> Self:
        return self

    def __exit__(self, *_: object) -> None:
        self.close()


def _install_bicep(version: str) -> Path:
    directory = Path.home() / ".bicep" / "bin" / f"v{version}"
    directory.mkdir(parents=True, exist_ok=True)
    artifact = _artifact_name()
    executable = directory / ("bicep.exe" if os.name == "nt" else "bicep")
    if executable.exists():
        return executable
    url = f"https://downloads.bicep.azure.com/v{version}/{artifact}"
    temporary = executable.with_suffix(executable.suffix + ".download")
    try:
        urllib.request.urlretrieve(url, temporary)
        temporary.chmod(temporary.stat().st_mode | stat.S_IXUSR | stat.S_IXGRP | stat.S_IXOTH)
        temporary.replace(executable)
    finally:
        temporary.unlink(missing_ok=True)
    return executable


def _artifact_name() -> str:
    operating_system = platform.system().lower()
    architecture = platform.machine().lower()
    architecture_name = {
        "amd64": "x64",
        "x86_64": "x64",
        "arm64": "arm64",
        "aarch64": "arm64",
    }.get(architecture)
    system_name = {"windows": "win", "linux": "linux", "darwin": "osx"}.get(operating_system)
    if system_name is None or architecture_name is None:
        raise RuntimeError(f"Bicep CLI is not available for {operating_system}/{architecture}")
    extension = ".exe" if operating_system == "windows" else ""
    return f"bicep-{system_name}-{architecture_name}{extension}"


def _version_tuple(version: str) -> tuple[int, ...]:
    return tuple(int(part.split("-")[0]) for part in version.split("."))


def _resource_type(resource_id: str) -> str | None:
    parts = resource_id.strip("/").split("/")
    try:
        provider_index = next(
            index for index, part in enumerate(parts) if part.lower() == "providers"
        )
    except StopIteration:
        return None
    if provider_index + 2 >= len(parts):
        return None
    return "/".join(
        [parts[provider_index + 1], *parts[provider_index + 2 :: 2]]
    )