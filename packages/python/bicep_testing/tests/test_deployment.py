from __future__ import annotations

from pathlib import Path
from types import SimpleNamespace
from unittest.mock import patch

import pytest
from anthonycmartin.bicep_rpc_client import CompileParamsRequest, CompileParamsResponse
from anthonycmartin.bicep_testing import (
    BicepTestSession,
    LiveBicepTestSession,
    ManagementGroupDeployOptions,
    ResourceGroupDeployOptions,
    SnapshotMetadata,
    SubscriptionDeployOptions,
)


class FakeRpcClient:
    def __init__(self, *, compilation: CompileParamsResponse | None = None) -> None:
        self.request: CompileParamsRequest | None = None
        self.compilation = compilation or CompileParamsResponse(
            success=True,
            diagnostics=(),
            template='{"resources":[]}',
            parameters='{"parameters":{"message":{"value":"hello"}}}',
        )

    def compile_params(self, request: CompileParamsRequest) -> CompileParamsResponse:
        self.request = request
        return self.compilation

    def get_snapshot(self, request: object) -> object:
        return SimpleNamespace(
            snapshot='{"predictedResources":[],"diagnostics":[],"outputs":{}}'
        )

    def close(self) -> None:
        pass


class FakePoller:
    def __init__(self, value: object = None, *, error: BaseException | None = None) -> None:
        self.value = value
        self.error = error

    def result(self) -> object:
        if self.error is not None:
            raise self.error
        return self.value


class FakeOperations:
    def __init__(self) -> None:
        self.create_rg_calls: list[tuple[object, ...]] = []
        self.create_sub_calls: list[tuple[object, ...]] = []
        self.create_mg_calls: list[tuple[object, ...]] = []
        self.validate_rg_calls: list[tuple[object, ...]] = []
        self.validate_sub_calls: list[tuple[object, ...]] = []
        self.validate_mg_calls: list[tuple[object, ...]] = []
        self.delete_rg_calls: list[tuple[object, ...]] = []
        self.delete_sub_calls: list[tuple[object, ...]] = []
        self.delete_mg_calls: list[tuple[object, ...]] = []
        self.create_error: BaseException | None = None
        self.delete_error: BaseException | None = None
        self.validate_result: object = SimpleNamespace(
            properties=SimpleNamespace(
                correlation_id="00000000-0000-0000-0000-000000000001",
                validated_resources=[
                    SimpleNamespace(
                        id="/subscriptions/sub/providers/Test/widgets/one",
                        type="Test/widgets",
                    )
                ],
            ),
            error=None,
        )

    def begin_create_or_update_at_resource_group(
        self,
        resource_group: str,
        stack_name: str,
        deployment_stack: dict[str, object],
    ) -> FakePoller:
        self.create_rg_calls.append((resource_group, stack_name, deployment_stack))
        if self.create_error is not None:
            return FakePoller(error=self.create_error)

        return FakePoller(
            SimpleNamespace(
                properties=SimpleNamespace(
                    outputs={"endpoint": {"value": "https://example.test"}},
                    resources=[
                        SimpleNamespace(
                            id="/subscriptions/sub/providers/Test/widgets/one",
                            type="Test/widgets",
                        )
                    ],
                )
            )
        )

    def begin_create_or_update_at_subscription(
        self,
        stack_name: str,
        deployment_stack: dict[str, object],
    ) -> FakePoller:
        self.create_sub_calls.append((stack_name, deployment_stack))
        return FakePoller(
            SimpleNamespace(
                outputs={"endpoint": {"value": "https://example.test"}},
                resources=[
                    SimpleNamespace(
                        id="/subscriptions/sub/providers/Test/widgets/one",
                        type="Test/widgets",
                    )
                ],
            )
        )

    def begin_create_or_update_at_management_group(
        self,
        management_group_id: str,
        stack_name: str,
        deployment_stack: dict[str, object],
    ) -> FakePoller:
        self.create_mg_calls.append((management_group_id, stack_name, deployment_stack))
        return FakePoller(
            SimpleNamespace(
                outputs={"endpoint": {"value": "https://example.test"}},
                resources=[
                    SimpleNamespace(
                        id="/providers/Microsoft.Management/managementGroups/mg/providers/Test/widgets/one",
                        type="Test/widgets",
                    )
                ],
            )
        )

    def begin_validate_stack_at_resource_group(
        self,
        resource_group: str,
        stack_name: str,
        deployment_stack: dict[str, object],
    ) -> FakePoller:
        self.validate_rg_calls.append((resource_group, stack_name, deployment_stack))
        return FakePoller(self.validate_result)

    def begin_validate_stack_at_subscription(
        self,
        stack_name: str,
        deployment_stack: dict[str, object],
    ) -> FakePoller:
        self.validate_sub_calls.append((stack_name, deployment_stack))
        return FakePoller(self.validate_result)

    def begin_validate_stack_at_management_group(
        self,
        management_group_id: str,
        stack_name: str,
        deployment_stack: dict[str, object],
    ) -> FakePoller:
        self.validate_mg_calls.append((management_group_id, stack_name, deployment_stack))
        return FakePoller(self.validate_result)

    def begin_delete_at_resource_group(
        self,
        resource_group: str,
        stack_name: str,
        **kwargs: object,
    ) -> FakePoller:
        self.delete_rg_calls.append((resource_group, stack_name, kwargs))
        if self.delete_error is not None:
            return FakePoller(error=self.delete_error)
        return FakePoller()

    def begin_delete_at_subscription(self, stack_name: str, **kwargs: object) -> FakePoller:
        self.delete_sub_calls.append((stack_name, kwargs))
        return FakePoller()

    def begin_delete_at_management_group(
        self,
        management_group_id: str,
        stack_name: str,
        **kwargs: object,
    ) -> FakePoller:
        self.delete_mg_calls.append((management_group_id, stack_name, kwargs))
        return FakePoller()


