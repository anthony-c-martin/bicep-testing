const path = require('node:path');
const { DefaultAzureCredential } = require('@azure/identity');
const { BicepTestSession } = require('@anthony-c-martin/bicep-testing');

const subscriptionId = process.env.AZURE_SUBSCRIPTION_ID;
const resourceGroup = process.env.AZURE_RESOURCE_GROUP;
const stackName = process.env.BICEP_TEST_STACK_NAME;
const resourcePrefix = process.env.BICEP_TEST_RESOURCE_PREFIX;
const liveDescribe = subscriptionId && resourceGroup && stackName && resourcePrefix
  && process.env.BICEP_TEST_VALIDATE_ONLY !== '1' ? describe : describe.skip;
const parametersPath = path.resolve(__dirname, '../infra/live-storage/main.bicepparam');

async function getAzureResource(credential, resourceId) {
  const token = await credential.getToken('https://management.azure.com/.default');
  return fetch(`https://management.azure.com${resourceId}?api-version=2023-05-01`, {
    headers: { Authorization: `Bearer ${token.token}` },
  });
}

liveDescribe('real-world Bicep deployments', () => {
  let session;

  beforeAll(async () => {
    session = await BicepTestSession.create('0.46.1');
  }, 60_000);

  afterAll(() => session.dispose());

  test('deploys secure storage, verifies Azure state, and cleans it up', async () => {
    const credential = new DefaultAzureCredential();
    let deployment;
    let primaryStorageId;
    try {
      deployment = await session.deploy(credential, {
        filePath: parametersPath,
        subscriptionId,
        resourceGroup,
        stackName: `${stackName}-secure`,
        parameterOverrides: { resourcePrefix, includeAuditStorage: false },
      });
      primaryStorageId = deployment.outputs.primaryStorageId;
      const response = await getAzureResource(credential, primaryStorageId);
      expect(response.ok).toBe(true);
      const storage = await response.json();
      expect(storage.properties).toEqual(expect.objectContaining({
        allowBlobPublicAccess: false,
        allowSharedKeyAccess: false,
        minimumTlsVersion: 'TLS1_2',
        publicNetworkAccess: 'Disabled',
        supportsHttpsTrafficOnly: true,
      }));
      expect(deployment.resources.map(resource => resource.id)).toContain(primaryStorageId);
    } finally {
      await deployment?.teardown();
    }
    expect((await getAzureResource(credential, primaryStorageId)).status).toBe(404);
  }, 15 * 60_000);

  test('reconciles a removed audit account and tears down remaining resources', async () => {
    const credential = new DefaultAzureCredential();
    let deployment;
    let primaryStorageId;
    let auditStorageId;
    try {
      deployment = await session.deploy(credential, {
        filePath: parametersPath,
        subscriptionId,
        resourceGroup,
        stackName,
        parameterOverrides: { resourcePrefix, includeAuditStorage: true },
      });
      primaryStorageId = deployment.outputs.primaryStorageId;
      auditStorageId = deployment.outputs.auditStorageId;
      expect(deployment.resources).toHaveLength(2);

      deployment = await session.deploy(credential, {
        filePath: parametersPath,
        subscriptionId,
        resourceGroup,
        stackName,
        parameterOverrides: { resourcePrefix, includeAuditStorage: false },
      });
      expect(deployment.resources.map(resource => resource.id)).toEqual([primaryStorageId]);
      expect((await getAzureResource(credential, auditStorageId)).status).toBe(404);
    } finally {
      await deployment?.teardown();
    }
    expect((await getAzureResource(credential, primaryStorageId)).status).toBe(404);
  }, 20 * 60_000);
});