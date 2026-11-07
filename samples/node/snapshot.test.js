const path = require('node:path');
const { BicepTestSession, resourcesOfType, requireResource, getResourceScope, getSnapshotValue } = require('@anthony-c-martin/bicep-testing');

const sampleDescribe = process.env.BICEP_TEST_VALIDATE_ONLY === '1' ? describe.skip : describe;
const metadata = {
  tenantId: 'ddbe463a-0554-485d-b589-0b17d60cd38b',
  subscriptionId: '28c9069e-23e8-47d2-b640-00d2e0f09616',
  resourceGroup: 'sample-rg',
  location: 'eastus',
  deploymentName: 'sample-deployment',
};

sampleDescribe('real-world Bicep snapshots', () => {
  let session;

  beforeAll(async () => {
    session = await BicepTestSession.create('0.47.16');
  }, 60_000);

  afterAll(() => session.dispose());

  async function snapshot(relativePath) {
    return session.snapshot(path.resolve(__dirname, '../infra', relativePath), metadata);
  }

  test('minimal context and management-group resource helpers work offline', async () => {
    const minimal = await session.snapshot(path.resolve(__dirname, '../infra/security-baseline/secure.bicepparam'));
    expect(minimal.diagnostics).toHaveLength(0);
    const result = await session.snapshot(path.resolve(__dirname, '../infra/snapshot-scopes/management-group.bicepparam'), {
      managementGroupId: 'sample-platform', location: 'eastus', deploymentName: 'sample-deployment',
    });
    expect(result.diagnostics).toHaveLength(0);
    expect(resourcesOfType(result, 'Microsoft.Authorization/policyDefinitions')).toHaveLength(1);
    const policy = requireResource(result, { type: 'Microsoft.Authorization/policyDefinitions', name: 'require-cost-center' });
    expect(requireResource(result, { id: result.outputs.policyId })).toBe(policy);
    expect(getResourceScope(policy.id).managementGroupId).toBe('sample-platform');
    expect(getSnapshotValue(policy, '/properties/policyRule/then/effect')).toBe('audit');
    expect(result.outputs.scopeId).toBe('/providers/Microsoft.Management/managementGroups/sample-platform');
  });

  test('environment parameters select the expected topology, SKUs, and tags', async () => {
    const development = await snapshot('environment-topology/dev.bicepparam');
    const production = await snapshot('environment-topology/prod.bicepparam');

    expect(development.diagnostics).toHaveLength(0);
    expect(development.predictedResources).toEqual([
      expect.objectContaining({
        name: 'ordersdevprimary',
        sku: { name: 'Standard_LRS' },
        tags: expect.objectContaining({ environment: 'dev', dataClassification: 'internal' }),
      }),
    ]);
    expect(development.outputs.auditStorageId).toBeNull();

    expect(production.diagnostics).toHaveLength(0);
    expect(production.predictedResources.map(resource => resource.name)).toEqual([
      'ordersprodprimary',
      'ordersprodaudit',
    ]);
    expect(production.predictedResources[0]).toEqual(expect.objectContaining({
      sku: { name: 'Standard_ZRS' },
      tags: expect.objectContaining({ environment: 'prod', dataClassification: 'confidential' }),
    }));
    expect(production.predictedResources[1].sku.name).toBe('Standard_GRS');
    expect(production.outputs.auditStorageId).toContain('/storageAccounts/ordersprodaudit');
  });

  test('the security baseline catches a deliberately weakened parameter set', async () => {
    const secure = await snapshot('security-baseline/secure.bicepparam');
    const insecure = await snapshot('security-baseline/insecure.bicepparam');
    const secureStorage = secure.predictedResources.find(resource => resource.type === 'Microsoft.Storage/storageAccounts');
    const secureVault = secure.predictedResources.find(resource => resource.type === 'Microsoft.KeyVault/vaults');
    const insecureStorage = insecure.predictedResources.find(resource => resource.type === 'Microsoft.Storage/storageAccounts');

    expect(secure.diagnostics).toHaveLength(0);
    expect(secureStorage.properties).toEqual(expect.objectContaining({
      allowBlobPublicAccess: false,
      allowSharedKeyAccess: false,
      minimumTlsVersion: 'TLS1_2',
      publicNetworkAccess: 'Disabled',
      supportsHttpsTrafficOnly: true,
    }));
    expect(secureVault.properties).toEqual(expect.objectContaining({
      enablePurgeProtection: true,
      enableRbacAuthorization: true,
      publicNetworkAccess: 'Disabled',
      softDeleteRetentionInDays: 90,
    }));
    expect(insecureStorage.properties.minimumTlsVersion).toBe('TLS1_0');
    expect(insecureStorage.properties.allowBlobPublicAccess).toBe(true);
  });

  test('private endpoint, subnet, NSG, and DNS references are wired together', async () => {
    const result = await snapshot('private-network/main.bicepparam');
    const resources = Object.fromEntries(result.predictedResources.map(resource => [resource.name, resource]));

    expect(result.diagnostics).toHaveLength(0);
    expect(resources['orders-vnet'].properties.addressSpace.addressPrefixes).toEqual(['10.42.0.0/16']);
    expect(resources['orders-vnet/app'].properties).toEqual(expect.objectContaining({
      addressPrefix: '10.42.1.0/24',
      networkSecurityGroup: expect.objectContaining({ id: expect.stringContaining('/orders-app-nsg') }),
    }));
    expect(resources['orders-vnet/data'].properties).toEqual(expect.objectContaining({
      addressPrefix: '10.42.2.0/24',
      privateEndpointNetworkPolicies: 'Disabled',
      networkSecurityGroup: expect.objectContaining({ id: expect.stringContaining('/orders-data-nsg') }),
    }));
    expect(resources['orders-storage-pe'].properties.subnet.id).toBe(result.outputs.networkIds.dataSubnetId);
    expect(resources['orders-storage-pe'].properties.privateLinkServiceConnections[0].properties).toEqual(expect.objectContaining({
      groupIds: ['blob'],
      privateLinkServiceId: expect.stringContaining('/storageAccounts/ordersprivatestore'),
    }));
    expect(resources['privatelink.blob.core.windows.net/orders-vnet-link'].properties.virtualNetwork.id)
      .toBe(result.outputs.networkIds.virtualNetworkId);
  });
});