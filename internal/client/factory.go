package client

import (
	"connectrpc.com/connect"
	"context"
	"github.com/chalk-ai/chalk-go"
	serverv1 "github.com/chalk-ai/chalk-go/gen/chalk/server/v1"
	"github.com/cockroachdb/errors"
)

type Inputs struct {
	APIServer    string
	ClientId     string
	ClientSecret string
	EnvId        string
	JWT          *serverv1.GetTokenResponse
}

func GetModuleClient[T interface{}](
	ctx context.Context,
	inputs *chalk.GRPCClientConfig,
	moduleFunc func(httpClient connect.HTTPClient, baseURL string, opts ...connect.ClientOption) T,
) (T, error) {
	c, err := chalk.NewGRPCClient(ctx, inputs)
	if err != nil {
		return *new(T), errors.Wrap(err, "get chalk client")
	}
	cfg := c.GetConfig()
	client := moduleFunc(
		cfg.HTTPClient,
		cfg.ApiServer,
		c.GetMetadataServerInterceptor()...,
	)
	return client, nil
}
