package claudecode

import "testing"

func TestDetectScenario(t *testing.T) {
	tests := []struct {
		msg  string
		want ScenarioType
	}{
		// Coding
		{"帮我写一个函数解析JSON", ScenarioCoding},
		{"How do I fix this compile error in my Go code?", ScenarioCoding},
		{"refactor this function to use goroutines", ScenarioCoding},
		{"```go\nfunc main() {}\n```", ScenarioCoding},

		// SysAdmin
		{"nginx反向代理怎么配置", ScenarioSysAdmin},
		{"How to deploy with docker and kubernetes?", ScenarioSysAdmin},
		{"check cpu and memory usage on the server", ScenarioSysAdmin},

		// Analysis
		{"分析一下这组数据的统计分布", ScenarioAnalysis},
		{"calculate the average and median of this dataset", ScenarioAnalysis},

		// Explain
		{"什么是goroutine的原理", ScenarioExplain},
		{"explain the difference between TCP and UDP", ScenarioExplain},

		// Creative
		{"帮我写一篇关于AI的小说", ScenarioCreative},
		{"brainstorm some creative slogans for my product", ScenarioCreative},

		// General (below threshold)
		{"hello", ScenarioGeneral},
		{"你好", ScenarioGeneral},
		{"", ScenarioGeneral},
		{"今天天气怎么样", ScenarioGeneral},
	}

	for _, tt := range tests {
		got := DetectScenario(tt.msg)
		if got != tt.want {
			t.Errorf("DetectScenario(%q) = %d, want %d", tt.msg, got, tt.want)
		}
	}
}

func TestScenarioToMode(t *testing.T) {
	if scenarioToMode(ScenarioCoding) != EnhanceDetailed {
		t.Error("coding should be detailed")
	}
	if scenarioToMode(ScenarioSysAdmin) != EnhanceDetailed {
		t.Error("sysadmin should be detailed")
	}
	if scenarioToMode(ScenarioAnalysis) != EnhanceStandard {
		t.Error("analysis should be standard")
	}
	if scenarioToMode(ScenarioGeneral) != EnhanceMinimal {
		t.Error("general should be minimal")
	}
}
