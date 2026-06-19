package maibot

import "time"

const (
	EnvelopeTypeStd = "sys_std"
	EnvelopeTypeAck = "sys_ack"
	ProtocolVersion = 1
)

// Envelope is the outer wrapper for all maim_message protocol messages.
type Envelope struct {
	Ver     int         `json:"ver"`
	MsgID   string      `json:"msg_id"`
	Type    string      `json:"type"`
	Meta    *Meta       `json:"meta"`
	Payload *Payload    `json:"payload,omitempty"`
}

// Meta contains sender metadata.
type Meta struct {
	SenderUser string  `json:"sender_user"`
	Platform   string  `json:"platform"`
	Timestamp  float64 `json:"timestamp"`
}

// Payload contains the business message data.
type Payload struct {
	MessageInfo    *MessageInfo    `json:"message_info"`
	MessageSegment *MessageSegment `json:"message_segment"`
	MessageDim     *MessageDim     `json:"message_dim"`
}

// MessageInfo contains message metadata.
type MessageInfo struct {
	Platform   string      `json:"platform"`
	MessageID  string      `json:"message_id"`
	Time       float64     `json:"time"`
	SenderInfo *SenderInfo `json:"sender_info,omitempty"`
}

// SenderInfo contains sender details.
type SenderInfo struct {
	UserInfo  *UserInfo  `json:"user_info,omitempty"`
	GroupInfo *GroupInfo `json:"group_info,omitempty"`
}

// UserInfo contains user details.
type UserInfo struct {
	Platform     string `json:"platform"`
	UserID       string `json:"user_id"`
	UserNickname string `json:"user_nickname"`
	UserCardname string `json:"user_cardname,omitempty"`
}

// GroupInfo contains group details.
type GroupInfo struct {
	Platform  string `json:"platform"`
	GroupID   string `json:"group_id"`
	GroupName string `json:"group_name,omitempty"`
}

// MessageSegment contains the actual message content.
type MessageSegment struct {
	Type string `json:"type"`
	Data string `json:"data"`
}

// MessageDim contains routing information (target receiver).
type MessageDim struct {
	APIKey   string `json:"api_key"`
	Platform string `json:"platform"`
}

// AckPayload contains ACK confirmation data.
type AckPayload struct {
	Status          string  `json:"status"`
	ServerTimestamp  float64 `json:"server_timestamp"`
}

// AckMeta contains ACK metadata.
type AckMeta struct {
	UUID       string  `json:"uuid"`
	AckedMsgID string  `json:"acked_msg_id"`
	Timestamp  float64 `json:"timestamp"`
}

func nowTimestamp() float64 {
	return float64(time.Now().UnixNano()) / 1e9
}
