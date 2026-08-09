from __future__ import annotations

from dataclasses import dataclass, field
from pathlib import Path
from typing import Any


@dataclass(frozen=True, slots=True)
class BicepClientConfiguration:
    bicep_version: str | None = None
    existing_cli_path: str | Path | None = None
    cache_root: str | Path | None = None


@dataclass(frozen=True, slots=True)
class CompileRequest:
    path: str | Path


@dataclass(frozen=True, slots=True)
class CompileResponse:
    success: bool
    diagnostics: tuple[dict[str, Any], ...]
    contents: str | None


@dataclass(frozen=True, slots=True)
class CompileParamsRequest:
    path: str | Path
    parameter_overrides: dict[str, Any] = field(default_factory=dict)


@dataclass(frozen=True, slots=True)
class CompileParamsResponse:
    success: bool
    diagnostics: tuple[dict[str, Any], ...]
    parameters: str | None
    template: str | None
    template_spec_id: str | None = None


@dataclass(frozen=True, slots=True)
class FormatRequest:
    path: str | Path


@dataclass(frozen=True, slots=True)
class FormatResponse:
    contents: str


@dataclass(frozen=True, slots=True)
class GetMetadataRequest:
    path: str | Path


@dataclass(frozen=True, slots=True)
class GetMetadataResponse:
    parameters: tuple[dict[str, Any], ...]
    outputs: tuple[dict[str, Any], ...]
    exports: tuple[dict[str, Any], ...]
    metadata: tuple[dict[str, Any], ...]


@dataclass(frozen=True, slots=True)
class GetFileReferencesRequest:
    path: str | Path


@dataclass(frozen=True, slots=True)
class GetFileReferencesResponse:
    file_paths: tuple[str, ...]


@dataclass(frozen=True, slots=True)
class GetDeploymentGraphRequest:
    path: str | Path


@dataclass(frozen=True, slots=True)
class GetDeploymentGraphResponse:
    nodes: tuple[dict[str, Any], ...]
    edges: tuple[dict[str, Any], ...]


@dataclass(frozen=True, slots=True)
class SnapshotMetadata:
    tenant_id: str | None = None
    subscription_id: str | None = None
    resource_group: str | None = None
    location: str | None = None
    deployment_name: str | None = None


@dataclass(frozen=True, slots=True)
class SnapshotExternalInput:
    kind: str
    value: Any
    config: Any = None


@dataclass(frozen=True, slots=True)
class GetSnapshotRequest:
    path: str | Path
    metadata: SnapshotMetadata = field(default_factory=SnapshotMetadata)
    external_inputs: tuple[SnapshotExternalInput, ...] = ()


@dataclass(frozen=True, slots=True)
class GetSnapshotResponse:
    snapshot: str