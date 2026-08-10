from __future__ import annotations

import json
import os
from pathlib import Path
from threading import Lock
from typing import Any, Mapping, Protocol, Self

from anthonycmartin.bicep_rpc_client import (
    BicepClient,
    BicepClientConfiguration,
    BicepClientFactory,
    CompileParamsRequest,
    GetSnapshotRequest,
    SnapshotMetadata as RpcSnapshotMetadata,
)

from .models import (
    DeployOptions,
    DeployResult,
    DeploymentResource,
    ManagementGroupDeployOptions,
    OperationError,
    ResourceGroupDeployOptions,
    SnapshotMetadata,
    SnapshotResult,
    SubscriptionDeployOptions,
    ValidateResult,
)


class _DeploymentStacksOperations(Protocol):
    def begin_create_or_update_at_resource_group(
        self,
        resource_group: str,
        stack_name: str,
        deployment_stack: dict[str, Any],
        /,
    ) -> Any: ...

    def begin_delete_at_resource_group(
        self, resource_group: str, stack_name: str, **kwargs: Any
    ) -> Any: ...


class _DeploymentTarget(Protocol):
    scope: str


class _ResourceGroupTarget:
    scope = "resource_group"

    def __init__(self, subscription_id: str, resource_group: str, location: str | None) -> None:
        self.subscription_id = subscription_id
        self.resource_group = resource_group
        self.location = location


class _SubscriptionTarget:
    scope = "subscription"

    def __init__(self, subscription_id: str, location: str) -> None:
        self.subscription_id = subscription_id
        self.location = location


class _ManagementGroupTarget:
    scope = "management_group"

    def __init__(self, management_group_id: str, location: str) -> None:
        self.management_group_id = management_group_id
        self.location = location


class _NormalizedDeployOptions:
    def __init__(
        self,
        file_path: str | os.PathLike[str],
        stack_name: str,
        parameter_overrides: Mapping[str, Any],
        target: _DeploymentTarget,
    ) -> None:
        self.file_path = file_path
        self.stack_name = stack_name
        self.parameter_overrides = parameter_overrides
        self.target = target


def _create_deployment_stacks_operations(
    credential: Any,
    target: _DeploymentTarget,
) -> _DeploymentStacksOperations:
    from azure.mgmt.resource.deploymentstacks import DeploymentStacksClient

    if target.scope == "management_group":
        return DeploymentStacksClient(credential).deployment_stacks

    subscription_id = getattr(target, "subscription_id")
    return DeploymentStacksClient(credential, subscription_id).deployment_stacks


class BicepTestSession:
    """Installs and invokes a pinned Bicep CLI for infrastructure tests."""

    def __init__(self, client: BicepClient) -> None:
        self._client = client

    @classmethod
    def create(cls, bicep_version: str) -> Self:
        """Install a Bicep CLI version if needed and start its RPC client."""
        if not bicep_version.strip():
            raise ValueError("bicep_version must not be empty")
        client = BicepClientFactory().initialize(
            BicepClientConfiguration(bicep_version=bicep_version)
        )
        version = client.get_version()
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
        snapshot_metadata = metadata or SnapshotMetadata()
        response = self._client.get_snapshot(
            GetSnapshotRequest(
                path,
                RpcSnapshotMetadata(
                    tenant_id=snapshot_metadata.tenant_id,
                    subscription_id=snapshot_metadata.subscription_id,
                    resource_group=snapshot_metadata.resource_group,
                    location=snapshot_metadata.location,
                    deployment_name=snapshot_metadata.deployment_name,
                ),
            )
        )
        return SnapshotResult._from_dict(json.loads(response.snapshot))

    def _compile_deployment(
        self,
        file_path: str | os.PathLike[str],
        parameter_overrides: Mapping[str, Any],
    ) -> tuple[dict[str, Any], dict[str, Any]]:
        compilation = self._client.compile_params(
            CompileParamsRequest(
                Path(file_path).resolve(), dict(parameter_overrides)
            )
        )
        if not compilation.success or not compilation.template or not compilation.parameters:
            diagnostics = "\n".join(
                _format_diagnostic(diagnostic)
                for diagnostic in compilation.diagnostics
            )
            suffix = f":\n{diagnostics}" if diagnostics else "."
            raise RuntimeError(f"Bicep parameter compilation failed{suffix}")

        template = json.loads(compilation.template)
        parameter_file = json.loads(compilation.parameters)
        return template, parameter_file.get("parameters", {})

    def close(self) -> None:
        """Disconnect from the Bicep CLI and terminate its process."""
        self._client.close()

    def __enter__(self) -> Self:
        return self

    def __exit__(self, *_: object) -> None:
        self.close()


