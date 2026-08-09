import os
from pathlib import Path

import pytest
from azure.identity import DefaultAzureCredential
from anthonycmartin.bicep_testing import BicepTestSession


def require_environment_variable(name: str) -> str:
    value = os.getenv(name)
    if not value:
        pytest.skip(f"set {name} to run the live deployment sample")
    return value


def test_infrastructure_deploys_and_is_removed_afterward() -> None:
    subscription_id = require_environment_variable("AZURE_SUBSCRIPTION_ID")
    resource_group = require_environment_variable("AZURE_RESOURCE_GROUP")
    stack_name = require_environment_variable("BICEP_TEST_STACK_NAME")
    resource_prefix = require_environment_variable("BICEP_TEST_RESOURCE_PREFIX")
    parameters = Path(__file__).parents[1] / "infra" / "main.bicepparam"

    with BicepTestSession.create("0.43.1") as session:
        with session.deploy(
            DefaultAzureCredential(),
            parameters,
            subscription_id,
            resource_group,
            stack_name,
            {"env": resource_prefix},
        ) as deployment:
            assert any(
                resource.type == "Microsoft.Storage/storageAccounts"
                for resource in deployment.resources
            )
            assert "/providers/Microsoft.Storage/storageAccounts/" in deployment.outputs["primaryStorageId"]