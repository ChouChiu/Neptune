package maibot

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	platformID    = "neptune"
	sendTimeout   = 120 * time.Second
	reconnectBase = 2 * time.Second
	reconnectMax  = 30 * time.Second
)

// pendingRequest holds a channel waiting for a response from MaiBot.
type pendingRequest struct {
	ch      chan string
	groupID string
}

// Client is a WebSocket client for the maim_message API Server.
type Client struct {
	wsURL   string
	apiKey  string
	conn    *websocket.Conn
	mu      sync.Mutex
	pending map[string]*pendingRequest // keyed by groupID
	done    chan struct{}
	ready   bool
}

// NewClient creates a new MaiBot WebSocket client.
func NewClient(wsURL, apiKey string) *Client {
	return &Client{
		wsURL:   wsURL,
		apiKey:  apiKey,
		pending: make(map[string]*pendingRequest),
		done:    make(chan struct{}),
	}
}

// Connect establishes a WebSocket connection to MaiBot.
func (c *Client) Connect() error {
	u, err := url.Parse(c.wsURL)
	if err != nil {
		return fmt.Errorf("invalid WebSocket URL: %w", err)
	}

	q := u.Query()
	q.Set("api_key", c.apiKey)
	q.Set("platform", platformID)
	u.RawQuery = q.Encode()

	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to connect to MaiBot: %w", err)
	}

	c.mu.Lock()
	c.conn = conn
	c.ready = true
	c.mu.Unlock()

	slog.Info("Connected to MaiBot WebSocket", "url", c.wsURL)

	go c.readPump()
	go c.reconnectLoop()

	return nil
}

// SendMessage sends a user message to MaiBot and waits for the reply.
func (c *Client) SendMessage(groupID, groupName, userID, nickname, text string) (string, error) {
	c.mu.Lock()
	if !c.ready {
		c.mu.Unlock()
		return "", fmt.Errorf("MaiBot client not connected")
	}

	if _, exists := c.pending[groupID]; exists {
		c.mu.Unlock()
		return "", fmt.Errorf("request already pending for group %s", groupID)
	}

	req := &pendingRequest{
		ch:      make(chan string, 1),
		groupID: groupID,
	}
	c.pending[groupID] = req
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		delete(c.pending, groupID)
		c.mu.Unlock()
	}()

	msgID := fmt.Sprintf("neptune_%d_%s", time.Now().UnixNano(), groupID)
	envelope := &Envelope{
		Ver:   ProtocolVersion,
		MsgID: msgID,
		Type:  EnvelopeTypeStd,
		Meta: &Meta{
			SenderUser: c.apiKey,
			Platform:   platformID,
			Timestamp:  nowTimestamp(),
		},
		Payload: &Payload{
			MessageInfo: &MessageInfo{
				Platform:  platformID,
				MessageID: msgID,
				Time:      nowTimestamp(),
				SenderInfo: &SenderInfo{
					UserInfo: &UserInfo{
						Platform:     platformID,
						UserID:       userID,
						UserNickname: nickname,
					},
					GroupInfo: &GroupInfo{
						Platform:  platformID,
						GroupID:   groupID,
						GroupName: groupName,
					},
				},
			},
			MessageSegment: &MessageSegment{
				Type: "text",
				Data: mustMarshalString(text),
			},
			MessageDim: &MessageDim{
				APIKey:   c.apiKey,
				Platform: platformID,
			},
		},
	}

	data, err := json.Marshal(envelope)
	if err != nil {
		return "", fmt.Errorf("failed to marshal message: %w", err)
	}

	c.mu.Lock()
	if c.conn == nil {
		c.mu.Unlock()
		return "", fmt.Errorf("WebSocket connection is nil")
	}
	err = c.conn.WriteMessage(websocket.TextMessage, data)
	c.mu.Unlock()
	if err != nil {
		return "", fmt.Errorf("failed to send message: %w", err)
	}

	slog.Info("Sent message to MaiBot", "groupID", groupID, "userID", userID)

	select {
	case reply := <-req.ch:
		return reply, nil
	case <-time.After(sendTimeout):
		return "", fmt.Errorf("MaiBot response timeout after %s", sendTimeout)
	case <-c.done:
		return "", fmt.Errorf("MaiBot client closed")
	}
}

// Close gracefully shuts down the client.
func (c *Client) Close() {
	select {
	case <-c.done:
	default:
		close(c.done)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		_ = c.conn.Close()
		c.conn = nil
	}
	c.ready = false

	for _, req := range c.pending {
		select {
		case req.ch <- "":
		default:
		}
	}
}

