import json
import os
from pathlib import Path
from typing import Any, Iterator
from urllib.error import HTTPError
from urllib.request import Request, urlopen

import pytest
from anthonycmartin.bicep_testing import LiveBicepTestSession, ResourceGroupDeployOptions
from azure.identity import DefaultAzureCredential


@pytest.fixture(scope="module")
def session() -> Iterator[LiveBicepTestSession]:
    live_settings()
    with LiveBicepTestSession.create("0.46.1", DefaultAzureCredential()) as value:
        yield value


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


def validate_storage(
    session: LiveBicepTestSession,
    parameters: Path,
    subscription_id: str,
    resource_group: str,
    stack_name: str,
    resource_prefix: str,
    include_audit_storage: bool,
) -> None:
    validation = session.validate(
        ResourceGroupDeployOptions(
            file_path=parameters,
            subscription_id=subscription_id,
            resource_group=resource_group,
            stack_name=stack_name,
            parameter_overrides={
                "resourcePrefix": resource_prefix,
                "includeAuditStorage": include_audit_storage,
            },
        )
    )
    message = validation.error.message if validation.error else "validation failed"
    assert validation.is_valid, message
    assert any(
        resource.type == "Microsoft.Storage/storageAccounts"
        for resource in validation.resources
    )


def deploy_storage(
    session: LiveBicepTestSession,
    parameters: Path,
    subscription_id: str,
    resource_group: str,
    stack_name: str,
    resource_prefix: str,
    include_audit_storage: bool,
):
    validate_storage(
        session=session,
        parameters=parameters,
        subscription_id=subscription_id,
        resource_group=resource_group,
        stack_name=f"{stack_name}-validate",
        resource_prefix=resource_prefix,
        include_audit_storage=include_audit_storage,
    )
    deployment = session.deploy(
        ResourceGroupDeployOptions(
            file_path=parameters,
            subscription_id=subscription_id,
            resource_group=resource_group,
            stack_name=stack_name,
            parameter_overrides={
                "resourcePrefix": resource_prefix,
                "includeAuditStorage": include_audit_storage,
            },
        )
    )
    assert deployment.succeeded, deployment.error_message
    return deployment


def test_secure_storage_is_verified_in_azure_and_removed(session: LiveBicepTestSession) -> None:
    subscription_id, resource_group, stack_name, resource_prefix = live_settings()
    parameters = Path(__file__).parents[1] / "infra" / "live-storage" / "main.bicepparam"
    credential = DefaultAzureCredential()
    primary_storage_id = ""

    deployment = deploy_storage(
        session=session,
        parameters=parameters,
        subscription_id=subscription_id,
        resource_group=resource_group,
        stack_name=f"{stack_name}-secure",
        resource_prefix=resource_prefix,
        include_audit_storage=False,
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


def test_deployment_reconciles_removed_audit_storage_and_cleans_up(session: LiveBicepTestSession) -> None:
    subscription_id, resource_group, stack_name, resource_prefix = live_settings()
    parameters = Path(__file__).parents[1] / "infra" / "live-storage" / "main.bicepparam"
    credential = DefaultAzureCredential()
    primary_storage_id = ""
    audit_storage_id = ""

    deployment = deploy_storage(
        session=session,
        parameters=parameters,
        subscription_id=subscription_id,
        resource_group=resource_group,
        stack_name=stack_name,
        resource_prefix=resource_prefix,
        include_audit_storage=True,
    )
    try:
        primary_storage_id = deployment.outputs["primaryStorageId"]
        audit_storage_id = deployment.outputs["auditStorageId"]
        assert len(deployment.resources) == 2

        deployment = deploy_storage(
            session=session,
            parameters=parameters,
            subscription_id=subscription_id,
            resource_group=resource_group,
            stack_name=stack_name,
            resource_prefix=resource_prefix,
            include_audit_storage=False,
        )
        assert [resource.id for resource in deployment.resources] == [primary_storage_id]
        assert get_azure_resource(credential, audit_storage_id)[0] == 404
    finally:
        deployment.close()

    assert get_azure_resource(credential, primary_storage_id)[0] == 404
