from __future__ import annotations

from collections.abc import Mapping
from dataclasses import dataclass
import re
from typing import Any

from .models import SnapshotResource, SnapshotResult


def resources_of_type(snapshot: SnapshotResult, type: str) -> tuple[SnapshotResource, ...]:
    """Select resources by case-insensitive resource type."""
    if not type.strip():
        raise ValueError("type must not be empty")
    return tuple(resource for resource in snapshot.predicted_resources if resource.type.lower() == type.lower())


def require_resource(
    snapshot: SnapshotResult, *, id: str | None = None, type: str | None = None, name: str | None = None
) -> SnapshotResource:
    """Require exactly one resource by ID, or type and exact name."""
    if id is not None:
        if not id.strip() or type is not None or name is not None:
            raise ValueError("specify either id or type and name")
        matches = tuple(resource for resource in snapshot.predicted_resources if resource.id.lower() == id.lower())
    else:
        if type is None or name is None or not name.strip():
            raise ValueError("specify either id or type and name")
        matches = tuple(resource for resource in resources_of_type(snapshot, type) if resource.name == name)
    if len(matches) != 1:
        raise ValueError(f"expected exactly one resource matching id={id!r}, type={type!r}, name={name!r}; found {len(matches)}")
    return matches[0]


@dataclass(frozen=True, slots=True)
class ResourceScope:
    """Scope components present in an Azure resource ID; tenant IDs are not encoded."""

    subscription_id: str | None = None
    resource_group: str | None = None
    management_group_id: str | None = None


def get_resource_scope(id: str) -> ResourceScope:
    """Parse subscription, resource-group, management-group, and tenant-scoped IDs."""
    parts = id.split("/")[1:]
    if not id.startswith("/") or "?" in id or "#" in id or any(not part.strip() for part in parts):
        raise ValueError(f"invalid Azure resource ID: {id!r}")
    subscription_id = resource_group = management_group_id = None
    index = 0
    if parts[0].lower() == "subscriptions":
        if len(parts) < 2 or not re.fullmatch(r"[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}", parts[1], re.IGNORECASE):
            raise ValueError(f"invalid Azure resource ID: {id!r}")
        subscription_id = parts[1]
        index = 2
        if index < len(parts) and parts[index].lower() == "resourcegroups":
            if index + 1 >= len(parts):
                raise ValueError(f"invalid Azure resource ID: {id!r}")
            resource_group = parts[index + 1]
            index += 2
    elif parts[0].lower() != "providers":
        raise ValueError(f"invalid Azure resource ID: {id!r}")
    if len(parts) >= 4 and [part.lower() for part in parts[:3]] == ["providers", "microsoft.management", "managementgroups"]:
        management_group_id = parts[3]
    while index < len(parts):
        if parts[index].lower() != "providers" or index + 1 >= len(parts):
            raise ValueError(f"invalid Azure resource ID: {id!r}")
        index += 2
        start = index
        while index < len(parts) and parts[index].lower() != "providers":
            index += 2
        if index == start or index > len(parts):
            raise ValueError(f"invalid Azure resource ID: {id!r}")
    return ResourceScope(subscription_id, resource_group, management_group_id)


def get_snapshot_value(value: Any, pointer: str) -> Any:
    """Resolve an RFC 6901 JSON Pointer, raising for missing paths and preserving None."""
    if pointer == "":
        return value
    if not pointer.startswith("/"):
        raise ValueError("a JSON Pointer must be empty or start with /")
    for encoded in pointer[1:].split("/"):
        if re.search(r"~(?:[^01]|$)", encoded):
            raise ValueError(f"invalid JSON Pointer escape: {pointer!r}")
        key = encoded.replace("~1", "/").replace("~0", "~")
        if isinstance(value, Mapping) and key in value:
            value = value[key]
        elif isinstance(value, list) and re.fullmatch(r"0|[1-9][0-9]*", key) and int(key) < len(value):
            value = value[int(key)]
        else:
            raise KeyError(f"JSON Pointer not found: {pointer!r}")
    return value