func (c *Client) readPump() {
	for {
		select {
		case <-c.done:
			return
		default:
		}

		c.mu.Lock()
		conn := c.conn
		c.mu.Unlock()

		if conn == nil {
			time.Sleep(500 * time.Millisecond)
			continue
		}

		_, message, err := conn.ReadMessage()
		if err != nil {
			slog.Error("MaiBot WebSocket read error", "error", err)
			c.mu.Lock()
			c.ready = false
			c.mu.Unlock()
			time.Sleep(time.Second)
			continue
		}

		c.handleMessage(message)
	}
}

func (c *Client) handleMessage(data []byte) {
	var envelope Envelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		slog.Error("Failed to unmarshal MaiBot message", "error", err)
		return
	}

	switch envelope.Type {
	case EnvelopeTypeAck:
		c.sendACK(envelope.MsgID)
		return

	case EnvelopeTypeStd:
		c.sendACK(envelope.MsgID)

		if envelope.Payload == nil || envelope.Payload.MessageSegment == nil {
			return
		}

		reply := envelope.Payload.MessageSegment.TextContent()
		if reply == "" {
			return
		}

		// Try to extract group_id from multiple possible locations
		groupID := ""

		// 1. From sender_info.group_info
		if envelope.Payload.MessageInfo != nil && envelope.Payload.MessageInfo.SenderInfo != nil &&
			envelope.Payload.MessageInfo.SenderInfo.GroupInfo != nil {
			groupID = envelope.Payload.MessageInfo.SenderInfo.GroupInfo.GroupID
		}

		// 2. From message_info.platform (sometimes group_id is encoded here)
		if groupID == "" && envelope.Payload.MessageInfo != nil {
			// Check if platform contains group info
			slog.Debug("MaiBot response debug",
				"msgID", envelope.MsgID,
				"platform", envelope.Payload.MessageInfo.Platform,
				"messageID", envelope.Payload.MessageInfo.MessageID)
		}

		// 3. From message_dim (target receiver info)
		if groupID == "" && envelope.Payload.MessageDim != nil {
			slog.Debug("MaiBot response message_dim",
				"apiKey", envelope.Payload.MessageDim.APIKey,
				"platform", envelope.Payload.MessageDim.Platform)
		}

		// 4. Fallback: try to find group_id from pending requests
		if groupID == "" {
			c.mu.Lock()
			for gid, req := range c.pending {
				if req != nil {
					groupID = gid
					slog.Debug("Using fallback group_id from pending", "groupID", gid)
					break
				}
			}
			c.mu.Unlock()
		}

		if groupID == "" {
			slog.Warn("Received MaiBot reply without group_id", "msgID", envelope.MsgID)
			return
		}

		c.mu.Lock()
		req, exists := c.pending[groupID]
		c.mu.Unlock()

		if exists {
			select {
			case req.ch <- reply:
				slog.Info("Received MaiBot reply", "groupID", groupID, "length", len(reply))
			default:
				slog.Warn("Pending channel full, discarding reply", "groupID", groupID)
			}
		} else {
			slog.Warn("No pending request for group, discarding reply", "groupID", groupID)
		}
	}
}

func (c *Client) sendACK(msgID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return
	}

	ack := &Envelope{
		Ver:   ProtocolVersion,
		MsgID: fmt.Sprintf("ack_%s", msgID),
		Type:  EnvelopeTypeAck,
		Meta: &Meta{
			SenderUser: c.apiKey,
			Platform:   platformID,
			Timestamp:  nowTimestamp(),
		},
	}

	data, err := json.Marshal(ack)
	if err != nil {
		return
	}

	_ = c.conn.WriteMessage(websocket.TextMessage, data)
}

func (c *Client) reconnectLoop() {
	for {
		select {
		case <-c.done:
			return
		default:
		}

		c.mu.Lock()
		ready := c.ready
		c.mu.Unlock()

		if ready {
			time.Sleep(time.Second)
			continue
		}

		backoff := reconnectBase
		for {
			select {
			case <-c.done:
				return
			case <-time.After(backoff):
			}

			slog.Info("Attempting MaiBot WebSocket reconnect", "backoff", backoff)

			u, err := url.Parse(c.wsURL)
			if err != nil {
				break
			}
			q := u.Query()
			q.Set("api_key", c.apiKey)
			q.Set("platform", platformID)
			u.RawQuery = q.Encode()

			conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
			if err != nil {
				slog.Error("MaiBot reconnect failed", "error", err)
				backoff *= 2
				if backoff > reconnectMax {
					backoff = reconnectMax
				}
				continue
			}

			c.mu.Lock()
			c.conn = conn
			c.ready = true
			c.mu.Unlock()

			slog.Info("Reconnected to MaiBot WebSocket")
			break
		}
	}
}

func mustMarshalString(s string) json.RawMessage {
	b, _ := json.Marshal(s)
	return b
}
