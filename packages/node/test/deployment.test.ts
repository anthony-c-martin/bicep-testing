import { Bicep } from '@azure/bicep-rpc-client';
import { TokenCredential } from '@azure/core-auth';
import { DeploymentStacksClient } from '@azure/arm-resourcesdeploymentstacks';
import {
  BicepTestSession,
  DeployOptions,
  LiveBicepTestSession,
} from '../src';

const createAtResourceGroup = jest.fn();
const createAtSubscription = jest.fn();
const createAtManagementGroup = jest.fn();
const validateAtResourceGroup = jest.fn();
const validateAtSubscription = jest.fn();
const validateAtManagementGroup = jest.fn();
const deleteAtResourceGroup = jest.fn();
const deleteAtSubscription = jest.fn();
const deleteAtManagementGroup = jest.fn();

jest.mock('@azure/arm-resourcesdeploymentstacks', () => ({
  DeploymentStacksClient: jest.fn().mockImplementation(() => ({
    deploymentStacks: {
      createOrUpdateAtResourceGroup: createAtResourceGroup,
      createOrUpdateAtSubscription: createAtSubscription,
      createOrUpdateAtManagementGroup: createAtManagementGroup,
      validateStackAtResourceGroup: validateAtResourceGroup,
      validateStackAtSubscription: validateAtSubscription,
      validateStackAtManagementGroup: validateAtManagementGroup,
      deleteAtResourceGroup,
      deleteAtSubscription,
      deleteAtManagementGroup,
    },
  })),
}));

const credential = {} as TokenCredential;
const stack = {
  properties: {
    outputs: { endpoint: { type: 'String', value: 'https://example.test' } },
    resources: [{ id: '/subscriptions/sub/providers/Test/widgets/one', type: 'Test/widgets' }],
  },
};
const validation = {
  properties: {
    correlationId: '00000000-0000-0000-0000-000000000001',
    validatedResources: stack.properties.resources,
  },
};
const deleteOptions = {
  unmanageActionManagementGroups: 'delete',
  unmanageActionResourceGroups: 'delete',
  unmanageActionResources: 'delete',
  unmanageActionResourcesWithoutDeleteSupport: 'fail',
};

function poller<T>(value: T): { pollUntilDone: jest.Mock<Promise<T>> } {
  return { pollUntilDone: jest.fn().mockResolvedValue(value) };
}

function createLiveSession(bicep: Partial<Bicep>): LiveBicepTestSession {
  const offline = Reflect.construct(BicepTestSession, [bicep]) as BicepTestSession;
  return Reflect.construct(LiveBicepTestSession, [offline, credential]) as LiveBicepTestSession;
}

function successfulBicep(parameters: Record<string, unknown> = {}): Partial<Bicep> {
  return {
    compileParams: jest.fn().mockResolvedValue({
      success: true,
      diagnostics: [],
      template: JSON.stringify({ resources: [] }),
      parameters: JSON.stringify({ parameters }),
    }),
  } as unknown as Partial<Bicep>;
}

