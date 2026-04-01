package bootstrap

import (
	"fmt"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/browser"
)

type rodServiceProvider struct {
	svc     *browser.RodService
	resolve func() *browser.RodService
	acquire func() (*browser.RodService, func(), error)
}

func (p rodServiceProvider) get() (*browser.RodService, error) {
	if p.svc != nil {
		return p.svc, nil
	}
	if p.resolve != nil {
		if svc := p.resolve(); svc != nil {
			return svc, nil
		}
	}
	return nil, fmt.Errorf("browser service not available")
}

func (p rodServiceProvider) acquireLease() (*browser.RodService, func(), error) {
	if p.acquire != nil {
		svc, release, err := p.acquire()
		if err != nil {
			return nil, nil, err
		}
		if release == nil {
			release = func() {}
		}
		if svc == nil {
			release()
			return nil, nil, fmt.Errorf("browser service not available")
		}
		return svc, release, nil
	}
	svc, err := p.get()
	if err != nil {
		return nil, nil, err
	}
	return svc, func() {}, nil
}
