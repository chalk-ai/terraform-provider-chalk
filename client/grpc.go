package client

import (
	"context"
	"net/http"

	"connectrpc.com/connect"
	serverv1 "github.com/chalk-ai/chalk-go/gen/chalk/server/v1"
	"github.com/chalk-ai/chalk-go/gen/chalk/server/v1/serverv1connect"
)

// GrpcClientOptions contains options for creating gRPC clients
type GrpcClientOptions struct {
	HTTPClient   *http.Client
	Host         string
	Interceptors []connect.Interceptor
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
		options.HTTPClient, options.Host, connect.WithInterceptors(options.Interceptors...))
}

// NewAuthClient creates a new AuthServiceClient
func NewAuthClient(ctx context.Context, options *GrpcClientOptions) serverv1connect.AuthServiceClient {
	return serverv1connect.NewAuthServiceClient(
		options.HTTPClient, options.Host, connect.WithInterceptors(options.Interceptors...))
}

// NewBuilderClient creates a new BuilderServiceClient
func NewBuilderClient(ctx context.Context, options *GrpcClientOptions) serverv1connect.BuilderServiceClient {
	return serverv1connect.NewBuilderServiceClient(
		options.HTTPClient, options.Host, connect.WithInterceptors(options.Interceptors...))
}

// NewCloudAccountCredentialsClient creates a new CloudAccountCredentialsServiceClient
func NewCloudAccountCredentialsClient(ctx context.Context, options *GrpcClientOptions) serverv1connect.CloudAccountCredentialsServiceClient {
	return serverv1connect.NewCloudAccountCredentialsServiceClient(
		options.HTTPClient, options.Host, connect.WithInterceptors(options.Interceptors...))
}

// NewCloudComponentsClient creates a new CloudComponentsServiceClient
func NewCloudComponentsClient(ctx context.Context, options *GrpcClientOptions) serverv1connect.CloudComponentsServiceClient {
	return serverv1connect.NewCloudComponentsServiceClient(
		options.HTTPClient, options.Host, connect.WithInterceptors(options.Interceptors...))
}

// NewIntegrationsClient creates a new IntegrationsServiceClient
func NewIntegrationsClient(ctx context.Context, options *GrpcClientOptions) serverv1connect.IntegrationsServiceClient {
	return serverv1connect.NewIntegrationsServiceClient(
		options.HTTPClient, options.Host, connect.WithInterceptors(options.Interceptors...))
}

// NewEnvironmentServiceClient creates a new EnvironmentServiceClient
func NewEnvironmentServiceClient(ctx context.Context, options *GrpcClientOptions) serverv1connect.EnvironmentServiceClient {
	return serverv1connect.NewEnvironmentServiceClient(
		options.HTTPClient, options.Host, connect.WithInterceptors(options.Interceptors...))
}

// NewOfflineStoreConnectionClient creates a new OfflineStoreConnectionServiceClient
func NewOfflineStoreConnectionClient(ctx context.Context, options *GrpcClientOptions) serverv1connect.OfflineStoreConnectionServiceClient {
	return serverv1connect.NewOfflineStoreConnectionServiceClient(
		options.HTTPClient, options.Host, connect.WithInterceptors(options.Interceptors...))
}
