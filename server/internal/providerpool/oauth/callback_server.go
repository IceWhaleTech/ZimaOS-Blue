package oauth

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"
)

// CallbackServer runs a temporary local HTTP server to capture OAuth callbacks.
type CallbackServer struct {
	port     int
	path     string
	server   *http.Server
	resultCh chan callbackResult
	once     sync.Once
}

type callbackResult struct {
	Code  string
	Error string
}

// NewCallbackServer creates a callback server on the given port and path.
func NewCallbackServer(port int, path string) *CallbackServer {
	return &CallbackServer{
		port:     port,
		path:     path,
		resultCh: make(chan callbackResult, 1),
	}
}

// RedirectURI returns the full redirect URI for this callback server.
func (s *CallbackServer) RedirectURI() string {
	return fmt.Sprintf("http://localhost:%d%s", s.port, s.path)
}

// Start starts the callback server and blocks until a callback is received or ctx is cancelled.
// Returns the authorization code or an error.
func (s *CallbackServer) Start(ctx context.Context) (string, error) {
	mux := http.NewServeMux()
	mux.HandleFunc(s.path, s.handleCallback)

	s.server = &http.Server{
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", s.port))
	if err != nil {
		return "", fmt.Errorf("listen on port %d: %w", s.port, err)
	}

	go func() {
		if err := s.server.Serve(ln); err != nil && err != http.ErrServerClosed {
			slog.Error("[oauth] callback server error", "error", err)
		}
	}()

	defer s.shutdown()

	select {
	case result := <-s.resultCh:
		if result.Error != "" {
			return "", fmt.Errorf("oauth error: %s", result.Error)
		}
		return result.Code, nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func (s *CallbackServer) handleCallback(w http.ResponseWriter, r *http.Request) {
	s.once.Do(func() {
		code := r.URL.Query().Get("code")
		errMsg := r.URL.Query().Get("error")

		if errMsg != "" {
			desc := r.URL.Query().Get("error_description")
			s.resultCh <- callbackResult{Error: errMsg + ": " + desc}
		} else if code != "" {
			s.resultCh <- callbackResult{Code: code}
		} else {
			s.resultCh <- callbackResult{Error: "no code or error in callback"}
		}
	})

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, `<!DOCTYPE html><html><body>
<h2>Authorization successful</h2>
<p>You can close this window and return to ZimaOS Blue.</p>
<script>window.close()</script>
</body></html>`)
}

func (s *CallbackServer) shutdown() {
	if s.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		s.server.Shutdown(ctx)
	}
}