describe('Live deployment helper', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    createAtResourceGroup.mockReturnValue(poller(stack));
    createAtSubscription.mockReturnValue(poller(stack));
    createAtManagementGroup.mockReturnValue(poller(stack));
    validateAtResourceGroup.mockReturnValue(poller(validation));
    validateAtSubscription.mockReturnValue(poller(validation));
    validateAtManagementGroup.mockReturnValue(poller(validation));
    deleteAtResourceGroup.mockReturnValue(poller(undefined));
    deleteAtSubscription.mockReturnValue(poller(undefined));
    deleteAtManagementGroup.mockReturnValue(poller(undefined));
  });

  it.each([
    {
      name: 'resource group',
      options: { filePath: 'main.bicepparam', subscriptionId: 'sub', resourceGroup: 'rg', location: 'westus' },
      expectedLocation: 'westus',
      expectedClientArguments: [credential, 'sub'],
      create: createAtResourceGroup,
      validate: validateAtResourceGroup,
      remove: deleteAtResourceGroup,
      leadingArguments: ['rg'],
    },
    {
      name: 'subscription',
      options: { filePath: 'main.bicepparam', subscriptionId: 'sub', location: 'eastus' },
      expectedLocation: 'eastus',
      expectedClientArguments: [credential, 'sub'],
      create: createAtSubscription,
      validate: validateAtSubscription,
      remove: deleteAtSubscription,
      leadingArguments: [],
    },
    {
      name: 'management group',
      options: { filePath: 'main.bicepparam', managementGroupId: 'mg', location: 'eastus' },
      expectedLocation: 'eastus',
      expectedClientArguments: [credential],
      create: createAtManagementGroup,
      validate: validateAtManagementGroup,
      remove: deleteAtManagementGroup,
      leadingArguments: ['mg'],
    },
  ])('creates, validates, and deletes at $name scope', async scenario => {
    const session = createLiveSession(successfulBicep());

    const validated = await session.validate(scenario.options as DeployOptions);
    const deployed = await session.deploy(scenario.options as DeployOptions);
    await deployed.teardown();

    expect(DeploymentStacksClient).toHaveBeenCalledWith(...scenario.expectedClientArguments);
    expect(validated).toEqual({
      isValid: true,
      resources: stack.properties.resources,
      correlationId: validation.properties.correlationId,
      error: undefined,
    });
    expect(deployed).toEqual(expect.objectContaining({
      succeeded: true,
      error: undefined,
      errorCode: undefined,
      errorMessage: undefined,
      outputs: { endpoint: 'https://example.test' },
      resources: stack.properties.resources,
    }));

    const validateArguments = scenario.validate.mock.calls[0];
    const createArguments = scenario.create.mock.calls[0];
    const stackName = createArguments[scenario.leadingArguments.length];
    const validationStackName = validateArguments[scenario.leadingArguments.length];
    expect(stackName).toMatch(/^bicep-test-[0-9a-f]{32}$/);
    expect(validationStackName).toMatch(/^bicep-test-[0-9a-f]{32}$/);
    expect(stackName).not.toBe(validationStackName);
    expect(createArguments.at(-1)).toEqual(expect.objectContaining({ location: scenario.expectedLocation }));
    expect(scenario.remove).toHaveBeenCalledWith(
      ...scenario.leadingArguments,
      stackName,
      deleteOptions,
    );
  });

  it('preserves compiled values and Key Vault references', async () => {
    const parameters = {
      environment: { value: 'test' },
      optionalValue: { value: null },
      secret: {
        reference: {
          keyVault: { id: '/subscriptions/sub/resourceGroups/rg/providers/Microsoft.KeyVault/vaults/test' },
          secretName: 'password',
          secretVersion: 'version',
        },
      },
    };
    const session = createLiveSession(successfulBicep(parameters));

    await session.deploy({
      filePath: 'main.bicepparam',
      subscriptionId: 'sub',
      resourceGroup: 'rg',
      stackName: 'stack',
      parameterOverrides: { environment: 'override' },
    });

    const compileParams = (session as unknown as {
      session: { bicep: { compileParams: jest.Mock } };
    }).session.bicep.compileParams;
    expect(compileParams).toHaveBeenCalledWith(expect.objectContaining({
      parameterOverrides: { environment: 'override' },
    }));
    expect(createAtResourceGroup.mock.calls[0][2].properties.parameters).toEqual(parameters);
  });

  it('returns rich validation errors', async () => {
    const error = {
      code: 'InvalidTemplate',
      message: 'The template is invalid.',
      target: 'resources[0]',
      details: [{ code: 'InvalidResource', message: 'The resource is invalid.' }],
    };
    validateAtResourceGroup.mockReturnValue(poller({ error, properties: {} }));
    const session = createLiveSession(successfulBicep());

    const result = await session.validate({
      filePath: 'main.bicepparam',
      subscriptionId: 'sub',
      resourceGroup: 'rg',
    });

    expect(result).toEqual({
      isValid: false,
      resources: [],
      correlationId: undefined,
      error: { code: error.code, message: error.message, rawData: error },
    });
  });

  it('returns post-submission failures with cleanup available', async () => {
    const serviceBody = {
      error: {
        code: 'DeploymentStackOutOfSync',
        message: 'The stack is out of sync.',
        details: [{ code: 'ManagedResourceFailure', message: 'A managed resource failed.' }],
      },
    };
    createAtResourceGroup.mockReturnValue({
      pollUntilDone: jest.fn().mockRejectedValue(Object.assign(
        new Error(serviceBody.error.message),
        { code: serviceBody.error.code, response: { status: 409, bodyAsText: JSON.stringify(serviceBody) } },
      )),
    });
    const session = createLiveSession(successfulBicep());

    const result = await session.deploy({
      filePath: 'main.bicepparam',
      subscriptionId: 'sub',
      resourceGroup: 'rg',
      stackName: 'failed-stack',
    });

    expect(result).toEqual(expect.objectContaining({
      succeeded: false,
      errorCode: serviceBody.error.code,
      errorMessage: serviceBody.error.message,
      outputs: {},
      resources: [],
      error: {
        code: serviceBody.error.code,
        message: serviceBody.error.message,
        rawData: serviceBody,
      },
    }));
    await Promise.all([result.teardown(), result.teardown()]);
    expect(deleteAtResourceGroup).toHaveBeenCalledTimes(1);
  });

  it('treats a 404 teardown response as success', async () => {
    deleteAtResourceGroup.mockReturnValue({
      pollUntilDone: jest.fn().mockRejectedValue({ statusCode: 404 }),
    });
    const session = createLiveSession(successfulBicep());
    const result = await session.deploy({
      filePath: 'main.bicepparam',
      subscriptionId: 'sub',
      resourceGroup: 'rg',
    });

    await expect(result.teardown()).resolves.toBeUndefined();
  });

  it('rejects option and compilation errors before Azure submission', async () => {
    const compileParams = jest.fn().mockResolvedValue({
      success: false,
      diagnostics: [{ level: 'Error', code: 'BCP001', message: 'Invalid Bicep.' }],
    });
    const session = createLiveSession({ compileParams } as unknown as Bicep);

    await expect(session.deploy({
      filePath: 'main.bicepparam',
      subscriptionId: 'sub',
    } as DeployOptions)).rejects.toThrow('location is required');
    await expect(session.deploy({
      filePath: 'main.bicepparam',
      subscriptionId: 'sub',
      resourceGroup: 'rg',
    })).rejects.toThrow('Error BCP001: Invalid Bicep.');
    expect(DeploymentStacksClient).not.toHaveBeenCalled();
  });

  it.each([
    {
      filePath: 'main.bicepparam',
      managementGroupId: 'mg',
      subscriptionId: 'sub',
      location: 'eastus',
    },
    {
      filePath: 'main.bicepparam',
      resourceGroup: 'rg',
    },
  ])('rejects an ambiguous or incomplete target before compilation', async invalidOptions => {
    const compileParams = jest.fn();
    const session = createLiveSession({ compileParams } as unknown as Bicep);

    await expect(session.deploy(invalidOptions as DeployOptions)).rejects.toThrow(TypeError);

    expect(compileParams).not.toHaveBeenCalled();
    expect(DeploymentStacksClient).not.toHaveBeenCalled();
  });

  it('forwards snapshots and disposes the owned offline session', async () => {
    const snapshot = jest.fn().mockResolvedValue({ predictedResources: [], diagnostics: [], outputs: {} });
    const dispose = jest.fn();
    const offline = { snapshot, dispose } as unknown as BicepTestSession;
    const session = Reflect.construct(
      LiveBicepTestSession,
      [offline, credential],
    ) as LiveBicepTestSession;

    await session.snapshot('main.bicepparam', 'tenant', 'sub', 'rg', 'eastus', 'deployment');
    session.dispose();

    expect(snapshot).toHaveBeenCalledWith('main.bicepparam', 'tenant', 'sub', 'rg', 'eastus', 'deployment');
    expect(dispose).toHaveBeenCalledTimes(1);
  });
});