# @anthony-c-martin/bicep-testing

Test the resources, outputs, and diagnostics produced by a Bicep deployment without deploying to Azure. The package can also run opt-in integration tests against a real Azure Deployment Stack when an assertion requires live resources.

This is an independent, non-official project.

## Requirements

- Node.js 22 or later
- A `.bicepparam` entry point for the deployment under test

## Installation

```sh
npm install --save-dev @anthony-c-martin/bicep-testing@0.1.6
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
	'ddbe463a-0554-485d-b589-0b17d60cd38b',
	'28c9069e-23e8-47d2-b640-00d2e0f09616',
    'example-rg',
    'eastus',
    'example-deployment',
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

Snapshot tests run locally. The subscription, tenant, resource group, location, and deployment values provide evaluation context only; they do not need to exist, and no Azure credentials are required.

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

- [Runnable Jest snapshot and deployment samples](https://github.com/anthony-c-martin/bicep-testing/tree/main/samples/node)
- [Exported API](https://github.com/anthony-c-martin/bicep-testing/blob/main/api/node/bicep-testing.d.ts)
- [Project repository](https://github.com/anthony-c-martin/bicep-testing)