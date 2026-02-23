package oauth

import (
	"encoding/json"
	"fmt"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// WrapCloudCodeRequest wraps an OpenAI-format request body in a Cloud Code envelope.
//
// Input (OpenAI format):
//
//	{"model":"claude-opus-4-6","messages":[...],"stream":true}
//
// Output (Cloud Code format):
//
//	{"project":"<projectID>","model":"claude-opus-4-6","request":{...},"userAgent":"blue","requestType":"agent"}
func WrapCloudCodeRequest(body []byte, projectID, userAgent string) ([]byte, error) {
	model := gjson.GetBytes(body, "model").String()
	if model == "" {
		return nil, fmt.Errorf("no model in request body")
	}

	// Build the envelope
	envelope := map[string]interface{}{
		"project":     projectID,
		"model":       model,
		"request":     json.RawMessage(body),
		"userAgent":   userAgent,
		"requestType": "agent",
	}

	return json.Marshal(envelope)
}

// UnwrapCloudCodeSSEData extracts the inner response data from a Cloud Code SSE data line.
// Cloud Code SSE format: data: {"candidates":[{"content":{"parts":[{"text":"..."}]}}]}
// We need to convert this to OpenAI SSE format.
func UnwrapCloudCodeSSEData(data []byte) ([]byte, error) {
	// Check if this is a Cloud Code response (has "candidates" field)
	if !gjson.GetBytes(data, "candidates").Exists() {
		// Not a Cloud Code response, return as-is
		return data, nil
	}

	// Extract text from candidates[0].content.parts[0].text
	text := gjson.GetBytes(data, "candidates.0.content.parts.0.text").String()
	finishReason := gjson.GetBytes(data, "candidates.0.finishReason").String()

	// Convert to OpenAI streaming format
	chunk := map[string]interface{}{
		"object": "chat.completion.chunk",
		"choices": []map[string]interface{}{
			{
				"index": 0,
				"delta": map[string]interface{}{},
			},
		},
	}

	choices := chunk["choices"].([]map[string]interface{})
	if text != "" {
		choices[0]["delta"].(map[string]interface{})["content"] = text
	}
	if finishReason != "" {
		choices[0]["finish_reason"] = mapFinishReason(finishReason)
	}

	return json.Marshal(chunk)
}

// ConvertToCloudCodeModel maps common model names to Cloud Code model identifiers.
func ConvertToCloudCodeModel(model string) string {
	// Cloud Code uses the same model names, no conversion needed for most cases
	return model
}

// SetCloudCodeHeaders sets the required headers for Cloud Code API requests.
func SetCloudCodeHeaders(headers map[string]string, providerType, version string) {
	switch providerType {
	case "antigravity":
		headers["User-Agent"] = "antigravity/" + version
		headers["X-Goog-Api-Client"] = "google-cloud-sdk vscode_cloudshelleditor/0.1"
	case "gemini-cli":
		headers["User-Agent"] = "google-api-nodejs-client/9.15.1"
		headers["X-Goog-Api-Client"] = "gl-node/22.17.0"
	}
}

// CloudCodeStreamEndpoint returns the streaming endpoint for Cloud Code.
func CloudCodeStreamEndpoint(baseURL string) string {
	return baseURL + "/v1internal:streamGenerateContent?alt=sse"
}

// RewriteModelInBody replaces the model field in the request body.
func RewriteModelInBody(body []byte, newModel string) ([]byte, error) {
	return sjson.SetBytes(body, "model", newModel)
}

func mapFinishReason(reason string) string {
	switch reason {
	case "STOP":
		return "stop"
	case "MAX_TOKENS":
		return "length"
	case "SAFETY":
		return "content_filter"
	case "RECITATION":
		return "content_filter"
	default:
		return reason
	}
}
