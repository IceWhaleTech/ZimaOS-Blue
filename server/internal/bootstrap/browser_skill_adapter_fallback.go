package bootstrap

import (
	"context"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/browser"
)

type fallbackBrowserAdapter struct {
	provider rodServiceProvider
}

var _ interface {
	Scrape(context.Context, *browser.ScrapeRequest) (*browser.ScrapeResponse, error)
} = (*fallbackBrowserAdapter)(nil)

func newLeaseAwareFallbackBrowserAdapter(acquire func() (*browser.RodService, func(), error)) *fallbackBrowserAdapter {
	return &fallbackBrowserAdapter{provider: rodServiceProvider{acquire: acquire}}
}

func (a *fallbackBrowserAdapter) acquire() (*browser.RodService, func(), error) {
	return a.provider.acquireLease()
}

func (a *fallbackBrowserAdapter) Start(ctx context.Context) error {
	svc, release, err := a.acquire()
	if err != nil {
		return err
	}
	defer release()
	return svc.Start(ctx)
}

func (a *fallbackBrowserAdapter) OpenTab(ctx context.Context, url string) (*browser.Tab, error) {
	svc, release, err := a.acquire()
	if err != nil {
		return nil, err
	}
	defer release()
	return svc.OpenTab(ctx, url)
}

func (a *fallbackBrowserAdapter) CloseTab(ctx context.Context, targetID string) error {
	svc, release, err := a.acquire()
	if err != nil {
		return err
	}
	defer release()
	return svc.CloseTab(ctx, targetID)
}

func (a *fallbackBrowserAdapter) Screenshot(ctx context.Context, req *browser.ScreenshotRequest) (*browser.ScreenshotResponse, error) {
	svc, release, err := a.acquire()
	if err != nil {
		return nil, err
	}
	defer release()
	return svc.Screenshot(ctx, req)
}

func (a *fallbackBrowserAdapter) Scrape(ctx context.Context, req *browser.ScrapeRequest) (*browser.ScrapeResponse, error) {
	svc, release, err := a.acquire()
	if err != nil {
		return nil, err
	}
	defer release()
	return svc.Scrape(ctx, req)
}

func (a *fallbackBrowserAdapter) ElementExists(ctx context.Context, targetID, selector string) (bool, error) {
	svc, release, err := a.acquire()
	if err != nil {
		return false, err
	}
	defer release()
	return svc.ElementExists(ctx, targetID, selector)
}

func (a *fallbackBrowserAdapter) ExtractFirstFromTab(ctx context.Context, targetID, selector, attribute string) (string, error) {
	svc, release, err := a.acquire()
	if err != nil {
		return "", err
	}
	defer release()
	return svc.ExtractFirstFromTab(ctx, targetID, selector, attribute)
}

func (a *fallbackBrowserAdapter) Act(ctx context.Context, req *browser.ActRequest) (*browser.ActResponse, error) {
	svc, release, err := a.acquire()
	if err != nil {
		return nil, err
	}
	defer release()
	return svc.Act(ctx, req)
}

func (a *fallbackBrowserAdapter) PageInfo(ctx context.Context, targetID string) (string, string, error) {
	svc, release, err := a.acquire()
	if err != nil {
		return "", "", err
	}
	defer release()
	return svc.PageInfo(ctx, targetID)
}
