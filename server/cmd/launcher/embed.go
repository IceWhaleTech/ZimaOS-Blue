package main

import "embed"

// embeddedBluecli contains the bluecli server binary.
// The file is copied into this directory by the Makefile before building.
var embeddedBluecli []byte

// embeddedDist contains the web frontend assets as a tar or tar.gz archive.
// The file is copied into this directory by the Makefile before building.
var embeddedDist []byte

// Embed the launcher directory so `go test ./...` can compile even when
// generated assets are not present. At build time (via Makefile), copied
// assets are picked up and loaded below.
//
//go:embed *
var embeddedLauncherFiles embed.FS

func init() {
	if b, err := embeddedLauncherFiles.ReadFile("bluecli.gz"); err == nil {
		embeddedBluecli = b
	} else if b, err := embeddedLauncherFiles.ReadFile("bluecli"); err == nil {
		embeddedBluecli = b
	}
	if b, err := embeddedLauncherFiles.ReadFile("dist.tar"); err == nil {
		embeddedDist = b
	} else if b, err := embeddedLauncherFiles.ReadFile("dist.tar.gz"); err == nil {
		embeddedDist = b
	}
}
