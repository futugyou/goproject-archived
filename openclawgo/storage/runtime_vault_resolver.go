package storage

import "context"

type RuntimeVaultResolver struct {
	vault          IVault
	configResolver *VaultConfigurationResolver
}

func NewRuntimeVaultResolver(vault IVault, configResolver *VaultConfigurationResolver) *RuntimeVaultResolver {
	return &RuntimeVaultResolver{
		vault:          vault,
		configResolver: configResolver,
	}
}

func (r *RuntimeVaultResolver) ResolveField(ctx context.Context, fieldValue, fieldName, callerId string, callerType VaultCallerType) (string, error) {
	if len(fieldValue) == 0 {
		return "", nil
	}

	secretName, f := TryParseVaultReference(fieldValue)
	if !f {
		return fieldValue, nil
	}

	resolvedValue, err := r.configResolver.ResolveSecret(ctx, secretName, r.vault)
	if err != nil {
		return "", err
	}

	return resolvedValue, nil
}

func (r *RuntimeVaultResolver) ResolveProviderFields(ctx context.Context, endpoint, apiKey, deploymentName, providerId string) (map[string]string, error) {
	resolved := map[string]string{}
	t := "ModelProvider:" + providerId
	if _, ok := TryParseVaultReference(endpoint); ok {
		if str, err := r.ResolveField(ctx, endpoint, "Endpoint", t, VaultCallerTypeSystem); err == nil {
			resolved["Endpoint"] = str
		}
	}

	if _, ok := TryParseVaultReference(apiKey); ok {
		if str, err := r.ResolveField(ctx, apiKey, "ApiKey", t, VaultCallerTypeSystem); err == nil {
			resolved["ApiKey"] = str
		}
	}

	if _, ok := TryParseVaultReference(deploymentName); ok {
		if str, err := r.ResolveField(ctx, deploymentName, "DeploymentName", t, VaultCallerTypeSystem); err == nil {
			resolved["DeploymentName"] = str
		}
	}

	return resolved, nil
}

func (r *RuntimeVaultResolver) ResolveProfileFields(ctx context.Context, endpoint, apiKey, deploymentName, profileName string) (map[string]string, error) {
	resolved := map[string]string{}
	t := "AgentProfile:" + profileName
	if _, ok := TryParseVaultReference(endpoint); ok {
		if str, err := r.ResolveField(ctx, endpoint, "Endpoint", t, VaultCallerTypeSystem); err == nil {
			resolved["Endpoint"] = str
		}
	}

	if _, ok := TryParseVaultReference(apiKey); ok {
		if str, err := r.ResolveField(ctx, apiKey, "ApiKey", t, VaultCallerTypeSystem); err == nil {
			resolved["ApiKey"] = str
		}
	}

	if _, ok := TryParseVaultReference(deploymentName); ok {
		if str, err := r.ResolveField(ctx, deploymentName, "DeploymentName", t, VaultCallerTypeSystem); err == nil {
			resolved["DeploymentName"] = str
		}
	}

	return resolved, nil
}
