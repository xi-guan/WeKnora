package rerank

import (
	"context"
	"testing"

	modelruntime "github.com/Tencent/WeKnora/internal/models/runtime"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The SDK-backed vendor has no protocol package, so its outbound shape is not
// covered by a golden test. These pin what the SDK actually puts on the wire,
// which is what the per-client tests did before the migration.

func TestLKEAPClientRequiresCredentials(t *testing.T) {
	for _, cfg := range []*RerankerConfig{
		{AppSecret: "sk"},
		{APIKey: "id"},
	} {
		_, err := newLKEAPClient(cfg, &modelruntime.Resolved{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "secret_id and secret_key are required")
	}
}

// The secret key has always been accepted from extra_config as well, because
// the model editor stores it there for this vendor.
func TestLKEAPClientTakesTheSecretKeyFromExtraConfig(t *testing.T) {
	client, err := newLKEAPClient(
		&RerankerConfig{APIKey: "AKIDxxx", ExtraConfig: map[string]string{"secret_key": "sk"}},
		&modelruntime.Resolved{RemoteModel: "lke-reranker-base"},
	)
	require.NoError(t, err)
	require.NotNil(t, client)
}

func TestLKEAPClientFallsBackToTheDefaultModel(t *testing.T) {
	upstream := newUpstream(t)
	client, err := newLKEAPClient(
		&RerankerConfig{APIKey: "AKIDxxx", AppSecret: "sk"},
		&modelruntime.Resolved{},
	)
	require.NoError(t, err)
	_, err = client.Rerank(context.Background(), "q", []string{"d"})
	require.NoError(t, err)
	require.Len(t, upstream.requests, 1)
	assert.Equal(t, LKEAPDefaultRerankModel, upstream.requests[0].body["Model"])
}
