import { Bicep } from '@azure/bicep-rpc-client';
import { DeploymentStacksClient } from '@azure/arm-resourcesdeploymentstacks';
import { TokenCredential } from '@azure/core-auth';
import { BicepTestSession } from '../src';

const createOrUpdate = jest.fn();
const deleteStack = jest.fn();

jest.mock('@azure/arm-resourcesdeploymentstacks', () => ({
  DeploymentStacksClient: jest.fn().mockImplementation(() => ({
    deploymentStacks: {
      beginCreateOrUpdateAtResourceGroupAndWait: createOrUpdate,
      beginDeleteAtResourceGroupAndWait: deleteStack,
    },
  })),
}));

const credential = {} as TokenCredential;

describe('Deployment helper', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    deleteStack.mockResolvedValue(undefined);
  });

  it('deploys compiled parameters and returns assertion-ready results', async () => {
    const compileParams = jest.fn().mockResolvedValue({
      success: true,
      diagnostics: [],
      template: JSON.stringify({ resources: [] }),
      parameters: JSON.stringify({ parameters: { environment: { value: 'test' } } }),
    });
    createOrUpdate.mockResolvedValue({
      properties: {
        outputs: {
          endpoint: { type: 'String', value: 'https://example.test' },
        },
        resources: [
          { id: '/subscriptions/sub/resourceGroups/rg/providers/Test/widgets/one', type: 'Test/widgets' },
        ],
      },
    });
    const tester = new BicepTestSession({ compileParams } as unknown as Bicep);

    const deployment = await tester.deploy(credential, {
      filePath: './main.bicepparam',
      subscriptionId: 'sub',
      resourceGroup: 'rg',
      stackName: 'stack',
    });

    expect(DeploymentStacksClient).toHaveBeenCalledWith(credential, 'sub');
    expect(createOrUpdate).toHaveBeenCalledWith('rg', 'stack', {
      properties: {
        template: { resources: [] },
        parameters: { environment: { value: 'test' } },
        actionOnUnmanage: {
          managementGroups: 'delete',
          resourceGroups: 'delete',
          resources: 'delete',
          resourcesWithoutDeleteSupport: 'fail',
        },
        denySettings: { mode: 'none' },
      },
    });
    expect(deployment.outputs).toEqual({ endpoint: 'https://example.test' });
    expect(deployment.resources).toEqual([
      { id: '/subscriptions/sub/resourceGroups/rg/providers/Test/widgets/one', type: 'Test/widgets' },
    ]);

    await Promise.all([deployment.teardown(), deployment.teardown()]);
    expect(deleteStack).toHaveBeenCalledTimes(1);
    expect(deleteStack).toHaveBeenCalledWith('rg', 'stack', {
      unmanageActionManagementGroups: 'delete',
      unmanageActionResourceGroups: 'delete',
      unmanageActionResources: 'delete',
      unmanageActionResourcesWithoutDeleteSupport: 'fail',
    });
  });

  it('does not deploy when Bicep compilation fails', async () => {
    const compileParams = jest.fn().mockResolvedValue({
      success: false,
      diagnostics: [{ level: 'Error', code: 'BCP001', message: 'Invalid Bicep.' }],
    });
    const tester = new BicepTestSession({ compileParams } as unknown as Bicep);

    await expect(tester.deploy(credential, {
      filePath: './main.bicepparam',
      subscriptionId: 'sub',
      resourceGroup: 'rg',
      stackName: 'stack',
    })).rejects.toThrow('Error BCP001: Invalid Bicep.');
    expect(createOrUpdate).not.toHaveBeenCalled();
  });
});