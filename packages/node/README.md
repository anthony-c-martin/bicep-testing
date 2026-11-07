# @anthony-c-martin/bicep-testing

Test the resources, outputs, and diagnostics produced by a Bicep deployment without deploying to Azure. The package can also run opt-in integration tests against a real Azure Deployment Stack when an assertion requires live resources.

This is an independent, non-official project.

## Requirements

- Node.js 22 or later
- A `.bicepparam` entry point for the deployment under test

## Installation

```sh
npm install --save-dev @anthony-c-martin/bicep-testing@0.1.7
```

## Snapshot testing

Create one session for the test suite, evaluate the parameters file during setup, and dispose the session when the suite completes:

```js
const { BicepTestSession } = require('@anthony-c-martin/bicep-testing');

let session;
let snapshot;

beforeAll(async () => {
  session = await BicepTestSession.create('0.46.1');
  snapshot = await session.snapshot(
    'infra/main.bicepparam',
  );
}, 60_000);

afterAll(() => session.dispose());

test('storage accounts disable public blob access', () => {
  const storageAccounts = snapshot.predictedResources.filter(
    resource => resource.type === 'Microsoft.Storage/storageAccounts',
  );

  expect(snapshot.diagnostics).toHaveLength(0);
  expect(storageAccounts.length).toBeGreaterThan(0);
  storageAccounts.forEach(resource => {
    expect(resource.properties.allowBlobPublicAccess).toBe(false);
  });
});
```

`BicepTestSession.create()` downloads the requested Bicep CLI version into `~/.bicep/bin` and reuses it on later runs. The session owns a Bicep process, so always call `dispose()` after the tests finish.

Installation publishes a completed executable atomically and never overwrites
an existing cached CLI. Parallel sessions can safely use the same pinned version.

Snapshot tests run locally. The subscription, tenant, resource group, location, and deployment values provide evaluation context only; they do not need to exist, and no Azure credentials are required.

All snapshot context is optional. Supply it when assertions depend on those
values. In particular, `deploymentName` also affects offline `deployment().name`
evaluation; it is not only for live deployments.

### Explicit context and helpers (unreleased)

The development checkout adds a metadata-object overload; existing positional
calls remain supported. Management-group snapshots require Bicep CLI 0.47.16+
and fail explicitly with older versions.

```js
const { BicepTestSession, resourcesOfType, requireResource, getResourceScope, getSnapshotValue } =
  require('@anthony-c-martin/bicep-testing');

const session = await BicepTestSession.create('0.47.16');
try {
  const snapshot = await session.snapshot('infra/management-group.bicepparam', {
    managementGroupId: 'example-platform',
    location: 'eastus',
    deploymentName: 'offline-test',
  });
  expect(snapshot.diagnostics).toHaveLength(0);
  expect(resourcesOfType(snapshot, 'Microsoft.Authorization/policyDefinitions')).toHaveLength(1);
  const policy = requireResource(snapshot, {
    type: 'Microsoft.Authorization/policyDefinitions', name: 'require-cost-center',
  });
  expect(getResourceScope(policy.id).managementGroupId).toBe('example-platform');
  expect(getSnapshotValue(policy, '/properties/policyRule/then/effect')).toBe('audit');
} finally {
  session.dispose();
}
```

Use the shared [management-group fixture](../../samples/infra/snapshot-scopes/)
for this example. A selector can instead be `{ id: resourceId }`.
Resource types and IDs match case-insensitively; names match exactly.
Selection by type returns an empty array when absent. Required selection throws
for zero or multiple matches, and empty selectors/types are errors.
`getResourceScope()` supports subscription, resource-group, management-group,
tenant, child, and extension IDs; absent scope fields are `undefined`. It rejects
malformed IDs and requires GUID subscription IDs. Tenant IDs cannot be inferred
from resource IDs.

`getSnapshotValue()` accepts decoded JSON and an RFC 6901 JSON Pointer: `""`
returns the input, `/items/0` indexes an array, and `~0`/`~1` escape `~`/`/`.
It returns the original JSON value, including `null`; missing paths, invalid
escapes, and noncanonical array indices throw. Raw results remain available.

## Snapshot results

`snapshot()` returns:

- `predictedResources`: resources and resolved properties predicted for the deployment
- `outputs`: resolved deployment outputs
- `diagnostics`: Bicep compilation warnings and errors

## Live deployment testing

Create a `LiveBicepTestSession` when a test needs Azure validation or a real resource. It owns both the credential and its offline `BicepTestSession`, and forwards `snapshot()` for suites that mix offline and live assertions:

```js
const { DefaultAzureCredential } = require('@azure/identity');
const { LiveBicepTestSession } = require('@anthony-c-martin/bicep-testing');

const session = await LiveBicepTestSession.create(
	'0.46.1',
	new DefaultAzureCredential(),
);
const options = {
  filePath: 'infra/main.bicepparam',
  subscriptionId: process.env.AZURE_SUBSCRIPTION_ID,
  resourceGroup: process.env.AZURE_RESOURCE_GROUP,
  parameterOverrides: { env: 'integration' },
};

try {
	const validation = await session.validate(options);
	expect(validation.isValid).toBe(true);

	const deployment = await session.deploy(options);
	expect(deployment.succeeded).toBe(true);
  expect(deployment.resources).toEqual(expect.arrayContaining([
    expect.objectContaining({ type: 'Microsoft.Storage/storageAccounts' }),
  ]));
	await deployment.teardown();
} finally {
	session.dispose();
}
```

`DeployOptions` is a discriminated union for resource-group, subscription, and management-group targets. Subscription and management-group stacks require `location`; it is optional for resource-group stacks, which otherwise inherit their location. `filePath` is always required, `parameterOverrides` is optional, and an omitted `stackName` gets a unique `bicep-test-...` value.

`validate()` returns `isValid`, validated `resources`, `correlationId`, and an optional `error`. `deploy()` rejects compilation and option errors before Azure submission. After submission it always returns a `DeployResult`: inspect `succeeded`, `error`, `errorCode`, `errorMessage`, `outputs`, and `resources`, then call `teardown()` even when deployment failed. Error `rawData` preserves the complete ARM error JSON.

Live tests require Azure credentials and permission to validate, create, and delete Deployment Stacks and their managed resources at the selected scope. `teardown()` deletes the stack and all managed resources, treats a missing stack as already removed, and shares one promise across concurrent calls. Keep teardown in a `finally` block; the generated stack name is unique per operation, so specify one when validation and deployment must address the same stack.

## More information

The [live samples](../../samples/node/deployment.test.js) include an opt-in
idempotency test that deploys identical inputs twice to the same stack before
cleanup. Set `stackName` explicitly for repeated Node deployments; an omitted
name is generated separately for each operation. No live tests run by default.

For consistent AI-authored consumer tests, copy the
[author-bicep-tests skill](../../.github/skills/author-bicep-tests/) into your
repository. Scenario coverage reporting is not implemented by this change.

- [Runnable Jest snapshot and deployment samples](https://github.com/anthony-c-martin/bicep-testing/tree/main/samples/node)
- [Exported API](https://github.com/anthony-c-martin/bicep-testing/blob/main/api/node/bicep-testing.d.ts)
- [Project repository](https://github.com/anthony-c-martin/bicep-testing)