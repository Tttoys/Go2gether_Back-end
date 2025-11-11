package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"GO2GETHER_BACK-END/internal/dto"
	"GO2GETHER_BACK-END/internal/utils"
)

type Type string

const (
	TypeTripInvitation     Type = "trip_invitation"
	TypeInvitationAccepted Type = "invitation_accepted"
	TypeInvitationDeclined Type = "invitation_declined"
	TypeTripUpdate         Type = "trip_update"
	TypeAvailability       Type = "availability_updated"
	TypeMemberJoined       Type = "member_joined"
	TypeMemberLeft         Type = "member_left"
)

// NotificationsService: helper (สร้าง noti)
type NotificationsService interface {
	Create(ctx context.Context, userID uuid.UUID, nType string, title string, message *string, data map[string]any, actionURL *string) error
}

// concrete service
type notificationsService struct {
	db *pgxpool.Pool
}

func NewNotificationsService(db *pgxpool.Pool) NotificationsService {
	return &notificationsService{db: db}
}

// Implement the Create method for notificationsService
func (s *notificationsService) Create(
	ctx context.Context,
	userID uuid.UUID,
	nType string,
	title string,
	message *string,
	data map[string]any,
	actionURL *string,
) error {
	var dataJSON []byte
	var err error
	if data != nil {
		dataJSON, err = json.Marshal(data)
		if err != nil {
			return err
		}
	}

	cmdTag, err := s.db.Exec(ctx, `
		INSERT INTO notifications (user_id, type, title, message, data, action_url)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, userID, nType, title, message, dataJSON, actionURL)
	if err != nil {
		return err
	}
	if cmdTag.RowsAffected() != 1 {
		return errors.New("failed to insert notification")
	}
	return nil
}

func Create(
	ctx context.Context,
	db *pgxpool.Pool,
	userID string, // ผู้รับ noti (uuid string)
	typ Type, // ต้องเป็นค่าที่ enum มีอยู่แล้ว
	title string, // หัวข้อ
	message *string, // เนื้อความ (nullable)
	data map[string]any, // payload (nullable)
	actionURL *string, // ลิงก์กดไปหน้าใด (nullable)
) error {
	var dataJSON []byte
	var err error
	if data != nil {
		dataJSON, err = json.Marshal(data)
		if err != nil {
			return err
		}
	}

	cmdTag, err := db.Exec(ctx, `
		INSERT INTO notifications (user_id, type, title, message, data, action_url)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, userID, string(typ), title, message, dataJSON, actionURL)
	if err != nil {
		return err
	}
	if cmdTag.RowsAffected() != 1 {
		return errors.New("failed to insert notification")
	}
	return nil
}

// NotificationsHandler: HTTP endpoints (list/mark read/mark all read)
type NotificationsHandler struct {
	db  *pgxpool.Pool
	svc NotificationsService
}

func NewNotificationsHandler(db *pgxpool.Pool) *NotificationsHandler {
	return &NotificationsHandler{
		db:  db,
		svc: NewNotificationsService(db),
	}
}

func (h *NotificationsHandler) Service() NotificationsService { return h.svc }

