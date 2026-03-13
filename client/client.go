package client

import "fmt"

// Config holds the configuration for connecting to Chalk APIs
type Config struct {
	ClientID     string
	ClientSecret string
	JWT          string
	ApiServer    string
}

func (c *Config) String() string {
	return fmt.Sprintf("Config{ApiServer: %s}", c.ApiServer)
}
