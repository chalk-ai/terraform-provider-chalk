package client

import (
	"context"
	"net/http"
	"sync"

	"connectrpc.com/connect"
	serverv1connect "github.com/chalk-ai/chalk-go/gen/chalk/server/v1/serverv1connect"
)

// Manager manages all gRPC clients for the Chalk provider
// It lazily initializes and caches clients to avoid duplication
type Manager struct {
	config     *Config
	httpClient *http.Client

	// Mutex for thread-safe lazy initialization
	mu sync.RWMutex

	// Cached clients
	authClient            serverv1connect.AuthServiceClient
	teamClient            serverv1connect.TeamServiceClient
	builderClient         serverv1connect.BuilderServiceClient
	cloudComponentsClient serverv1connect.CloudComponentsServiceClient
	credentialsClient     serverv1connect.CloudAccountCredentialsServiceClient
}

// NewManager creates a new Manager instance
func NewManager(config *Config) *Manager {
	return &Manager{
		config:     config,
		httpClient: &http.Client{},
	}
}

// GetConfig returns the underlying Config for cases requiring custom configuration
func (cm *Manager) GetConfig() *Config {
	return cm.config
}

// GetHTTPClient returns the shared HTTP client
func (cm *Manager) GetHTTPClient() *http.Client {
	return cm.httpClient
}

// makeAuthInterceptor returns the appropriate authentication interceptor based on available credentials
func (cm *Manager) makeAuthInterceptor(ctx context.Context) connect.Interceptor {
	if cm.config.JWT != "" {
		return MakeJWTInterceptor(cm.config.JWT)
	}
	authClient := cm.GetAuthClient(ctx)
	return MakeTokenInjectionInterceptor(authClient, cm.config.ClientID, cm.config.ClientSecret)
}

// NewTeamClient creates a TeamServiceClient with standard headers and auth
// If envId is provided (non-empty string), adds x-chalk-env-id header
func (cm *Manager) NewTeamClient(ctx context.Context, envId ...string) serverv1connect.TeamServiceClient {
	interceptors := []connect.Interceptor{}

	// Add x-chalk-env-id header if envId is provided
	if len(envId) > 0 && envId[0] != "" {
		interceptors = append(interceptors, MakeApiServerHeaderInterceptor("x-chalk-env-id", envId[0]))
	}

	// Add standard headers and authentication
	interceptors = append(interceptors,
		MakeApiServerHeaderInterceptor("x-chalk-server", "go-api"),
		cm.makeAuthInterceptor(ctx),
	)

	return NewTeamClient(ctx, &GrpcClientOptions{
		HTTPClient:   cm.httpClient,
		Host:         cm.config.ApiServer,
		Interceptors: interceptors,
	})
}

// NewBuilderClient creates a BuilderServiceClient with standard headers and auth
func (cm *Manager) NewBuilderClient(ctx context.Context) serverv1connect.BuilderServiceClient {
	return NewBuilderClient(ctx, &GrpcClientOptions{
		HTTPClient: cm.httpClient,
		Host:       cm.config.ApiServer,
		Interceptors: []connect.Interceptor{
			MakeApiServerHeaderInterceptor("x-chalk-server", "go-api"),
			cm.makeAuthInterceptor(ctx),
		},
	})
}

// NewCloudComponentsClient creates a CloudComponentsServiceClient with standard headers and auth
func (cm *Manager) NewCloudComponentsClient(ctx context.Context) serverv1connect.CloudComponentsServiceClient {
	return NewCloudComponentsClient(ctx, &GrpcClientOptions{
		HTTPClient: cm.httpClient,
		Host:       cm.config.ApiServer,
		Interceptors: []connect.Interceptor{
			MakeApiServerHeaderInterceptor("x-chalk-server", "go-api"),
			cm.makeAuthInterceptor(ctx),
		},
	})
}

// NewCloudAccountCredentialsClient creates a CloudAccountCredentialsServiceClient with standard headers and auth
func (cm *Manager) NewCloudAccountCredentialsClient(ctx context.Context) serverv1connect.CloudAccountCredentialsServiceClient {
	return NewCloudAccountCredentialsClient(ctx, &GrpcClientOptions{
		HTTPClient: cm.httpClient,
		Host:       cm.config.ApiServer,
		Interceptors: []connect.Interceptor{
			MakeApiServerHeaderInterceptor("x-chalk-server", "go-api"),
			cm.makeAuthInterceptor(ctx),
		},
	})
}

// NewEnvironmentServiceClient creates an EnvironmentServiceClient with standard headers and auth.
// If envId is provided (non-empty), adds the x-chalk-env-id header to scope the call to that environment.
func (cm *Manager) NewEnvironmentServiceClient(ctx context.Context, envId ...string) serverv1connect.EnvironmentServiceClient {
	interceptors := []connect.Interceptor{}
	if len(envId) > 0 && envId[0] != "" {
		interceptors = append(interceptors, MakeApiServerHeaderInterceptor("x-chalk-env-id", envId[0]))
	}
	interceptors = append(interceptors,
		MakeApiServerHeaderInterceptor("x-chalk-server", "go-api"),
		cm.makeAuthInterceptor(ctx),
	)
	return NewEnvironmentServiceClient(ctx, &GrpcClientOptions{
		HTTPClient:   cm.httpClient,
		Host:         cm.config.ApiServer,
		Interceptors: interceptors,
	})
}

