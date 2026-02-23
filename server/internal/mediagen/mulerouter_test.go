package mediagen

import (
	"encoding/json"
	"testing"
)

func TestMuleRouterMediaURLsUnmarshal(t *testing.T) {
	// Format 1: flat string array (nano-banana-pro)
	raw1 := `["https://example.com/img1.png","https://example.com/img2.png"]`
	var urls1 muleRouterMediaURLs
	if err := json.Unmarshal([]byte(raw1), &urls1); err != nil {
		t.Fatalf("flat []string unmarshal: %v", err)
	}
	if len(urls1) != 2 || urls1[0] != "https://example.com/img1.png" {
		t.Fatalf("unexpected urls1: %v", urls1)
	}

	// Format 2: array of objects
	raw2 := `[{"url":"https://example.com/img3.png"}]`
	var urls2 muleRouterMediaURLs
	if err := json.Unmarshal([]byte(raw2), &urls2); err != nil {
		t.Fatalf("object array unmarshal: %v", err)
	}
	if len(urls2) != 1 || urls2[0] != "https://example.com/img3.png" {
		t.Fatalf("unexpected urls2: %v", urls2)
	}
}

func TestMuleRouterTaskResponseParse(t *testing.T) {
	// Real nano-banana-pro response format
	raw := `{
		"task_info":{"id":"abc-123","status":"completed","created_at":"2026-02-23T09:10:04Z","updated_at":"2026-02-23T09:10:50Z"},
		"images":["https://mule-router-assets.example.com/result_00.png"],
		"description":"A cute cat on a windowsill"
	}`
	var resp muleRouterTaskResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if resp.TaskInfo.Status != "completed" {
		t.Fatalf("status: %s", resp.TaskInfo.Status)
	}
	if len(resp.Images) != 1 {
		t.Fatalf("images count: %d", len(resp.Images))
	}
	if resp.Images[0] != "https://mule-router-assets.example.com/result_00.png" {
		t.Fatalf("image url: %s", resp.Images[0])
	}
	if resp.Description != "A cute cat on a windowsill" {
		t.Fatalf("description: %s", resp.Description)
	}
}
