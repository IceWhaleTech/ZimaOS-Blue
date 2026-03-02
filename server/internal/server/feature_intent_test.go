package server

import "testing"

func TestClassifyFeatureIntent_DeepResearch(t *testing.T) {
	intent := classifyFeatureIntent("请帮我做一次深度研究并给出引用来源")
	if !intent.DeepResearch {
		t.Fatalf("expected deep research intent to be true")
	}
	if intent.AgentMode {
		t.Fatalf("expected agent mode intent to be false")
	}
}

func TestClassifyFeatureIntent_AgentMode(t *testing.T) {
	intent := classifyFeatureIntent("Please run in agent mode and execute this task end-to-end")
	if !intent.AgentMode {
		t.Fatalf("expected agent mode intent to be true")
	}
}

func TestClassifyFeatureIntent_AgentLoop(t *testing.T) {
	intent := classifyFeatureIntent("Please switch to agent loop and keep iterating improvements")
	if !intent.AgentMode {
		t.Fatalf("expected agent loop intent to map to agent mode")
	}
}

func TestClassifyFeatureIntent_Negation(t *testing.T) {
	intent := classifyFeatureIntent("不要深度搜索，直接简单回答")
	if intent.DeepResearch {
		t.Fatalf("expected deep research intent to be false for negation")
	}
}

func TestClassifyFeatureIntent_AgentLoopNegation(t *testing.T) {
	intent := classifyFeatureIntent("disable agent loop and just answer directly")
	if intent.AgentMode {
		t.Fatalf("expected agent mode intent to be false for agent loop negation")
	}
}

func TestClassifyFeatureIntent_DefinitionQuestion(t *testing.T) {
	intent := classifyFeatureIntent("什么是 agent mode")
	if intent.AgentMode || intent.DeepResearch {
		t.Fatalf("expected definition question not to trigger feature intent")
	}
}

func TestFeatureIntentDictionaryLoaded(t *testing.T) {
	if len(featureIntentDict.DeepResearchExplicit) == 0 {
		t.Fatalf("expected deep research terms to be loaded")
	}
	if len(featureIntentDict.AgentModeExplicit) == 0 {
		t.Fatalf("expected agent mode terms to be loaded")
	}
	if len(featureIntentDict.DefinitionPrefixes) == 0 {
		t.Fatalf("expected definition prefixes to be loaded")
	}
}
