package rerank

import (
	"testing"

	"github.com/Tencent/WeKnora/internal/models/providers"
	modelruntime "github.com/Tencent/WeKnora/internal/models/runtime"
)

// TestDetectByURLSeesVendorCatalog guards the blank import of
// runtime composition.
//
// modelruntime.DetectByURL returns "generic" for every URL while the catalog is
// empty. Without the vendor packages linked in, a stored rerank row that
// carries no provider id resolves to the generic Cohere client, and LKEAP
// loses its signed SDK client.
func TestDetectByURLSeesVendorCatalog(t *testing.T) {
	cases := map[string]string{
		"https://api.lkeap.cloud.tencent.com/v1":               "lkeap",
		"https://api.jina.ai/v1":                               "jina",
		"https://some-self-hosted-gateway.example.internal/v1": providers.GenericID,
	}
	for baseURL, want := range cases {
		if got := modelruntime.DetectByURL(baseURL); got != want {
			t.Errorf("DetectByURL(%q) = %q, want %q", baseURL, got, want)
		}
	}
}
