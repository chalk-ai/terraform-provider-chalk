package provider

import (
	"context"
	"net/http"
	"sync"

	"connectrpc.com/connect"
	serverv1connect "github.com/chalk-ai/chalk-go/gen/chalk/server/v1/serverv1connect"
)

// ClientManager manages all gRPC clients for the Chalk provider
// It lazily initializes and caches clients to avoid duplication
type ClientManager struct {
	chalkClient *ChalkClient
	httpClient  *http.Client

	// Mutex for thread-safe lazy initialization
	mu sync.RWMutex

	// Cached clients
	authClient            serverv1connect.AuthServiceClient
	teamClient            serverv1connect.TeamServiceClient
	builderClient         serverv1connect.BuilderServiceClient
	cloudComponentsClient serverv1connect.CloudComponentsServiceClient
	credentialsClient     serverv1connect.CloudAccountCredentialsServiceClient
}

// NewClientManager creates a new ClientManager instance
func NewClientManager(chalkClient *ChalkClient) *ClientManager {
	return &ClientManager{
		chalkClient: chalkClient,
		httpClient:  &http.Client{},
	}
}

// GetChalkClient returns the underlying ChalkClient for cases requiring custom configuration
func (cm *ClientManager) GetChalkClient() *ChalkClient {
	return cm.chalkClient
}

// GetHTTPClient returns the shared HTTP client
func (cm *ClientManager) GetHTTPClient() *http.Client {
	return cm.httpClient
}

// makeAuthInterceptor returns the appropriate authentication interceptor based on available credentials
func (cm *ClientManager) makeAuthInterceptor(ctx context.Context) connect.Interceptor {
	if cm.chalkClient.JWT != "" {
		return MakeJWTInterceptor(cm.chalkClient.JWT)
	}
	authClient := cm.GetAuthClient(ctx)
	return MakeTokenInjectionInterceptor(authClient, cm.chalkClient.ClientID, cm.chalkClient.ClientSecret)
}

// NewTeamClient creates a TeamServiceClient with standard headers and auth
// If envId is provided (non-empty string), adds x-chalk-env-id header
func (cm *ClientManager) NewTeamClient(ctx context.Context, envId ...string) serverv1connect.TeamServiceClient {
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
		httpClient:   cm.httpClient,
		host:         cm.chalkClient.ApiServer,
		interceptors: interceptors,
	})
}

// NewBuilderClient creates a BuilderServiceClient with standard headers and auth
func (cm *ClientManager) NewBuilderClient(ctx context.Context) serverv1connect.BuilderServiceClient {
	return NewBuilderClient(ctx, &GrpcClientOptions{
		httpClient: cm.httpClient,
		host:       cm.chalkClient.ApiServer,
		interceptors: []connect.Interceptor{
			MakeApiServerHeaderInterceptor("x-chalk-server", "go-api"),
			cm.makeAuthInterceptor(ctx),
		},
	})
}

// NewCloudComponentsClient creates a CloudComponentsServiceClient with standard headers and auth
func (cm *ClientManager) NewCloudComponentsClient(ctx context.Context) serverv1connect.CloudComponentsServiceClient {
	return NewCloudComponentsClient(ctx, &GrpcClientOptions{
		httpClient: cm.httpClient,
		host:       cm.chalkClient.ApiServer,
		interceptors: []connect.Interceptor{
			MakeApiServerHeaderInterceptor("x-chalk-server", "go-api"),
			cm.makeAuthInterceptor(ctx),
		},
	})
}

// NewCloudAccountCredentialsClient creates a CloudAccountCredentialsServiceClient with standard headers and auth
func (cm *ClientManager) NewCloudAccountCredentialsClient(ctx context.Context) serverv1connect.CloudAccountCredentialsServiceClient {
	return NewCloudAccountCredentialsClient(ctx, &GrpcClientOptions{
		httpClient: cm.httpClient,
		host:       cm.chalkClient.ApiServer,
		interceptors: []connect.Interceptor{
			MakeApiServerHeaderInterceptor("x-chalk-server", "go-api"),
			cm.makeAuthInterceptor(ctx),
		},
	})
}

// GetAuthClient returns the AuthServiceClient, creating it if necessary
func (cm *ClientManager) GetAuthClient(ctx context.Context) serverv1connect.AuthServiceClient {
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
		httpClient: cm.httpClient,
		host:       cm.chalkClient.ApiServer,
		interceptors: []connect.Interceptor{
			MakeApiServerHeaderInterceptor("x-chalk-server", "go-api"),
		},
	})

	return cm.authClient
}

// GetTeamClient returns the TeamServiceClient, creating it if necessary
func (cm *ClientManager) GetTeamClient(ctx context.Context) serverv1connect.TeamServiceClient {
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
		httpClient: cm.httpClient,
		host:       cm.chalkClient.ApiServer,
		interceptors: []connect.Interceptor{
			MakeApiServerHeaderInterceptor("x-chalk-server", "go-api"),
			cm.makeAuthInterceptor(ctx),
		},
	})

	return cm.teamClient
}

// GetBuilderClient returns the BuilderServiceClient, creating it if necessary
func (cm *ClientManager) GetBuilderClient(ctx context.Context) serverv1connect.BuilderServiceClient {
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
		httpClient: cm.httpClient,
		host:       cm.chalkClient.ApiServer,
		interceptors: []connect.Interceptor{
			MakeApiServerHeaderInterceptor("x-chalk-server", "go-api"),
			cm.makeAuthInterceptor(ctx),
		},
	})

	return cm.builderClient
}

// GetCloudComponentsClient returns the CloudComponentsServiceClient, creating it if necessary
func (cm *ClientManager) GetCloudComponentsClient(ctx context.Context) serverv1connect.CloudComponentsServiceClient {
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
		httpClient: cm.httpClient,
		host:       cm.chalkClient.ApiServer,
		interceptors: []connect.Interceptor{
			MakeApiServerHeaderInterceptor("x-chalk-server", "go-api"),
			cm.makeAuthInterceptor(ctx),
		},
	})

	return cm.cloudComponentsClient
}

// GetCloudAccountCredentialsClient returns the CloudAccountCredentialsServiceClient, creating it if necessary
func (cm *ClientManager) GetCloudAccountCredentialsClient(ctx context.Context) serverv1connect.CloudAccountCredentialsServiceClient {
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
		httpClient: cm.httpClient,
		host:       cm.chalkClient.ApiServer,
		interceptors: []connect.Interceptor{
			MakeApiServerHeaderInterceptor("x-chalk-server", "go-api"),
			cm.makeAuthInterceptor(ctx),
		},
	})

	return cm.credentialsClient
}
