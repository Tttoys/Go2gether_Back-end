package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"GO2GETHER_BACK-END/internal/dto"
	"GO2GETHER_BACK-END/internal/utils"
)

type NotificationsHandler struct {
	db *pgxpool.Pool
}

func NewNotificationsHandler(db *pgxpool.Pool) *NotificationsHandler {
	return &NotificationsHandler{db: db}
}

// List handles GET /api/notifications
// @Summary List notifications
// @Description Get user's notifications with optional filters.
// @Tags notifications
// @Produce json
// @Security BearerAuth
// @Param unread_only query bool false "true to return only unread notifications"
// @Param type query string false "filter by notification type (exact match)"
// @Param limit query int false "items per page (default 20, max 100)"
// @Param offset query int false "offset (default 0)"
// @Success 200 {object} dto.NotificationsListResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/notifications [get]
func (h *NotificationsHandler) List(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	uid, ok := r.Context().Value("user_id").(uuid.UUID)
	if !ok {
		utils.WriteErrorResponse(w, http.StatusUnauthorized, "Unauthorized", "Invalid user context")
		return
	}

	q := r.URL.Query()
	unreadOnly := strings.EqualFold(strings.TrimSpace(q.Get("unread_only")), "true")
	typ := strings.TrimSpace(q.Get("type"))

	limit := 20
	if v := strings.TrimSpace(q.Get("limit")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			if n > 100 {
				n = 100
			}
			limit = n
		}
	}
	offset := 0
	if v := strings.TrimSpace(q.Get("offset")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}

	ctx := r.Context()

	// NOTE: ในสคีมาของคุณ column ชื่อ "read" (boolean) ไม่ใช่ is_read
	// และ "type" เป็น ENUM (USER-DEFINED) จึงแนะนำให้ cast ขณะ filter เป็น ::text
	where := `WHERE user_id = $1`
	args := []any{uid}
	arg := 2
	if unreadOnly {
		where += ` AND read = false`
	}
	if typ != "" {
		// ใช้ cast ::text เพื่อเปรียบเทียบกับ string ตัวกรอง
		where += ` AND type::text = $` + strconv.Itoa(arg)
		args = append(args, typ)
		arg++
	}

	// Total (ตาม filter)
	var total int
	if err := h.db.QueryRow(ctx, `SELECT COUNT(1) FROM notifications `+where, args...).Scan(&total); err != nil {
		utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", err.Error())
		return
	}

	// Unread count (ไม่สน filter type เพื่อแสดง badge โดยรวม)
	var unreadCount int
	if err := h.db.QueryRow(ctx,
		`SELECT COUNT(1) FROM notifications WHERE user_id = $1 AND read = false`, uid,
	).Scan(&unreadCount); err != nil {
		utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", err.Error())
		return
	}

	// Page
	rows, err := h.db.Query(ctx, `
		SELECT id, type::text, title, message, data, action_url, read, created_at
		  FROM notifications
		`+where+`
		  ORDER BY created_at DESC
		  LIMIT $`+strconv.Itoa(arg)+` OFFSET $`+strconv.Itoa(arg+1),
		append(args, limit, offset)...,
	)
	if err != nil {
		utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", err.Error())
		return
	}
	defer rows.Close()

	list := make([]dto.NotificationItem, 0, limit)
	for rows.Next() {
		var (
			id        uuid.UUID
			nType     string
			title     string
			message   *string
			dataRaw   []byte
			actionURL *string
			isRead    bool
			createdAt time.Time
		)
		if err := rows.Scan(&id, &nType, &title, &message, &dataRaw, &actionURL, &isRead, &createdAt); err != nil {
			utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", err.Error())
			return
		}

		var payload map[string]interface{}
		if len(dataRaw) > 0 {
			_ = json.Unmarshal(dataRaw, &payload)
		}

		item := dto.NotificationItem{
			ID:        id.String(),
			Type:      nType,
			Title:     title,
			Message:   "",
			Data:      payload,
			Read:      isRead,
			CreatedAt: createdAt.UTC().Format(time.RFC3339),
		}
		if message != nil {
			item.Message = *message
		}
		if actionURL != nil {
			item.ActionURL = actionURL
		}
		list = append(list, item)
	}
	if err := rows.Err(); err != nil {
		utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", err.Error())
		return
	}

	resp := dto.NotificationsListResponse{
		Notifications: list,
		Pagination: dto.NotificationsPagination{
			Total:       total,
			UnreadCount: unreadCount,
			Limit:       limit,
			Offset:      offset,
		},
	}
	utils.WriteJSONResponse(w, http.StatusOK, resp)
}

type DBRunner interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// CreateNotification inserts a new notification row.
// NOTE: Replace "notification_type" with your real enum type name for notifications.type.
func CreateNotification(ctx context.Context, db DBRunner, userID uuid.UUID, nType string, title string, message *string, data map[string]any, actionURL *string) error {
	var dataBytes []byte
	if data != nil {
		if b, err := json.Marshal(data); err == nil {
			dataBytes = b
		}
	}

	// ใช้ CAST ให้ตรง enum: CAST($2 AS notification_type)
	_, err := db.Exec(ctx, `
		INSERT INTO notifications (user_id, type, title, message, data, action_url, read, created_at)
		VALUES ($1, CAST($2 AS notification_type), $3, $4, $5, $6, false, $7)
	`, userID, nType, title, message, dataBytes, actionURL, time.Now())
	return err
}