// -----------------------------------------------------------------------------
// 5.1 GET /api/notifications
// @Summary List notifications
// @Description List user notifications with filters and pagination.
// @Tags notifications
// @Produce json
// @Security BearerAuth
// @Param unread_only query bool false "true|false (default false)"
// @Param type query string false "filter by type"
// @Param limit query int false "default 20 (max 100)"
// @Param offset query int false "default 0"
// @Success 200 {object} dto.NotificationListResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/notifications [get]
func (h *NotificationsHandler) ListNotifications(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	userID, ok := r.Context().Value("user_id").(uuid.UUID)
	if !ok {
		utils.WriteErrorResponse(w, http.StatusUnauthorized, "Unauthorized", "Invalid user context")
		return
	}

	q := r.URL.Query()
	unreadOnly := strings.EqualFold(q.Get("unread_only"), "true")
	typ := strings.TrimSpace(q.Get("type"))
	limit := 20
	offset := 0
	if v := q.Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			if n > 100 {
				n = 100
			}
			limit = n
		}
	}
	if v := q.Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}

	// count unread
	var unreadCount int
	if err := h.db.QueryRow(r.Context(),
		`SELECT COUNT(1) FROM notifications WHERE user_id=$1 AND read=false`, userID,
	).Scan(&unreadCount); err != nil {
		utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", err.Error())
		return
	}

	// total and list query
	args := []any{userID}
	where := `WHERE user_id=$1`
	arg := 2
	if unreadOnly {
		where += " AND read=false"
	}
	if typ != "" {
		where += " AND type=$" + strconv.Itoa(arg)
		args = append(args, typ)
		arg++
	}

	// total
	total := 0
	if err := h.db.QueryRow(r.Context(),
		`SELECT COUNT(1) FROM notifications `+where, args...,
	).Scan(&total); err != nil {
		utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", err.Error())
		return
	}

	// page
	args = append(args, limit, offset)
	rows, err := h.db.Query(r.Context(),
		`SELECT id, type, title, message, data, action_url, read, created_at
		   FROM notifications `+where+`
		 ORDER BY created_at DESC
		 LIMIT $`+strconv.Itoa(arg)+` OFFSET $`+strconv.Itoa(arg+1), args...)
	if err != nil {
		utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", err.Error())
		return
	}
	defer rows.Close()

	items := make([]dto.NotificationItem, 0, limit)
	for rows.Next() {
		var (
			id        uuid.UUID
			typStr    string
			title     string
			message   *string
			dataRaw   []byte
			actionURL *string
			read      bool
			createdAt time.Time
		)
		if err := rows.Scan(&id, &typStr, &title, &message, &dataRaw, &actionURL, &read, &createdAt); err != nil {
			utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", err.Error())
			return
		}
		var data map[string]any
		if len(dataRaw) > 0 && string(dataRaw) != "null" {
			_ = json.Unmarshal(dataRaw, &data) // ถ้า error ปล่อยว่าง
		}
		items = append(items, dto.NotificationItem{
			ID:        id.String(),
			Type:      typStr,
			Title:     title,
			Message:   message,
			Data:      data,
			ActionURL: actionURL,
			Read:      read,
			CreatedAt: createdAt.UTC().Format(time.RFC3339),
		})
	}
	if err := rows.Err(); err != nil {
		utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", err.Error())
		return
	}

	utils.WriteJSONResponse(w, http.StatusOK, dto.NotificationListResponse{
		Notifications: items,
		Pagination: dto.NotificationListPagination{
			Total:       total,
			UnreadCount: unreadCount,
			Limit:       limit,
			Offset:      offset,
		},
	})
}

// -----------------------------------------------------------------------------
// 5.2 POST /api/notifications/{id}/read  (mark one as read)
// @Summary Mark a notification as read
// @Tags notifications
// @Produce json
// @Security BearerAuth
// @Param id path string true "Notification ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/notifications/{id}/read [post]
func (h *NotificationsHandler) MarkRead(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	userID, ok := r.Context().Value("user_id").(uuid.UUID)
	if !ok {
		utils.WriteErrorResponse(w, http.StatusUnauthorized, "Unauthorized", "Invalid user context")
		return
	}

	path := r.URL.Path // /api/notifications/{id}/read
	rest := strings.TrimPrefix(path, "/api/notifications/")
	slash := strings.Index(rest, "/")
	if slash <= 0 || !strings.HasSuffix(path, "/read") {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Invalid path", "missing or invalid id")
		return
	}
	idStr := rest[:slash]
	nID, err := uuid.Parse(idStr)
	if err != nil {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Invalid id", "id must be UUID")
		return
	}

	// อนุญาตเฉพาะ noti ของตัวเอง
	cmd, err := h.db.Exec(r.Context(),
		`UPDATE notifications SET read=true WHERE id=$1 AND user_id=$2`, nID, userID,
	)
	if err != nil {
		utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", err.Error())
		return
	}
	if cmd.RowsAffected() == 0 {
		utils.WriteErrorResponse(w, http.StatusNotFound, "Not Found", "Notification not found")
		return
	}
	utils.WriteJSONResponse(w, http.StatusOK, map[string]string{"message": "Notification marked as read"})
}

// -----------------------------------------------------------------------------
// 5.3 POST /api/notifications/read-all
// @Summary Mark all notifications as read
// @Tags notifications
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]string
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/notifications/read-all [post]
func (h *NotificationsHandler) MarkAllRead(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	userID, ok := r.Context().Value("user_id").(uuid.UUID)
	if !ok {
		utils.WriteErrorResponse(w, http.StatusUnauthorized, "Unauthorized", "Invalid user context")
		return
	}
	_, err := h.db.Exec(r.Context(),
		`UPDATE notifications SET read=true WHERE user_id=$1 AND read=false`, userID,
	)
	if err != nil {
		utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", err.Error())
		return
	}
	utils.WriteJSONResponse(w, http.StatusOK, map[string]string{"message": "All notifications marked as read"})
}
