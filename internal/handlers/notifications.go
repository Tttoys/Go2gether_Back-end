package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
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
// Production-ready: includes validation, proper error handling, and logging
func (s *notificationsService) Create(
	ctx context.Context,
	userID uuid.UUID,
	nType string,
	title string,
	message *string,
	data map[string]any,
	actionURL *string,
) error {
	// Validation
	if userID == uuid.Nil {
		return errors.New("user_id cannot be nil")
	}
	if strings.TrimSpace(nType) == "" {
		return errors.New("notification type is required")
	}
	if strings.TrimSpace(title) == "" {
		return errors.New("notification title is required")
	}
	if len(title) > 255 {
		return errors.New("notification title exceeds maximum length of 255 characters")
	}
	if message != nil && len(*message) > 10000 {
		return errors.New("notification message exceeds maximum length of 10000 characters")
	}
	if actionURL != nil && len(*actionURL) > 2048 {
		return errors.New("action_url exceeds maximum length of 2048 characters")
	}

	// Validate notification type
	validTypes := map[string]bool{
		"trip_invitation":      true,
		"invitation_accepted":  true,
		"invitation_declined":  true,
		"trip_update":          true,
		"availability_updated": true,
		"member_joined":        true,
		"member_left":          true,
	}
	if !validTypes[nType] {
		log.Printf("Warning: Unknown notification type: %s (user_id=%s)", nType, userID.String())
		// ไม่ return error เพื่อไม่ให้บล็อกการทำงาน แต่ log warning
	}

	// Prepare JSON data
	var dataJSON interface{}
	if len(data) > 0 {
		jsonBytes, err := json.Marshal(data)
		if err != nil {
			return fmt.Errorf("failed to marshal notification data: %w", err)
		}
		// Limit JSON size to prevent abuse (1MB limit)
		if len(jsonBytes) > 1024*1024 {
			return errors.New("notification data exceeds maximum size of 1MB")
		}
		dataJSON = string(jsonBytes)
	} else {
		dataJSON = nil
	}

	// Insert with context timeout
	insertCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	cmdTag, err := s.db.Exec(insertCtx, `
		INSERT INTO notifications (user_id, type, title, message, data, action_url)
		VALUES ($1, $2, $3, $4, $5::jsonb, $6)
	`, userID, nType, title, message, dataJSON, actionURL)

	if err != nil {
		// Check for specific database errors
		if errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf("notification creation timeout: %w", err)
		}
		// Log database errors for monitoring
		if strings.Contains(err.Error(), "connection") || strings.Contains(err.Error(), "network") {
			log.Printf("Database connection error creating notification: %v (user_id=%s, type=%s)",
				err, userID.String(), nType)
		}
		return fmt.Errorf("failed to insert notification: %w", err)
	}

	if cmdTag.RowsAffected() != 1 {
		log.Printf("Warning: Notification insert affected %d rows instead of 1 (user_id=%s, type=%s)",
			cmdTag.RowsAffected(), userID.String(), nType)
		return errors.New("unexpected number of rows affected")
	}

	return nil
}

