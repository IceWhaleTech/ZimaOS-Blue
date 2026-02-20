package main

import _ "embed"

// embeddedBluecli contains the bluecli server binary.
// The file is copied into this directory by the Makefile before building.
//
//go:embed bluecli
var embeddedBluecli []byte

// embeddedDist contains the web frontend assets as a tar.gz archive.
// The file is copied into this directory by the Makefile before building.
//
//go:embed dist.tar.gz
var embeddedDist []byte
