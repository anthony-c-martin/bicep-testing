import { fakeSubscriptionId, fakeTenantId, repositoryUrl } from "./constants";
import type { LanguageSample } from "./types";

export const python: LanguageSample = {
  id: "python",
  label: "Python",
  highlightLanguage: "python",
  packageManager: "PyPI",
  runtime: "Python 3.11+",
  install: "python -m pip install anthonycmartin-bicep-testing",
  registry: "PyPI",
  packageUrl: "https://pypi.org/project/anthonycmartin-bicep-testing/",
  guideUrl: `${repositoryUrl}/blob/main/packages/python/bicep_testing/README.md`,
  testCommand: "python -m pytest",
    offlineStarter: `import pytest
from anthonycmartin.bicep_testing import BicepTestSession, SnapshotMetadata

@pytest.fixture(scope="module")
def session():
        with BicepTestSession.create("0.46.1") as value:
                yield value

def test_all_storage_accounts_disable_anonymous_access(session):
    metadata = SnapshotMetadata(
        tenant_id="${fakeTenantId}",
        subscription_id="${fakeSubscriptionId}",
        resource_group="example-rg",
        location="eastus",
    )
    snapshot = session.snapshot("infra/main.bicepparam", metadata)

    assert all(
        resource.properties["allowBlobPublicAccess"] is False
        for resource in snapshot.predicted_resources
        if resource.type.casefold() == "microsoft.storage/storageaccounts"
    )`,
  liveValidateStarter: `from azure.identity import DefaultAzureCredential
import pytest
from anthonycmartin.bicep_testing import (
    LiveBicepTestSession,
    ResourceGroupDeployOptions,
)

@pytest.fixture(scope="module")
def session():
    with LiveBicepTestSession.create(
        "0.46.1", DefaultAzureCredential()
    ) as value:
        yield value

def test_template_passes_azure_validation(session):
    validation = session.validate(ResourceGroupDeployOptions(
        file_path="infra/main.bicepparam",
        subscription_id=subscription_id,
        resource_group=resource_group,
    ))
    assert validation.is_valid`,
  liveDeployStarter: `from azure.identity import DefaultAzureCredential
import pytest
from anthonycmartin.bicep_testing import (
    LiveBicepTestSession,
    ResourceGroupDeployOptions,
)

@pytest.fixture(scope="module")
def session():
    with LiveBicepTestSession.create(
        "0.46.1", DefaultAzureCredential()
    ) as value:
        yield value

def test_template_deploys_successfully(session):
    options = ResourceGroupDeployOptions(
        file_path="infra/main.bicepparam",
        subscription_id=subscription_id,
        resource_group=resource_group,
    )
    with session.deploy(options) as deployment:
        assert deployment.succeeded`,
  offlineSampleUrl: `${repositoryUrl}/blob/main/samples/python/test_snapshot.py`,
  liveSampleUrl: `${repositoryUrl}/blob/main/samples/python/test_deployment.py`,
};
