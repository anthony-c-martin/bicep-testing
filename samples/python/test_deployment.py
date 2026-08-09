import json
import os
from pathlib import Path
from typing import Any
from urllib.error import HTTPError
from urllib.request import Request, urlopen

import pytest
from anthonycmartin.bicep_testing import BicepTestSession
from azure.identity import DefaultAzureCredential


def require_environment_variable(name: str) -> str:
    value = os.getenv(name)
    if not value:
        pytest.skip(f"set {name} to run the live deployment samples")
    return value


def live_settings() -> tuple[str, str, str, str]:
    return (
        require_environment_variable("AZURE_SUBSCRIPTION_ID"),
        require_environment_variable("AZURE_RESOURCE_GROUP"),
        require_environment_variable("BICEP_TEST_STACK_NAME"),
        require_environment_variable("BICEP_TEST_RESOURCE_PREFIX"),
    )


def get_azure_resource(credential: DefaultAzureCredential, resource_id: str) -> tuple[int, dict[str, Any] | None]:
    token = credential.get_token("https://management.azure.com/.default")
    request = Request(
        f"https://management.azure.com{resource_id}?api-version=2023-05-01",
        headers={"Authorization": f"Bearer {token.token}"},
    )
    try:
        with urlopen(request) as response:
            return response.status, json.load(response)
    except HTTPError as error:
        return error.code, None


def test_secure_storage_is_verified_in_azure_and_removed() -> None:
    subscription_id, resource_group, stack_name, resource_prefix = live_settings()
    parameters = Path(__file__).parents[1] / "infra" / "live-storage" / "main.bicepparam"
    credential = DefaultAzureCredential()
    primary_storage_id = ""

    with BicepTestSession.create("0.43.1") as session:
        deployment = session.deploy(
            credential,
            parameters,
            subscription_id,
            resource_group,
            f"{stack_name}-secure",
            {"resourcePrefix": resource_prefix, "includeAuditStorage": False},
        )
        try:
            primary_storage_id = deployment.outputs["primaryStorageId"]
            status, storage = get_azure_resource(credential, primary_storage_id)
            assert status == 200
            assert storage is not None
            assert storage["properties"]["allowBlobPublicAccess"] is False
            assert storage["properties"]["allowSharedKeyAccess"] is False
            assert storage["properties"]["minimumTlsVersion"] == "TLS1_2"
            assert storage["properties"]["publicNetworkAccess"] == "Disabled"
            assert storage["properties"]["supportsHttpsTrafficOnly"] is True
            assert primary_storage_id in {resource.id for resource in deployment.resources}
        finally:
            deployment.close()

    assert get_azure_resource(credential, primary_storage_id)[0] == 404


def test_deployment_reconciles_removed_audit_storage_and_cleans_up() -> None:
    subscription_id, resource_group, stack_name, resource_prefix = live_settings()
    parameters = Path(__file__).parents[1] / "infra" / "live-storage" / "main.bicepparam"
    credential = DefaultAzureCredential()
    primary_storage_id = ""
    audit_storage_id = ""

    with BicepTestSession.create("0.43.1") as session:
        initial = session.deploy(
            credential,
            parameters,
            subscription_id,
            resource_group,
            stack_name,
            {"resourcePrefix": resource_prefix, "includeAuditStorage": True},
        )
        primary_storage_id = initial.outputs["primaryStorageId"]
        audit_storage_id = initial.outputs["auditStorageId"]
        assert len(initial.resources) == 2

        reconciled = session.deploy(
            credential,
            parameters,
            subscription_id,
            resource_group,
            stack_name,
            {"resourcePrefix": resource_prefix, "includeAuditStorage": False},
        )
        try:
            assert [resource.id for resource in reconciled.resources] == [primary_storage_id]
            assert get_azure_resource(credential, audit_storage_id)[0] == 404
        finally:
            reconciled.close()

    assert get_azure_resource(credential, primary_storage_id)[0] == 404
