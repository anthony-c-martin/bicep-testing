from pathlib import Path
from types import SimpleNamespace
from unittest.mock import patch

from anthonycmartin.bicep_rpc_client import CompileParamsRequest, CompileParamsResponse
from anthonycmartin.bicep_testing import BicepTestSession


class FakeRpcClient:
    def __init__(self) -> None:
        self.request: CompileParamsRequest | None = None

    def compile_params(self, request: CompileParamsRequest) -> CompileParamsResponse:
        self.request = request
        return CompileParamsResponse(
            success=True,
            diagnostics=(),
            template='{"resources":[]}',
            parameters='{"parameters":{"message":{"value":"hello"}}}',
        )

    def close(self) -> None:
        pass


class FakePoller:
    def __init__(self, value: object = None) -> None:
        self.value = value

    def result(self) -> object:
        return self.value


class FakeOperations:
    def __init__(self) -> None:
        self.request: dict[str, object] = {}
        self.delete_calls = 0

    def begin_create_or_update_at_resource_group(
        self, resource_group: str, stack_name: str, deployment_stack: dict[str, object]
    ) -> FakePoller:
        self.request = deployment_stack
        resource = SimpleNamespace(
            id="/subscriptions/sub/resourceGroups/rg/providers/Microsoft.Storage/storageAccounts/example"
        )
        stack = SimpleNamespace(
            outputs={"endpoint": {"value": "https://example.test"}},
            resources=[resource],
        )
        return FakePoller(stack)

    def begin_delete_at_resource_group(
        self, resource_group: str, stack_name: str, **kwargs: object
    ) -> FakePoller:
        self.delete_calls += 1
        assert kwargs["unmanage_action_resources_without_delete_support"] == "fail"
        return FakePoller()


def test_deploy_compiles_deploys_and_tears_down_once() -> None:
    rpc = FakeRpcClient()
    operations = FakeOperations()
    session = BicepTestSession(rpc)  # type: ignore[arg-type]

    with patch(
            "anthonycmartin.bicep_testing.session._create_deployment_stacks_operations",
        return_value=operations,
    ):
        result = session.deploy(
            object(),
            "main.bicepparam",
            "subscription",
            "resource-group",
            "stack",
            {"message": "override"},
        )

    assert rpc.request is not None
    assert Path(rpc.request.path).is_absolute()
    assert rpc.request.parameter_overrides == {"message": "override"}
    properties = operations.request["properties"]
    assert isinstance(properties, dict)
    assert properties["parameters"] == {"message": {"value": "hello"}}
    assert result.outputs == {"endpoint": "https://example.test"}
    assert result.resources[0].type == "Microsoft.Storage/storageAccounts"

    result.close()
    result.close()
    assert operations.delete_calls == 1