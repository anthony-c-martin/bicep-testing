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
    return session.deploy(
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


def test_deployed_storage_meets_the_security_baseline_in_azure(session: LiveBicepTestSession) -> None:
    subscription_id, resource_group, stack_name, resource_prefix = live_settings()
    parameters = Path(__file__).parents[1] / "infra" / "live-storage" / "main.bicepparam"
    credential = DefaultAzureCredential()

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
        assert deployment.succeeded, deployment.error_message
        primary_storage_id = deployment.outputs["primaryStorageId"]
        status, storage = get_azure_resource(credential, primary_storage_id)
        assert status == 200
        assert storage is not None
        assert storage["properties"]["allowBlobPublicAccess"] is False
        assert storage["properties"]["allowSharedKeyAccess"] is False
        assert storage["properties"]["minimumTlsVersion"] == "TLS1_2"
        assert storage["properties"]["publicNetworkAccess"] == "Disabled"
        assert storage["properties"]["supportsHttpsTrafficOnly"] is True
    finally:
        deployment.close()


def test_disabling_audit_storage_removes_the_account_from_azure(session: LiveBicepTestSession) -> None:
    subscription_id, resource_group, stack_name, resource_prefix = live_settings()
    parameters = Path(__file__).parents[1] / "infra" / "live-storage" / "main.bicepparam"
    credential = DefaultAzureCredential()

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
        assert deployment.succeeded, deployment.error_message
        primary_storage_id = deployment.outputs["primaryStorageId"]
        audit_storage_id = deployment.outputs["auditStorageId"]

        deployment = deploy_storage(
            session=session,
            parameters=parameters,
            subscription_id=subscription_id,
            resource_group=resource_group,
            stack_name=stack_name,
            resource_prefix=resource_prefix,
            include_audit_storage=False,
        )
        assert deployment.succeeded, deployment.error_message
        assert deployment.outputs["primaryStorageId"] == primary_storage_id
        assert get_azure_resource(credential, primary_storage_id)[0] == 200
        assert get_azure_resource(credential, audit_storage_id)[0] == 404
    finally:
        deployment.close()


def test_identical_redeployment_preserves_resource_identity_and_security(session: LiveBicepTestSession) -> None:
    subscription_id, resource_group, stack_name, resource_prefix = live_settings()
    options = ResourceGroupDeployOptions(
        file_path=Path(__file__).parents[1] / "infra" / "live-storage" / "main.bicepparam",
        subscription_id=subscription_id, resource_group=resource_group,
        stack_name=f"{stack_name}-idempotency",
        parameter_overrides={"resourcePrefix": resource_prefix, "includeAuditStorage": False},
    )
    credential = DefaultAzureCredential()
    deployment = None
    try:
        deployment = session.deploy(options)
        assert deployment.succeeded, deployment.error_message
        primary_storage_id = deployment.outputs["primaryStorageId"]

        deployment = session.deploy(options)
        assert deployment.succeeded, deployment.error_message
        assert deployment.outputs["primaryStorageId"] == primary_storage_id
        status, storage = get_azure_resource(credential, primary_storage_id)
        assert status == 200
        assert storage is not None
        assert storage["properties"]["allowBlobPublicAccess"] is False
        assert storage["properties"]["minimumTlsVersion"] == "TLS1_2"
        assert storage["properties"]["publicNetworkAccess"] == "Disabled"
    finally:
        if deployment is not None:
            deployment.close()
