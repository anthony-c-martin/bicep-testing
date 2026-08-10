from __future__ import annotations

from dataclasses import dataclass, field
from collections.abc import Callable
from threading import Lock
from typing import Any, Self


@dataclass(frozen=True, slots=True)
class DeploymentResource:
    """A resource managed by a Deployment Stack."""

    id: str
    type: str | None = None


@dataclass(frozen=True, slots=True, init=False)
class DeployResult:
    """Outputs and resources from a deployment, with deterministic cleanup."""

    outputs: dict[str, Any]
    resources: tuple[DeploymentResource, ...]
    _teardown: Callable[[], None] = field(repr=False, compare=False)
    _lock: Lock = field(repr=False, compare=False)
    _closed: list[bool] = field(repr=False, compare=False)
    _teardown_error: list[BaseException | None] = field(repr=False, compare=False)

    @classmethod
    def _create(
        cls,
        outputs: dict[str, Any],
        resources: tuple[DeploymentResource, ...],
        teardown: Callable[[], None],
    ) -> DeployResult:
        result = object.__new__(cls)
        object.__setattr__(result, "outputs", outputs)
        object.__setattr__(result, "resources", resources)
        object.__setattr__(result, "_teardown", teardown)
        object.__setattr__(result, "_lock", Lock())
        object.__setattr__(result, "_closed", [False])
        object.__setattr__(result, "_teardown_error", [None])
        return result

    def close(self) -> None:
        """Delete the Deployment Stack and all resources it manages."""
        with self._lock:
            if self._closed[0]:
                if self._teardown_error[0] is not None:
                    raise self._teardown_error[0]
                return
            self._closed[0] = True
            try:
                self._teardown()
            except BaseException as error:
                self._teardown_error[0] = error
                raise

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
    properties: dict[str, Any] = field(default_factory=dict)
    additional_properties: dict[str, Any] = field(default_factory=dict)

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
    outputs: dict[str, Any]

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