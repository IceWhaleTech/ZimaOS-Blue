package sandbox

import (
	"runtime"
	"strconv"
)

func testEchoCommand(message string) (string, []string) {
	if runtime.GOOS == "windows" {
		return "cmd", []string{"/c", "echo", message}
	}
	return "echo", []string{message}
}

func testStdinCommand() (string, []string) {
	if runtime.GOOS == "windows" {
		return "cmd", []string{"/c", "more"}
	}
	return "cat", nil
}

func testSleepCommand(seconds int) (string, []string) {
	if runtime.GOOS == "windows" {
		return "ping", []string{"127.0.0.1", "-n", strconv.Itoa(seconds + 1)}
	}
	return "sleep", []string{strconv.Itoa(seconds)}
}

func testFailCommand(exitCode int) (string, []string) {
	if runtime.GOOS == "windows" {
		return "cmd", []string{"/c", "exit", strconv.Itoa(exitCode)}
	}
	return "sh", []string{"-c", "exit " + strconv.Itoa(exitCode)}
}

func testEnvCommand(varName string) (string, []string) {
	if runtime.GOOS == "windows" {
		return "cmd", []string{"/c", "echo", "%" + varName + "%"}
	}
	return "sh", []string{"-c", "printf %s \"$" + varName + "\""}
}