class HttpError(Exception):
    def __init__(
        self,
        message: str,
        *,
        code: str | None = None,
        status_code: int | None = None,
        body: str | None = None,
        details: dict[str, object] | None = None,
    ) -> None:
        super().__init__(message)
        self.code = code
        self.status_code = status_code
        self.details = details
        self.response = SimpleNamespace(body_as_text=body, status=status_code)


def create_live_session(
    rpc: FakeRpcClient | None = None,
) -> tuple[LiveBicepTestSession, FakeRpcClient]:
    rpc_client = rpc or FakeRpcClient()
    offline = BicepTestSession(rpc_client)  # type: ignore[arg-type]
    return LiveBicepTestSession(offline, object()), rpc_client


@pytest.mark.parametrize(
    "options_factory, expected_scope",
    [
        (
            lambda: ResourceGroupDeployOptions(
                file_path="main.bicepparam",
                subscription_id="sub",
                resource_group="rg",
                location="westus",
            ),
            "resource_group",
        ),
        (
            lambda: SubscriptionDeployOptions(
                file_path="main.bicepparam",
                subscription_id="sub",
                location="eastus",
            ),
            "subscription",
        ),
        (
            lambda: ManagementGroupDeployOptions(
                file_path="main.bicepparam",
                management_group_id="mg",
                location="eastus",
            ),
            "management_group",
        ),
    ],
)
def test_validate_deploy_and_teardown_at_all_scopes(options_factory, expected_scope: str) -> None:
    session, rpc = create_live_session()
    operations = FakeOperations()

    with patch(
        "anthonycmartin.bicep_testing.session._create_deployment_stacks_operations",
        return_value=operations,
    ):
        options = options_factory()
        validated = session.validate(options)
        deployed = session.deploy(options)
        deployed.close()

    assert validated.is_valid is True
    assert validated.error is None
    assert validated.resources[0].type == "Test/widgets"
    assert validated.correlation_id == "00000000-0000-0000-0000-000000000001"

    assert deployed.succeeded is True
    assert deployed.error is None
    assert deployed.error_code is None
    assert deployed.error_message is None
    assert deployed.outputs == {"endpoint": "https://example.test"}
    assert deployed.resources[0].type == "Test/widgets"

    assert rpc.request is not None
    assert Path(rpc.request.path).is_absolute()

    if expected_scope == "resource_group":
        assert len(operations.create_rg_calls) == 1
        assert len(operations.validate_rg_calls) == 1
        assert len(operations.delete_rg_calls) == 1
    elif expected_scope == "subscription":
        assert len(operations.create_sub_calls) == 1
        assert len(operations.validate_sub_calls) == 1
        assert len(operations.delete_sub_calls) == 1
    else:
        assert len(operations.create_mg_calls) == 1
        assert len(operations.validate_mg_calls) == 1
        assert len(operations.delete_mg_calls) == 1


