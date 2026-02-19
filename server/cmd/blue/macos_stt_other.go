//go:build !darwin

package main

func macosRequestSTTAuthorization() {}
func macosRunMainRunLoop()          {}
func macosStopMainRunLoop()         {}
func isDarwin() bool                { return false }
