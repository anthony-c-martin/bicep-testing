package com.github.anthonycmartin.biceptesting;

import com.azure.core.credential.TokenCredential;
import com.azure.core.management.AzureEnvironment;
import com.azure.core.management.profile.AzureProfile;
import com.azure.resourcemanager.resources.deploymentstacks.DeploymentStacksManager;
import com.azure.resourcemanager.resources.deploymentstacks.models.ActionOnUnmanage;
import com.azure.resourcemanager.resources.deploymentstacks.models.DenySettings;
import com.azure.resourcemanager.resources.deploymentstacks.models.DenySettingsMode;
import com.azure.resourcemanager.resources.deploymentstacks.models.DeploymentParameter;
import com.azure.resourcemanager.resources.deploymentstacks.models.DeploymentStack;
import com.azure.resourcemanager.resources.deploymentstacks.models.DeploymentStackProperties;
import com.azure.resourcemanager.resources.deploymentstacks.models.DeploymentStacksDeleteDetachEnum;
import com.azure.resourcemanager.resources.deploymentstacks.models.UnmanageActionManagementGroupMode;
import com.azure.resourcemanager.resources.deploymentstacks.models.UnmanageActionResourceGroupMode;
import com.azure.resourcemanager.resources.deploymentstacks.models.UnmanageActionResourceMode;
import com.azure.core.util.Context;
import java.util.Map;

interface DeploymentStackService {
        DeploymentStack deploy(
                        String resourceGroup, String stackName, Object template, Map<String, DeploymentParameter> parameters);

    void delete(String resourceGroup, String stackName);

    static DeploymentStackService create(TokenCredential credential, String subscriptionId) {
        AzureProfile profile = new AzureProfile(null, subscriptionId, AzureEnvironment.AZURE);
        DeploymentStacksManager manager = DeploymentStacksManager.authenticate(credential, profile);
        return new DeploymentStackService() {
            @Override
            public DeploymentStack deploy(
                    String resourceGroup,
                    String stackName,
                    Object template,
                    Map<String, DeploymentParameter> parameters) {
                DeploymentStacksDeleteDetachEnum delete = DeploymentStacksDeleteDetachEnum.DELETE;
                DeploymentStackProperties properties = new DeploymentStackProperties()
                        .withTemplate(template)
                        .withParameters(parameters)
                        .withActionOnUnmanage(new ActionOnUnmanage()
                                .withResources(delete)
                                .withResourceGroups(delete)
                                .withManagementGroups(delete))
                        .withDenySettings(new DenySettings().withMode(DenySettingsMode.NONE));
                return manager.deploymentStacks()
                        .define(stackName)
                        .withExistingResourceGroup(resourceGroup)
                        .withProperties(properties)
                        .create();
            }

            @Override
            public void delete(String resourceGroup, String stackName) {
                manager.deploymentStacks().delete(
                        resourceGroup,
                        stackName,
                        UnmanageActionResourceMode.DELETE,
                        UnmanageActionResourceGroupMode.DELETE,
                        UnmanageActionManagementGroupMode.DELETE,
                        false,
                        Context.NONE);
            }
        };
    }
}