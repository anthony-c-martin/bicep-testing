const path = require('node:path');
const { DefaultAzureCredential } = require('@azure/identity');
const { BicepTestSession } = require('bicep-test');

const subscriptionId = process.env.AZURE_SUBSCRIPTION_ID;
const resourceGroup = process.env.AZURE_RESOURCE_GROUP;
const stackName = process.env.BICEP_TEST_STACK_NAME;
const resourcePrefix = process.env.BICEP_TEST_RESOURCE_PREFIX;
const liveDescribe = subscriptionId && resourceGroup && stackName && resourcePrefix
  && process.env.BICEP_TEST_VALIDATE_ONLY !== '1'
  ? describe
  : describe.skip;

liveDescribe('Bicep infrastructure deployment', () => {
  test('deploys resources and removes them afterward', async () => {
    const session = await BicepTestSession.create('0.43.1');
    let deployment;
    try {
      deployment = await session.deploy(new DefaultAzureCredential(), {
        filePath: path.resolve(__dirname, '../infra/main.bicepparam'),
        subscriptionId,
        resourceGroup,
        stackName,
        parameterOverrides: { env: resourcePrefix },
      });

      expect(deployment.resources).toEqual(expect.arrayContaining([
        expect.objectContaining({ type: 'Microsoft.Storage/storageAccounts' }),
      ]));
      expect(deployment.outputs.primaryStorageId).toContain('/providers/Microsoft.Storage/storageAccounts/');
    } finally {
      await deployment?.teardown();
      session.dispose();
    }
  }, 15 * 60_000);
});