package tools

import (
	"regexp"
	"strings"
	"sync"
)

// RiskCategory classifies the type of risk a command poses.
type RiskCategory string

const (
	RiskDestructive RiskCategory = "destructive"
	RiskPrivilege   RiskCategory = "privilege"
	RiskNetwork     RiskCategory = "network"
	RiskPersistence RiskCategory = "persistence"
	RiskSystem      RiskCategory = "system"
)

// RiskScore is the result of analyzing a command's risk profile.
type RiskScore struct {
	Total       int       `json:"total"`
	Destructive int       `json:"destructive"`
	Privilege   int       `json:"privilege"`
	Network     int       `json:"network"`
	Persistence int       `json:"persistence"`
	System      int       `json:"system"`
	Reasons     []string  `json:"reasons"`
	Level       RiskLevel `json:"level"`
}

// RiskLevel is a human-readable risk classification.
type RiskLevel string

const (
	RiskLevelLow      RiskLevel = "low"
	RiskLevelMedium   RiskLevel = "medium"
	RiskLevelHigh     RiskLevel = "high"
	RiskLevelCritical RiskLevel = "critical"
)

// riskRule maps a regex pattern to a score contribution.
type riskRule struct {
	re       *regexp.Regexp
	category RiskCategory
	score    int
	reason   string
}

var (
	riskRules     []riskRule
	riskRulesOnce sync.Once
)

// riskRules is the scored rule set. Unlike dangerousCommandPatterns (hard block),
// these produce a numeric score that the policy engine evaluates.
func ensureRiskRules() {
	riskRulesOnce.Do(func() {
		raw := []struct {
			pattern  string
			category RiskCategory
			score    int
			reason   string
		}{
			// --- Destructive (high scores) ---
			{`(?:^|\s)rm\s+.*-[a-zA-Z]*r`, RiskDestructive, 40, "recursive rm"},
			{`(?:^|\s)rm\s+.*-[a-zA-Z]*f`, RiskDestructive, 20, "force rm"},
			{`(?:^|\s)rm\s+(-[a-zA-Z]*\s+)*/(?:\s|$|\*)`, RiskDestructive, 50, "rm targeting root"},
			{`--no-preserve-root`, RiskDestructive, 50, "no-preserve-root flag"},
			{`(?:^|\s)mkfs`, RiskDestructive, 90, "format filesystem"},
			{`(?:^|\s)dd\s+.*\bof=/dev/`, RiskDestructive, 90, "dd to device"},
			{`(?:^|\s)wipefs`, RiskDestructive, 90, "wipe filesystem signatures"},
			{`(?:^|\s)fdisk`, RiskDestructive, 80, "partition table modification"},
			{`(?:^|\s)parted`, RiskDestructive, 80, "partition modification"},
			{`(?:^|\s)diskutil\s+(eraseDisk|partitionDisk|secureErase)`, RiskDestructive, 90, "diskutil destructive op"},
			{`(?:^|\s)format\s+[a-zA-Z]:`, RiskDestructive, 90, "format drive"},

			// --- System state ---
			{`(?:^|\s)shutdown`, RiskSystem, 70, "shutdown"},
			{`(?:^|\s)reboot\b`, RiskSystem, 70, "reboot"},
			{`(?:^|\s)halt\b`, RiskSystem, 70, "halt"},
			{`(?:^|\s)init\s+[06]`, RiskSystem, 70, "init runlevel change"},
			{`(?:^|\s)systemctl\s+(poweroff|reboot|halt|suspend|hibernate)`, RiskSystem, 70, "systemctl power control"},
			{`(?:^|\s)systemctl\s+(start|stop|restart|enable|disable)`, RiskSystem, 30, "systemctl service control"},
			{`(?:^|\s)launchctl\s+(unload|remove)`, RiskSystem, 40, "launchctl service removal"},

			// --- Privilege escalation ---
			{`(?:^|\s)sudo\s`, RiskPrivilege, 50, "sudo"},
			{`(?:^|\s)su\s+-?\s`, RiskPrivilege, 50, "su"},
			{`(?:^|\s)doas\s`, RiskPrivilege, 50, "doas"},
			{`(?:^|\s)useradd`, RiskPrivilege, 60, "useradd"},
			{`(?:^|\s)userdel`, RiskPrivilege, 60, "userdel"},
			{`(?:^|\s)usermod`, RiskPrivilege, 50, "usermod"},
			{`(?:^|\s)passwd`, RiskPrivilege, 50, "passwd"},
			{`(?:^|\s)visudo`, RiskPrivilege, 60, "visudo"},
			{`(?:^|\s)chmod\s+[0-7]*7[0-7]*\s`, RiskPrivilege, 30, "chmod world-writable"},
			{`(?:^|\s)chmod\s+(-[a-zA-Z]*R)`, RiskPrivilege, 30, "recursive chmod"},
			{`(?:^|\s)chown\s+(-[a-zA-Z]*R)`, RiskPrivilege, 30, "recursive chown"},
			{`(?:^|\s)setcap\s`, RiskPrivilege, 40, "setcap"},

			// --- Network ---
			{`\|\s*(ba)?sh\b`, RiskNetwork, 90, "pipe-to-shell"},
			{`\|\s*zsh\b`, RiskNetwork, 90, "pipe-to-shell"},
			{`\|\s*python[23]?\b`, RiskNetwork, 80, "pipe-to-interpreter"},
			{`\|\s*perl\b`, RiskNetwork, 80, "pipe-to-interpreter"},
			{`\|\s*ruby\b`, RiskNetwork, 80, "pipe-to-interpreter"},
			{`\|\s*node\b`, RiskNetwork, 80, "pipe-to-interpreter"},
			{`(?:^|\s)curl\s`, RiskNetwork, 10, "curl"},
			{`(?:^|\s)wget\s`, RiskNetwork, 10, "wget"},
			{`(?:^|\s)nc\s`, RiskNetwork, 30, "netcat"},
			{`(?:^|\s)ncat\s`, RiskNetwork, 30, "ncat"},
			{`(?:^|\s)ssh\s`, RiskNetwork, 20, "ssh"},
			{`(?:^|\s)scp\s`, RiskNetwork, 20, "scp"},
			{`(?:^|\s)rsync\s`, RiskNetwork, 15, "rsync"},

			// --- Persistence ---
			{`(?:^|\s)crontab\s`, RiskPersistence, 40, "crontab modification"},
			{`(?:^|\s)at\s`, RiskPersistence, 30, "at job scheduling"},
			{`/etc/cron`, RiskPersistence, 40, "cron directory access"},
			{`(?i)reg\s+(add|delete)`, RiskPersistence, 50, "registry modification"},
			{`(?:^|\s)launchctl\s+load`, RiskPersistence, 40, "launchctl load"},
			{`\.bashrc|\.bash_profile|\.zshrc|\.profile`, RiskPersistence, 30, "shell profile modification"},

			// --- Low-risk common operations (for baseline) ---
			{`(?:^|\s)npm\s+install`, RiskSystem, 5, "npm install"},
			{`(?:^|\s)pip\s+install`, RiskSystem, 5, "pip install"},
			{`(?:^|\s)apt\s+install`, RiskSystem, 15, "apt install"},
			{`(?:^|\s)brew\s+install`, RiskSystem, 5, "brew install"},
			{`(?:^|\s)go\s+install`, RiskSystem, 5, "go install"},
			{`(?:^|\s)cargo\s+install`, RiskSystem, 5, "cargo install"},
		}

		rules := make([]riskRule, 0, len(raw))
		for _, r := range raw {
			rules = append(rules, riskRule{
				re:       regexp.MustCompile("(?i)" + r.pattern),
				category: r.category,
				score:    r.score,
				reason:   r.reason,
			})
		}
		riskRules = rules
	})
}