def test_stack_name_defaults_to_unique_bicep_test_prefix() -> None:
    first = SubscriptionDeployOptions(
        file_path="main.bicepparam",
        subscription_id="sub",
        location="eastus",
    )
    second = SubscriptionDeployOptions(
        file_path="main.bicepparam",
        subscription_id="sub",
        location="eastus",
    )

    assert first.stack_name.startswith("bicep-test-")
    assert len(first.stack_name) == len("bicep-test-") + 32
    assert first.stack_name != second.stack_name


def test_deploy_preserves_compiled_values_and_key_vault_references() -> None:
    compilation = CompileParamsResponse(
        success=True,
        diagnostics=(),
        template='{"resources":[]}',
        parameters="""
        {
          "parameters": {
            "environment": {"value": "test"},
            "optionalValue": {"value": null},
            "secret": {
              "reference": {
                "keyVault": {
                  "id": "/subscriptions/sub/resourceGroups/rg/providers/Microsoft.KeyVault/vaults/test"
                },
                "secretName": "password",
                "secretVersion": "version"
              }
            }
          }
        }
        """,
    )
    rpc = FakeRpcClient(compilation=compilation)
    session, _ = create_live_session(rpc)
    operations = FakeOperations()

    with patch(
        "anthonycmartin.bicep_testing.session._create_deployment_stacks_operations",
        return_value=operations,
    ):
        result = session.deploy(
            ResourceGroupDeployOptions(
                file_path="main.bicepparam",
                subscription_id="sub",
                resource_group="rg",
                stack_name="stack",
                parameter_overrides={"environment": "override"},
            )
        )

    assert result.succeeded is True
    assert rpc.request is not None
    assert rpc.request.parameter_overrides == {"environment": "override"}
    payload = operations.create_rg_calls[0][2]
    properties = payload["properties"]
    assert properties["parameters"]["optionalValue"] == {"value": None}
    assert properties["parameters"]["secret"]["reference"]["secretName"] == "password"


def test_validate_returns_rich_error_data() -> None:
    session, _ = create_live_session()
    operations = FakeOperations()
    operations.validate_result = SimpleNamespace(
        properties=SimpleNamespace(validated_resources=[], correlation_id=None),
        error=SimpleNamespace(
            code="InvalidTemplate",
            message="The template is invalid.",
            target="resources[0]",
            details=[
                {
                    "code": "InvalidResource",
                    "message": "The resource is invalid.",
                }
            ],
            as_dict=lambda: {
                "code": "InvalidTemplate",
                "message": "The template is invalid.",
                "target": "resources[0]",
                "details": [
                    {
                        "code": "InvalidResource",
                        "message": "The resource is invalid.",
                    }
                ],
            },
        ),
    )

    with patch(
        "anthonycmartin.bicep_testing.session._create_deployment_stacks_operations",
        return_value=operations,
    ):
        result = session.validate(
            ResourceGroupDeployOptions(
                file_path="main.bicepparam",
                subscription_id="sub",
                resource_group="rg",
            )
        )

    assert result.is_valid is False
    assert result.error is not None
    assert result.error.code == "InvalidTemplate"
    assert result.error.message == "The template is invalid."
    assert result.error.raw_data["details"][0]["code"] == "InvalidResource"


