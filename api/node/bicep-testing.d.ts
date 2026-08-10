import { TokenCredential } from '@azure/core-auth';
export type DeployOptions = {
    filePath: string;
    subscriptionId: string;
    resourceGroup: string;
    stackName: string;
    parameterOverrides?: Record<string, unknown>;
};
export declare class BicepTestSession {
    private bicep;
    private constructor();
    static create(bicepVersion: string): Promise<BicepTestSession>;
    snapshot(filePath: string, tenantId?: string, subscriptionId?: string, resourceGroup?: string, location?: string, deploymentName?: string): Promise<SnapshotResult>;
    deploy(credential: TokenCredential, options: DeployOptions): Promise<DeployResult>;
    dispose(): void;
}
export type DeployResult = {
    readonly outputs: Record<string, unknown>;
    readonly resources: DeploymentResource[];
    teardown(): Promise<void>;
};
export type DeploymentResource = {
    id: string;
    type?: string;
};
export type SnapshotResource = {
    id: string;
    type: string;
    name: string;
    apiVersion: string;
    location?: string;
    [key: string]: any;
};
export type SnapshotResult = {
    predictedResources: SnapshotResource[];
    diagnostics: string[];
    outputs: Record<string, unknown>;
};
