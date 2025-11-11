package dto

// NotificationItem ใช้สำหรับ list
type NotificationItem struct {
	ID        string         `json:"id"`
	Type      string         `json:"type"`
	Title     string         `json:"title"`
	Message   *string        `json:"message,omitempty"`
	Data      map[string]any `json:"data,omitempty"`
	ActionURL *string        `json:"action_url,omitempty"`
	Read      bool           `json:"read"`
	CreatedAt string         `json:"created_at"`
}

// NotificationListPagination
type NotificationListPagination struct {
	Total       int `json:"total"`
	UnreadCount int `json:"unread_count"`
	Limit       int `json:"limit"`
	Offset      int `json:"offset"`
}

// NotificationListResponse
type NotificationListResponse struct {
	Notifications []NotificationItem         `json:"notifications"`
	Pagination    NotificationListPagination `json:"pagination"`
}

// ---- (optional) สำหรับ mark read ทั้งหมดไม่มี body ----

// ErrorResponse (คุณมีอยู่แล้วในโปรเจกต์)
// type ErrorResponse struct {
// 	Error   string `json:"error"`
// 	Message string `json:"message"`
// }
