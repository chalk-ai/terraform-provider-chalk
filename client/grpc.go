package client

import (
	"context"
	"net/http"
	"time"

	"connectrpc.com/connect"
	"github.com/chalk-ai/chalk-go/gen/chalk/sandbox/v1/sandboxv1connect"
	"github.com/chalk-ai/chalk-go/gen/chalk/scalinggroup/v1/scalinggroupv1connect"
	serverv1 "github.com/chalk-ai/chalk-go/gen/chalk/server/v1"
	"github.com/chalk-ai/chalk-go/gen/chalk/server/v1/serverv1connect"
)

// GrpcClientOptions contains options for creating gRPC clients
type GrpcClientOptions struct {
	HTTPClient   *http.Client
	Host         string
	Interceptors []connect.Interceptor
}

type retryPolicy struct {
	maxAttempts       int
	initialBackoff    time.Duration
	maxBackoff        time.Duration
	backoffMultiplier float64
	retryableCodes    map[connect.Code]struct{}
}

var defaultRetryPolicy = retryPolicy{
	maxAttempts:       3,
	initialBackoff:    100 * time.Millisecond,
	maxBackoff:        time.Second,
	backoffMultiplier: 2,
	retryableCodes: map[connect.Code]struct{}{
		connect.CodeUnavailable: {},
	},
}

var idempotencyRetryInterceptor = newIdempotencyRetryInterceptor(defaultRetryPolicy)

func connectClientOptions(interceptors []connect.Interceptor) []connect.ClientOption {
	clientOptions := []connect.ClientOption{connect.WithGRPC()}
	allInterceptors := make([]connect.Interceptor, 0, len(interceptors)+1)
	allInterceptors = append(allInterceptors, interceptors...)
	allInterceptors = append(allInterceptors, idempotencyRetryInterceptor)
	clientOptions = append(clientOptions, connect.WithInterceptors(allInterceptors...))
	return clientOptions
}

func newIdempotencyRetryInterceptor(policy retryPolicy) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			if policy.maxAttempts <= 1 || !isRetryableIdempotencyLevel(req.Spec().IdempotencyLevel) {
				return next(ctx, req)
			}

			var resp connect.AnyResponse
			var err error
			for attempt := 1; attempt <= policy.maxAttempts; attempt++ {
				resp, err = next(ctx, req)
				if err == nil || !policy.isRetryableCode(connect.CodeOf(err)) || attempt == policy.maxAttempts {
					return resp, err
				}
				if sleepErr := sleepWithContext(ctx, policy.backoff(attempt)); sleepErr != nil {
					return nil, sleepErr
				}
			}
			return resp, err
		}
	}
}

func isRetryableIdempotencyLevel(level connect.IdempotencyLevel) bool {
	return level == connect.IdempotencyNoSideEffects || level == connect.IdempotencyIdempotent
}

func (p retryPolicy) isRetryableCode(code connect.Code) bool {
	_, ok := p.retryableCodes[code]
	return ok
}

func (p retryPolicy) backoff(attempt int) time.Duration {
	if p.initialBackoff <= 0 {
		return 0
	}
	backoff := p.initialBackoff
	for i := 1; i < attempt; i++ {
		backoff = time.Duration(float64(backoff) * p.backoffMultiplier)
		if backoff >= p.maxBackoff {
			return p.maxBackoff
		}
	}
	return backoff
}

func sleepWithContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// MakeApiServerHeaderInterceptor creates an interceptor that adds a header to requests
func MakeApiServerHeaderInterceptor(headerName string, headerValue string) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			req.Header().Set(headerName, headerValue)
			return next(ctx, req)
		}
	}
}

// MakeTokenInjectionInterceptor creates an interceptor that fetches and injects auth tokens
func MakeTokenInjectionInterceptor(authService serverv1connect.AuthServiceClient, clientID, clientSecret string) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			token, err := authService.GetToken(ctx, connect.NewRequest(&serverv1.GetTokenRequest{
				ClientId:     clientID,
				ClientSecret: clientSecret,
				GrantType:    "client_credentials",
			}))
			if err != nil {
				return nil, err
			}
			req.Header().Set("Authorization", "Bearer "+token.Msg.AccessToken)
			return next(ctx, req)
		}
	}
}