class LiveBicepTestSession:
    """Owns credentials and deploy/validate operations while reusing a Bicep session."""

    def __init__(self, session: BicepTestSession, credential: Any) -> None:
        if credential is None:
            raise ValueError("credential must not be None")
        self._session = session
        self._credential = credential

    @classmethod
    def create(cls, bicep_version: str, credential: Any) -> Self:
        return cls(BicepTestSession.create(bicep_version), credential)

    def snapshot(
        self,
        file_path: str | os.PathLike[str],
        metadata: SnapshotMetadata | None = None,
    ) -> SnapshotResult:
        return self._session.snapshot(file_path, metadata)

    def validate(self, options: DeployOptions) -> ValidateResult:
        normalized = _normalize_deploy_options(options)
        template, parameters = self._session._compile_deployment(
            normalized.file_path,
            normalized.parameter_overrides,
        )
        deployment_stack = _build_deployment_stack(normalized, template, parameters)
        operations = _create_deployment_stacks_operations(self._credential, normalized.target)
        validation = _validate_stack(operations, normalized, deployment_stack)
        return _to_validate_result(validation)

    def deploy(self, options: DeployOptions) -> DeployResult:
        normalized = _normalize_deploy_options(options)
        template, parameters = self._session._compile_deployment(
            normalized.file_path,
            normalized.parameter_overrides,
        )
        deployment_stack = _build_deployment_stack(normalized, template, parameters)
        operations = _create_deployment_stacks_operations(self._credential, normalized.target)
        teardown = _create_teardown(operations, normalized)
        try:
            stack = _deploy_stack(operations, normalized, deployment_stack)
            return DeployResult._create(
                succeeded=True,
                error=None,
                outputs=_extract_outputs(stack),
                resources=_extract_resources(stack),
                teardown=teardown,
            )
        except BaseException as error:
            return DeployResult._create(
                succeeded=False,
                error=_to_operation_error(error),
                outputs={},
                resources=(),
                teardown=teardown,
            )

    def close(self) -> None:
        self._session.close()

    def __enter__(self) -> Self:
        return self

    def __exit__(self, *_: object) -> None:
        self.close()


def _version_tuple(version: str) -> tuple[int, ...]:
    return tuple(int(part.split("-")[0]) for part in version.split("."))


def _normalize_deploy_options(options: DeployOptions) -> _NormalizedDeployOptions:
    if isinstance(options, ResourceGroupDeployOptions):
        return _NormalizedDeployOptions(
            options.file_path,
            options.stack_name,
            options.parameter_overrides,
            _ResourceGroupTarget(options.subscription_id, options.resource_group, options.location),
        )

    if isinstance(options, SubscriptionDeployOptions):
        return _NormalizedDeployOptions(
            options.file_path,
            options.stack_name,
            options.parameter_overrides,
            _SubscriptionTarget(options.subscription_id, options.location),
        )

    if isinstance(options, ManagementGroupDeployOptions):
        return _NormalizedDeployOptions(
            options.file_path,
            options.stack_name,
            options.parameter_overrides,
            _ManagementGroupTarget(options.management_group_id, options.location),
        )

    raise TypeError("options must be a recognized DeployOptions value")


def _build_deployment_stack(
    options: _NormalizedDeployOptions,
    template: dict[str, Any],
    parameters: dict[str, Any],
) -> dict[str, Any]:
    deployment_stack: dict[str, Any] = {
        "properties": {
            "template": template,
            "parameters": parameters,
            "actionOnUnmanage": {
                "resources": "delete",
                "resourceGroups": "delete",
                "managementGroups": "delete",
                "resourcesWithoutDeleteSupport": "fail",
            },
            "denySettings": {"mode": "none"},
        }
    }
    location = getattr(options.target, "location", None)
    if location is not None:
        deployment_stack["location"] = location
    return deployment_stack


