import { fakeSubscriptionId, fakeTenantId, repositoryUrl } from "./constants";
import type { LanguageSample } from "./types";

export const node: LanguageSample = {
  id: "node",
  label: "Node",
  highlightLanguage: "javascript",
  packageManager: "npm",
  runtime: "Node.js 22+",
  install: "npm install --save-dev @anthony-c-martin/bicep-testing",
  registry: "npm",
  packageUrl: "https://www.npmjs.com/package/@anthony-c-martin/bicep-testing",
  guideUrl: `${repositoryUrl}/blob/main/packages/node/README.md`,
  testCommand: "npm test",
  offlineStarter: `const { BicepTestSession } = require('@anthony-c-martin/bicep-testing');

describe('Bicep snapshots', () => {
  let session;

  beforeAll(async () => {
    session = await BicepTestSession.create('0.46.1');
  });

  afterAll(() => session.dispose());

  test('all storage accounts disable anonymous access', async () => {
    const snapshot = await session.snapshot(
      'infra/main.bicepparam',
      '${fakeTenantId}',
      '${fakeSubscriptionId}',
      'example-rg',
      'eastus',
    );
    expect(snapshot.predictedResources
      .filter(resource => resource.type.toLowerCase() === 'microsoft.storage/storageaccounts')
      .every(resource => resource.properties.allowBlobPublicAccess === false)
    ).toBe(true);
  });
});`,
  liveValidateStarter: `const { DefaultAzureCredential } = require('@azure/identity');
const { LiveBicepTestSession } = require('@anthony-c-martin/bicep-testing');

describe('live Bicep tests', () => {
  let session;

  beforeAll(async () => {
    session = await LiveBicepTestSession.create(
      '0.46.1', new DefaultAzureCredential());
  });

  afterAll(() => session.dispose());

  test('template passes Azure validation', async () => {
    const validation = await session.validate({
      filePath: 'infra/main.bicepparam',
      subscriptionId: process.env.AZURE_SUBSCRIPTION_ID,
      resourceGroup: process.env.AZURE_RESOURCE_GROUP,
    });
    expect(validation.isValid).toBe(true);
  });
});`,
  liveDeployStarter: `const { DefaultAzureCredential } = require('@azure/identity');
const { LiveBicepTestSession } = require('@anthony-c-martin/bicep-testing');

describe('live Bicep tests', () => {
  let session;

  beforeAll(async () => {
    session = await LiveBicepTestSession.create(
      '0.46.1', new DefaultAzureCredential());
  });

  afterAll(() => session.dispose());

  test('template deploys successfully', async () => {
    let deployment;
    try {
      deployment = await session.deploy({
        filePath: 'infra/main.bicepparam',
        subscriptionId: process.env.AZURE_SUBSCRIPTION_ID,
        resourceGroup: process.env.AZURE_RESOURCE_GROUP,
      });
      expect(deployment.succeeded).toBe(true);
    } finally {
      await deployment?.teardown();
    }
  });
});`,
  offlineSampleUrl: `${repositoryUrl}/blob/main/samples/node/snapshot.test.js`,
  liveSampleUrl: `${repositoryUrl}/blob/main/samples/node/deployment.test.js`,
};
