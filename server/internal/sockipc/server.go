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

// Server listens on a Unix domain socket (or Windows named pipe) and dispatches
// incoming JSON messages to registered command handlers.
type Server struct {
	path     string
	handlers map[string]Handler
	log      *zap.Logger

	mu       sync.Mutex
	listener net.Listener
	wg       sync.WaitGroup
	closed   bool
}

// NewServer creates a server that will listen on the given socket path.
func NewServer(path string, log *zap.Logger) *Server {
	if log == nil {
		log = zap.NewNop()
	}
	return &Server{
		path:     path,
		handlers: make(map[string]Handler),
		log:      log,
	}
}

// Handle registers a handler for the given command name.
func (s *Server) Handle(cmd string, h Handler) {
	s.handlers[cmd] = h
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
	s.closed = true
	ln := s.listener
	s.mu.Unlock()

	if ln != nil {
		ln.Close()
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
		s.wg.Add(1)
		go s.handleConn(conn)
	}
}

func (s *Server) handleConn(conn net.Conn) {
	defer s.wg.Done()
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

		h, ok := s.handlers[req.Cmd]
		if !ok {
			WriteJSON(conn, ErrResponse("unknown cmd: "+req.Cmd))
			continue
		}

		resp := h(context.Background(), req)
		if resp == nil {
			resp = OkResponse(nil)
		}
		WriteJSON(conn, resp)
	}
}
