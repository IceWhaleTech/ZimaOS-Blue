package config

// ResearchConfig keeps a placeholder namespace for deep-research settings.
// The research pipeline is now fixed to the built-in web flow.
type ResearchConfig struct{}

func DefaultResearchConfig() *ResearchConfig {
	return &ResearchConfig{}
}
