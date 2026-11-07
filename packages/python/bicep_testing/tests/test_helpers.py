import json
from pathlib import Path

import pytest

from anthonycmartin.bicep_testing import (
    BicepTestSession, SnapshotMetadata, SnapshotResource, SnapshotResult,
    get_resource_scope, get_snapshot_value, require_resource, resources_of_type,
)

INFRA = Path(__file__).parents[4] / "samples" / "infra"
CASES = json.loads((INFRA / "snapshot-scopes/helper-cases.json").read_text())


@pytest.mark.parametrize("case", CASES["scopes"])
def test_resource_scopes(case) -> None:
    scope = get_resource_scope(case["id"])
    assert scope.subscription_id == case.get("subscriptionId")
    assert scope.resource_group == case.get("resourceGroup")
    assert scope.management_group_id == case.get("managementGroupId")


@pytest.mark.parametrize("id", CASES["invalidIds"])
def test_invalid_resource_ids(id: str) -> None:
    with pytest.raises(ValueError):
        get_resource_scope(id)


@pytest.mark.parametrize("case", CASES["pointers"])
def test_json_pointers(case) -> None:
    assert get_snapshot_value(CASES["document"], case["pointer"]) == case["value"]


@pytest.mark.parametrize("pointer", CASES["invalidPointers"])
def test_invalid_json_pointers(pointer: str) -> None:
    with pytest.raises((ValueError, KeyError)):
        get_snapshot_value(CASES["document"], pointer)


def test_selectors_and_empty_pointer() -> None:
    assert get_snapshot_value(CASES["document"], "") is CASES["document"]
    empty = SnapshotResult((), (), {})
    assert resources_of_type(empty, "Missing/type") == ()
    with pytest.raises(ValueError, match="found 0"):
        require_resource(empty, id="/missing")
    resource = SnapshotResource("/id", "Test/type", "test", "1")
    duplicate = SnapshotResult((resource, resource), (), {})
    for selector in [{"id": "/ID"}, {"type": "test/TYPE", "name": "test"}]:
        with pytest.raises(ValueError, match="found 2"):
            require_resource(duplicate, **selector)
    with pytest.raises(ValueError, match="found 0"):
        require_resource(duplicate, type="Test/type", name="TEST")
    for selector in [{}, {"id": "/id", "type": "Test/type"}, {"type": "", "name": "test"}]:
        with pytest.raises(ValueError):
            require_resource(duplicate, **selector)
    with pytest.raises(ValueError):
        resources_of_type(empty, "")


def test_every_snapshot_scope_and_optional_context() -> None:
    with BicepTestSession.create("0.47.16") as session:
        snapshot = session.snapshot(
            INFRA / "snapshot-scopes/management-group.bicepparam",
            SnapshotMetadata(management_group_id="platform", location="eastus", deployment_name="offline-test"),
        )
        assert snapshot.diagnostics == ()
        assert snapshot.outputs["scopeId"] == "/providers/Microsoft.Management/managementGroups/platform"
        assert snapshot.outputs["deploymentName"] == "offline-test"
        assert len(resources_of_type(snapshot, "microsoft.authorization/POLICYDEFINITIONS")) == 1
        policy = require_resource(snapshot, type="Microsoft.Authorization/policyDefinitions", name="require-cost-center")
        assert require_resource(snapshot, id=policy.id.upper()) is policy
        assert get_resource_scope(policy.id).management_group_id == "platform"
        assert get_snapshot_value(policy.properties, "/policyRule/then/effect") == "audit"
        subscription = session.snapshot(
            INFRA / "snapshot-scopes/subscription.bicepparam",
            SnapshotMetadata(subscription_id="28c9069e-23e8-47d2-b640-00d2e0f09616"),
        )
        assert subscription.diagnostics == ()
        assert subscription.outputs["scopeId"] == "/subscriptions/28c9069e-23e8-47d2-b640-00d2e0f09616"
        tenant = session.snapshot(
            INFRA / "snapshot-scopes/tenant.bicepparam",
            SnapshotMetadata(tenant_id="ddbe463a-0554-485d-b589-0b17d60cd38b"),
        )
        assert tenant.diagnostics == ()
        assert tenant.outputs["tenantId"] == "ddbe463a-0554-485d-b589-0b17d60cd38b"
        assert session.snapshot(INFRA / "security-baseline/secure.bicepparam").diagnostics == ()