// NewIntegrationsClient creates an IntegrationsServiceClient with standard headers and auth.
// The envId parameter sets the x-chalk-env-id header to scope to a specific environment.
func (cm *Manager) NewIntegrationsClient(ctx context.Context, envId string) serverv1connect.IntegrationsServiceClient {
	interceptors := []connect.Interceptor{
		MakeApiServerHeaderInterceptor("x-chalk-env-id", envId),
		MakeApiServerHeaderInterceptor("x-chalk-server", "go-api"),
		cm.makeAuthInterceptor(ctx),
	}
	return NewIntegrationsClient(ctx, &GrpcClientOptions{
		HTTPClient:   cm.httpClient,
		Host:         cm.config.ApiServer,
		Interceptors: interceptors,
	})
}

// NewOfflineStoreConnectionClient creates an OfflineStoreConnectionServiceClient with standard headers and auth.
// The envId parameter sets the x-chalk-env-id header to scope to a specific environment.
func (cm *Manager) NewOfflineStoreConnectionClient(ctx context.Context, envId string) serverv1connect.OfflineStoreConnectionServiceClient {
	interceptors := []connect.Interceptor{
		MakeApiServerHeaderInterceptor("x-chalk-env-id", envId),
		MakeApiServerHeaderInterceptor("x-chalk-server", "go-api"),
		cm.makeAuthInterceptor(ctx),
	}
	return NewOfflineStoreConnectionClient(ctx, &GrpcClientOptions{
		HTTPClient:   cm.httpClient,
		Host:         cm.config.ApiServer,
		Interceptors: interceptors,
	})
}

// GetAuthClient returns the AuthServiceClient, creating it if necessary
func (cm *Manager) GetAuthClient(ctx context.Context) serverv1connect.AuthServiceClient {
	cm.mu.RLock()
	if cm.authClient != nil {
		client := cm.authClient
		cm.mu.RUnlock()
		return client
	}
	cm.mu.RUnlock()

	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Double-check after acquiring write lock
	if cm.authClient != nil {
		return cm.authClient
	}

	cm.authClient = NewAuthClient(ctx, &GrpcClientOptions{
		HTTPClient: cm.httpClient,
		Host:       cm.config.ApiServer,
		Interceptors: []connect.Interceptor{
			MakeApiServerHeaderInterceptor("x-chalk-server", "go-api"),
		},
	})

	return cm.authClient
}

// GetTeamClient returns the TeamServiceClient, creating it if necessary
func (cm *Manager) GetTeamClient(ctx context.Context) serverv1connect.TeamServiceClient {
	cm.mu.RLock()
	if cm.teamClient != nil {
		client := cm.teamClient
		cm.mu.RUnlock()
		return client
	}
	cm.mu.RUnlock()

	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Double-check after acquiring write lock
	if cm.teamClient != nil {
		return cm.teamClient
	}

	cm.teamClient = NewTeamClient(ctx, &GrpcClientOptions{
		HTTPClient: cm.httpClient,
		Host:       cm.config.ApiServer,
		Interceptors: []connect.Interceptor{
			MakeApiServerHeaderInterceptor("x-chalk-server", "go-api"),
			cm.makeAuthInterceptor(ctx),
		},
	})

	return cm.teamClient
}

// GetBuilderClient returns the BuilderServiceClient, creating it if necessary
func (cm *Manager) GetBuilderClient(ctx context.Context) serverv1connect.BuilderServiceClient {
	cm.mu.RLock()
	if cm.builderClient != nil {
		client := cm.builderClient
		cm.mu.RUnlock()
		return client
	}
	cm.mu.RUnlock()

	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Double-check after acquiring write lock
	if cm.builderClient != nil {
		return cm.builderClient
	}

	cm.builderClient = NewBuilderClient(ctx, &GrpcClientOptions{
		HTTPClient: cm.httpClient,
		Host:       cm.config.ApiServer,
		Interceptors: []connect.Interceptor{
			MakeApiServerHeaderInterceptor("x-chalk-server", "go-api"),
			cm.makeAuthInterceptor(ctx),
		},
	})

	return cm.builderClient
}

// GetCloudComponentsClient returns the CloudComponentsServiceClient, creating it if necessary
func (cm *Manager) GetCloudComponentsClient(ctx context.Context) serverv1connect.CloudComponentsServiceClient {
	cm.mu.RLock()
	if cm.cloudComponentsClient != nil {
		client := cm.cloudComponentsClient
		cm.mu.RUnlock()
		return client
	}
	cm.mu.RUnlock()

	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Double-check after acquiring write lock
	if cm.cloudComponentsClient != nil {
		return cm.cloudComponentsClient
	}

	cm.cloudComponentsClient = NewCloudComponentsClient(ctx, &GrpcClientOptions{
		HTTPClient: cm.httpClient,
		Host:       cm.config.ApiServer,
		Interceptors: []connect.Interceptor{
			MakeApiServerHeaderInterceptor("x-chalk-server", "go-api"),
			cm.makeAuthInterceptor(ctx),
		},
	})

	return cm.cloudComponentsClient
}

// GetCloudAccountCredentialsClient returns the CloudAccountCredentialsServiceClient, creating it if necessary
func (cm *Manager) GetCloudAccountCredentialsClient(ctx context.Context) serverv1connect.CloudAccountCredentialsServiceClient {
	cm.mu.RLock()
	if cm.credentialsClient != nil {
		client := cm.credentialsClient
		cm.mu.RUnlock()
		return client
	}
	cm.mu.RUnlock()

	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Double-check after acquiring write lock
	if cm.credentialsClient != nil {
		return cm.credentialsClient
	}

	cm.credentialsClient = NewCloudAccountCredentialsClient(ctx, &GrpcClientOptions{
		HTTPClient: cm.httpClient,
		Host:       cm.config.ApiServer,
		Interceptors: []connect.Interceptor{
			MakeApiServerHeaderInterceptor("x-chalk-server", "go-api"),
			cm.makeAuthInterceptor(ctx),
		},
	})

	return cm.credentialsClient
}
