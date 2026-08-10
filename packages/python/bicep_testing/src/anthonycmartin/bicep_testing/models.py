from __future__ import annotations

from dataclasses import dataclass, field
from types import MappingProxyType
import os
from uuid import uuid4
from collections.abc import Callable
from threading import Lock
from typing import Any, Mapping, Self, TypeAlias


def _default_stack_name() -> str:
    return f"bicep-test-{uuid4().hex}"


def _freeze_mapping(value: Mapping[str, Any] | None) -> Mapping[str, Any]:
    return MappingProxyType(dict(value or {}))


def _required(name: str, value: str) -> str:
    if not value.strip():
        raise ValueError(f"{name} must not be empty")
    return value


@dataclass(frozen=True, slots=True)
class _DeployOptionsBase:
    """Common deployment options across all deployment stack targets."""

    file_path: str | os.PathLike[str]
    stack_name: str = field(default_factory=_default_stack_name)
    parameter_overrides: Mapping[str, Any] = field(default_factory=dict)

    def __post_init__(self) -> None:
        if not str(self.file_path).strip():
            raise ValueError("file_path must not be empty")
        _required("stack_name", self.stack_name)
        object.__setattr__(self, "parameter_overrides", _freeze_mapping(self.parameter_overrides))


@dataclass(frozen=True, slots=True)
class ResourceGroupDeployOptions(_DeployOptionsBase):
    """Deploy to a resource-group target within a subscription."""

    subscription_id: str = ""
    resource_group: str = ""
    location: str | None = None

    def __post_init__(self) -> None:
        _DeployOptionsBase.__post_init__(self)
        _required("subscription_id", self.subscription_id)
        _required("resource_group", self.resource_group)
        if self.location is not None:
            _required("location", self.location)


@dataclass(frozen=True, slots=True)
class SubscriptionDeployOptions(_DeployOptionsBase):
    """Deploy to a subscription target."""

    subscription_id: str = ""
    location: str = ""

    def __post_init__(self) -> None:
        _DeployOptionsBase.__post_init__(self)
        _required("subscription_id", self.subscription_id)
        _required("location", self.location)


@dataclass(frozen=True, slots=True)
class ManagementGroupDeployOptions(_DeployOptionsBase):
    """Deploy to a management-group target."""

    management_group_id: str = ""
    location: str = ""

    def __post_init__(self) -> None:
        _DeployOptionsBase.__post_init__(self)
        _required("management_group_id", self.management_group_id)
        _required("location", self.location)


DeployOptions: TypeAlias = (
    ResourceGroupDeployOptions | SubscriptionDeployOptions | ManagementGroupDeployOptions
)


@dataclass(frozen=True, slots=True)
class OperationError:
    """A normalized Azure operation error with the source payload."""

    code: str | None
    message: str | None
    raw_data: Mapping[str, Any]

    def __post_init__(self) -> None:
        object.__setattr__(self, "raw_data", _freeze_mapping(self.raw_data))


@dataclass(frozen=True, slots=True)
class DeploymentResource:
    """A resource managed by a Deployment Stack."""

    id: str
    type: str | None = None


@dataclass(frozen=True, slots=True, init=False)
class DeployResult:
    """Outputs and resources from a deployment, with deterministic cleanup."""

    succeeded: bool
    error: OperationError | None
    error_code: str | None
    error_message: str | None
    outputs: Mapping[str, Any]
    resources: tuple[DeploymentResource, ...]
    _teardown: Callable[[], None] = field(repr=False, compare=False)
    _lock: Lock = field(repr=False, compare=False)
    _closed: list[bool] = field(repr=False, compare=False)

    @classmethod
    def _create(
        cls,
        succeeded: bool,
        error: OperationError | None,
        outputs: Mapping[str, Any],
        resources: tuple[DeploymentResource, ...],
        teardown: Callable[[], None],
    ) -> DeployResult:
        result = object.__new__(cls)
        object.__setattr__(result, "succeeded", succeeded)
        object.__setattr__(result, "error", error)
        object.__setattr__(result, "error_code", error.code if error else None)
        object.__setattr__(result, "error_message", error.message if error else None)
        object.__setattr__(result, "outputs", _freeze_mapping(outputs))
        object.__setattr__(result, "resources", resources)
        object.__setattr__(result, "_teardown", teardown)
        object.__setattr__(result, "_lock", Lock())
        object.__setattr__(result, "_closed", [False])
        return result

    def close(self) -> None:
        """Delete the Deployment Stack and all resources it manages."""
        with self._lock:
            if self._closed[0]:
                return
            try:
                self._teardown()
            except BaseException:
                # Keep the result open so callers can retry teardown.
                raise
            self._closed[0] = True

    def __enter__(self) -> Self:
        return self

    def __exit__(self, *_: object) -> None:
        self.close()


@dataclass(frozen=True, slots=True)
class SnapshotMetadata:
    """Azure deployment context used to evaluate a snapshot."""

    tenant_id: str | None = None
    subscription_id: str | None = None
    resource_group: str | None = None
    location: str | None = None
    deployment_name: str | None = None

    def _as_rpc_dict(self) -> dict[str, str]:
        values = {
            "tenantId": self.tenant_id,
            "subscriptionId": self.subscription_id,
            "resourceGroup": self.resource_group,
            "location": self.location,
            "deploymentName": self.deployment_name,
        }
        return {name: value for name, value in values.items() if value is not None}


@dataclass(frozen=True, slots=True)
class SnapshotResource:
    """A resource predicted by a Bicep snapshot."""

    id: str
    type: str
    name: str
    api_version: str
    location: str | None = None
    properties: Mapping[str, Any] = field(default_factory=dict)
    additional_properties: Mapping[str, Any] = field(default_factory=dict)

    def __post_init__(self) -> None:
        object.__setattr__(self, "properties", _freeze_mapping(self.properties))
        object.__setattr__(self, "additional_properties", _freeze_mapping(self.additional_properties))

    @classmethod
    def _from_dict(cls, value: dict[str, Any]) -> SnapshotResource:
        known = {"id", "type", "name", "apiVersion", "location", "properties"}
        return cls(
            id=value["id"],
            type=value["type"],
            name=value["name"],
            api_version=value["apiVersion"],
            location=value.get("location"),
            properties=value.get("properties", {}),
            additional_properties={key: item for key, item in value.items() if key not in known},
        )


@dataclass(frozen=True, slots=True)
class SnapshotResult:
    """The predicted result of evaluating a Bicep parameters file."""

    predicted_resources: tuple[SnapshotResource, ...]
    diagnostics: tuple[str, ...]
    outputs: Mapping[str, Any]

    def __post_init__(self) -> None:
        object.__setattr__(self, "outputs", _freeze_mapping(self.outputs))

    @classmethod
    def _from_dict(cls, value: dict[str, Any]) -> SnapshotResult:
        return cls(
            predicted_resources=tuple(
                SnapshotResource._from_dict(resource)
                for resource in value.get("predictedResources", [])
            ),
            diagnostics=tuple(value.get("diagnostics", [])),
            outputs=value.get("outputs", {}),
        )


@dataclass(frozen=True, slots=True)
class ValidateResult:
    """The result of validating a Deployment Stack deployment."""

    resources: tuple[DeploymentResource, ...]
    correlation_id: str | None
    error: OperationError | None = None

    @property
    def is_valid(self) -> bool:
        return self.error is None