package main

import (
	"os"
	"strings"
)

var cliDispatchExit = os.Exit

// cliDispatch handles CLI subcommands on a fast path, bypassing cobra
// to avoid touching additional code pages and reduce RSS.
// Returns true if the command was handled.
func cliDispatch(args []string) bool {
	// Parse global flags, collect positional args
	var positional []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-h", "--help":
			return false // let cobra handle help
		case "--dev":
			devMode = true
		case "--no-color":
			noColor = true
		case "--no-intercept":
			disableIntercepts = true
		case "--json":
			jsonOutput = true
		case "-v", "--verbose":
			verbose = true
		case "--config":
			if i+1 < len(args) {
				i++
				cfgFile = args[i]
			}
		case "--profile":
			if i+1 < len(args) {
				i++
				profile = args[i]
			}
		case "--":
			positional = append(positional, args[i+1:]...)
			i = len(args)
		default:
			positional = append(positional, args[i])
		}
	}

	if len(positional) == 0 {
		return false // no subcommand → fall through to cobra (starts server)
	}

	cmd := positional[0]
	rest := positional[1:]

	switch cmd {
	case "help":
		return false // let cobra handle help
	case "status":
		runStatus(nil, rest)
	case "health":
		runHealth(nil, rest)
	case "version":
		versionCmd.Run(nil, rest)
	case "doctor":
		doctorFix = flagBool(rest, "--fix")
		runDoctor(nil, rest)
	case "config":
		return false // config uses viper, let cobra handle it
	case "models":
		return false // models subcommands need cobra arg validation
	case "skills", "skill":
		return false // skills subcommands need cobra arg validation
	case "context":
		if shouldIPCDispatchContext(rest) {
			ipcDispatchCommand(cmd, rest, true)
			break
		}
		return false // let cobra render help/usage for unsupported shapes
	case "computer_use":
		if dispatchA11yFastPath(rest) {
			break
		}
		return false // let cobra render help/usage for unsupported shapes
	case "sessions":
		if dispatchSessionsFastPath(rest) {
			break
		}
		return false // fall back to cobra for unsupported/invalid shapes
	case "cron":
		return false // cron subcommands need cobra arg validation
	case "harness":
		return false // harness subcommands need cobra arg validation
	case "logs":
		logsFollow = flagBool(rest, "-f") || flagBool(rest, "--follow")
		logsLines = flagInt(rest, "-n", 50)
		logsLevel = flagVal(rest, "--level", "")
		runLogs(nil, rest)
	case "media":
		return false // media subcommands need cobra arg validation
	case "pdf":
		if dispatchPDFNativeFastPath(rest) {
			break
		}
		return false
	case "complete-bootstrap":
		runCompleteBootstrap()
	case "gateway":
		return false // gateway run needs runServer, let cobra handle
	default:
		// Unknown command → try IPC fallback (forward to main process)
		if ipcDispatchCommand(cmd, rest, false) {
			break
		}
		return false // server not reachable → let cobra handle
	}

	cliDispatchExit(0)
	return true
}

func shouldIPCDispatchContext(args []string) bool {
	if len(args) == 0 {
		return false
	}
	switch strings.TrimSpace(args[0]) {
	case "search", "get", "annotate", "import", "validate":
		return true
	default:
		return false
	}
}

func dispatchSessionsFastPath(args []string) bool {
	if len(args) == 0 {
		return false
	}

	switch args[0] {
	case "list":
		sessionsActive = flagBool(args[1:], "--active")
		runSessionsList(nil, nil)
		return true
	case "show":
		if len(args) != 2 {
			return false
		}
		runSessionsShow(nil, []string{args[1]})
		return true
	case "delete":
		if len(args) != 2 {
			return false
		}
		runSessionsDelete(nil, []string{args[1]})
		return true
	case "clear":
		if len(args) != 1 {
			return false
		}
		runSessionsClear(nil, nil)
		return true
	default:
		return false
	}
}

func flagBool(args []string, name string) bool {
	for _, a := range args {
		if a == name {
			return true
		}
	}
	return false
}

func flagVal(args []string, name, def string) string {
	for i, a := range args {
		if a == name && i+1 < len(args) {
			return args[i+1]
		}
	}
	return def
}

func flagInt(args []string, name string, def int) int {
	s := flagVal(args, name, "")
	if s == "" {
		return def
	}
	var v int
	for _, c := range s {
		if c >= '0' && c <= '9' {
			v = v*10 + int(c-'0')
		}
	}
	if v == 0 {
		return def
	}
	return v
}
