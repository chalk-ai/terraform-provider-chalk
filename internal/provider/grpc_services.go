package provider

import (
	"connectrpc.com/connect"
	"context"
	serverv1 "github.com/chalk-ai/chalk-go/gen/chalk/server/v1"
	"github.com/chalk-ai/chalk-go/gen/chalk/server/v1/serverv1connect"
	"net/http"
)

type GrpcClientOptions struct {
	httpClient   *http.Client
	host         string
	interceptors []connect.Interceptor
}

func MakeApiServerHeaderInterceptor(headerName string, headerValue string) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			req.Header().Set(headerName, headerValue)
			return next(ctx, req)
		}
	}
}

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

func NewTeamClient(ctx context.Context, options *GrpcClientOptions) serverv1connect.TeamServiceClient {
	return serverv1connect.NewTeamServiceClient(
		options.httpClient, options.host, connect.WithInterceptors(options.interceptors...))
}

func NewAuthClient(ctx context.Context, options *GrpcClientOptions) serverv1connect.AuthServiceClient {
	return serverv1connect.NewAuthServiceClient(
		options.httpClient, options.host, connect.WithInterceptors(options.interceptors...))
}
