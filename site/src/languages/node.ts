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

test('template compiles without diagnostics', async () => {
  const session = await BicepTestSession.create('0.46.1');
  try {
    const snapshot = await session.snapshot(
      'infra/main.bicepparam',
      '${fakeTenantId}',
      '${fakeSubscriptionId}',
      'example-rg',
      'eastus',
    );
    expect(snapshot.diagnostics).toHaveLength(0);
  } finally {
    session.dispose();
  }
});`,
  liveValidateStarter: `const { DefaultAzureCredential } = require('@azure/identity');
const { LiveBicepTestSession } = require('@anthony-c-martin/bicep-testing');

const session = await LiveBicepTestSession.create(
  '0.46.1',
  new DefaultAzureCredential(),
);
try {
  const validation = await session.validate({
    filePath: 'infra/main.bicepparam',
    subscriptionId: process.env.AZURE_SUBSCRIPTION_ID,
    resourceGroup: process.env.AZURE_RESOURCE_GROUP,
  });
  expect(validation.isValid).toBe(true);
} finally {
  session.dispose();
}`,
  liveDeployStarter: `const { DefaultAzureCredential } = require('@azure/identity');
const { LiveBicepTestSession } = require('@anthony-c-martin/bicep-testing');

const session = await LiveBicepTestSession.create(
  '0.46.1',
  new DefaultAzureCredential(),
);
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
  session.dispose();
}`,
  offlineSampleUrl: `${repositoryUrl}/blob/main/samples/node/snapshot.test.js`,
  liveSampleUrl: `${repositoryUrl}/blob/main/samples/node/deployment.test.js`,
};