def test_deploy_returns_post_submission_failure_with_cleanup() -> None:
    session, _ = create_live_session()
    operations = FakeOperations()
    service_body = {
        "error": {
            "code": "DeploymentStackOutOfSync",
            "message": "The stack is out of sync.",
            "details": [{"code": "ManagedResourceFailure", "message": "A managed resource failed."}],
        }
    }
    operations.create_error = HttpError(
        "The stack is out of sync.",
        code="DeploymentStackOutOfSync",
        status_code=409,
        body='{"error":{"code":"DeploymentStackOutOfSync","message":"The stack is out of sync.","details":[{"code":"ManagedResourceFailure","message":"A managed resource failed."}]}}',
    )

    with patch(
        "anthonycmartin.bicep_testing.session._create_deployment_stacks_operations",
        return_value=operations,
    ):
        result = session.deploy(
            ResourceGroupDeployOptions(
                file_path="main.bicepparam",
                subscription_id="sub",
                resource_group="rg",
                stack_name="failed-stack",
            )
        )
        result.close()
        result.close()

    assert result.succeeded is False
    assert result.error_code == "DeploymentStackOutOfSync"
    assert result.error_message == "The stack is out of sync."
    assert result.outputs == {}
    assert result.resources == ()
    assert result.error is not None
    assert result.error.raw_data == service_body
    assert len(operations.delete_rg_calls) == 1


def test_teardown_treats_404_as_already_removed() -> None:
    session, _ = create_live_session()
    operations = FakeOperations()
    operations.delete_error = HttpError("Not found", status_code=404)

    with patch(
        "anthonycmartin.bicep_testing.session._create_deployment_stacks_operations",
        return_value=operations,
    ):
        result = session.deploy(
            ResourceGroupDeployOptions(
                file_path="main.bicepparam",
                subscription_id="sub",
                resource_group="rg",
            )
        )
        result.close()

    assert len(operations.delete_rg_calls) == 1


def test_teardown_failure_can_be_retried() -> None:
    session, _ = create_live_session()
    operations = FakeOperations()
    operations.delete_error = HttpError("Conflict", status_code=409)

    with patch(
        "anthonycmartin.bicep_testing.session._create_deployment_stacks_operations",
        return_value=operations,
    ):
        result = session.deploy(
            ResourceGroupDeployOptions(
                file_path="main.bicepparam",
                subscription_id="sub",
                resource_group="rg",
            )
        )

        with pytest.raises(HttpError):
            result.close()

        operations.delete_error = None
        result.close()

    assert len(operations.delete_rg_calls) == 2


def test_invalid_options_or_compile_errors_raise_before_azure_submission() -> None:
    bad_compile = CompileParamsResponse(
        success=False,
        diagnostics=(
            SimpleNamespace(level="Error", code="BCP001", message="Invalid Bicep."),
        ),
        template=None,
        parameters=None,
    )
    session, _ = create_live_session(FakeRpcClient(compilation=bad_compile))

    with pytest.raises(ValueError, match="location must not be empty"):
        session.deploy(
            SubscriptionDeployOptions(
                file_path="main.bicepparam",
                subscription_id="sub",
                location="",
            )
        )

    operations = FakeOperations()
    with patch(
        "anthonycmartin.bicep_testing.session._create_deployment_stacks_operations",
        return_value=operations,
    ):
        with pytest.raises(RuntimeError, match="Error BCP001: Invalid Bicep."):
            session.deploy(
                ResourceGroupDeployOptions(
                    file_path="main.bicepparam",
                    subscription_id="sub",
                    resource_group="rg",
                )
            )

    assert operations.create_rg_calls == []


def test_live_session_forwards_snapshot_and_owns_offline_lifecycle() -> None:
    offline = BicepTestSession(FakeRpcClient())  # type: ignore[arg-type]
    live = LiveBicepTestSession(offline, object())

    snapshot = live.snapshot(
        "main.bicepparam",
        SnapshotMetadata(
            tenant_id="tenant",
            subscription_id="sub",
            resource_group="rg",
            location="eastus",
            deployment_name="deployment",
        ),
    )
    live.close()

    assert snapshot.predicted_resources == ()
    assert snapshot.diagnostics == ()
    assert snapshot.outputs == {}


def test_live_session_requires_credential_when_created() -> None:
    with pytest.raises(ValueError, match="credential must not be None"):
        LiveBicepTestSession.create("0.46.1", None)
