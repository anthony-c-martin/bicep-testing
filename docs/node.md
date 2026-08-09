# Node

The Node package is the current reference implementation of `bicep-test`. It provides a Jest-based workflow for testing the predicted resources, outputs, and diagnostics of a Bicep deployment without deploying to Azure.

## Requirements

- Node.js 22 or later
- A `.bicepparam` entry point for the Bicep deployment under test

## Installation

```sh
npm install --save-dev bicep-test
```

## Usage

Create one session for the suite, capture the snapshot in setup, and dispose the Bicep process when the suite completes:

```ts
import { BicepTestSession, SnapshotResult } from 'bicep-test';

let session: BicepTestSession;
let snapshot: SnapshotResult;

beforeAll(async () => {
	session = await BicepTestSession.create('0.43.1');
	snapshot = await session.snapshot(
		'infra/main.bicepparam',
		'00000000-0000-0000-0000-000000000000',
		'00000000-0000-0000-0000-000000000000',
		'my-resource-group',
		'eastus',
		'my-deployment',
	);
}, 60000);

afterAll(() => session.dispose());

it('disables public blob access', () => {
	const storageAccounts = snapshot.predictedResources.filter(
		resource => resource.type === 'Microsoft.Storage/storageAccounts',
	);

	expect(storageAccounts.length).toBeGreaterThan(0);
	storageAccounts.forEach(resource => {
		expect(resource.properties?.allowBlobPublicAccess).toBe(false);
	});
});
```

`BicepTestSession.create()` downloads the requested Bicep CLI version into `~/.bicep/bin` and reuses it on later runs.

## Snapshot result

A snapshot contains:

- `predictedResources`: resources and resolved properties predicted for the deployment
- `outputs`: resolved deployment outputs
- `diagnostics`: compilation warnings and errors

Snapshot tests do not require Azure credentials or an Azure subscription.

## Live deployment tests

Use `deploy` when an assertion needs a real Azure resource or service response. The caller supplies an Azure `TokenCredential`, subscription, existing resource group, and unique stack name:

```ts
const deployment = await session.deploy(credential, {
	filePath: 'infra/main.bicepparam',
	subscriptionId,
	resourceGroup,
	stackName: `storage-test-${Date.now()}`,
});

try {
	expect(deployment.resources).toContainEqual(
		expect.objectContaining({ type: 'Microsoft.Storage/storageAccounts' }),
	);
	const endpoint = deployment.outputs.endpoint as string;
	expect((await fetch(endpoint)).status).toBe(200);
} finally {
	await deployment.teardown();
}
```

Deployment results expose normalized `outputs` and the IDs and types of stack-managed `resources`. `teardown()` is idempotent and deletes the Deployment Stack and its managed resources. Live tests require Azure credentials and deployment/deletion permissions and should run only in an explicitly configured integration-test job.

## Sample

See the runnable [Jest sample](../samples/node/snapshot.test.js) for a complete consumer test using the shared example infrastructure.

## Public API

The complete exported Node API is available in [`api/node/bicep-test.d.ts`](../api/node/bicep-test.d.ts).