// Create is deprecated - use NotificationsService.Create instead
// This function is kept for backward compatibility but should not be used in new code
// Deprecated: Use NotificationsService.Create instead
func Create(
	ctx context.Context,
	db *pgxpool.Pool,
	userID string,
	typ Type,
	title string,
	message *string,
	data map[string]any,
	actionURL *string,
) error {
	// Parse userID to UUID
	uid, err := uuid.Parse(userID)
	if err != nil {
		return fmt.Errorf("invalid user_id format: %w", err)
	}

	// Use the service implementation
	service := NewNotificationsService(db)
	return service.Create(ctx, uid, string(typ), title, message, data, actionURL)
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

	// Create context with timeout
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// Parse and validate query parameters
	q := r.URL.Query()
	unreadOnly := strings.EqualFold(q.Get("unread_only"), "true")
	typ := strings.TrimSpace(q.Get("type"))

	// Validate and parse limit (default 20, max 100)
	limit := 20
	if v := q.Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			if n > 100 {
				n = 100
			}
			limit = n
		} else {
			utils.WriteErrorResponse(w, http.StatusBadRequest, "Invalid limit", "limit must be a positive integer")
			return
		}
	}

	// Validate and parse offset (default 0, min 0)
	offset := 0
	if v := q.Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		} else {
			utils.WriteErrorResponse(w, http.StatusBadRequest, "Invalid offset", "offset must be a non-negative integer")
			return
		}
	}

	// Validate notification type if provided
	if typ != "" {
		validTypes := map[string]bool{
			"trip_invitation":      true,
			"invitation_accepted":  true,
			"invitation_declined":  true,
			"trip_update":          true,
			"availability_updated": true,
			"member_joined":        true,
			"member_left":          true,
		}
		if !validTypes[typ] {
			utils.WriteErrorResponse(w, http.StatusBadRequest, "Invalid type", "invalid notification type")
			return
		}
	}

	// Count unread notifications
	var unreadCount int
	if err := h.db.QueryRow(ctx,
		`SELECT COUNT(1) FROM notifications WHERE user_id=$1 AND read=false`, userID,
	).Scan(&unreadCount); err != nil {
		log.Printf("Error counting unread notifications: %v (user_id=%s)", err, userID.String())
		utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", "Failed to count unread notifications")
		return
	}

	// Build query with proper parameterization to prevent SQL injection
	args := []any{userID}
	where := `WHERE user_id=$1`
	argNum := 2

	if unreadOnly {
		where += " AND read=false"
	}
	if typ != "" {
		where += fmt.Sprintf(" AND type=$%d", argNum)
		args = append(args, typ)
		argNum++
	}

	// Count total matching notifications
	var total int
	if err := h.db.QueryRow(ctx,
		fmt.Sprintf(`SELECT COUNT(1) FROM notifications %s`, where), args...,
	).Scan(&total); err != nil {
		log.Printf("Error counting notifications: %v (user_id=%s)", err, userID.String())
		utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", "Failed to count notifications")
		return
	}

	// Fetch notifications with pagination
	args = append(args, limit, offset)
	query := fmt.Sprintf(`
		SELECT id, type, title, message, data, action_url, read, created_at
		FROM notifications %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, where, argNum, argNum+1)

	rows, err := h.db.Query(ctx, query, args...)
	if err != nil {
		log.Printf("Error querying notifications: %v (user_id=%s)", err, userID.String())
		utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", "Failed to fetch notifications")
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
			log.Printf("Error scanning notification row: %v (user_id=%s)", err, userID.String())
			utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", "Failed to process notification data")
			return
		}

		// Parse JSON data safely
		var data map[string]any
		if len(dataRaw) > 0 && string(dataRaw) != "null" {
			if err := json.Unmarshal(dataRaw, &data); err != nil {
				log.Printf("Warning: Failed to unmarshal notification data: %v (notification_id=%s)", err, id.String())
				// Continue with empty data instead of failing
				data = nil
			}
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
		log.Printf("Error iterating notification rows: %v (user_id=%s)", err, userID.String())
		utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", "Failed to process notifications")
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

	// Parse notification ID from URL path
	path := r.URL.Path // /api/notifications/{id}/read
	rest := strings.TrimPrefix(path, "/api/notifications/")
	slash := strings.Index(rest, "/")
	if slash <= 0 || !strings.HasSuffix(path, "/read") {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Invalid path", "missing or invalid notification id")
		return
	}
	idStr := strings.TrimSpace(rest[:slash])
	if idStr == "" {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Invalid path", "notification id is required")
		return
	}

	nID, err := uuid.Parse(idStr)
	if err != nil {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Invalid id", "notification id must be a valid UUID")
		return
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	// Update notification - only allow users to mark their own notifications as read
	cmd, err := h.db.Exec(ctx,
		`UPDATE notifications SET read=true WHERE id=$1 AND user_id=$2 AND read=false`,
		nID, userID,
	)
	if err != nil {
		log.Printf("Error marking notification as read: %v (notification_id=%s, user_id=%s)",
			err, nID.String(), userID.String())
		utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", "Failed to update notification")
		return
	}

	if cmd.RowsAffected() == 0 {
		// Check if notification exists but belongs to another user or already read
		var exists bool
		if err := h.db.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM notifications WHERE id=$1)`, nID,
		).Scan(&exists); err == nil && exists {
			utils.WriteErrorResponse(w, http.StatusForbidden, "Forbidden",
				"Notification not found or already marked as read")
		} else {
			utils.WriteErrorResponse(w, http.StatusNotFound, "Not Found", "Notification not found")
		}
		return
	}

	utils.WriteJSONResponse(w, http.StatusOK, map[string]string{
		"message": "Notification marked as read",
	})
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

	// Create context with timeout
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// Update all unread notifications for the user
	cmd, err := h.db.Exec(ctx,
		`UPDATE notifications SET read=true WHERE user_id=$1 AND read=false`, userID,
	)
	if err != nil {
		log.Printf("Error marking all notifications as read: %v (user_id=%s)", err, userID.String())
		utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", "Failed to update notifications")
		return
	}

	updatedCount := cmd.RowsAffected()
	utils.WriteJSONResponse(w, http.StatusOK, map[string]interface{}{
		"message":       "All notifications marked as read",
		"updated_count": updatedCount,
	})
}
