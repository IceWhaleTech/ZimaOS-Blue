package sockipc

import (
	"context"
	"io"
	"net"
	"sync"

	"go.uber.org/zap"
)

// Handler processes an incoming IPC request and returns a response.
type Handler func(ctx context.Context, req *Request) *Response

// StreamHandler processes one IPC request and can write multiple length-prefixed
// responses to the same connection before returning.
type StreamHandler func(ctx context.Context, req *Request, conn net.Conn) error

// Server listens on a Unix domain socket (or Windows named pipe) and dispatches
// incoming JSON messages to registered command handlers.
type Server struct {
	path           string
	handlers       map[string]Handler
	streamHandlers map[string]StreamHandler
	fallback       Handler // called when no handler matches
	log            *zap.Logger

	mu       sync.Mutex
	listener net.Listener
	conns    map[net.Conn]struct{}
	wg       sync.WaitGroup
	closed   bool
}

// NewServer creates a server that will listen on the given socket path.
func NewServer(path string, log *zap.Logger) *Server {
	if log == nil {
		log = zap.NewNop()
	}
	return &Server{
		path:           path,
		handlers:       make(map[string]Handler),
		streamHandlers: make(map[string]StreamHandler),
		conns:          make(map[net.Conn]struct{}),
		log:            log,
	}
}

// Handle registers a handler for the given command name.
func (s *Server) Handle(cmd string, h Handler) {
	s.handlers[cmd] = h
}

// HandleStream registers a streaming handler for the given command name.
func (s *Server) HandleStream(cmd string, h StreamHandler) {
	s.streamHandlers[cmd] = h
}

// HandleFallback sets a fallback handler for unmatched commands.
// Called when no exact handler matches the incoming cmd.
func (s *Server) HandleFallback(h Handler) {
	s.fallback = h
}

// Start begins listening. On Unix, it removes any stale socket file first.
// On Windows, it creates a named pipe (\\.\pipe\<name>).
// Spawns a goroutine to accept connections. Returns once the listener is ready.
func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	ln, err := listen(s.path)
	if err != nil {
		return err
	}
	s.listener = ln
	s.log.Info("sockipc listening", zap.String("path", s.path))

	s.wg.Add(1)
	go s.acceptLoop()
	return nil
}

// Close shuts down the listener and waits for in-flight connections.
func (s *Server) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		s.wg.Wait()
		cleanup(s.path)
		return nil
	}
	s.closed = true
	ln := s.listener
	conns := make([]net.Conn, 0, len(s.conns))
	for conn := range s.conns {
		conns = append(conns, conn)
	}
	s.mu.Unlock()

	if ln != nil {
		_ = ln.Close()
	}
	for _, conn := range conns {
		_ = conn.Close()
	}
	s.wg.Wait()
	cleanup(s.path)
	return nil
}

func (s *Server) acceptLoop() {
	defer s.wg.Done()
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			s.mu.Lock()
			closed := s.closed
			s.mu.Unlock()
			if closed {
				return
			}
			s.log.Warn("sockipc accept error", zap.Error(err))
			continue
		}
		if !s.trackConn(conn) {
			_ = conn.Close()
			return
		}
		s.wg.Add(1)
		go s.handleConn(conn)
	}
}

func (s *Server) trackConn(conn net.Conn) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return false
	}
	s.conns[conn] = struct{}{}
	return true
}

func (s *Server) untrackConn(conn net.Conn) {
	s.mu.Lock()
	delete(s.conns, conn)
	s.mu.Unlock()
}

func (s *Server) handleConn(conn net.Conn) {
	defer s.wg.Done()
	defer s.untrackConn(conn)
	defer conn.Close()

	for {
		req, err := ReadJSON[Request](conn)
		if err == io.EOF {
			return
		}
		if err != nil {
			s.log.Warn("sockipc read error", zap.Error(err))
			return
		}

		if req.Cmd == "" {
			WriteJSON(conn, ErrResponse("missing cmd"))
			continue
		}

		if stream, ok := s.streamHandlers[req.Cmd]; ok {
			ctx, cancel := context.WithCancel(context.Background())
			go func() {
				var one [1]byte
				_, _ = conn.Read(one[:])
				cancel()
			}()
			if err := stream(ctx, req, conn); err != nil {
				s.log.Warn("sockipc stream handler error", zap.String("cmd", req.Cmd), zap.Error(err))
			}
			cancel()
			return
		}

		h, ok := s.handlers[req.Cmd]
		if !ok {
			if s.fallback != nil {
				h = s.fallback
			} else {
				WriteJSON(conn, ErrResponse("unknown cmd: "+req.Cmd))
				continue
			}
		}

		resp := h(context.Background(), req)
		if resp == nil {
			resp = OkResponse(nil)
		}
		WriteJSON(conn, resp)
	}
}
