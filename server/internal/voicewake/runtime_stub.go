//go:build !darwin

package voicewake

type stubRuntime struct{}

func newRuntime() runtimeController {
	return &stubRuntime{}
}

func (r *stubRuntime) Start(cfg RuntimeConfig) error {
	return ErrRecognizerUnavailable
}

func (r *stubRuntime) Stop() error {
	return nil
}

func (r *stubRuntime) Running() bool {
	return false
}
