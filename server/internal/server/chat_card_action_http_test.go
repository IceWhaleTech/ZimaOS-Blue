package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/cards"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	"github.com/labstack/echo/v4"
)

type cardActionHTTPResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func makeCardActionContext(t *testing.T, convID string, body []byte) (echo.Context, *httptest.ResponseRecorder) {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+convID+"/messages/msg-1/card-action", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req = req.WithContext(context.WithValue(req.Context(), auth.UserContextKey, &auth.UserClaims{
		UserID: "user-1",
		Role:   "user",
	}))
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id", "msgid")
	c.SetParamValues(convID, "msg-1")
	return c, rec
}

func firstCardAction(t *testing.T, card map[string]interface{}) (string, string, map[string]interface{}) {
	t.Helper()
	actions, ok := card["actions"].([]map[string]interface{})
	if !ok || len(actions) == 0 {
		t.Fatalf("expected card actions, got %#v", card["actions"])
	}
	actionID, _ := actions[0]["id"].(string)
	label, _ := actions[0]["label"].(string)
	formData, _ := actions[0]["form_data"].(map[string]interface{})
	return actionID, label, formData
}

func submitCardActionHTTP(t *testing.T, h *ChatHandler, convID string, card map[string]interface{}) cardActionHTTPResponse {
	t.Helper()
	actionID, label, formData := firstCardAction(t, card)
	cardID, _ := card["id"].(string)
	title, _ := card["title"].(string)

	body, _ := json.Marshal(map[string]interface{}{
		"card_id":      cardID,
		"action_id":    actionID,
		"action_label": label,
		"card_type":    card["type"],
		"card_title":   title,
		"form_data":    formData,
	})
	c, rec := makeCardActionContext(t, convID, body)

	if err := h.HandleCardAction(c); err != nil {
		t.Fatalf("HandleCardAction: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var resp cardActionHTTPResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return resp
}

func TestHandleCardAction_WebFetchUseBrowser_HTTP(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("memory.NewStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	conv, err := store.CreateConversation(context.Background(), "web fetch action", "user-1")
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}

	h := NewChatHandler(store, nil, tools.NewRegistry())
	card := cards.ToCard("web_fetch", `{"url":"https://www.reddit.com/r/golang","title":"Sign in","content":"Log in to continue","content_type":"text/html","extract_mode":"text","extractor":"html","warning":"page appears to be a login wall; use browser or pass browser_target_id","warning_code":"login_wall"}`)
	if card == nil {
		t.Fatal("expected web_fetch card")
	}
	actionID, label, formData := firstCardAction(t, card)
	cardID, _ := card["id"].(string)
	title, _ := card["title"].(string)

	body, _ := json.Marshal(map[string]interface{}{
		"card_id":      cardID,
		"action_id":    actionID,
		"action_label": label,
		"card_type":    card["type"],
		"card_title":   title,
		"form_data":    formData,
	})
	c, rec := makeCardActionContext(t, conv.ID, body)

	if err := h.HandleCardAction(c); err != nil {
		t.Fatalf("HandleCardAction: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var resp cardActionHTTPResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	want := "Open https://www.reddit.com/r/golang with the browser tool. If the page needs login, challenge handling, or dynamic interaction, continue in the browser and summarize the relevant content."
	if !resp.Success || resp.Message != want {
		t.Fatalf("response = %#v, want message %q", resp, want)
	}
}

func TestHandleCardAction_RedditLoginWallToBrowserExtract_HTTP(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("memory.NewStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	conv, err := store.CreateConversation(context.Background(), "reddit login wall chain", "user-1")
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}

	h := NewChatHandler(store, nil, tools.NewRegistry())
	const redditURL = "https://www.reddit.com/r/test"

	webFetchCard := cards.ToCard("web_fetch", `{"url":"https://www.reddit.com/r/test","title":"Sign in","content":"Log in to continue","content_type":"text/html","extract_mode":"text","extractor":"html","warning":"page appears to be a login wall; use browser or pass browser_target_id","warning_code":"login_wall"}`)
	if webFetchCard == nil {
		t.Fatal("expected web_fetch card")
	}
	stepOneActionID, _, stepOneFormData := firstCardAction(t, webFetchCard)
	if stepOneActionID != "use_browser" {
		t.Fatalf("expected use_browser action, got %q", stepOneActionID)
	}
	if stepOneFormData["url"] != redditURL {
		t.Fatalf("expected web_fetch action url %q, got %#v", redditURL, stepOneFormData)
	}

	stepOneResp := submitCardActionHTTP(t, h, conv.ID, webFetchCard)
	wantStepOne := "Open https://www.reddit.com/r/test with the browser tool. If the page needs login, challenge handling, or dynamic interaction, continue in the browser and summarize the relevant content."
	if !stepOneResp.Success || stepOneResp.Message != wantStepOne {
		t.Fatalf("step one response = %#v, want message %q", stepOneResp, wantStepOne)
	}

	browserCard := cards.ToCard("browser", `{"url":"https://www.reddit.com/r/test","title":"r/test","target_id":"tab-42","strategy":"interactive","tree":"@1 link \"Log in\""}`)
	if browserCard == nil {
		t.Fatal("expected browser card")
	}
	stepTwoActionID, _, stepTwoFormData := firstCardAction(t, browserCard)
	if stepTwoActionID != "extract_with_web_fetch" {
		t.Fatalf("expected extract_with_web_fetch action, got %q", stepTwoActionID)
	}
	if stepTwoFormData["url"] != redditURL || stepTwoFormData["browser_target_id"] != "tab-42" {
		t.Fatalf("expected browser extract form data with url %q and browser_target_id tab-42, got %#v", redditURL, stepTwoFormData)
	}

	stepTwoResp := submitCardActionHTTP(t, h, conv.ID, browserCard)
	wantStepTwo := "Use web_fetch on https://www.reddit.com/r/test with browser_target_id=tab-42 to extract readable content using the current browser session cookies, then summarize the relevant content."
	if !stepTwoResp.Success || stepTwoResp.Message != wantStepTwo {
		t.Fatalf("step two response = %#v, want message %q", stepTwoResp, wantStepTwo)
	}
}

func TestHandleCardAction_BrowserExtractWithWebFetch_HTTP(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("memory.NewStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	conv, err := store.CreateConversation(context.Background(), "browser action", "user-1")
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}

	h := NewChatHandler(store, nil, tools.NewRegistry())
	card := cards.ToCard("browser", `{"url":"https://www.reddit.com/r/test","title":"r/test","target_id":"tab-42","strategy":"interactive","tree":"@1 link \"Log in\""}`)
	if card == nil {
		t.Fatal("expected browser card")
	}
	actionID, label, formData := firstCardAction(t, card)
	cardID, _ := card["id"].(string)
	title, _ := card["title"].(string)

	body, _ := json.Marshal(map[string]interface{}{
		"card_id":      cardID,
		"action_id":    actionID,
		"action_label": label,
		"card_type":    card["type"],
		"card_title":   title,
		"form_data":    formData,
	})
	c, rec := makeCardActionContext(t, conv.ID, body)

	if err := h.HandleCardAction(c); err != nil {
		t.Fatalf("HandleCardAction: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var resp cardActionHTTPResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	want := "Use web_fetch on https://www.reddit.com/r/test with browser_target_id=tab-42 to extract readable content using the current browser session cookies, then summarize the relevant content."
	if !resp.Success || resp.Message != want {
		t.Fatalf("response = %#v, want message %q", resp, want)
	}
}