// AnalyzeRisk scores a command string against the risk rules.
// Scores are additive but capped per category to avoid double-counting.
func AnalyzeRisk(command string) RiskScore {
	return analyzeRisk(command, nil)
}

// AnalyzeRiskWithSuppressedReasons scores a command while omitting selected
// risk reasons that have already been explicitly approved at runtime.
func AnalyzeRiskWithSuppressedReasons(command string, suppressedReasons []string) RiskScore {
	if len(suppressedReasons) == 0 {
		return analyzeRisk(command, nil)
	}
	suppressed := make(map[string]struct{}, len(suppressedReasons))
	for _, reason := range suppressedReasons {
		reason = strings.TrimSpace(reason)
		if reason == "" {
			continue
		}
		suppressed[reason] = struct{}{}
	}
	return analyzeRisk(command, suppressed)
}

func analyzeRisk(command string, suppressedReasons map[string]struct{}) RiskScore {
	ensureRiskRules()

	var score RiskScore
	catScores := map[RiskCategory]*int{
		RiskDestructive: &score.Destructive,
		RiskPrivilege:   &score.Privilege,
		RiskNetwork:     &score.Network,
		RiskPersistence: &score.Persistence,
		RiskSystem:      &score.System,
	}

	for _, rule := range riskRules {
		if rule.re.MatchString(command) {
			if suppressedReasons != nil {
				if _, ok := suppressedReasons[rule.reason]; ok {
					continue
				}
			}
			ptr := catScores[rule.category]
			*ptr += rule.score
			score.Reasons = append(score.Reasons, rule.reason)
		}
	}

	// Cap each category at 100.
	for _, ptr := range catScores {
		if *ptr > 100 {
			*ptr = 100
		}
	}

	// Total = max of all categories (not sum, to avoid inflating multi-category commands).
	score.Total = max(score.Destructive, score.Privilege, score.Network, score.Persistence, score.System)

	// Classify level.
	switch {
	case score.Total >= 90:
		score.Level = RiskLevelCritical
	case score.Total >= 60:
		score.Level = RiskLevelHigh
	case score.Total >= 30:
		score.Level = RiskLevelMedium
	default:
		score.Level = RiskLevelLow
	}

	return score
}

// NormalizeCommand performs basic normalization on a command string:
// collapse whitespace, trim, lowercase for analysis (original preserved for execution).
func NormalizeCommand(cmd string) string {
	cmd = strings.TrimSpace(cmd)
	// Collapse runs of whitespace to single space.
	parts := strings.Fields(cmd)
	return strings.Join(parts, " ")
}
