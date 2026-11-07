import * as fs from 'fs';
import * as path from 'path';
import {
  BicepTestSession, LiveBicepTestSession, getResourceScope, getSnapshotValue, requireResource, resourcesOfType, SnapshotResult,
} from '../src';

const infra = path.resolve(__dirname, '../../../samples/infra');
const testdata = path.resolve(__dirname, '../../testdata');
const cases = JSON.parse(fs.readFileSync(path.join(testdata, 'snapshot-helpers.json'), 'utf8')) as {
  scopes: { id: string; subscriptionId?: string; resourceGroup?: string; managementGroupId?: string }[];
  invalidIds: string[];
  document: unknown;
  pointers: { pointer: string; value: unknown }[];
  invalidPointers: string[];
};

describe('snapshot helpers', () => {
  test.each(cases.scopes)('parses scope for $id', ({ id, ...scope }) => {
    expect(getResourceScope(id)).toEqual(scope);
  });
  test.each(cases.invalidIds)('rejects invalid ID %s', id => {
    expect(() => getResourceScope(id)).toThrow();
  });
  test.each(cases.pointers)('resolves $pointer', ({ pointer, value }) => {
    expect(getSnapshotValue(cases.document, pointer)).toEqual(value);
  });
  test.each(cases.invalidPointers)('rejects missing or invalid pointer %s', pointer => {
    expect(() => getSnapshotValue(cases.document, pointer)).toThrow();
  });
  test('empty pointer returns the original value', () => {
    expect(getSnapshotValue(cases.document, '')).toBe(cases.document);
  });
  test('selectors reject missing and ambiguous resources', () => {
    const snapshot: SnapshotResult = { predictedResources: [], diagnostics: [], outputs: {} };
    expect(resourcesOfType(snapshot, 'Missing/type')).toEqual([]);
    expect(() => resourcesOfType(snapshot, '')).toThrow();
    expect(() => requireResource(snapshot, { id: 'missing' })).toThrow('found 0');
    expect(() => requireResource(snapshot, { type: 'Missing/type', name: 'missing' })).toThrow('found 0');
    const resource = { id: '/id', type: 'Test/type', name: 'test', apiVersion: '1' };
    snapshot.predictedResources.push(resource, resource);
    expect(() => requireResource(snapshot, { id: '/ID' })).toThrow('found 2');
    expect(() => requireResource(snapshot, { type: 'test/TYPE', name: 'test' })).toThrow('found 2');
    expect(() => requireResource(snapshot, { type: 'Test/type', name: 'TEST' })).toThrow('found 0');
    expect(() => requireResource(snapshot, { type: '', name: 'test' })).toThrow();
  });
});

test('evaluates all scopes and uses helpers against real snapshots', async () => {
  const session = await BicepTestSession.create('0.47.16');
  try {
    const management = await session.snapshot(path.join(infra, 'snapshot-scopes/management-group.bicepparam'), {
      managementGroupId: 'platform', deploymentName: 'offline-test', location: 'eastus',
    });
    expect(management.diagnostics).toEqual([]);
    expect(management.outputs.scopeId).toBe('/providers/Microsoft.Management/managementGroups/platform');
    expect(management.outputs.deploymentName).toBe('offline-test');
    const policies = resourcesOfType(management, 'microsoft.authorization/POLICYDEFINITIONS');
    expect(policies).toHaveLength(1);
    const policy = requireResource(management, { type: policies[0].type, name: 'require-cost-center' });
    expect(requireResource(management, { id: policy.id.toUpperCase() })).toBe(policy);
    expect(getResourceScope(policy.id).managementGroupId).toBe('platform');
    expect(getSnapshotValue(policy, '/properties/policyRule/then/effect')).toBe('audit');
    const subscription = await session.snapshot(path.join(testdata, 'snapshot-scopes/subscription.bicepparam'), {
      subscriptionId: '28c9069e-23e8-47d2-b640-00d2e0f09616',
    });
    expect(subscription.diagnostics).toEqual([]);
    expect(subscription.outputs.scopeId).toBe('/subscriptions/28c9069e-23e8-47d2-b640-00d2e0f09616');
    const tenant = await session.snapshot(path.join(testdata, 'snapshot-scopes/tenant.bicepparam'), {
      tenantId: 'ddbe463a-0554-485d-b589-0b17d60cd38b',
    });
    expect(tenant.diagnostics).toEqual([]);
    expect(tenant.outputs.tenantId).toBe('ddbe463a-0554-485d-b589-0b17d60cd38b');
    expect((await session.snapshot(path.join(infra, 'security-baseline/secure.bicepparam'))).diagnostics).toEqual([]);
  } finally {
    session.dispose();
  }
}, 120_000);

test('rejects management-group metadata on older CLIs', async () => {
  const session = await BicepTestSession.create('0.46.1');
  try {
    await expect(session.snapshot(path.join(infra, 'snapshot-scopes/management-group.bicepparam'), {
      managementGroupId: 'platform',
    })).rejects.toThrow('0.47.16');
    await expect(session.snapshot('unused', { managementGroupId: ' ' })).rejects.toThrow('must not be empty');
  } finally {
    session.dispose();
  }
}, 60_000);

test('live sessions forward new metadata and preserve positional snapshot calls without credentials', async () => {
  const session = await LiveBicepTestSession.create('0.47.16', {
    async getToken() { throw new Error('Offline snapshots must not request Azure tokens.'); },
  });
  try {
    const management = await session.snapshot(path.join(infra, 'snapshot-scopes/management-group.bicepparam'), {
      managementGroupId: 'platform', location: 'eastus', deploymentName: 'offline-test',
    });
    expect(management.diagnostics).toEqual([]);
    expect(management.outputs.scopeId).toBe('/providers/Microsoft.Management/managementGroups/platform');
    const resourceGroup = await session.snapshot(path.join(infra, 'security-baseline/secure.bicepparam'),
      'ddbe463a-0554-485d-b589-0b17d60cd38b', '28c9069e-23e8-47d2-b640-00d2e0f09616',
      'test-rg', 'eastus', 'offline-test');
    expect(resourceGroup.diagnostics).toEqual([]);
    expect(resourceGroup.predictedResources).toHaveLength(2);
    for (const resource of resourceGroup.predictedResources) {
      expect(getResourceScope(resource.id).resourceGroup).toBe('test-rg');
    }
  } finally {
    session.dispose();
  }
}, 60_000);