def _deploy_stack(
    operations: _DeploymentStacksOperations,
    options: _NormalizedDeployOptions,
    deployment_stack: dict[str, Any],
) -> Any:
    target = options.target
    if target.scope == "resource_group":
        return _invoke_lro(
            operations,
            ("begin_create_or_update_at_resource_group",),
            target.resource_group,
            options.stack_name,
            deployment_stack,
        )
    if target.scope == "subscription":
        return _invoke_lro(
            operations,
            ("begin_create_or_update_at_subscription",),
            options.stack_name,
            deployment_stack,
        )
    return _invoke_lro(
        operations,
        ("begin_create_or_update_at_management_group",),
        target.management_group_id,
        options.stack_name,
        deployment_stack,
    )


def _validate_stack(
    operations: _DeploymentStacksOperations,
    options: _NormalizedDeployOptions,
    deployment_stack: dict[str, Any],
) -> Any:
    target = options.target
    if target.scope == "resource_group":
        return _invoke_lro(
            operations,
            (
                "begin_validate_stack_at_resource_group",
                "begin_validate_at_resource_group",
            ),
            target.resource_group,
            options.stack_name,
            deployment_stack,
        )
    if target.scope == "subscription":
        return _invoke_lro(
            operations,
            (
                "begin_validate_stack_at_subscription",
                "begin_validate_at_subscription",
            ),
            options.stack_name,
            deployment_stack,
        )
    return _invoke_lro(
        operations,
        (
            "begin_validate_stack_at_management_group",
            "begin_validate_at_management_group",
        ),
        target.management_group_id,
        options.stack_name,
        deployment_stack,
    )


def _create_teardown(
    operations: _DeploymentStacksOperations,
    options: _NormalizedDeployOptions,
) -> Any:
    lock = Lock()
    closed = [False]

    def teardown() -> None:
        with lock:
            if closed[0]:
                return
            try:
                _delete_stack(operations, options)
            except BaseException as error:
                if _get_status_code(error) == 404:
                    closed[0] = True
                    return
                raise
            closed[0] = True

    return teardown


def _delete_stack(
    operations: _DeploymentStacksOperations,
    options: _NormalizedDeployOptions,
) -> None:
    delete_options = {
        "unmanage_action_resources": "delete",
        "unmanage_action_resource_groups": "delete",
        "unmanage_action_management_groups": "delete",
        "unmanage_action_resources_without_delete_support": "fail",
    }
    target = options.target
    if target.scope == "resource_group":
        _invoke_lro(
            operations,
            ("begin_delete_at_resource_group",),
            target.resource_group,
            options.stack_name,
            **delete_options,
        )
        return
    if target.scope == "subscription":
        _invoke_lro(
            operations,
            ("begin_delete_at_subscription",),
            options.stack_name,
            **delete_options,
        )
        return
    _invoke_lro(
        operations,
        ("begin_delete_at_management_group",),
        target.management_group_id,
        options.stack_name,
        **delete_options,
    )


def _extract_outputs(stack: Any) -> dict[str, Any]:
    output_values = (
        getattr(getattr(stack, "properties", None), "outputs", None)
        or getattr(stack, "outputs", None)
        or {}
    )
    return {
        name: value.get("value", value) if isinstance(value, dict) else value
        for name, value in output_values.items()
    }


def _extract_resources(stack: Any) -> tuple[DeploymentResource, ...]:
    resource_values = (
        getattr(getattr(stack, "properties", None), "resources", None)
        or getattr(stack, "resources", None)
        or ()
    )
    resources: list[DeploymentResource] = []
    for resource in resource_values:
        resource_id = str(getattr(resource, "id", "") or "")
        if not resource_id:
            continue
        resource_type = getattr(resource, "type", None)
        resources.append(
            DeploymentResource(
                id=resource_id,
                type=str(resource_type) if resource_type else _resource_type(resource_id),
            )
        )
    return tuple(resources)


