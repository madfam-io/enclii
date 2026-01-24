package helpers

import (
	"context"

	"github.com/madfam-org/enclii/packages/cli/internal/client"
	"github.com/madfam-org/enclii/packages/cli/internal/config"
	"github.com/madfam-org/enclii/packages/cli/internal/spec"
	"github.com/madfam-org/enclii/packages/sdk-go/pkg/types"
)

// ServiceContext holds resolved service information for CLI commands.
// This consolidates the common pattern of parsing service.yaml and finding the service.
type ServiceContext struct {
	Service     *client.ServiceInfo
	ProjectSlug string
	ServiceSpec *types.ServiceSpec
	APIClient   *client.APIClient
}

// ResolveService gets service info from --service flag or service.yaml.
// This consolidates the resolveService pattern used across multiple commands.
//
// Parameters:
//   - cfg: CLI configuration with API endpoint and token
//   - serviceName: Optional service name override (if empty, uses service.yaml)
//   - specFile: Path to service.yaml file (default: "service.yaml")
//
// Example:
//
//	svcCtx, err := ResolveService(ctx, cfg, serviceName, "service.yaml")
//	if err != nil {
//	    return err
//	}
//	domains, err := svcCtx.APIClient.ListCustomDomains(ctx, svcCtx.Service.ID.String())
func ResolveService(ctx context.Context, cfg *config.Config, serviceName, specFile string) (*ServiceContext, error) {
	apiClient := client.NewAPIClient(cfg.APIEndpoint, cfg.APIToken)

	// Parse service.yaml (always needed for project context)
	parser := spec.NewParser()
	serviceSpec, err := parser.ParseServiceSpec(specFile)
	if err != nil {
		if serviceName != "" {
			return nil, WrapErrorf(ActionParse, err, "%s (use --service with a valid service.yaml)", specFile)
		}
		return nil, WrapError(ActionParse, specFile, err)
	}

	// Determine service name
	svcName := serviceName
	if svcName == "" {
		svcName = serviceSpec.Metadata.Name
	}

	// Find service by name
	service, err := FindServiceByName(ctx, apiClient, serviceSpec.Metadata.Project, svcName)
	if err != nil {
		return nil, WrapErrorf(ActionFind, err, "service %s", svcName)
	}

	return &ServiceContext{
		Service:     service,
		ProjectSlug: serviceSpec.Metadata.Project,
		ServiceSpec: serviceSpec,
		APIClient:   apiClient,
	}, nil
}

// FindServiceByName finds a service by project slug and name.
// This consolidates the getServiceByName pattern used across commands.
func FindServiceByName(ctx context.Context, apiClient *client.APIClient, projectSlug, serviceName string) (*client.ServiceInfo, error) {
	services, err := apiClient.ListServicesWithInfo(ctx, projectSlug)
	if err != nil {
		return nil, err
	}

	for _, svc := range services {
		if svc.Name == serviceName {
			return svc, nil
		}
	}

	return nil, NewNotFoundError("service", serviceName, "project", projectSlug)
}

// FindEnvironmentByName finds an environment by project slug and name.
// This consolidates the getEnvironmentByName pattern used across commands.
func FindEnvironmentByName(ctx context.Context, apiClient *client.APIClient, projectSlug, envName string) (*client.EnvironmentInfo, error) {
	envs, err := apiClient.ListEnvironments(ctx, projectSlug)
	if err != nil {
		return nil, err
	}

	for _, env := range envs {
		if env.Name == envName {
			return env, nil
		}
	}

	return nil, NewNotFoundError("environment", envName, "project", projectSlug)
}

// ResolveEnvironmentID resolves an optional environment name to an ID.
// Returns nil if envName is empty (meaning "all environments").
func ResolveEnvironmentID(ctx context.Context, apiClient *client.APIClient, projectSlug, envName string) (*string, error) {
	if envName == "" {
		return nil, nil
	}

	env, err := FindEnvironmentByName(ctx, apiClient, projectSlug, envName)
	if err != nil {
		return nil, WrapErrorf(ActionFind, err, "environment %s", envName)
	}

	id := env.ID.String()
	return &id, nil
}
