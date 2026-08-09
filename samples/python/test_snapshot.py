from pathlib import Path
from typing import Any

from anthonycmartin.bicep_testing import BicepTestSession, SnapshotMetadata


def snapshot(relative_path: str):
    parameters = Path(__file__).parents[1] / "infra" / relative_path
    metadata = SnapshotMetadata(
        tenant_id="ddbe463a-0554-485d-b589-0b17d60cd38b",
        subscription_id="28c9069e-23e8-47d2-b640-00d2e0f09616",
        resource_group="sample-rg",
        location="eastus",
        deployment_name="sample-deployment",
    )
    with BicepTestSession.create("0.43.1") as session:
        return session.snapshot(parameters, metadata)


def resource_by_type(result, resource_type: str):
    return next(resource for resource in result.predicted_resources if resource.type == resource_type)


def nested(value: dict[str, Any], *keys: str) -> Any:
    current: Any = value
    for key in keys:
        current = current[key]
    return current


def test_environment_parameters_select_topology_skus_and_tags() -> None:
    development = snapshot("environment-topology/dev.bicepparam")
    production = snapshot("environment-topology/prod.bicepparam")

    assert development.diagnostics == ()
    assert len(development.predicted_resources) == 1
    development_storage = development.predicted_resources[0]
    assert development_storage.name == "ordersdevprimary"
    assert nested(development_storage.additional_properties, "sku", "name") == "Standard_LRS"
    assert nested(development_storage.additional_properties, "tags", "environment") == "dev"
    assert development.outputs["auditStorageId"] is None

    assert [resource.name for resource in production.predicted_resources] == [
        "ordersprodprimary",
        "ordersprodaudit",
    ]
    assert nested(production.predicted_resources[0].additional_properties, "sku", "name") == "Standard_ZRS"
    assert nested(production.predicted_resources[0].additional_properties, "tags", "dataClassification") == "confidential"
    assert nested(production.predicted_resources[1].additional_properties, "sku", "name") == "Standard_GRS"


def test_security_baseline_catches_weakened_parameters() -> None:
    secure = snapshot("security-baseline/secure.bicepparam")
    insecure = snapshot("security-baseline/insecure.bicepparam")
    secure_storage = resource_by_type(secure, "Microsoft.Storage/storageAccounts")
    secure_vault = resource_by_type(secure, "Microsoft.KeyVault/vaults")
    insecure_storage = resource_by_type(insecure, "Microsoft.Storage/storageAccounts")

    assert secure_storage.properties["allowBlobPublicAccess"] is False
    assert secure_storage.properties["allowSharedKeyAccess"] is False
    assert secure_storage.properties["minimumTlsVersion"] == "TLS1_2"
    assert secure_storage.properties["publicNetworkAccess"] == "Disabled"
    assert secure_vault.properties["enablePurgeProtection"] is True
    assert secure_vault.properties["enableRbacAuthorization"] is True
    assert secure_vault.properties["softDeleteRetentionInDays"] == 90
    assert insecure_storage.properties["minimumTlsVersion"] == "TLS1_0"
    assert insecure_storage.properties["allowBlobPublicAccess"] is True


def test_private_network_references_are_wired_together() -> None:
    result = snapshot("private-network/main.bicepparam")
    resources = {resource.name: resource for resource in result.predicted_resources}
    network_ids = result.outputs["networkIds"]

    assert result.diagnostics == ()
    assert resources["orders-vnet"].properties["addressSpace"]["addressPrefixes"] == ["10.42.0.0/16"]
    assert resources["orders-vnet/app"].properties["addressPrefix"] == "10.42.1.0/24"
    assert resources["orders-vnet/app"].properties["networkSecurityGroup"]["id"].endswith("/orders-app-nsg")
    assert resources["orders-vnet/data"].properties["privateEndpointNetworkPolicies"] == "Disabled"
    assert resources["orders-storage-pe"].properties["subnet"]["id"] == network_ids["dataSubnetId"]
    connection = resources["orders-storage-pe"].properties["privateLinkServiceConnections"][0]["properties"]
    assert connection["groupIds"] == ["blob"]
    assert connection["privateLinkServiceId"].endswith("/storageAccounts/ordersprivatestore")
    assert resources["privatelink.blob.core.windows.net/orders-vnet-link"].properties["virtualNetwork"]["id"] == network_ids["virtualNetworkId"]
