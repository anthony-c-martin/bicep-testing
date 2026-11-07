"""Test Bicep infrastructure by evaluating deployment snapshots locally."""

from .models import (
    DeployOptions,
    DeployResult,
    DeploymentResource,
    ManagementGroupDeployOptions,
    OperationError,
    ResourceGroupDeployOptions,
    SnapshotMetadata,
    SnapshotResource,
    SnapshotResult,
    SubscriptionDeployOptions,
    ValidateResult,
)
from .session import BicepTestSession, LiveBicepTestSession
from .helpers import ResourceScope, resources_of_type, require_resource, get_resource_scope, get_snapshot_value

__all__ = [
    "BicepTestSession",
    "LiveBicepTestSession",
    "DeployOptions",
    "ResourceGroupDeployOptions",
    "SubscriptionDeployOptions",
    "ManagementGroupDeployOptions",
    "DeployResult",
    "DeploymentResource",
    "ValidateResult",
    "OperationError",
    "SnapshotMetadata",
    "SnapshotResource",
    "SnapshotResult",
    "ResourceScope",
    "resources_of_type",
    "require_resource",
    "get_resource_scope",
    "get_snapshot_value",
]