package providerpool

func defaultProviderMetadataMode(provider *Provider) ProviderMetadataMode {
	if provider == nil {
		return ProviderMetadataModeDynamic
	}

	switch provider.ID {
	case "openai", "anthropic", "google", "qwen", "deepseek", "moonshot", "grok", "glm", "minimax", "ollama", "bedrock":
		return ProviderMetadataModeCatalog
	default:
		return ProviderMetadataModeDynamic
	}
}

func EffectiveProviderMetadataMode(provider *Provider) ProviderMetadataMode {
	if provider == nil {
		return ProviderMetadataModeDynamic
	}
	if provider.MetadataMode != "" {
		return provider.MetadataMode
	}
	return defaultProviderMetadataMode(provider)
}

func isCatalogMetadataProvider(provider *Provider) bool {
	return EffectiveProviderMetadataMode(provider) == ProviderMetadataModeCatalog
}
