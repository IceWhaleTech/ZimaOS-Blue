package bootstrap

type runtimeCapabilityAdapter struct {
	contract runtimeCapabilityContract
}

func newRuntimeCapabilityAdapter(contract runtimeCapabilityContract) runtimeCapabilityAdapter {
	return runtimeCapabilityAdapter{contract: contract}
}