def _to_validate_result(validation: Any) -> ValidateResult:
    properties = getattr(validation, "properties", None)
    resources = getattr(properties, "validated_resources", None)
    if resources is None:
        resources = getattr(properties, "validatedResources", None)
    if resources is None:
        resources = getattr(validation, "validated_resources", None)
    if resources is None:
        resources = getattr(validation, "validatedResources", None)

    correlation_id = getattr(properties, "correlation_id", None)
    if correlation_id is None:
        correlation_id = getattr(properties, "correlationId", None)
    if correlation_id is None:
        correlation_id = getattr(validation, "correlation_id", None)
    if correlation_id is None:
        correlation_id = getattr(validation, "correlationId", None)

    error = _to_operation_error_from_payload(getattr(validation, "error", None))
    deployment_resources: list[DeploymentResource] = []
    for resource in resources or ():
        resource_id = str(getattr(resource, "id", "") or "")
        if not resource_id:
            continue
        resource_type = getattr(resource, "type", None)
        deployment_resources.append(
            DeploymentResource(
                id=resource_id,
                type=str(resource_type) if resource_type else _resource_type(resource_id),
            )
        )

    return ValidateResult(
        resources=tuple(deployment_resources),
        correlation_id=correlation_id,
        error=error,
    )


def _to_operation_error_from_payload(payload: Any) -> OperationError | None:
    if payload is None:
        return None
    raw_data = _payload_to_raw_data(payload)
    code = _as_str(raw_data.get("code")) or _as_str(getattr(payload, "code", None))
    message = _as_str(raw_data.get("message")) or _as_str(getattr(payload, "message", None))
    return OperationError(code=code, message=message, raw_data=raw_data)


def _to_operation_error(error: BaseException) -> OperationError:
    raw_data = _extract_raw_error_data(error)
    service_error = raw_data.get("error")
    error_payload = service_error if isinstance(service_error, dict) else raw_data
    code = _as_str(error_payload.get("code")) or _as_str(getattr(error, "code", None)) or type(error).__name__
    message = _as_str(error_payload.get("message")) or str(error)
    return OperationError(code=code, message=message, raw_data=raw_data)


def _extract_raw_error_data(error: BaseException) -> dict[str, Any]:
    response = getattr(error, "response", None)
    body = getattr(response, "body", None)
    if isinstance(body, str):
        try:
            parsed = json.loads(body)
            if isinstance(parsed, dict):
                return parsed
        except json.JSONDecodeError:
            pass

    body_as_text = getattr(response, "body_as_text", None)
    if not isinstance(body_as_text, str):
        body_as_text = getattr(response, "bodyAsText", None)
    if isinstance(body_as_text, str):
        try:
            parsed = json.loads(body_as_text)
            if isinstance(parsed, dict):
                return parsed
        except json.JSONDecodeError:
            pass

    details = getattr(error, "details", None)
    if isinstance(details, dict):
        return details

    return {
        "code": _as_str(getattr(error, "code", None)) or type(error).__name__,
        "message": str(error),
    }


def _payload_to_raw_data(payload: Any) -> dict[str, Any]:
    if isinstance(payload, dict):
        return payload
    as_dict = getattr(payload, "as_dict", None)
    if callable(as_dict):
        converted = as_dict()
        if isinstance(converted, dict):
            return converted
    return {
        "code": _as_str(getattr(payload, "code", None)),
        "message": _as_str(getattr(payload, "message", None)),
    }


def _get_status_code(error: BaseException) -> int | None:
    status_code = getattr(error, "status_code", None)
    if isinstance(status_code, int):
        return status_code

    status = getattr(error, "status", None)
    if isinstance(status, int):
        return status

    response = getattr(error, "response", None)
    response_status = getattr(response, "status", None)
    if isinstance(response_status, int):
        return response_status

    return None


def _invoke_lro(
    operations: _DeploymentStacksOperations,
    method_names: tuple[str, ...],
    *args: Any,
    **kwargs: Any,
) -> Any:
    method = None
    for name in method_names:
        candidate = getattr(operations, name, None)
        if callable(candidate):
            method = candidate
            break

    if method is None:
        names = ", ".join(method_names)
        raise AttributeError(f"Deployment stacks operations are missing required method(s): {names}")

    poller = method(*args, **kwargs)
    result = getattr(poller, "result", None)
    if callable(result):
        return result()
    return poller


def _format_diagnostic(diagnostic: Any) -> str:
    level = getattr(diagnostic, "level", None)
    code = getattr(diagnostic, "code", None)
    message = getattr(diagnostic, "message", None)
    if level is not None and code is not None and message is not None:
        return f"{level} {code}: {message}"
    return str(diagnostic)


def _as_str(value: Any) -> str | None:
    return value if isinstance(value, str) else None


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