package providers

import "github.com/Tencent/WeKnora/internal/models/internal/configcopy"

// Builtins returns independent provider definitions without registering global state.
func Builtins() []*Definition {
	return configcopy.Clone([]*Definition{
		newAnthropicProvider(),
		newAzureOpenaiProvider(),
		newDeepseekProvider(),
		newGeminiProvider(),
		newGenericProvider(),
		newGpustackProvider(),
		newJinaProvider(),
		newLitellmProvider(),
		newLkeapProvider(),
		newNovitaProvider(),
		newNvidiaProvider(),
		newOpenaiProvider(),
		newOpenrouterProvider(),
		newRequestyProvider(),
		newWeKnoraCloudProvider(),
	})
}
