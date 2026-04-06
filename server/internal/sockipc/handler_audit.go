package sockipc

import (
	"context"
	"encoding/json"
	"net"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sessionaudit"
	"go.uber.org/zap"
)

const (
	defaultAuditRecentLimit = 50
	maxAuditRecentLimit     = 500
)

// AuditBackend defines audit recall operations exposed via IPC.
type AuditBackend interface {
	Recent(ctx context.Context, conversationID string, limit int) ([]sessionaudit.Entry, error)
}

// AuditWatchBackend extends audit recall with live subscriptions.
type AuditWatchBackend interface {
	Subscribe(conversationID string) (<-chan sessionaudit.Entry, func())
}

// RegisterAuditHandlers wires up audit IPC commands.
func RegisterAuditHandlers(srv *Server, backend AuditBackend, log *zap.Logger) {
	if srv == nil || backend == nil {
		return
	}
	if log == nil {
		log = zap.NewNop()
	}

	srv.Handle("audit.recent", func(ctx context.Context, req *Request) *Response {
		ctx = withIPCUserID(ctx, req)

		entries, err := backend.Recent(
			ctx,
			strings.TrimSpace(req.Params["conversation_id"]),
			parseAuditRecentLimit(req.Params["limit"]),
		)
		if err != nil {
			log.Warn("sockipc audit.recent failed", zap.Error(err))
			return ErrResponse("audit recent failed: " + err.Error())
		}

		if query := strings.TrimSpace(req.Params["query"]); query != "" {
			entries = filterAuditEntriesByQuery(entries, query)
		}

		payload, err := json.Marshal(entries)
		if err != nil {
			log.Warn("sockipc audit.recent marshal failed", zap.Error(err))
			return ErrResponse("audit recent marshal failed")
		}

		return OkResponse(map[string]string{
			"entries": string(payload),
			"count":   strconv.Itoa(len(entries)),
		})
	})

	if watcher, ok := backend.(AuditWatchBackend); ok {
		srv.HandleStream("audit.watch", func(ctx context.Context, req *Request, conn net.Conn) error {
			ctx = withIPCUserID(ctx, req)

			conversationID := strings.TrimSpace(req.Params["conversation_id"])
			query := strings.TrimSpace(req.Params["query"])
			eventCh, cleanup := watcher.Subscribe(conversationID)
			defer cleanup()

			entries, err := backend.Recent(ctx, conversationID, parseAuditRecentLimit(req.Params["limit"]))
			if err != nil {
				log.Warn("sockipc audit.watch snapshot failed", zap.Error(err))
				return WriteJSON(conn, ErrResponse("audit watch failed: "+err.Error()))
			}
			if query != "" {
				entries = filterAuditEntriesByQuery(entries, query)
			}
			sortAuditEntriesAscending(entries)

			snapshot, err := json.Marshal(entries)
			if err != nil {
				log.Warn("sockipc audit.watch snapshot marshal failed", zap.Error(err))
				return WriteJSON(conn, ErrResponse("audit watch snapshot marshal failed"))
			}
			if err := WriteJSON(conn, OkResponse(map[string]string{
				"mode":    "snapshot",
				"entries": string(snapshot),
				"count":   strconv.Itoa(len(entries)),
			})); err != nil {
				return err
			}

			seen := make(map[string]struct{}, len(entries))
			for _, entry := range entries {
				seen[auditEntryStreamKey(entry)] = struct{}{}
			}

			for {
				select {
				case <-ctx.Done():
					return nil
				case entry, ok := <-eventCh:
					if !ok {
						return nil
					}
					if conversationID != "" && strings.TrimSpace(entry.ConversationID) != conversationID {
						continue
					}
					if !auditEntryMatchesQuery(entry, query) {
						continue
					}
					key := auditEntryStreamKey(entry)
					if _, exists := seen[key]; exists {
						continue
					}
					seen[key] = struct{}{}
					payload, err := json.Marshal(entry)
					if err != nil {
						log.Warn("sockipc audit.watch event marshal failed", zap.Error(err))
						continue
					}
					if err := WriteJSON(conn, OkResponse(map[string]string{
						"mode":  "event",
						"entry": string(payload),
					})); err != nil {
						return err
					}
				}
			}
		})
	}
}

func parseAuditRecentLimit(raw string) int {
	limit := defaultAuditRecentLimit
	if parsed, err := strconv.Atoi(strings.TrimSpace(raw)); err == nil && parsed > 0 {
		limit = parsed
	}
	if limit > maxAuditRecentLimit {
		limit = maxAuditRecentLimit
	}
	return limit
}

func filterAuditEntriesByQuery(entries []sessionaudit.Entry, query string) []sessionaudit.Entry {
	filtered := make([]sessionaudit.Entry, 0, len(entries))
	for _, entry := range entries {
		if auditEntryMatchesQuery(entry, query) {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

func auditEntryMatchesQuery(entry sessionaudit.Entry, query string) bool {
	terms := strings.Fields(strings.ToLower(strings.TrimSpace(query)))
	if len(terms) == 0 {
		return true
	}
	haystack := strings.ToLower(strings.Join([]string{
		entry.ConversationID,
		entry.SessionID,
		entry.UserID,
		entry.Source,
		entry.EventType,
		entry.Role,
		entry.ToolName,
		entry.Payload,
	}, " "))
	for _, term := range terms {
		if !strings.Contains(haystack, term) {
			return false
		}
	}
	return true
}

func sortAuditEntriesAscending(entries []sessionaudit.Entry) {
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].CreatedAt.Equal(entries[j].CreatedAt) {
			return strings.Compare(entries[i].ID, entries[j].ID) < 0
		}
		return entries[i].CreatedAt.Before(entries[j].CreatedAt)
	})
}

func auditEntryStreamKey(entry sessionaudit.Entry) string {
	if id := strings.TrimSpace(entry.ID); id != "" {
		return id
	}
	return strings.Join([]string{
		strings.TrimSpace(entry.ConversationID),
		strings.TrimSpace(entry.EventType),
		strings.TrimSpace(entry.Role),
		strings.TrimSpace(entry.ToolName),
		entry.CreatedAt.UTC().Format(time.RFC3339Nano),
		strings.TrimSpace(entry.Payload),
	}, "|")
}