// MakeJWTInterceptor creates an interceptor that adds a JWT token to requests
func MakeJWTInterceptor(jwt string) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			req.Header().Set("Authorization", "Bearer "+jwt)
			return next(ctx, req)
		}
	}
}

// NewTeamClient creates a new TeamServiceClient
func NewTeamClient(ctx context.Context, options *GrpcClientOptions) serverv1connect.TeamServiceClient {
	return serverv1connect.NewTeamServiceClient(
		options.HTTPClient, options.Host, connectClientOptions(options.Interceptors)...)
}

// NewAuthClient creates a new AuthServiceClient
func NewAuthClient(ctx context.Context, options *GrpcClientOptions) serverv1connect.AuthServiceClient {
	return serverv1connect.NewAuthServiceClient(
		options.HTTPClient, options.Host, connectClientOptions(options.Interceptors)...)
}

// NewBuilderClient creates a new BuilderServiceClient
func NewBuilderClient(ctx context.Context, options *GrpcClientOptions) serverv1connect.BuilderServiceClient {
	return serverv1connect.NewBuilderServiceClient(
		options.HTTPClient, options.Host, connectClientOptions(options.Interceptors)...)
}

// NewCloudAccountCredentialsClient creates a new CloudAccountCredentialsServiceClient
func NewCloudAccountCredentialsClient(ctx context.Context, options *GrpcClientOptions) serverv1connect.CloudAccountCredentialsServiceClient {
	return serverv1connect.NewCloudAccountCredentialsServiceClient(
		options.HTTPClient, options.Host, connectClientOptions(options.Interceptors)...)
}

// NewCloudComponentsClient creates a new CloudComponentsServiceClient
func NewCloudComponentsClient(ctx context.Context, options *GrpcClientOptions) serverv1connect.CloudComponentsServiceClient {
	return serverv1connect.NewCloudComponentsServiceClient(
		options.HTTPClient, options.Host, connectClientOptions(options.Interceptors)...)
}

// NewIntegrationsClient creates a new IntegrationsServiceClient
func NewIntegrationsClient(ctx context.Context, options *GrpcClientOptions) serverv1connect.IntegrationsServiceClient {
	return serverv1connect.NewIntegrationsServiceClient(
		options.HTTPClient, options.Host, connectClientOptions(options.Interceptors)...)
}

// NewEnvironmentServiceClient creates a new EnvironmentServiceClient
func NewEnvironmentServiceClient(ctx context.Context, options *GrpcClientOptions) serverv1connect.EnvironmentServiceClient {
	return serverv1connect.NewEnvironmentServiceClient(
		options.HTTPClient, options.Host, connectClientOptions(options.Interceptors)...)
}

// NewOfflineStoreConnectionClient creates a new OfflineStoreConnectionServiceClient
func NewOfflineStoreConnectionClient(ctx context.Context, options *GrpcClientOptions) serverv1connect.OfflineStoreConnectionServiceClient {
	return serverv1connect.NewOfflineStoreConnectionServiceClient(
		options.HTTPClient, options.Host, connectClientOptions(options.Interceptors)...)
}

// NewScalingGroupManagerClient creates a new ScalingGroupManagerServiceClient
func NewScalingGroupManagerClient(ctx context.Context, options *GrpcClientOptions) scalinggroupv1connect.ScalingGroupManagerServiceClient {
	return scalinggroupv1connect.NewScalingGroupManagerServiceClient(
		options.HTTPClient, options.Host, connectClientOptions(options.Interceptors)...)
}

// NewSandboxClient creates a new SandboxServiceClient
func NewSandboxClient(ctx context.Context, options *GrpcClientOptions) sandboxv1connect.SandboxServiceClient {
	return sandboxv1connect.NewSandboxServiceClient(
		options.HTTPClient, options.Host, connect.WithInterceptors(options.Interceptors...))
}
