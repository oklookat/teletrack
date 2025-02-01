package config

type Telegram struct {
	Token         string `json:"token"`
	UserID        int64  `json:"userID"`
	ChatID        string `json:"chatID"`
	ServiceChatID string `json:"serviceChatID"`
	MessageID     int    `json:"messageID"`
}
