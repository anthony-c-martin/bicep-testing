from pathlib import Path
from typing import Iterator

import pytest
from anthonycmartin.bicep_testing import (
    BicepTestSession,
    SnapshotMetadata,
    get_resource_scope,
    require_resource,
    resources_of_type,
)

INFRA = Path(__file__).parents[1] / "infra"
RESOURCE_GROUP = SnapshotMetadata(
    subscription_id="28c9069e-23e8-47d2-b640-00d2e0f09616",
    resource_group="sample-rg",
    location="eastus",
)


@pytest.fixture(scope="module")
def session() -> Iterator[BicepTestSession]:
    with BicepTestSession.create("0.47.16") as value:
        yield value


def test_storage_and_key_vault_meet_the_security_baseline(session: BicepTestSession) -> None:
    # Context is unnecessary when the assertions only depend on explicit parameter values.
    result = session.snapshot(INFRA / "security-baseline/secure.bicepparam")
    assert result.diagnostics == ()
    storage = require_resource(result, type="Microsoft.Storage/storageAccounts", name="orderssecurestore")
    vault = require_resource(result, type="Microsoft.KeyVault/vaults", name="orders-secure-kv")

    assert storage.properties["allowBlobPublicAccess"] is False
    assert storage.properties["allowSharedKeyAccess"] is False
    assert storage.properties["minimumTlsVersion"] == "TLS1_2"
    assert storage.properties["publicNetworkAccess"] == "Disabled"
    assert storage.properties["supportsHttpsTrafficOnly"] is True
    assert vault.properties["enablePurgeProtection"] is True
    assert vault.properties["enableRbacAuthorization"] is True
    assert vault.properties["publicNetworkAccess"] == "Disabled"
    assert vault.properties["softDeleteRetentionInDays"] == 90


def test_development_uses_inexpensive_storage_without_an_audit_account(session: BicepTestSession) -> None:
    result = session.snapshot(INFRA / "environment-topology/dev.bicepparam", RESOURCE_GROUP)
    assert result.diagnostics == ()
    assert len(resources_of_type(result, "Microsoft.Storage/storageAccounts")) == 1
    primary = require_resource(result, type="Microsoft.Storage/storageAccounts", name="ordersdevprimary")

    assert primary.additional_properties["sku"]["name"] == "Standard_LRS"
    assert primary.additional_properties["tags"]["environment"] == "dev"
    assert primary.additional_properties["tags"]["dataClassification"] == "internal"
    assert result.outputs["primaryStorageId"] == primary.id
    assert result.outputs["auditStorageId"] is None


def test_production_uses_resilient_primary_and_audit_storage(session: BicepTestSession) -> None:
    result = session.snapshot(INFRA / "environment-topology/prod.bicepparam", RESOURCE_GROUP)
    assert result.diagnostics == ()
    assert len(resources_of_type(result, "Microsoft.Storage/storageAccounts")) == 2
    primary = require_resource(result, type="Microsoft.Storage/storageAccounts", name="ordersprodprimary")
    audit = require_resource(result, type="Microsoft.Storage/storageAccounts", name="ordersprodaudit")

    assert primary.additional_properties["sku"]["name"] == "Standard_ZRS"
    assert audit.additional_properties["sku"]["name"] == "Standard_GRS"
    assert primary.additional_properties["tags"]["environment"] == "prod"
    assert primary.additional_properties["tags"]["dataClassification"] == "confidential"
    assert result.outputs["primaryStorageId"] == primary.id
    assert result.outputs["auditStorageId"] == audit.id


def test_private_storage_connects_to_the_data_subnet_and_private_dns(session: BicepTestSession) -> None:
    result = session.snapshot(INFRA / "private-network/main.bicepparam", RESOURCE_GROUP)
    assert result.diagnostics == ()
    network_ids = result.outputs["networkIds"]
    network = require_resource(result, id=network_ids["virtualNetworkId"])
    app_subnet = require_resource(result, id=network_ids["appSubnetId"])
    data_subnet = require_resource(result, id=network_ids["dataSubnetId"])
    endpoint = require_resource(result, id=network_ids["privateEndpointId"])
    storage = require_resource(result, type="Microsoft.Storage/storageAccounts", name="ordersprivatestore")
    app_nsg = require_resource(result, type="Microsoft.Network/networkSecurityGroups", name="orders-app-nsg")
    data_nsg = require_resource(result, type="Microsoft.Network/networkSecurityGroups", name="orders-data-nsg")
    dns_link = require_resource(
        result,
        type="Microsoft.Network/privateDnsZones/virtualNetworkLinks",
        name="privatelink.blob.core.windows.net/orders-vnet-link",
    )

    assert network.properties["addressSpace"]["addressPrefixes"] == ["10.42.0.0/16"]
    assert app_subnet.properties["addressPrefix"] == "10.42.1.0/24"
    assert app_subnet.properties["networkSecurityGroup"]["id"] == app_nsg.id
    assert data_subnet.properties["addressPrefix"] == "10.42.2.0/24"
    assert data_subnet.properties["networkSecurityGroup"]["id"] == data_nsg.id
    assert data_subnet.properties["privateEndpointNetworkPolicies"] == "Disabled"
    assert endpoint.properties["subnet"]["id"] == data_subnet.id
    connection = endpoint.properties["privateLinkServiceConnections"][0]["properties"]
    assert connection["groupIds"] == ["blob"]
    assert connection["privateLinkServiceId"] == storage.id
    assert dns_link.properties["virtualNetwork"]["id"] == network.id


def test_platform_policy_audits_resources_missing_a_cost_center_tag(session: BicepTestSession) -> None:
    result = session.snapshot(
        INFRA / "snapshot-scopes/management-group.bicepparam",
        SnapshotMetadata(management_group_id="sample-platform", location="eastus"),
    )
    assert result.diagnostics == ()
    policy = require_resource(result, type="Microsoft.Authorization/policyDefinitions", name="require-cost-center")

    assert get_resource_scope(policy.id).management_group_id == "sample-platform"
    assert policy.properties["policyRule"]["if"] == {"field": "tags[costCenter]", "exists": "false"}
    assert policy.properties["policyRule"]["then"]["effect"] == "audit"
    assert result.outputs["policyId"] == policy.id
