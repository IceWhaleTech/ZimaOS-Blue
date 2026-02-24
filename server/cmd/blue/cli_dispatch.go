package main

import "os"

// cliDispatch handles CLI subcommands on a fast path, bypassing cobra
// to avoid touching additional code pages and reduce RSS.
// Returns true if the command was handled.
func cliDispatch(args []string) bool {
	// Parse global flags, collect positional args
	var positional []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--dev":
			devMode = true
		case "--no-color":
			noColor = true
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
	case "plugins":
		return false // plugins subcommands need cobra arg validation
	case "skills":
		return false // skills subcommands need cobra arg validation
	case "sessions":
		return false // sessions subcommands need cobra arg validation
	case "cron":
		return false // cron subcommands need cobra arg validation
	case "logs":
		logsFollow = flagBool(rest, "-f") || flagBool(rest, "--follow")
		logsLines = flagInt(rest, "-n", 50)
		logsLevel = flagVal(rest, "--level", "")
		runLogs(nil, rest)
	case "media":
		return false // media subcommands need cobra arg validation
	case "complete-bootstrap":
		runCompleteBootstrap()
	case "gateway":
		return false // gateway run needs runServer, let cobra handle
	default:
		// Unknown command → try IPC fallback (forward to main process)
		if ipcFallback(cmd, rest) {
			break
		}
		return false // server not reachable → let cobra handle
	}

	os.Exit(0)
	return true
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
