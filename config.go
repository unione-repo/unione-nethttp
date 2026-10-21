// Package unionewrapper plugs the UniOne Go SDK into net/http applications: the client configured from the
// environment, a middleware that puts it into the request context and the webhook endpoint.
package unionewrapper

import (
	"errors"
	"net/http"
	"os"
	"strconv"
	"time"

	unione "github.com/unione-repo/unione-go-sdk"
	"github.com/unione-repo/unione-go-sdk/client"
)

// Platform is the package and its version in the "platform" query parameter of the requests, before the SDK one.
const Platform = "nethttp.1.2.3"

// Environment variables read by ConfigFromEnv.
const (
	EnvAPIKey      = "UNIONE_API_KEY"
	EnvHost        = "UNIONE_HOST"
	EnvTimeout     = "UNIONE_TIMEOUT"
	EnvWebhookKey  = "UNIONE_WEBHOOK_KEY"
	EnvWebhookPath = "UNIONE_WEBHOOK_PATH"
)

// DefaultWebhookPath is the path of the webhook endpoint when WebhookPath is empty.
const DefaultWebhookPath = "/unione/webhook"

// Config holds the settings of the client and of the webhook endpoint.
type Config struct {
	// APIKey is the user or project API key.
	APIKey string
	// Host is an API host (datacenter) or a full base URL; empty: the default host.
	Host string
	// Timeout of the requests; 0: 30 seconds.
	Timeout time.Duration
	// WebhookKey is the API key the webhooks are signed with; empty: APIKey.
	WebhookKey string
	// WebhookPath is the path of the webhook endpoint; empty: DefaultWebhookPath.
	WebhookPath string
}

// ConfigFromEnv reads the configuration from the UNIONE_* environment variables
// (UNIONE_TIMEOUT is in seconds).
func ConfigFromEnv() Config {
	c := Config{
		APIKey:      os.Getenv(EnvAPIKey),
		Host:        os.Getenv(EnvHost),
		WebhookKey:  os.Getenv(EnvWebhookKey),
		WebhookPath: os.Getenv(EnvWebhookPath),
	}
	if s, err := strconv.ParseFloat(os.Getenv(EnvTimeout), 64); err == nil && s > 0 {
		c.Timeout = time.Duration(s * float64(time.Second))
	}
	return c
}

// Client returns the API client for the configuration.
func (c Config) Client() *unione.Client {
	config := client.NewConfiguration(c.APIKey)
	config.Platform = Platform
	if c.Host != "" {
		config.BaseURL = client.BaseURLFor(c.Host)
	}
	if c.Timeout > 0 {
		config.HTTPClient = &http.Client{Timeout: c.Timeout}
	}
	return unione.NewClientWithConfig(config)
}

// SigningKey is the key the webhooks are checked with: WebhookKey or APIKey.
func (c Config) SigningKey() string {
	if c.WebhookKey != "" {
		return c.WebhookKey
	}
	return c.APIKey
}

// Path is the path of the webhook endpoint: WebhookPath or DefaultWebhookPath.
func (c Config) Path() string {
	if c.WebhookPath != "" {
		return c.WebhookPath
	}
	return DefaultWebhookPath
}

// ErrNoAPIKey is returned by NewClientFromEnv when UNIONE_API_KEY is not set.
var ErrNoAPIKey = errors.New("unionewrapper: " + EnvAPIKey + " is not set")

// NewClientFromEnv returns the API client configured from the environment.
func NewClientFromEnv() (*unione.Client, error) {
	c := ConfigFromEnv()
	if c.APIKey == "" {
		return nil, ErrNoAPIKey
	}
	return c.Client(), nil
}
