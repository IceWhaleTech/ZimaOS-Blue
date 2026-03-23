package sockipc

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"go.uber.org/zap"
)

// BrowserBackend defines the browser operations exposed via IPC.
type BrowserBackend interface {
	Start(ctx context.Context) error
	Navigate(ctx context.Context, url string, targetID string) (map[string]string, error)
	AccessibilityTree(ctx context.Context, targetID string, maxDepth int) (map[string]string, error)
	InteractiveElements(ctx context.Context, targetID string) (map[string]string, error)
	Screenshot(ctx context.Context, url string) (string, error)
	ScreenshotTab(ctx context.Context, targetID string) (string, error)
	Act(ctx context.Context, targetID string, ref int, actType, value string) error
	Tabs(ctx context.Context) (string, error) // JSON array
	CloseTab(ctx context.Context, targetID string) error
	ExecuteRecipe(ctx context.Context, recipe string, params map[string]string) (map[string]interface{}, error)
	ListRecipes(ctx context.Context) (string, error) // JSON array
}

// RegisterBrowserHandlers wires up browser IPC commands.
// Auth is handled by Unix socket file permissions (0600).
func RegisterBrowserHandlers(srv *Server, browser BrowserBackend, log *zap.Logger) {
	// browser.navigate — open URL, return page info
	srv.Handle("browser.navigate", func(ctx context.Context, req *Request) *Response {
		url := req.Params["url"]
		if url == "" {
			return ErrResponse("missing url")
		}
		_ = browser.Start(ctx)
		result, err := browser.Navigate(ctx, url, req.Params["target_id"])
		if err != nil {
			return ErrResponse("navigate failed: " + err.Error())
		}
		return OkResponse(result)
	})

	// browser.snapshot — get accessibility tree
	srv.Handle("browser.snapshot", func(ctx context.Context, req *Request) *Response {
		maxDepth := 10
		if v := req.Params["max_depth"]; v != "" {
			if d, err := strconv.Atoi(v); err == nil {
				maxDepth = d
			}
		}
		result, err := browser.AccessibilityTree(ctx, req.Params["target_id"], maxDepth)
		if err != nil {
			return ErrResponse("snapshot failed: " + err.Error())
		}
		return OkResponse(result)
	})

	// browser.snapshot_interactive — get interactive elements
	srv.Handle("browser.snapshot_interactive", func(ctx context.Context, req *Request) *Response {
		result, err := browser.InteractiveElements(ctx, req.Params["target_id"])
		if err != nil {
			return ErrResponse("snapshot_interactive failed: " + err.Error())
		}
		return OkResponse(result)
	})

	// browser.act — interact with element by @ref
	srv.Handle("browser.act", func(ctx context.Context, req *Request) *Response {
		refStr := req.Params["ref"]
		if refStr == "" {
			return ErrResponse("missing ref")
		}
		ref, err := strconv.Atoi(refStr)
		if err != nil {
			return ErrResponse("invalid ref: " + refStr)
		}
		actType := req.Params["act_type"]
		if actType == "" {
			return ErrResponse("missing act_type")
		}
		if err := browser.Act(ctx, req.Params["target_id"], ref, actType, req.Params["value"]); err != nil {
			return ErrResponse("act failed: " + err.Error())
		}
		return OkResponse(map[string]string{"message": fmt.Sprintf("Performed %s on @%d", actType, ref)})
	})

	// browser.screenshot — capture page as base64 PNG
	srv.Handle("browser.screenshot", func(ctx context.Context, req *Request) *Response {
		var data string
		var err error
		if targetID := req.Params["target_id"]; targetID != "" {
			data, err = browser.ScreenshotTab(ctx, targetID)
		} else {
			url := req.Params["url"]
			if url != "" {
				_ = browser.Start(ctx)
				data, err = browser.Screenshot(ctx, url)
			} else {
				data, err = browser.ScreenshotTab(ctx, "")
			}
		}
		if err != nil {
			return ErrResponse("screenshot failed: " + err.Error())
		}
		return OkResponse(map[string]string{"screenshot": data})
	})

	// browser.tabs — list open tabs
	srv.Handle("browser.tabs", func(ctx context.Context, req *Request) *Response {
		tabsJSON, err := browser.Tabs(ctx)
		if err != nil {
			return ErrResponse("tabs failed: " + err.Error())
		}
		return OkResponse(map[string]string{"tabs": tabsJSON})
	})

	// browser.close — close a tab
	srv.Handle("browser.close", func(ctx context.Context, req *Request) *Response {
		targetID := req.Params["target_id"]
		if targetID == "" {
			return ErrResponse("missing target_id")
		}
		if err := browser.CloseTab(ctx, targetID); err != nil {
			return ErrResponse("close failed: " + err.Error())
		}
		return OkResponse(map[string]string{"message": "Tab closed"})
	})

	// browser.recipe — execute a recipe
	srv.Handle("browser.recipe", func(ctx context.Context, req *Request) *Response {
		recipe := req.Params["recipe"]
		if recipe == "" {
			return ErrResponse("missing recipe")
		}
		// Collect all params except "recipe" itself
		params := make(map[string]string)
		for k, v := range req.Params {
			if k != "recipe" {
				params[k] = v
			}
		}
		_ = browser.Start(ctx)
		result, err := browser.ExecuteRecipe(ctx, recipe, params)
		if err != nil {
			return ErrResponse("recipe failed: " + err.Error())
		}
		resultJSON, _ := json.Marshal(result)
		return OkResponse(map[string]string{"result": string(resultJSON)})
	})

	// browser.recipes — list available recipes
	srv.Handle("browser.recipes", func(ctx context.Context, req *Request) *Response {
		recipesJSON, err := browser.ListRecipes(ctx)
		if err != nil {
			return ErrResponse("recipes failed: " + err.Error())
		}
		return OkResponse(map[string]string{"recipes": recipesJSON})
	})

	_ = log // reserved for future debug logging
}
