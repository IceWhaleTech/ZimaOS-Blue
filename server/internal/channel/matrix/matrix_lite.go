package matrix

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// matrixClient is a lightweight Matrix Client-Server API client replacing maunium.net/go/mautrix.
type matrixClient struct {
	homeserver  string
	userID      string
	accessToken string
	deviceID    string
	nextBatch   string
	http        *http.Client
}

func newMatrixClient(homeserver, userID, accessToken string) *matrixClient {
	return &matrixClient{
		homeserver:  homeserver,
		userID:      userID,
		accessToken: accessToken,
		http:        &http.Client{Timeout: 60 * time.Second},
	}
}

func (c *matrixClient) do(ctx context.Context, method, path string, body interface{}) (json.RawMessage, error) {
	var reqBody io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		reqBody = bytes.NewReader(b)
	}
	u := c.homeserver + "/_matrix/client/v3" + path
	req, err := http.NewRequestWithContext(ctx, method, u, reqBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("matrix: HTTP %d: %s", resp.StatusCode, data)
	}
	return data, nil
}

type whoamiResp struct {
	UserID   string `json:"user_id"`
	DeviceID string `json:"device_id"`
}

func (c *matrixClient) whoami(ctx context.Context) (*whoamiResp, error) {
	data, err := c.do(ctx, "GET", "/account/whoami", nil)
	if err != nil {
		return nil, err
	}
	var r whoamiResp
	json.Unmarshal(data, &r)
	return &r, nil
}

// syncResp is a minimal Matrix /sync response.
type syncResp struct {
	NextBatch string `json:"next_batch"`
	Rooms     struct {
		Join map[string]struct {
			Timeline struct {
				Events []matrixEvent `json:"events"`
			} `json:"timeline"`
		} `json:"join"`
	} `json:"rooms"`
}

type matrixEvent struct {
	Type     string          `json:"type"`
	EventID  string          `json:"event_id"`
	Sender   string          `json:"sender"`
	RoomID   string          `json:"room_id"`
	Time     int64           `json:"origin_server_ts"`
	Content  json.RawMessage `json:"content"`
	Unsigned json.RawMessage `json:"unsigned,omitempty"`
}

type messageContent struct {
	MsgType       string       `json:"msgtype"`
	Body          string       `json:"body"`
	Format        string       `json:"format,omitempty"`
	FormattedBody string       `json:"formatted_body,omitempty"`
	URL           string       `json:"url,omitempty"`
	Info          *contentInfo `json:"info,omitempty"`
	RelatesTo     *relatesTo   `json:"m.relates_to,omitempty"`
	NewContent    *messageContent `json:"m.new_content,omitempty"`
}

type contentInfo struct {
	Size     int    `json:"size,omitempty"`
	MimeType string `json:"mimetype,omitempty"`
}

type relatesTo struct {
	InReplyTo *inReplyTo `json:"m.in_reply_to,omitempty"`
	RelType   string     `json:"rel_type,omitempty"`
	EventID   string     `json:"event_id,omitempty"`
}

type inReplyTo struct {
	EventID string `json:"event_id"`
}

func (c *matrixClient) sync(ctx context.Context, handler func(roomID string, evt matrixEvent)) error {
	params := url.Values{"timeout": {"30000"}}
	if c.nextBatch != "" {
		params.Set("since", c.nextBatch)
	}
	filter := `{"room":{"timeline":{"limit":50},"state":{"lazy_load_members":true}}}`
	params.Set("filter", filter)

	u := c.homeserver + "/_matrix/client/v3/sync?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("matrix sync: HTTP %d: %s", resp.StatusCode, b)
	}
	var sr syncResp
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return err
	}
	c.nextBatch = sr.NextBatch
	for roomID, room := range sr.Rooms.Join {
		for _, evt := range room.Timeline.Events {
			evt.RoomID = roomID
			handler(roomID, evt)
		}
	}
	return nil
}

type sendResp struct {
	EventID string `json:"event_id"`
}

func (c *matrixClient) sendMessage(ctx context.Context, roomID string, content interface{}) (string, error) {
	txnID := fmt.Sprintf("m%d", time.Now().UnixNano())
	path := fmt.Sprintf("/rooms/%s/send/m.room.message/%s", url.PathEscape(roomID), txnID)
	data, err := c.do(ctx, "PUT", path, content)
	if err != nil {
		return "", err
	}
	var r sendResp
	json.Unmarshal(data, &r)
	return r.EventID, nil
}
