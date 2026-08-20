package conns

import (
	"fmt"
	"os"
	"sync"

	kion "github.com/kionsoftware/kion-sdk-go"
	generated "github.com/kionsoftware/kion-sdk-go/generated/v3_16"
)

var (
	sharedClient     *KionClient
	sharedClientOnce sync.Once
	errSharedClient  error
)

// SharedClient returns a KionClient built from environment variables. It is
// safe for concurrent use and caches the client after the first call. Tests
// use this in CheckDestroy / CheckExists helpers and sweepers that need to
// call the SDK directly rather than going through the provider.
func SharedClient() (*KionClient, error) {
	sharedClientOnce.Do(func() {
		sharedClient, errSharedClient = buildSharedClient()
	})
	return sharedClient, errSharedClient
}

// buildSharedClient reads the environment and constructs a KionClient. It holds
// no global state, so tests can exercise it directly with t.Setenv without the
// sync.Once caching in SharedClient getting in the way.
func buildSharedClient() (*KionClient, error) {
	apiURL := os.Getenv("KION_API_URL")
	if apiURL == "" {
		return nil, fmt.Errorf("KION_API_URL must be set")
	}

	apiKey := os.Getenv("KION_API_KEY")
	authToken := os.Getenv("KION_AUTH_TOKEN")
	if apiKey == "" && authToken == "" {
		return nil, fmt.Errorf("KION_API_KEY or KION_AUTH_TOKEN must be set")
	}

	var opts []kion.Option
	if apiKey != "" {
		opts = append(opts, kion.WithAPIKey(apiKey))
	}
	if authToken != "" {
		opts = append(opts, kion.WithBearerToken(authToken))
	}
	if v := os.Getenv("KION_SKIP_SSL_VALIDATION"); v == "true" || v == "1" {
		opts = append(opts, kion.WithSkipVerify(true))
	}

	sdkClient, err := generated.New(apiURL, opts...)
	if err != nil {
		return nil, fmt.Errorf("creating shared Kion client: %w", err)
	}

	// APIURL must be the API root, matching what the provider's Configure
	// stores — generated.New normalizes the same way for the SDK client, so the
	// raw helpers and the SDK agree on where the API lives.
	return &KionClient{Client: sdkClient, APIURL: kion.NormalizeServerURL(apiURL)}, nil
}
