const path = require('node:path');
const { BicepTestSession } = require('bicep-test');

const sampleDescribe = process.env.BICEP_TEST_VALIDATE_ONLY === '1' ? describe.skip : describe;

sampleDescribe('Bicep infrastructure snapshot', () => {
  let session;
  let snapshot;

  beforeAll(async () => {
    session = await BicepTestSession.create('0.43.1');
    snapshot = await session.snapshot(
      path.resolve(__dirname, '../infra/main.bicepparam'),
      '00000000-0000-0000-0000-000000000000',
      '00000000-0000-0000-0000-000000000000',
      'sample-rg',
      'eastus',
      'sample-deployment',
    );
  }, 60_000);

  afterAll(() => session.dispose());

  test('predicts the expected resources without diagnostics', () => {
    expect(snapshot.diagnostics).toHaveLength(0);
    expect(snapshot.predictedResources).toHaveLength(3);
    expect(snapshot.predictedResources).toEqual(
      expect.arrayContaining([
        expect.objectContaining({ type: 'Microsoft.Storage/storageAccounts', name: 'sampleprimary' }),
        expect.objectContaining({ type: 'Microsoft.Storage/storageAccounts', name: 'samplebackup' }),
        expect.objectContaining({ type: 'Microsoft.KeyVault/vaults', name: 'samplekv' }),
      ]),
    );
  });
});