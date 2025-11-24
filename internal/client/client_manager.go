package client

import (
	"context"
	"github.com/chalk-ai/chalk-go"
	"github.com/chalk-ai/chalk-go/gen/chalk/server/v1/serverv1connect"
	"github.com/cockroachdb/errors"
	"sync"
)

// Manager manages all gRPC clients for the Chalk provider
// It lazily initializes and caches clients to avoid duplication
type Manager struct {
	factoryInputs          *chalk.GRPCClientConfig
	builderClient          serverv1connect.BuilderServiceClient
	builderOnce            sync.Once
	builderOnceErr         error
	cloudComponentsClient  serverv1connect.CloudComponentsServiceClient
	cloudComponentsOnce    sync.Once
	cloudComponentsOnceErr error
	credentialsClient      serverv1connect.CloudAccountCredentialsServiceClient
	credentialsOnce        sync.Once
	credentialsOnceErr     error
}

// NewManager creates a new Manager instance
func NewManager(factoryInputs *chalk.GRPCClientConfig) *Manager {
	return &Manager{
		factoryInputs: factoryInputs,
	}
}

func (m *Manager) NewBuilderClient(ctx context.Context) (serverv1connect.BuilderServiceClient, error) {
	m.builderOnce.Do(func() {
		builderClient, err := GetModuleClient(ctx, m.factoryInputs, serverv1connect.NewBuilderServiceClient)
		if err != nil {
			m.builderOnceErr = errors.Wrap(err, "getting builder client")
			return
		}
		m.builderClient = builderClient
	})

	if m.builderOnceErr != nil {
		return nil, errors.Wrap(m.builderOnceErr, "get builder client")
	}
	return m.builderClient, nil
}

func (m *Manager) NewTeamClient(ctx context.Context, envid string) (serverv1connect.TeamServiceClient, error) {
	// Team client depends on environment, so we don't use sync.Once here
	teamInputs := *m.factoryInputs
	teamInputs.EnvironmentId = envid
	teamClient, err := GetModuleClient(ctx, &teamInputs, serverv1connect.NewTeamServiceClient)
	if err != nil {
		return nil, errors.Wrap(err, "getting team client")
	}
	return teamClient, nil
}

func (m *Manager) NewCloudComponentsClient(ctx context.Context) (serverv1connect.CloudComponentsServiceClient, error) {
	m.cloudComponentsOnce.Do(func() {
		cloudComponentsClient, err := GetModuleClient(ctx, m.factoryInputs, serverv1connect.NewCloudComponentsServiceClient)
		if err != nil {
			m.cloudComponentsOnceErr = errors.Wrap(err, "getting cloud components client")
			return
		}
		m.cloudComponentsClient = cloudComponentsClient
	})

	if m.cloudComponentsOnceErr != nil {
		return nil, errors.Wrap(m.cloudComponentsOnceErr, "get cloud components client")
	}
	return m.cloudComponentsClient, nil
}

func (m *Manager) NewCloudAccountCredentialsClient(ctx context.Context) (serverv1connect.CloudAccountCredentialsServiceClient, error) {
	m.credentialsOnce.Do(func() {
		credentialsClient, err := GetModuleClient(ctx, m.factoryInputs, serverv1connect.NewCloudAccountCredentialsServiceClient)
		if err != nil {
			m.credentialsOnceErr = errors.Wrap(err, "getting cloud account credentials client")
			return
		}
		m.credentialsClient = credentialsClient
	})

	if m.credentialsOnceErr != nil {
		return nil, errors.Wrap(m.credentialsOnceErr, "get cloud account credentials client")
	}
	return m.credentialsClient, nil
}
