package sockipc

import (
	"context"

	"go.uber.org/zap"
)

// UIReviewBackend defines the UI review operations exposed via IPC.
type UIReviewBackend interface {
	// ReviewURL performs a full UI review of a URL. Returns JSON result.
	ReviewURL(ctx context.Context, url, lang, device string) (string, error)
	// ReviewImage performs VLM review of a base64 image. Returns JSON result.
	ReviewImage(ctx context.Context, imageBase64, lang string) (string, error)
	// CheckAccessibility runs accessibility checks only. Returns JSON result.
	CheckAccessibility(ctx context.Context, url, lang string) (string, error)
}

// RegisterUIReviewHandlers wires up UI review IPC commands.
// Auth is handled by Unix socket file permissions (0600).
func RegisterUIReviewHandlers(srv *Server, reviewer UIReviewBackend, log *zap.Logger) {
	// ui.review_url — full UI review of a URL
	srv.Handle("ui.review_url", func(ctx context.Context, req *Request) *Response {
		url := req.Params["url"]
		if url == "" {
			return ErrResponse("missing url")
		}
		lang := req.Params["lang"]
		if lang == "" {
			lang = "en-US"
		}
		device := req.Params["device"]
		result, err := reviewer.ReviewURL(ctx, url, lang, device)
		if err != nil {
			return ErrResponse("review failed: " + err.Error())
		}
		return OkResponse(map[string]string{"_card": "ui_reviewer", "result": result})
	})

	// ui.review_image — VLM review of a base64 image
	srv.Handle("ui.review_image", func(ctx context.Context, req *Request) *Response {
		image := req.Params["image"]
		if image == "" {
			return ErrResponse("missing image")
		}
		lang := req.Params["lang"]
		if lang == "" {
			lang = "en-US"
		}
		result, err := reviewer.ReviewImage(ctx, image, lang)
		if err != nil {
			return ErrResponse("review failed: " + err.Error())
		}
		return OkResponse(map[string]string{"_card": "ui_reviewer", "result": result})
	})

	// ui.check_accessibility — accessibility check only
	srv.Handle("ui.check_accessibility", func(ctx context.Context, req *Request) *Response {
		url := req.Params["url"]
		if url == "" {
			return ErrResponse("missing url")
		}
		lang := req.Params["lang"]
		if lang == "" {
			lang = "en-US"
		}
		result, err := reviewer.CheckAccessibility(ctx, url, lang)
		if err != nil {
			return ErrResponse("check failed: " + err.Error())
		}
		return OkResponse(map[string]string{"_card": "ui_reviewer", "result": result})
	})

	_ = log // reserved for future debug logging
}
