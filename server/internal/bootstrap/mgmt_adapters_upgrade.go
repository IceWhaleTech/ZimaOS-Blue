package bootstrap

import (
	"context"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/update"
)

type mgmtUpgradeAdapter struct {
	handler    *update.Handler
	otaChecker *update.OTAChecker
	version    string
}

func (a *mgmtUpgradeAdapter) GetOTAStatus(ctx context.Context) (*tools.AdminUpgradeInfo, error) {
	if a.otaChecker == nil {
		return &tools.AdminUpgradeInfo{
			CurrentVersion:  a.version,
			UpdateAvailable: false,
		}, nil
	}

	latest := a.otaChecker.GetLatest()
	available := latest != nil && latest.Version != "" && update.IsNewerVersionString(a.version, latest.Version)

	info := &tools.AdminUpgradeInfo{
		CurrentVersion:  a.version,
		UpdateAvailable: available,
	}
	if latest != nil {
		info.LatestVersion = latest.Version
		info.DownloadURLs = latest.Packages
		info.ReleaseNoteURL = latest.ReleaseNoteURL
		if latest.Delay > 0 {
			info.Delay = latest.Delay
		}
	}
	return info, nil
}

func (a *mgmtUpgradeAdapter) CheckForUpdate(ctx context.Context) (*tools.AdminUpgradeInfo, error) {
	if a.handler == nil {
		return &tools.AdminUpgradeInfo{
			CurrentVersion:  a.version,
			UpdateAvailable: false,
		}, nil
	}

	updateInfo, err := a.handler.CheckForUpdate(ctx)
	if err != nil {
		return &tools.AdminUpgradeInfo{
			CurrentVersion:  a.version,
			UpdateAvailable: false,
		}, nil
	}

	info := &tools.AdminUpgradeInfo{
		CurrentVersion:  a.version,
		UpdateAvailable: updateInfo.UpdateAvailable,
	}
	if updateInfo.UpdateAvailable {
		info.LatestVersion = updateInfo.LatestVersion
		if updateInfo.DownloadURL != "" {
			info.DownloadURLs = []string{updateInfo.DownloadURL}
		}
	}
	return info, nil
}

func (a *mgmtUpgradeAdapter) GetUpdateStatus(ctx context.Context) (*tools.AdminUpgradeInfo, error) {
	if a.handler == nil {
		return &tools.AdminUpgradeInfo{
			CurrentVersion: a.version,
			State:          "idle",
		}, nil
	}

	status := a.handler.GetStatus()
	info := &tools.AdminUpgradeInfo{
		CurrentVersion: a.version,
		State:          status.State,
		Progress:       status.Progress,
		Error:          status.Error,
		DownloadedPath: status.DownloadedPath,
	}
	return info, nil
}

func (a *mgmtUpgradeAdapter) StartDownload(ctx context.Context) (*tools.AdminUpgradeInfo, error) {
	if a.handler == nil {
		return &tools.AdminUpgradeInfo{
			CurrentVersion: a.version,
			Error:          "update handler not available",
		}, nil
	}

	status := a.handler.GetStatus()
	if status.State == "downloading" || status.State == "applying" {
		return &tools.AdminUpgradeInfo{
			CurrentVersion: a.version,
			State:          status.State,
			Progress:       status.Progress,
			Error:          "update already in progress",
		}, nil
	}

	if err := a.handler.StartDownload(ctx); err != nil {
		return &tools.AdminUpgradeInfo{
			CurrentVersion: a.version,
			State:          status.State,
			Error:          err.Error(),
		}, nil
	}

	return &tools.AdminUpgradeInfo{
		CurrentVersion: a.version,
		State:          "downloading",
		Progress:       0,
	}, nil
}

func (a *mgmtUpgradeAdapter) ApplyUpdate(ctx context.Context) (*tools.AdminUpgradeInfo, error) {
	if a.handler == nil {
		return &tools.AdminUpgradeInfo{
			CurrentVersion: a.version,
			Error:          "update handler not available",
		}, nil
	}

	status := a.handler.GetStatus()
	if status.DownloadedPath == "" {
		return &tools.AdminUpgradeInfo{
			CurrentVersion: a.version,
			State:          status.State,
			Error:          "no update downloaded. use upgrade.download first",
		}, nil
	}

	if status.State == "applying" || status.State == "restarting" {
		return &tools.AdminUpgradeInfo{
			CurrentVersion: a.version,
			State:          status.State,
			Error:          "update already in progress",
		}, nil
	}

	if err := a.handler.ApplyUpdate(); err != nil {
		return &tools.AdminUpgradeInfo{
			CurrentVersion: a.version,
			State:          "failed",
			Error:          err.Error(),
		}, nil
	}

	return &tools.AdminUpgradeInfo{
		CurrentVersion: a.version,
		State:          "applying",
		Progress:       100,
	}, nil
}
