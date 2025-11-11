package dto

type NotificationItem struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Title     string                 `json:"title"`
	Message   string                 `json:"message"`
	Data      map[string]interface{} `json:"data,omitempty"`
	ActionURL *string                `json:"action_url,omitempty"`
	Read      bool                   `json:"read"`
	CreatedAt string                 `json:"created_at"`
}

type NotificationsPagination struct {
	Total       int `json:"total"`
	UnreadCount int `json:"unread_count"`
	Limit       int `json:"limit"`
	Offset      int `json:"offset"`
}

type NotificationsListResponse struct {
	Notifications []NotificationItem      `json:"notifications"`
	Pagination    NotificationsPagination `json:"pagination"`
}
