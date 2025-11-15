package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"GO2GETHER_BACK-END/internal/config"
	"GO2GETHER_BACK-END/internal/dto"
	"GO2GETHER_BACK-END/internal/middleware"
	"GO2GETHER_BACK-END/internal/models"
	"GO2GETHER_BACK-END/internal/utils"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TripsHandler manages trip-related endpoints
type TripsHandler struct {
	db     *pgxpool.Pool
	config *config.Config
	noti   NotificationsService
}

// NewTripsHandler creates a new TripsHandler
func NewTripsHandler(db *pgxpool.Pool, cfg *config.Config) *TripsHandler {
	return &TripsHandler{
		db:     db,
		config: cfg,
		noti:   NewNotificationsService(db), // <- ผูก service
	}
}

func cleanPath(p string) string {
	if p == "/" {
		return p
	}
	return strings.TrimRight(p, "/")
}

// Trips dispatches by HTTP method for /api/trips
func (h *TripsHandler) Trips(w http.ResponseWriter, r *http.Request) {
	path := cleanPath(r.URL.Path)

	switch r.Method {
	case http.MethodPost:
		// POST /api/trips/join - Join trip via invitation link
		if path == "/api/trips/join" {
			h.JoinViaLink(w, r)
			return
		}
		// 2.2 POST /api/trips/{trip_id}/availability
		if strings.HasPrefix(r.URL.Path, "/api/trips/") && strings.HasSuffix(r.URL.Path, "/availability") {
			h.SaveAvailability(w, r)
			return
		}
		// 2.4 POST /api/trips/{trip_id}/availability/generate-periods
		if strings.HasPrefix(r.URL.Path, "/api/trips/") && strings.HasSuffix(r.URL.Path, "/availability/generate-periods") {
			h.GenerateAvailablePeriods(w, r)
			return
		}
		// FR3.5 POST /api/trips/{trip_id}/leave
		if strings.HasPrefix(path, "/api/trips/") && strings.HasSuffix(path, "/leave") {
			h.LeaveTrip(w, r)
			return
		}
		// FR3.1 POST /api/trips/{trip_id}/invitations
		if strings.HasPrefix(path, "/api/trips/") && strings.HasSuffix(path, "/invitations") {
			h.InviteMembers(w, r)
			return
		}
		// FR1.1 POST /api/trips
		if path == "/api/trips" {
			h.CreateTrip(w, r)
			return
		}

		utils.WriteErrorResponse(w, http.StatusNotFound, "Not Found", "unknown POST route")
		return

	case http.MethodGet:
		p := r.URL.Path
		// 2.1 GET /api/trips/{trip_id}/dates
		if strings.HasPrefix(p, "/api/trips/") && strings.HasSuffix(p, "/dates") {
			h.TripDates(w, r)
			return
		}

		// 2.3 GET /api/trips/{trip_id}/availability/me
		if strings.HasPrefix(r.URL.Path, "/api/trips/") && strings.HasSuffix(r.URL.Path, "/availability/me") {
			h.GetMyAvailability(w, r)
			return
		}

		// 2.5 GET /api/trips/{trip_id}/available-periods
		if strings.HasPrefix(r.URL.Path, "/api/trips/") && strings.HasSuffix(r.URL.Path, "/available-periods") {
			h.GetAvailablePeriods(w, r)
			return
		}

		// FR3.3 GET /api/trips/{trip_id}/invitations
		if strings.HasPrefix(path, "/api/trips/") && strings.HasSuffix(path, "/invitations") {
			h.ListInvitations(w, r)
			return
		}
		// FR1.3 GET /api/trips/{trip_id}
		if strings.HasPrefix(path, "/api/trips/") {
			rest := strings.TrimPrefix(path, "/api/trips/")
			if !strings.Contains(rest, "/") {
				h.TripDetail(w, r)
				return
			}
		}
		// FR1.2 GET /api/trips
		if path == "/api/trips" {
			h.ListTrips(w, r)
			return
		}
		utils.WriteErrorResponse(w, http.StatusNotFound, "Not Found", "unknown GET route")
		return

	case http.MethodPut, http.MethodPatch:
		// FR1.4 PUT/PATCH /api/trips/{trip_id}
		rest := strings.TrimPrefix(path, "/api/trips/")
		if rest != "" && !strings.Contains(rest, "/") {
			h.UpdateTrip(w, r)
			return
		}
		utils.WriteErrorResponse(w, http.StatusNotFound, "Not Found", "unknown PUT/PATCH route")
		return

	case http.MethodDelete:
		// FR3.6 DELETE /api/trips/{trip_id}/members/{user_id}
		if strings.HasPrefix(path, "/api/trips/") && strings.Contains(path, "/members/") {
			h.RemoveMember(w, r)
			return
		}
		// FR1.5 DELETE /api/trips/{trip_id}
		rest := strings.TrimPrefix(path, "/api/trips/")
		if rest != "" && !strings.Contains(rest, "/") {
			h.DeleteTrip(w, r)
			return
		}
		utils.WriteErrorResponse(w, http.StatusNotFound, "Not Found", "unknown DELETE route")
		return

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

//
// ===================== FR1 (เดิม) — ไม่ได้แก้ logic =====================
//

// CreateTrip handles POST /api/trips
// @Summary Create a new trip
// @Tags trips
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param payload body dto.CreateTripRequest true "Trip payload"
// @Success 201 {object} dto.CreateTripResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/trips [post]
func (h *TripsHandler) CreateTrip(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	uid := r.Context().Value("user_id")
	userID, ok := uid.(uuid.UUID)
	if !ok {
		utils.WriteErrorResponse(w, http.StatusUnauthorized, "Unauthorized", "Invalid user context")
		return
	}

	var req dto.CreateTripRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		log.Printf("decode error: %v", err) // เพิ่มบรรทัดนี้
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Invalid request data", "Malformed JSON body")
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Destination = strings.TrimSpace(req.Destination)
	req.Status = strings.ToLower(strings.TrimSpace(req.Status))
	if req.Name == "" || req.Destination == "" || req.StartDate == "" || req.EndDate == "" {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Validation error", "name, destination, start_date, end_date are required")
		return
	}
	switch req.Status {
	case "", "draft", "published", "cancelled":
		if req.Status == "" {
			req.Status = "draft"
		}
	default:
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Validation error", "status must be draft, published, or cancelled")
		return
	}

	parseDate := func(s string) (time.Time, error) {
		if len(s) == 10 {
			return time.Parse("2006-01-02", s)
		}
		return time.Parse(time.RFC3339, s)
	}
	startAt, err := parseDate(req.StartDate)
	if err != nil {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Validation error", "start_date must be YYYY-MM-DD or RFC3339")
		return
	}
	endAt, err := parseDate(req.EndDate)
	if err != nil {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Validation error", "end_date must be YYYY-MM-DD or RFC3339")
		return
	}
	if endAt.Before(startAt) {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Validation error", "end_date cannot be before start_date")
		return
	}

	now := time.Now()
	newID := uuid.New()

	currency := strings.ToUpper(strings.TrimSpace(req.Currency))
	if currency == "" {
		currency = "THB"
	}

	// NEW: ดึง budget แยกหมวดจาก request
	food := req.Food
	hotel := req.Hotel
	shopping := req.Shopping
	transport := req.Transport

	if food < 0 || hotel < 0 || shopping < 0 || transport < 0 {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Validation error", "budget categories cannot be negative")
		return
	}

	// totalBudget เริ่มจากของเดิม (รองรับ client เก่า)
	totalBudget := req.TotalBudget

	// ถ้ามี breakdown อย่างน้อย 1 หมวด → ใช้ breakdown เป็นหลัก
	if food != 0 || hotel != 0 || shopping != 0 || transport != 0 {
		totalBudget = food + hotel + shopping + transport
	} else {
		// ถ้าไม่มี breakdown แต่มี total_budget → เอา total_budget ไปลง food ทั้งก้อน
		if totalBudget < 0 {
			utils.WriteErrorResponse(w, http.StatusBadRequest, "Validation error", "total_budget cannot be negative")
			return
		}
		if totalBudget > 0 {
			food = totalBudget
		}
	}

	_, err = h.db.Exec(context.Background(),
		`INSERT INTO trips (id, name, destination, start_date, end_date, description, status, total_budget, currency, creator_id, created_at, updated_at)
         VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
		newID, req.Name, req.Destination, startAt, endAt, req.Description, req.Status, totalBudget, currency, userID, now, now,
	)
	if err != nil {
		utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", err.Error())
		return
	}
	// NEW: บันทึก budget breakdown ลง budget_categories
	_, err = h.db.Exec(
		context.Background(),
		`INSERT INTO budget_categories (trip_id, order_index, food, hotel, shopping, transport)
         VALUES ($1, 1, $2, $3, $4, $5)
         ON CONFLICT (trip_id, order_index)
         DO UPDATE SET
            food = EXCLUDED.food,
            hotel = EXCLUDED.hotel,
            shopping = EXCLUDED.shopping,
            transport = EXCLUDED.transport,
            updated_at = now()`,
		newID, food, hotel, shopping, transport,
	)
	if err != nil {
		utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", err.Error())
		return
	}

	_, _ = h.db.Exec(context.Background(),
		`INSERT INTO trip_members (trip_id, user_id, role, status, availability_submitted, invited_at, joined_at)
         VALUES ($1, $2, 'creator', 'accepted', FALSE, $3, $3)
         ON CONFLICT (trip_id, user_id) DO NOTHING`,
		newID, userID, now,
	)

	trip := models.Trip{
		ID:          newID,
		Name:        req.Name,
		Destination: req.Destination,
		StartDate:   startAt,
		EndDate:     endAt,
		Description: req.Description,
		Status:      req.Status,
		TotalBudget: totalBudget,
		Currency:    currency,
		CreatorID:   userID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	resp := dto.CreateTripResponse{Trip: dto.TripResponse{
		ID:          trip.ID.String(),
		Name:        trip.Name,
		Destination: trip.Destination,
		StartDate:   trip.StartDate.Format("2006-01-02"),
		EndDate:     trip.EndDate.Format("2006-01-02"),
		Description: trip.Description,
		Status:      trip.Status,
		TotalBudget: trip.TotalBudget,
		Currency:    trip.Currency,
		CreatorID:   trip.CreatorID.String(),
		CreatedAt:   trip.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   trip.UpdatedAt.Format(time.RFC3339),
		// NEW
		Budget: dto.TripBudgetResponse{
			Food:      food,
			Hotel:     hotel,
			Shopping:  shopping,
			Transport: transport,
			Total:     trip.TotalBudget,
		},
	}}

	utils.WriteJSONResponse(w, http.StatusCreated, resp)
}

// ListTrips handles GET /api/trips with filters and pagination
// @Summary List trips
// @Tags trips
// @Produce json
// @Security BearerAuth
// @Param status query string false "draft|published|cancelled|all"
// @Param limit query int false "items per page"
// @Param offset query int false "offset"
// @Success 200 {object} dto.TripListResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/trips [get]
func (h *TripsHandler) ListTrips(w http.ResponseWriter, r *http.Request) {
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
	status := strings.ToLower(strings.TrimSpace(q.Get("status")))
	if status == "" {
		status = "all"
	}
	if status != "all" && status != "draft" && status != "published" && status != "cancelled" {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Validation error", "invalid status")
		return
	}
	limit := 20
	offset := 0
	if v := strings.TrimSpace(q.Get("limit")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			if n > 100 {
				n = 100
			}
			limit = n
		}
	}
	if v := strings.TrimSpace(q.Get("offset")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}

	var total int
	if err := h.db.QueryRow(context.Background(),
		`SELECT COUNT(1)
           FROM trips t
           JOIN trip_members tm ON tm.trip_id = t.id
          WHERE tm.user_id = $1
            AND tm.status = 'accepted'
            AND ($2 = 'all' OR t.status = $2)`,
		userID, status,
	).Scan(&total); err != nil {
		utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", err.Error())
		return
	}

	rows, err := h.db.Query(context.Background(),
		`SELECT t.id, t.name, t.destination, t.start_date, t.end_date, t.status, t.total_budget, t.currency, t.creator_id, t.created_at,
                COALESCE((SELECT COUNT(DISTINCT tm2.user_id) FROM trip_members tm2 WHERE tm2.trip_id = t.id), 0) AS member_count
           FROM trips t
           JOIN trip_members tm ON tm.trip_id = t.id
          WHERE tm.user_id = $1
            AND tm.status = 'accepted'
            AND ($2 = 'all' OR t.status = $2)
          ORDER BY t.created_at DESC
          LIMIT $3 OFFSET $4`, userID, status, limit, offset)
	if err != nil {
		utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", err.Error())
		return
	}
	defer rows.Close()

	items := make([]dto.TripListItem, 0, limit)
	for rows.Next() {
		var id uuid.UUID
		var name, destination, st, currency string
		var startAt, endAt, createdAt time.Time
		var creatorID uuid.UUID
		var memberCount int
		var totalBudget float64
		if err := rows.Scan(&id, &name, &destination, &startAt, &endAt, &st, &totalBudget, &currency, &creatorID, &createdAt, &memberCount); err != nil {
			utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", err.Error())
			return
		}
		items = append(items, dto.TripListItem{
			ID:          id.String(),
			Name:        name,
			Destination: destination,
			StartDate:   startAt.Format("2006-01-02"),
			EndDate:     endAt.Format("2006-01-02"),
			Status:      st,
			TotalBudget: totalBudget,
			Currency:    currency,
			CreatorID:   creatorID.String(),
			MemberCount: memberCount,
			CreatedAt:   createdAt.Format(time.RFC3339),
		})
	}
	if err := rows.Err(); err != nil {
		utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", err.Error())
		return
	}

	resp := dto.TripListResponse{
		Trips: items,
		Pagination: dto.Pagination{
			Total:  total,
			Limit:  limit,
			Offset: offset,
		},
	}
	utils.WriteJSONResponse(w, http.StatusOK, resp)
}

// TripDetail handles GET /api/trips/{trip_id}
// @Summary Get trip detail
// @Tags trips
// @Produce json
// @Security BearerAuth
// @Param trip_id path string true "Trip ID"
// @Success 200 {object} dto.TripDetailResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/trips/{trip_id} [get]
func (h *TripsHandler) TripDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	requesterID, ok := r.Context().Value("user_id").(uuid.UUID)
	if !ok {
		utils.WriteErrorResponse(w, http.StatusUnauthorized, "Unauthorized", "Invalid user context")
		return
	}

	path := cleanPath(r.URL.Path)
	idStr := strings.TrimPrefix(path, "/api/trips/")
	tripID, err := uuid.Parse(idStr)
	if err != nil {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Invalid trip id", "trip_id must be UUID")
		return
	}

	var t models.Trip
	err = h.db.QueryRow(context.Background(),
		`SELECT id, name, destination, start_date, end_date, description, status, total_budget, currency, creator_id, created_at, updated_at
           FROM trips WHERE id = $1`, tripID).Scan(
		&t.ID, &t.Name, &t.Destination, &t.StartDate, &t.EndDate, &t.Description, &t.Status, &t.TotalBudget, &t.Currency, &t.CreatorID, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		utils.WriteErrorResponse(w, http.StatusNotFound, "Not Found", "Trip not found")
		return
	}

	// NEW: ดึง budget breakdown จาก budget_categories
	var food, hotel, shopping, transport float64
	err = h.db.QueryRow(
		context.Background(),
		`SELECT food, hotel, shopping, transport
           FROM budget_categories
          WHERE trip_id = $1 AND order_index = 1`,
		t.ID,
	).Scan(&food, &hotel, &shopping, &transport)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			food, hotel, shopping, transport = 0, 0, 0, 0
		} else {
			utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", err.Error())
			return
		}
	}

	rows, err := h.db.Query(context.Background(),
		`SELECT tm.user_id, tm.role, tm.status, tm.availability_submitted, tm.invited_at, tm.joined_at,
                COALESCE(u.email, '') as username
           FROM trip_members tm
           LEFT JOIN users u ON u.id = tm.user_id
          WHERE tm.trip_id = $1`, tripID)
	if err != nil {
		utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", err.Error())
		return
	}
	defer rows.Close()

	members := make([]dto.TripMember, 0)
	isCreatorMember := false
	for rows.Next() {
		var uid uuid.UUID
		var role, mstatus, username string
		var availability bool
		var invitedAt, joinedAt *time.Time
		if err := rows.Scan(&uid, &role, &mstatus, &availability, &invitedAt, &joinedAt, &username); err != nil {
			utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", err.Error())
			return
		}
		log.Printf("TripDetail debug: requester=%s member=%s role=%s", requesterID.String(), uid.String(), role)
		if uid == requesterID && strings.EqualFold(strings.TrimSpace(role), "creator") {
			isCreatorMember = true
		}
		m := dto.TripMember{
			UserID:                uid.String(),
			Username:              username,
			DisplayName:           "",
			FirstName:             "",
			LastName:              "",
			AvatarURL:             "",
			Role:                  role,
			Status:                mstatus,
			AvailabilitySubmitted: availability,
			InvitedAt:             "",
			JoinedAt:              "",
		}
		if invitedAt != nil {
			m.InvitedAt = invitedAt.Format(time.RFC3339)
		}
		if joinedAt != nil {
			m.JoinedAt = joinedAt.Format(time.RFC3339)
		}
		members = append(members, m)
	}
	if err := rows.Err(); err != nil {
		utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", err.Error())
		return
	}

	var total, accepted, pending, availability int
	if err := h.db.QueryRow(context.Background(), `SELECT COUNT(1) FROM trip_members WHERE trip_id = $1`, tripID).Scan(&total); err != nil {
		utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", err.Error())
		return
	}
	if err := h.db.QueryRow(context.Background(), `SELECT COUNT(1) FROM trip_members WHERE trip_id = $1 AND status = 'accepted'`, tripID).Scan(&accepted); err != nil {
		utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", err.Error())
		return
	}
	if err := h.db.QueryRow(context.Background(), `SELECT COUNT(1) FROM trip_members WHERE trip_id = $1 AND status = 'pending'`, tripID).Scan(&pending); err != nil {
		utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", err.Error())
		return
	}
	if err := h.db.QueryRow(context.Background(), `SELECT COUNT(1) FROM trip_members WHERE trip_id = $1 AND availability_submitted = TRUE`, tripID).Scan(&availability); err != nil {
		utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", err.Error())
		return
	}

	if !isCreatorMember {
		var exists bool
		if err := h.db.QueryRow(context.Background(),
			`SELECT EXISTS(SELECT 1 FROM trip_members WHERE trip_id = $1 AND user_id = $2 AND LOWER(role) = 'creator')`,
			t.ID, requesterID,
		).Scan(&exists); err == nil && exists {
			isCreatorMember = true
		}
	}

	isCreator := requesterID == t.CreatorID || isCreatorMember
	log.Printf("TripDetail debug: t.CreatorID=%s requester=%s isCreatorMember=%v isCreator=%v", t.CreatorID.String(), requesterID.String(), isCreatorMember, isCreator)
	perms := dto.TripPermissions{
		CanEdit:         isCreator,
		CanDelete:       isCreator,
		CanInvite:       isCreator,
		CanManageBudget: isCreator,
	}

	resp := dto.TripDetailResponse{
		Trip: dto.TripDetailTrip{
			ID:          t.ID.String(),
			Name:        t.Name,
			Destination: t.Destination,
			Description: t.Description,
			StartDate:   time.Date(t.StartDate.Year(), t.StartDate.Month(), 1, 0, 0, 0, 0, t.StartDate.Location()).Format("2006-01-02"),
			EndDate:     time.Date(t.EndDate.Year(), t.EndDate.Month(), 1, 0, 0, 0, 0, t.EndDate.Location()).Format("2006-01-02"),
			TotalBudget: t.TotalBudget,
			Currency:    t.Currency,
			Status:      t.Status,
			CreatorID:   t.CreatorID.String(),
			CreatedAt:   t.CreatedAt.Format(time.RFC3339),
			UpdatedAt:   t.UpdatedAt.Format(time.RFC3339),
			// NEW
			Budget: dto.TripBudgetResponse{
				Food:      food,
				Hotel:     hotel,
				Shopping:  shopping,
				Transport: transport,
				Total:     t.TotalBudget,
			},
		},
		Members:     members,
		Permissions: perms,
		Stats: dto.TripStats{
			TotalMembers:            total,
			AcceptedMembers:         accepted,
			PendingInvitations:      pending,
			MembersWithAvailability: availability,
		},
	}
	utils.WriteJSONResponse(w, http.StatusOK, resp)
}

// UpdateTrip handles PUT/PATCH /api/trips/{trip_id}
// @Summary Update a trip
// @Tags trips
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param trip_id path string true "Trip ID"
// @Param payload body dto.UpdateTripRequest true "Update payload"
// @Success 200 {object} dto.CreateTripResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/trips/{trip_id} [put]
func (h *TripsHandler) UpdateTrip(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut && r.Method != http.MethodPatch {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	requesterID, ok := r.Context().Value("user_id").(uuid.UUID)
	if !ok {
		utils.WriteErrorResponse(w, http.StatusUnauthorized, "Unauthorized", "Invalid user context")
		return
	}

	path := cleanPath(r.URL.Path)
	idStr := strings.TrimPrefix(path, "/api/trips/")
	tripID, err := uuid.Parse(idStr)
	if err != nil {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Invalid trip id", "trip_id must be UUID")
		return
	}

	// ดึง trip ปัจจุบัน
	var cur models.Trip
	err = h.db.QueryRow(
		context.Background(),
		`SELECT id, name, destination, start_date, end_date, description, status, total_budget, currency, creator_id, created_at, updated_at
		   FROM trips
		  WHERE id = $1`,
		tripID,
	).Scan(
		&cur.ID,
		&cur.Name,
		&cur.Destination,
		&cur.StartDate,
		&cur.EndDate,
		&cur.Description,
		&cur.Status,
		&cur.TotalBudget,
		&cur.Currency,
		&cur.CreatorID,
		&cur.CreatedAt,
		&cur.UpdatedAt,
	)
	if err != nil {
		utils.WriteErrorResponse(w, http.StatusNotFound, "Not Found", "Trip not found")
		return
	}

	// ต้องเป็น creator (โดยตรง หรือเป็น member role=creator)
	if requesterID != cur.CreatorID {
		var exists bool
		if err := h.db.QueryRow(
			context.Background(),
			`SELECT EXISTS(
                 SELECT 1
                   FROM trip_members
                  WHERE trip_id = $1
                    AND user_id = $2
                    AND LOWER(role) = 'creator'
             )`,
			cur.ID, requesterID,
		).Scan(&exists); err != nil || !exists {
			utils.WriteErrorResponse(w, http.StatusForbidden, "Forbidden", "Only creator can update this trip")
			return
		}
	}

	// อ่าน request body
	var req dto.UpdateTripRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Invalid request data", "Malformed JSON body")
		return
	}

	// ----------- field ทั่วไป -----------
	name := cur.Name
	if req.Name != nil {
		name = strings.TrimSpace(*req.Name)
	}

	destination := cur.Destination
	if req.Destination != nil {
		destination = strings.TrimSpace(*req.Destination)
	}

	description := cur.Description
	if req.Description != nil {
		description = *req.Description
	}

	status := cur.Status
	if req.Status != nil {
		st := strings.ToLower(strings.TrimSpace(*req.Status))
		switch st {
		case "draft", "published", "cancelled":
			status = st
		default:
			utils.WriteErrorResponse(w, http.StatusBadRequest, "Validation error", "status must be draft, published, or cancelled")
			return
		}
	}

	// ----------- วันที่: ใช้ StartDate / EndDate (YYYY-MM-DD) -----------
	startDate := cur.StartDate
	if req.StartDate != nil {
		sd := strings.TrimSpace(*req.StartDate)
		t, err := time.Parse("2006-01-02", sd)
		if err != nil {
			utils.WriteErrorResponse(w, http.StatusBadRequest, "Validation error", "start_date must be YYYY-MM-DD")
			return
		}
		startDate = t
	}

	endDate := cur.EndDate
	if req.EndDate != nil {
		ed := strings.TrimSpace(*req.EndDate)
		t, err := time.Parse("2006-01-02", ed)
		if err != nil {
			utils.WriteErrorResponse(w, http.StatusBadRequest, "Validation error", "end_date must be YYYY-MM-DD")
			return
		}
		endDate = t
	}

	if endDate.Before(startDate) {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Validation error", "end_date cannot be before start_date")
		return
	}

	// ----------- ดึง budget เดิมจาก budget_categories -----------
	var curFood, curHotel, curShopping, curTransport float64
	err = h.db.QueryRow(
		context.Background(),
		`SELECT food, hotel, shopping, transport
           FROM budget_categories
          WHERE trip_id = $1 AND order_index = 1`,
		cur.ID,
	).Scan(&curFood, &curHotel, &curShopping, &curTransport)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			curFood, curHotel, curShopping, curTransport = 0, 0, 0, 0
		} else {
			utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", err.Error())
			return
		}
	}

	newFood := curFood
	if req.Food != nil {
		newFood = *req.Food
	}
	newHotel := curHotel
	if req.Hotel != nil {
		newHotel = *req.Hotel
	}
	newShopping := curShopping
	if req.Shopping != nil {
		newShopping = *req.Shopping
	}
	newTransport := curTransport
	if req.Transport != nil {
		newTransport = *req.Transport
	}

	if newFood < 0 || newHotel < 0 || newShopping < 0 || newTransport < 0 {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Validation error", "budget categories cannot be negative")
		return
	}

	// base = ของเดิมใน trips
	totalBudget := cur.TotalBudget

	// ถ้าส่ง total_budget มาโดยตรง
	if req.TotalBudget != nil {
		if *req.TotalBudget < 0 {
			utils.WriteErrorResponse(w, http.StatusBadRequest, "Validation error", "total_budget cannot be negative")
			return
		}
		totalBudget = *req.TotalBudget

		// ถ้าก่อนหน้าไม่มี breakdown เลย และ request รอบนี้ก็ไม่ได้ส่ง breakdown มาด้วย
		// → เอา totalBudget ลงที่ food ช่องเดียว (ไว้กันกรณี client เก่าใช้แค่ total_budget)
		if req.Food == nil && req.Hotel == nil && req.Shopping == nil && req.Transport == nil &&
			curFood == 0 && curHotel == 0 && curShopping == 0 && curTransport == 0 {
			newFood = totalBudget
			newHotel, newShopping, newTransport = 0, 0, 0
		}
	}

	// ถ้ามีส่ง breakdown มาอย่างน้อย 1 หมวด → ให้ totalBudget = sum(breakdown)
	if req.Food != nil || req.Hotel != nil || req.Shopping != nil || req.Transport != nil {
		totalBudget = newFood + newHotel + newShopping + newTransport
	}

	now := time.Now()

	// ----------- อัปเดต trips -----------
	_, err = h.db.Exec(
		context.Background(),
		`UPDATE trips
            SET name = $1,
                destination = $2,
                description = $3,
                start_date = $4,
                end_date = $5,
                status = $6,
                total_budget = $7,
                updated_at = $8
          WHERE id = $9`,
		name,
		destination,
		description,
		startDate,
		endDate,
		status,
		totalBudget,
		now,
		cur.ID,
	)
	if err != nil {
		utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", err.Error())
		return
	}

	// ----------- sync budget breakdown ไป budget_categories -----------
	_, err = h.db.Exec(
		context.Background(),
		`INSERT INTO budget_categories (trip_id, order_index, food, hotel, shopping, transport)
         VALUES ($1, 1, $2, $3, $4, $5)
         ON CONFLICT (trip_id, order_index)
         DO UPDATE SET
            food = EXCLUDED.food,
            hotel = EXCLUDED.hotel,
            shopping = EXCLUDED.shopping,
            transport = EXCLUDED.transport,
            updated_at = now()`,
		cur.ID,
		newFood,
		newHotel,
		newShopping,
		newTransport,
	)
	if err != nil {
		utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", err.Error())
		return
	}

	// ----------- สร้าง response -----------
	updated := dto.TripResponse{
		ID:          cur.ID.String(),
		Name:        name,
		Destination: destination,
		StartDate:   startDate.Format("2006-01-02"),
		EndDate:     endDate.Format("2006-01-02"),
		Description: description,
		Status:      status,
		TotalBudget: totalBudget,
		Currency:    cur.Currency,
		CreatorID:   cur.CreatorID.String(),
		CreatedAt:   cur.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   now.Format(time.RFC3339),
	}

	// ถ้าคุณเพิ่ม dto.TripBudgetResponse และ field Budget ใน TripResponse แล้ว
	// ให้เติมตรงนี้ได้เลย:
	// updated.Budget = dto.TripBudgetResponse{
	// 	Food:      newFood,
	// 	Hotel:     newHotel,
	// 	Shopping:  newShopping,
	// 	Transport: newTransport,
	// 	Total:     totalBudget,
	// }

	utils.WriteJSONResponse(w, http.StatusOK, dto.CreateTripResponse{Trip: updated})
}

// GetTripBudget handles GET /api/trips/{trip_id}/budget
// @Summary Get trip budget
// @Description Get total budget and category breakdown for a trip. Any member of the trip can view this.
// @Tags trips
// @Produce json
// @Security BearerAuth
// @Param trip_id path string true "Trip ID"
// @Success 200 {object} dto.GetTripBudgetResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/trips/{trip_id}/budget [get]
func (h *TripsHandler) GetTripBudget(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// เอา user_id จาก context (middleware auth ใส่ไว้ให้แล้ว)
	userID, ok := r.Context().Value("user_id").(uuid.UUID)
	if !ok {
		utils.WriteErrorResponse(w, http.StatusUnauthorized, "Unauthorized", "Invalid user context")
		return
	}

	// path: /api/trips/{trip_id}/budget
	path := cleanPath(r.URL.Path) // ตัด trailing / ถ้ามี
	trimmed := strings.TrimPrefix(path, "/api/trips/")
	trimmed = strings.TrimSuffix(trimmed, "/budget")

	tripID, err := uuid.Parse(trimmed)
	if err != nil {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Invalid trip id", "trip_id must be UUID")
		return
	}

	// ---------- เช็กสิทธิ์: ต้องเป็น creator หรือ member ของทริป ----------
	var allowed bool
	err = h.db.QueryRow(
		context.Background(),
		`SELECT EXISTS(
             SELECT 1 FROM trips
              WHERE id = $1 AND creator_id = $2
         ) OR EXISTS(
             SELECT 1 FROM trip_members
              WHERE trip_id = $1 AND user_id = $2
         )`,
		tripID, userID,
	).Scan(&allowed)
	if err != nil {
		utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", err.Error())
		return
	}
	if !allowed {
		utils.WriteErrorResponse(w, http.StatusForbidden, "Forbidden", "You are not a member of this trip")
		return
	}

	// ---------- ดึง total_budget จาก trips ----------
	var totalBudget float64
	err = h.db.QueryRow(
		context.Background(),
		`SELECT total_budget
           FROM trips
          WHERE id = $1`,
		tripID,
	).Scan(&totalBudget)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			utils.WriteErrorResponse(w, http.StatusNotFound, "Not Found", "Trip not found")
			return
		}
		utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", err.Error())
		return
	}

	// ---------- ดึง breakdown จาก budget_categories ----------
	var food, hotel, shopping, transport float64
	err = h.db.QueryRow(
		context.Background(),
		`SELECT food, hotel, shopping, transport
           FROM budget_categories
          WHERE trip_id = $1 AND order_index = 1`,
		tripID,
	).Scan(&food, &hotel, &shopping, &transport)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// ไม่มี row → ถือว่า 0 ทุกหมวด
			food, hotel, shopping, transport = 0, 0, 0, 0
		} else {
			utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", err.Error())
			return
		}
	}

	resp := dto.GetTripBudgetResponse{
		Budget: dto.TripBudgetResponse{
			Food:      food,
			Hotel:     hotel,
			Shopping:  shopping,
			Transport: transport,
			Total:     totalBudget,
		},
	}

	utils.WriteJSONResponse(w, http.StatusOK, resp)
}

// DeleteTrip handles DELETE /api/trips/{trip_id}
// @Summary Delete a trip
// @Tags trips
// @Produce json
// @Security BearerAuth
// @Param trip_id path string true "Trip ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/trips/{trip_id} [delete]
func (h *TripsHandler) DeleteTrip(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	requesterID, ok := r.Context().Value("user_id").(uuid.UUID)
	if !ok {
		utils.WriteErrorResponse(w, http.StatusUnauthorized, "Unauthorized", "Invalid user context")
		return
	}

	path := cleanPath(r.URL.Path)
	idStr := strings.TrimPrefix(path, "/api/trips/")
	tripID, err := uuid.Parse(idStr)
	if err != nil {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Invalid trip id", "trip_id must be UUID")
		return
	}

	var creatorID uuid.UUID
	if err := h.db.QueryRow(context.Background(), `SELECT creator_id FROM trips WHERE id = $1`, tripID).Scan(&creatorID); err != nil {
		utils.WriteErrorResponse(w, http.StatusNotFound, "Not Found", "Trip not found")
		return
	}

	if requesterID != creatorID {
		var exists bool
		if err := h.db.QueryRow(context.Background(),
			`SELECT EXISTS(SELECT 1 FROM trip_members WHERE trip_id = $1 AND user_id = $2 AND LOWER(role) = 'creator')`,
			tripID, requesterID,
		).Scan(&exists); err != nil || !exists {
			utils.WriteErrorResponse(w, http.StatusForbidden, "Forbidden", "Only creator can delete this trip")
			return
		}
	}

	if _, err := h.db.Exec(context.Background(), `DELETE FROM trips WHERE id = $1`, tripID); err != nil {
		utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", err.Error())
		return
	}

	utils.WriteJSONResponse(w, http.StatusOK, map[string]string{"message": "Trip deleted successfully"})
}

//
// ===================== FR3: Invitations & Membership =====================
//

// InviteMembers handles POST /api/trips/{trip_id}/invitations
// @Summary Generate invitation link for a trip
// @Description Generate a shareable invitation link for a trip. No request body required.
// @Tags trips
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param trip_id path string true "Trip ID"
// @Success 200 {object} dto.TripInviteResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/trips/{trip_id}/invitations [post]
func (h *TripsHandler) InviteMembers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	requesterID, ok := r.Context().Value("user_id").(uuid.UUID)
	if !ok {
		utils.WriteErrorResponse(w, http.StatusUnauthorized, "Unauthorized", "Invalid user context")
		return
	}

	path := cleanPath(r.URL.Path) // /api/trips/{trip_id}/invitations
	rest := strings.TrimPrefix(path, "/api/trips/")
	idx := strings.Index(rest, "/")
	if idx <= 0 || !strings.HasSuffix(path, "/invitations") {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Invalid path", "missing or invalid trip_id")
		return
	}
	tripIDStr := rest[:idx]
	tripID, err := uuid.Parse(tripIDStr)
	if err != nil {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Invalid trip id", "trip_id must be UUID")
		return
	}

	var creatorID uuid.UUID
	if err := h.db.QueryRow(r.Context(), `SELECT creator_id FROM trips WHERE id = $1`, tripID).Scan(&creatorID); err != nil {
		utils.WriteErrorResponse(w, http.StatusNotFound, "Not Found", "Trip not found")
		return
	}
	if requesterID != creatorID {
		var isCreatorMember bool
		if err := h.db.QueryRow(r.Context(),
			`SELECT EXISTS(SELECT 1 FROM trip_members WHERE trip_id = $1 AND user_id = $2 AND LOWER(role) = 'creator')`,
			tripID, requesterID,
		).Scan(&isCreatorMember); err != nil || !isCreatorMember {
			utils.WriteErrorResponse(w, http.StatusForbidden, "Forbidden", "Only creator can generate invitation link")
			return
		}
	}

	// Generate invitation token
	invitationToken, err := middleware.GenerateInvitationToken(tripID, &h.config.JWT)
	if err != nil {
		utils.WriteErrorResponse(w, http.StatusInternalServerError, "Failed to generate invitation token", err.Error())
		return
	}

	// Create invitation link (frontend URL + token)
	// You can configure this in config or use environment variable
	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "http://localhost:8081" // Default for development
	}
	invitationLink := fmt.Sprintf("%s/trips/%s/join?token=%s", frontendURL, tripID.String(), invitationToken)

	// Calculate expiration (30 days from now)
	expiresAt := time.Now().Add(30 * 24 * time.Hour)

	resp := dto.TripInviteResponse{
		InvitationLink: invitationLink,
		ExpiresAt:      expiresAt.UTC().Format(time.RFC3339),
		Message:        "Invitation link generated successfully. Share this link to invite members to your trip.",
	}

	utils.WriteJSONResponse(w, http.StatusOK, resp)
}

// JoinViaLink handles POST /api/trips/join
// @Summary Join a trip via invitation link
// @Tags trips
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param payload body dto.TripJoinViaLinkRequest true "Invitation token"
// @Success 200 {object} dto.TripJoinViaLinkResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 409 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/trips/join [post]
func (h *TripsHandler) JoinViaLink(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get authenticated user
	userID, ok := r.Context().Value("user_id").(uuid.UUID)
	if !ok {
		utils.WriteErrorResponse(w, http.StatusUnauthorized, "Unauthorized", "Invalid user context")
		return
	}

	var req dto.TripJoinViaLinkRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Invalid request data", "Malformed JSON body")
		return
	}

	if req.InvitationToken == "" {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Validation error", "invitation_token is required")
		return
	}

	// Validate invitation token
	claims, err := middleware.ValidateInvitationToken(req.InvitationToken, &h.config.JWT)
	if err != nil {
		utils.WriteErrorResponse(w, http.StatusUnauthorized, "Invalid invitation token", "The invitation link is invalid or has expired")
		return
	}

	tripID := claims.TripID
	ctx := r.Context()
	now := time.Now()

	// Check if trip exists
	var tripName, tripDestination string
	err = h.db.QueryRow(ctx,
		`SELECT name, destination FROM trips WHERE id = $1`,
		tripID,
	).Scan(&tripName, &tripDestination)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			utils.WriteErrorResponse(w, http.StatusNotFound, "Not Found", "Trip not found")
		} else {
			utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", err.Error())
		}
		return
	}

	// Check if user is already a member
	var curStatus string
	var curRole string
	err = h.db.QueryRow(ctx,
		`SELECT role, status FROM trip_members WHERE trip_id = $1 AND user_id = $2`,
		tripID, userID,
	).Scan(&curRole, &curStatus)

	if err == nil {
		// User is already a member
		switch strings.ToLower(curStatus) {
		case "accepted":
			utils.WriteErrorResponse(w, http.StatusConflict, "Conflict", "You are already a member of this trip")
			return
		case "pending":
			// Update to accepted
			_, err = h.db.Exec(ctx,
				`UPDATE trip_members
				   SET status = 'accepted', joined_at = $3
				 WHERE trip_id = $1 AND user_id = $2`,
				tripID, userID, now,
			)
			if err != nil {
				utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", err.Error())
				return
			}
		default:
			// Re-open as accepted
			_, err = h.db.Exec(ctx,
				`UPDATE trip_members
				   SET status = 'accepted', joined_at = $3
				 WHERE trip_id = $1 AND user_id = $2`,
				tripID, userID, now,
			)
			if err != nil {
				utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", err.Error())
				return
			}
		}
	} else if errors.Is(err, pgx.ErrNoRows) {
		// User is not a member, insert as accepted
		// Get creator ID for invited_by
		var creatorID uuid.UUID
		err = h.db.QueryRow(ctx,
			`SELECT creator_id FROM trips WHERE id = $1`,
			tripID,
		).Scan(&creatorID)
		if err != nil {
			utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", err.Error())
			return
		}

		_, err = h.db.Exec(ctx,
			`INSERT INTO trip_members (trip_id, user_id, role, status, invited_by, invited_at, joined_at, availability_submitted)
			 VALUES ($1, $2, 'member', 'accepted', $3, $4, $4, FALSE)
			 ON CONFLICT (trip_id, user_id) DO UPDATE
			 SET status = 'accepted', joined_at = $4`,
			tripID, userID, creatorID, now,
		)
		if err != nil {
			utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", err.Error())
			return
		}
		curRole = "member"
	} else {
		utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", err.Error())
		return
	}

	// แจ้ง creator ว่ามีสมาชิก join
	{
		ctx := r.Context()
		var creatorID uuid.UUID
		_ = h.db.QueryRow(ctx, `SELECT creator_id FROM trips WHERE id=$1`, tripID).Scan(&creatorID)

		// ดึงชื่อผู้ใช้จาก profile
		userDisplayName := h.getUserDisplayName(ctx, userID)
		msg := fmt.Sprintf("%s has joined %s", userDisplayName, tripName)
		h.sendNoti(
			ctx,
			creatorID,
			TypeMemberJoined, // <- enum ใน noti ของคุณ
			"Member Joined Trip",
			&msg,
			map[string]any{
				"trip_id":           tripID.String(),
				"user_id":           userID.String(),
				"role":              curRole,
				"tripName":          tripName,
				"user_display_name": userDisplayName,
			},
			h.tripURL(tripID),
		)
	}

	resp := dto.TripJoinViaLinkResponse{
		Message: "Successfully joined the trip",
	}
	resp.Trip.ID = tripID.String()
	resp.Trip.Name = tripName
	resp.Trip.Destination = tripDestination
	resp.Member.UserID = userID.String()
	resp.Member.Role = curRole
	resp.Member.Status = "accepted"
	resp.Member.JoinedAt = now.UTC().Format(time.RFC3339)

	utils.WriteJSONResponse(w, http.StatusOK, resp)
}

// ListInvitations handles GET /api/trips/{trip_id}/invitations
// @Summary List invitations of a trip (creator only)
// @Tags trips
// @Produce json
// @Security BearerAuth
// @Param trip_id path string true "Trip ID"
// @Success 200 {object} dto.TripInvitationsListResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/trips/{trip_id}/invitations [get]
func (h *TripsHandler) ListInvitations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	requesterID, ok := r.Context().Value("user_id").(uuid.UUID)
	if !ok {
		utils.WriteErrorResponse(w, http.StatusUnauthorized, "Unauthorized", "Invalid user context")
		return
	}

	path := cleanPath(r.URL.Path) // /api/trips/{trip_id}/invitations
	rest := strings.TrimPrefix(path, "/api/trips/")
	slash := strings.Index(rest, "/")
	if slash <= 0 || !strings.HasSuffix(path, "/invitations") {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Invalid path", "missing or invalid trip_id")
		return
	}
	tripIDStr := rest[:slash]
	tripID, err := uuid.Parse(tripIDStr)
	if err != nil {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Invalid trip id", "trip_id must be UUID")
		return
	}

	ctx := r.Context()

	var creatorID uuid.UUID
	if err := h.db.QueryRow(ctx, `SELECT creator_id FROM trips WHERE id = $1`, tripID).Scan(&creatorID); err != nil {
		utils.WriteErrorResponse(w, http.StatusNotFound, "Not Found", "Trip not found")
		return
	}
	if requesterID != creatorID {
		var isCreatorMember bool
		if err := h.db.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM trip_members WHERE trip_id = $1 AND user_id = $2 AND LOWER(role) = 'creator')`,
			tripID, requesterID,
		).Scan(&isCreatorMember); err != nil || !isCreatorMember {
			utils.WriteErrorResponse(w, http.StatusForbidden, "Forbidden", "Only creator can view invitations")
			return
		}
	}

	rows, err := h.db.Query(ctx, `
		SELECT
			tm.user_id,
			p.username,
			p.display_name,
			p.avatar_url,
			tm.status,
			COALESCE(tm.invited_by::text, '') AS invited_by,
			tm.invited_at
		FROM trip_members tm
		LEFT JOIN profiles p ON p.user_id = tm.user_id
		WHERE tm.trip_id = $1
		  AND tm.status IN ('pending','accepted','declined')
		ORDER BY tm.invited_at DESC NULLS LAST, tm.user_id ASC
	`, tripID)
	if err != nil {
		utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", err.Error())
		return
	}
	defer rows.Close()

	invites := make([]dto.TripInvitationListItem, 0, 16)
	for rows.Next() {
		var (
			uid                              uuid.UUID
			username, displayName, avatarURL *string
			status                           string
			invitedByStr                     string
			invitedAt                        *time.Time
		)
		if err := rows.Scan(&uid, &username, &displayName, &avatarURL, &status, &invitedByStr, &invitedAt); err != nil {
			utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", err.Error())
			return
		}

		var invitedAtStr *string
		if invitedAt != nil {
			s := invitedAt.UTC().Format(time.RFC3339)
			invitedAtStr = &s
		}
		if invitedByStr == "" {
			invitedByStr = creatorID.String()
		}

		invites = append(invites, dto.TripInvitationListItem{
			UserID:      uid.String(),
			Username:    username,
			DisplayName: displayName,
			AvatarURL:   avatarURL,
			Status:      status,
			InvitedBy:   invitedByStr,
			InvitedAt:   invitedAtStr,
		})
	}
	if err := rows.Err(); err != nil {
		utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", err.Error())
		return
	}

	var pending, accepted, declined int
	if err := h.db.QueryRow(ctx,
		`SELECT 
			COALESCE(SUM(CASE WHEN status='pending'  THEN 1 ELSE 0 END),0),
			COALESCE(SUM(CASE WHEN status='accepted' THEN 1 ELSE 0 END),0),
			COALESCE(SUM(CASE WHEN status='declined' THEN 1 ELSE 0 END),0)
		 FROM trip_members WHERE trip_id=$1`, tripID,
	).Scan(&pending, &accepted, &declined); err != nil {
		utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", err.Error())
		return
	}
	stats := dto.TripInvitationsStats{
		Total:    pending + accepted + declined,
		Pending:  pending,
		Accepted: accepted,
		Declined: declined,
	}

	utils.WriteJSONResponse(w, http.StatusOK, dto.TripInvitationsListResponse{
		Invitations: invites,
		Stats:       stats,
	})
}

// LeaveTrip handles POST /api/trips/{trip_id}/leave
// @Summary Leave a trip (for accepted members)
// @Tags trips
// @Produce json
// @Security BearerAuth
// @Param trip_id path string true "Trip ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 409 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/trips/{trip_id}/leave [post]
func (h *TripsHandler) LeaveTrip(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, ok := r.Context().Value("user_id").(uuid.UUID)
	if !ok {
		utils.WriteErrorResponse(w, http.StatusUnauthorized, "Unauthorized", "Invalid user context")
		return
	}

	path := cleanPath(r.URL.Path) // /api/trips/{trip_id}/leave
	rest := strings.TrimPrefix(path, "/api/trips/")
	slash := strings.Index(rest, "/")
	if slash <= 0 || !strings.HasSuffix(path, "/leave") {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Invalid path", "missing or invalid trip_id")
		return
	}
	tripIDStr := rest[:slash]
	tripID, err := uuid.Parse(tripIDStr)
	if err != nil {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Invalid trip id", "trip_id must be UUID")
		return
	}

	ctx := r.Context()

	var creatorID uuid.UUID
	if err := h.db.QueryRow(ctx, `SELECT creator_id FROM trips WHERE id = $1`, tripID).Scan(&creatorID); err != nil {
		utils.WriteErrorResponse(w, http.StatusNotFound, "Not Found", "Trip not found")
		return
	}

	if userID == creatorID {
		utils.WriteErrorResponse(w, http.StatusForbidden, "Forbidden", "Creator cannot leave their own trip")
		return
	}

	var role, status string
	err = h.db.QueryRow(ctx,
		`SELECT role, status FROM trip_members WHERE trip_id = $1 AND user_id = $2`,
		tripID, userID,
	).Scan(&role, &status)
	if err != nil {
		utils.WriteErrorResponse(w, http.StatusNotFound, "Not Found", "You are not invited to this trip")
		return
	}

	if strings.ToLower(status) != "accepted" {
		utils.WriteErrorResponse(w, http.StatusConflict, "Conflict", "You are not an active member of this trip")
		return
	}

	cmd, err := h.db.Exec(ctx,
		`DELETE FROM trip_members
       WHERE trip_id = $1 AND user_id = $2 AND status = 'accepted'`,
		tripID, userID,
	)
	if err != nil {
		utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", err.Error())
		return
	}
	if cmd.RowsAffected() == 0 {
		utils.WriteErrorResponse(w, http.StatusConflict, "Conflict", "You are not an active member of this trip")
		return
	}

	// แจ้ง creator ว่าสมาชิกออกจากทริป
	{
		ctx := r.Context()
		var creatorID uuid.UUID
		var tName string
		_ = h.db.QueryRow(ctx, `SELECT creator_id, name FROM trips WHERE id=$1`, tripID).Scan(&creatorID, &tName)

		// ดึงชื่อผู้ใช้จาก profile
		userDisplayName := h.getUserDisplayName(ctx, userID)
		msg := fmt.Sprintf("%s has left %s", userDisplayName, tName)
		h.sendNoti(
			ctx,
			creatorID,
			TypeMemberLeft, // <- enum มีอยู่แล้ว
			"Member Left Trip",
			&msg,
			map[string]any{
				"trip_id":           tripID.String(),
				"user_id":           userID.String(),
				"tripName":          tName,
				"user_display_name": userDisplayName,
			},
			h.tripURL(tripID),
		)
	}

	utils.WriteJSONResponse(w, http.StatusOK, map[string]string{
		"message": "You have left the trip successfully",
	})
}

// RemoveMember handles DELETE /api/trips/{trip_id}/members/{user_id}
// @Summary Remove a member from a trip (creator only)
// @Tags trips
// @Produce json
// @Security BearerAuth
// @Param trip_id path string true "Trip ID"
// @Param user_id path string true "User ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 409 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/trips/{trip_id}/members/{user_id} [delete]
func (h *TripsHandler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	requesterID, ok := r.Context().Value("user_id").(uuid.UUID)
	if !ok {
		utils.WriteErrorResponse(w, http.StatusUnauthorized, "Unauthorized", "Invalid user context")
		return
	}

	path := cleanPath(r.URL.Path) // /api/trips/{trip_id}/members/{user_id}
	rest := strings.TrimPrefix(path, "/api/trips/")
	slash := strings.Index(rest, "/")
	if slash <= 0 {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Invalid path", "missing trip_id")
		return
	}
	tripIDStr := rest[:slash]
	rest2 := rest[slash+1:] // members/{user_id}
	if !strings.HasPrefix(rest2, "members/") {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Invalid path", "missing members segment")
		return
	}
	userIDStr := strings.TrimPrefix(rest2, "members/")

	tripID, err := uuid.Parse(tripIDStr)
	if err != nil {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Invalid trip id", "trip_id must be UUID")
		return
	}
	targetUserID, err := uuid.Parse(userIDStr)
	if err != nil {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Invalid user id", "user_id must be UUID")
		return
	}

	ctx := r.Context()

	var creatorID uuid.UUID
	if err := h.db.QueryRow(ctx, `SELECT creator_id FROM trips WHERE id = $1`, tripID).Scan(&creatorID); err != nil {
		utils.WriteErrorResponse(w, http.StatusNotFound, "Not Found", "Trip not found")
		return
	}

	if requesterID != creatorID {
		var isCreatorMember bool
		if err := h.db.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM trip_members WHERE trip_id = $1 AND user_id = $2 AND LOWER(role) = 'creator')`,
			tripID, requesterID,
		).Scan(&isCreatorMember); err != nil || !isCreatorMember {
			utils.WriteErrorResponse(w, http.StatusForbidden, "Forbidden", "Only creator can remove a member")
			return
		}
	}

	if targetUserID == creatorID {
		utils.WriteErrorResponse(w, http.StatusForbidden, "Forbidden", "Cannot remove the trip creator")
		return
	}

	var role, status string
	err = h.db.QueryRow(ctx,
		`SELECT role, status FROM trip_members WHERE trip_id = $1 AND user_id = $2`,
		tripID, targetUserID,
	).Scan(&role, &status)
	if err != nil {
		utils.WriteErrorResponse(w, http.StatusNotFound, "Not Found", "Member not found in this trip")
		return
	}

	if strings.ToLower(status) == "removed" {
		utils.WriteErrorResponse(w, http.StatusConflict, "Conflict", "Member already removed")
		return
	}

	cmd, err := h.db.Exec(ctx,
		`DELETE FROM trip_members
       WHERE trip_id = $1 AND user_id = $2`,
		tripID, targetUserID,
	)
	if err != nil {
		utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", err.Error())
		return
	}
	if cmd.RowsAffected() == 0 {
		utils.WriteErrorResponse(w, http.StatusNotFound, "Not Found", "Member not found in this trip")
		return
	}

	// แจ้งผู้ถูกลบว่าโดนถอดออกจากทริป
	{
		ctx := r.Context()
		var tName string
		_ = h.db.QueryRow(ctx, `SELECT name FROM trips WHERE id=$1`, tripID).Scan(&tName)

		msg := fmt.Sprintf("You were removed from %s", tName)
		h.sendNoti(
			ctx,
			targetUserID,
			TypeTripUpdate, // ใช้ประเภทอัปเดตทริป
			"You Were Removed from Trip",
			&msg,
			map[string]any{
				"trip_id":  tripID.String(),
				"tripName": tName,
				"event":    "removed",
			},
			h.tripURL(tripID),
		)
	}

	utils.WriteJSONResponse(w, http.StatusOK, map[string]string{
		"message": "Member removed successfully",
	})
}

// TripDates godoc
// @Summary      Get trip's exact date range (for availability picking)
// @Description  ส่งช่วงวันที่ตรงตาม start_date ถึง end_date ของทริป และจำนวนวันรวมแบบ inclusive
// @Tags         trips
// @Produce      json
// @Security     BearerAuth
// @Param        trip_id path string true "Trip ID"
// @Success      200 {object} dto.TripDatesResponse
// @Failure      400 {object} dto.ErrorResponse
// @Failure      401 {object} dto.ErrorResponse
// @Failure      403 {object} dto.ErrorResponse
// @Failure      404 {object} dto.ErrorResponse
// @Failure      500 {object} dto.ErrorResponse
// @Router       /api/trips/{trip_id}/dates [get]
func (h *TripsHandler) TripDates(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// auth
	requesterID, ok := r.Context().Value("user_id").(uuid.UUID)
	if !ok {
		utils.WriteErrorResponse(w, http.StatusUnauthorized, "Unauthorized", "Invalid user context")
		return
	}

	// parse /api/trips/{trip_id}/dates
	path := strings.TrimRight(r.URL.Path, "/")
	rest := strings.TrimPrefix(path, "/api/trips/")
	slash := strings.Index(rest, "/")
	if slash <= 0 || !strings.HasSuffix(path, "/dates") {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Invalid path", "missing or invalid trip_id")
		return
	}
	tripIDStr := rest[:slash]
	tripID, err := uuid.Parse(tripIDStr)
	if err != nil {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Invalid trip id", "trip_id must be UUID")
		return
	}

	ctx := r.Context()

	// load trip
	var (
		id        uuid.UUID
		name      string
		startDate time.Time
		endDate   time.Time
	)
	err = h.db.QueryRow(ctx, `
		SELECT id, name, start_date, end_date
		  FROM trips
		 WHERE id = $1
		 LIMIT 1
	`, tripID).Scan(&id, &name, &startDate, &endDate)
	if err != nil {
		utils.WriteErrorResponse(w, http.StatusNotFound, "Not Found", "Trip not found")
		return
	}

	// basic validation (กันข้อมูลเพี้ยน)
	if endDate.Before(startDate) {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Validation error", "trip end_date cannot be before start_date")
		return
	}

	// permission: ต้องเป็น creator หรือมีแถวใน trip_members (สถานะใดก็ได้)
	var allowed bool
	if err := h.db.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM trips WHERE id = $1 AND creator_id = $2)
		    OR EXISTS (SELECT 1 FROM trip_members WHERE trip_id = $1 AND user_id = $2)
	`, tripID, requesterID).Scan(&allowed); err != nil || !allowed {
		utils.WriteErrorResponse(w, http.StatusForbidden, "Forbidden", "Only trip members can view date range")
		return
	}

	// exact range = start_date .. end_date (inclusive)
	start := time.Date(startDate.Year(), startDate.Month(), startDate.Day(), 0, 0, 0, 0, time.UTC)
	end := time.Date(endDate.Year(), endDate.Month(), endDate.Day(), 0, 0, 0, 0, time.UTC)
	total := int(end.Sub(start).Hours()/24) + 1

	resp := dto.TripDatesResponse{
		Trip: dto.TripDatesTrip{
			ID:        id.String(),
			Name:      name,
			StartDate: start.Format("2006-01-02"),
			EndDate:   end.Format("2006-01-02"),
		},
		DateRange: dto.TripDateRange{
			StartDate:  start.Format("2006-01-02"),
			EndDate:    end.Format("2006-01-02"),
			TotalDates: total,
		},
	}
	utils.WriteJSONResponse(w, http.StatusOK, resp)
}

// SaveAvailability godoc
// @Summary      Save my availability for a trip (one row per day per user)
// @Description  บันทึกวันว่างของผู้ใช้ในทริป โดยรูปแบบ normalized: หนึ่งแถว/หนึ่งวัน/หนึ่งคน/หนึ่งทริป
// @Tags         trips
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        trip_id path string true "Trip ID"
// @Param        payload body dto.TripAvailabilityRequest true "Availability payload"
// @Success      200 {object} dto.TripAvailabilityResponse
// @Failure      400 {object} dto.ErrorResponse
// @Failure      401 {object} dto.ErrorResponse
// @Failure      403 {object} dto.ErrorResponse
// @Failure      404 {object} dto.ErrorResponse
// @Failure      409 {object} dto.ErrorResponse
// @Failure      500 {object} dto.ErrorResponse
// @Router       /api/trips/{trip_id}/availability [post]
func (h *TripsHandler) SaveAvailability(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// auth
	userID, ok := r.Context().Value("user_id").(uuid.UUID)
	if !ok {
		utils.WriteErrorResponse(w, http.StatusUnauthorized, "Unauthorized", "Invalid user context")
		return
	}

	// parse: /api/trips/{trip_id}/availability
	path := strings.TrimRight(r.URL.Path, "/")
	rest := strings.TrimPrefix(path, "/api/trips/")
	slash := strings.Index(rest, "/")
	if slash <= 0 || !strings.HasSuffix(path, "/availability") {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Invalid path", "missing or invalid trip_id")
		return
	}
	tripIDStr := rest[:slash]
	tripID, err := uuid.Parse(tripIDStr)
	if err != nil {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Invalid trip id", "trip_id must be UUID")
		return
	}

	ctx := r.Context()

	// โหลดช่วงวันของทริป
	var (
		tStart time.Time
		tEnd   time.Time
		tName  string
	)
	if err := h.db.QueryRow(ctx,
		`SELECT start_date, end_date, name FROM trips WHERE id = $1`,
		tripID,
	).Scan(&tStart, &tEnd, &tName); err != nil {
		utils.WriteErrorResponse(w, http.StatusNotFound, "Not Found", "Trip not found")
		return
	}
	if tEnd.Before(tStart) {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Validation error", "trip end_date cannot be before start_date")
		return
	}

	// สิทธิ์: ต้องเป็นสมาชิกทริป (สถานะใดก็ได้) หรือ creator
	var allowed bool
	if err := h.db.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM trips WHERE id = $1 AND creator_id = $2)
		    OR EXISTS (SELECT 1 FROM trip_members WHERE trip_id = $1 AND user_id = $2)
	`, tripID, userID).Scan(&allowed); err != nil || !allowed {
		utils.WriteErrorResponse(w, http.StatusForbidden, "Forbidden", "Only trip members can submit availability")
		return
	}

	// decode body และดัก unknown fields
	var req dto.TripAvailabilityRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}
	if len(req.Dates) == 0 {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Validation error", "dates is required and must not be empty")
		return
	}

	// เตรียม helper
	start := dateOnlyUTC(tStart)
	end := dateOnlyUTC(tEnd)
	total := daysInclusive(start, end) // จำนวนวันที่เป็นไปได้ทั้งหมดในทริป

	// แปลง/validate วันที่ที่ส่งมา
	uniq := make(map[time.Time]struct{}, len(req.Dates))
	validDates := make([]time.Time, 0, len(req.Dates))

	for _, s := range req.Dates {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		// รองรับรูปแบบ YYYY-MM-DD เท่านั้น เพื่อความชัดเจน
		d, err := time.ParseInLocation("2006-01-02", s, time.UTC)
		if err != nil {
			utils.WriteErrorResponse(w, http.StatusBadRequest, "Validation error", "dates must be in YYYY-MM-DD format")
			return
		}
		d = dateOnlyUTC(d)

		// ต้องอยู่ในช่วงทริป
		if d.Before(start) || d.After(end) {
			utils.WriteErrorResponse(w, http.StatusBadRequest, "Validation error", "date out of trip range: "+s)
			return
		}
		// dedup
		if _, seen := uniq[d]; seen {
			continue
		}
		uniq[d] = struct{}{}
		validDates = append(validDates, d)
	}

	// ถ้าไม่มีอะไรเหลือหลัง dedup
	if len(validDates) == 0 {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Validation error", "no valid dates to save")
		return
	}

	// NOTE: ตาราง availabilities มีคอลัมน์ status เป็น USER-DEFINED NOT NULL
	// สมมุติ enum มีค่า 'free'|'flexible'|'busy' (ปรับได้)
	const availStatusFree = "free"

	tx, err := h.db.Begin(ctx)
	if err != nil {
		utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", err.Error())
		return
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// ลบข้อมูลเดิมของ user นี้ในทริปนี้ (เพื่อ idempotent)
	if _, err := tx.Exec(ctx,
		`DELETE FROM availabilities
		  WHERE trip_id = $1 AND user_id = $2`,
		tripID, userID,
	); err != nil {
		utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", err.Error())
		return
	}

	// ใส่ใหม่แบบ bulk ผ่าน UNNEST
	// เตรียม arrays
	dateArr := make([]time.Time, 0, len(validDates))
	statusArr := make([]string, 0, len(validDates))
	for _, d := range validDates {
		dateArr = append(dateArr, d)
		statusArr = append(statusArr, availStatusFree)
	}

	// INSERT USING UNNEST
	// หมายเหตุ: ถ้ามีข้อกำหนด unique (trip_id, user_id, date) ให้สร้าง unique index ไว้ใน DB
	_, err = tx.Exec(ctx, `
		INSERT INTO availabilities (trip_id, user_id, date, status)
		SELECT $1, $2, d::date, s::availability_status
		  FROM UNNEST($3::date[], $4::text[]) AS t(d, s)
	`, tripID, userID, dateArr, statusArr)
	if err != nil {
		// ถ้า enum ชื่อไม่ใช่ availability_status ให้ปรับ type cast ให้ถูกกับ enum จริงใน DB
		// เช่น s::text แล้วคอลัมน์บังคับ cast เอง (ถ้า enum ต่างชื่อ ให้เอา "::availability_status" ออก)
		utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", err.Error())
		return
	}

	// อัปเดต trip_members.availability_submitted = true (ถ้ามีแถว)
	_, _ = tx.Exec(ctx, `
		UPDATE trip_members
		   SET availability_submitted = TRUE
		 WHERE trip_id = $1 AND user_id = $2
	`, tripID, userID)

	if err := tx.Commit(ctx); err != nil {
		utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", err.Error())
		return
	}

	// แจ้ง creator ว่าสมาชิกส่งวันว่างแล้ว
	{
		ctx := r.Context()
		var creatorID uuid.UUID
		var tName string
		_ = h.db.QueryRow(ctx, `SELECT creator_id, name FROM trips WHERE id=$1`, tripID).Scan(&creatorID, &tName)

		// ดึงชื่อผู้ใช้จาก profile
		userDisplayName := h.getUserDisplayName(ctx, userID)
		msg := fmt.Sprintf("%s create availability for %s (%d days)", userDisplayName, tName, len(validDates))
		h.sendNoti(
			ctx,
			creatorID,
			TypeAvailability, // enum: availability_updated
			"Created Availability",
			&msg,
			map[string]any{
				"trip_id":           tripID.String(),
				"user_id":           userID.String(),
				"submitted_days":    len(validDates),
				"tripName":          tName,
				"user_display_name": userDisplayName,
			},
			h.tripURL(tripID),
		)
	}

	resp := dto.TripAvailabilityResponse{
		Message: "Availability saved successfully",
		Summary: dto.TripAvailabilitySummary{
			TotalDates:     total,
			SubmittedDates: len(validDates),
		},
	}
	utils.WriteJSONResponse(w, http.StatusOK, resp)
}

// GetMyAvailability godoc
// @Summary      Get my availability dates for a trip
// @Description  คืนรายการวันที่ผู้ใช้ (me) ทำเครื่องหมายว่าว่างในทริป พร้อมสรุปจำนวนวันทั้งหมดของทริป/จำนวนที่ส่งมา
// @Tags         trips
// @Produce      json
// @Security     BearerAuth
// @Param        trip_id path string true "Trip ID"
// @Success      200 {object} dto.TripMyAvailabilityResponse
// @Failure      400 {object} dto.ErrorResponse
// @Failure      401 {object} dto.ErrorResponse
// @Failure      403 {object} dto.ErrorResponse
// @Failure      404 {object} dto.ErrorResponse
// @Failure      500 {object} dto.ErrorResponse
// @Router       /api/trips/{trip_id}/availability/me [get]
func (h *TripsHandler) GetMyAvailability(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// auth
	userID, ok := r.Context().Value("user_id").(uuid.UUID)
	if !ok {
		utils.WriteErrorResponse(w, http.StatusUnauthorized, "Unauthorized", "Invalid user context")
		return
	}

	// parse: /api/trips/{trip_id}/availability/me
	path := strings.TrimRight(r.URL.Path, "/")
	rest := strings.TrimPrefix(path, "/api/trips/")
	slash := strings.Index(rest, "/")
	if slash <= 0 || !strings.HasSuffix(path, "/availability/me") {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Invalid path", "missing or invalid trip_id")
		return
	}
	tripIDStr := rest[:slash]
	tripID, err := uuid.Parse(tripIDStr)
	if err != nil {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Invalid trip id", "trip_id must be UUID")
		return
	}

	ctx := r.Context()

	// โหลดช่วงวันของทริป (คำนวณ total_dates)
	var tStart, tEnd time.Time
	if err := h.db.QueryRow(ctx,
		`SELECT start_date, end_date FROM trips WHERE id = $1`,
		tripID,
	).Scan(&tStart, &tEnd); err != nil {
		utils.WriteErrorResponse(w, http.StatusNotFound, "Not Found", "Trip not found")
		return
	}
	if tEnd.Before(tStart) {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Validation error", "trip end_date cannot be before start_date")
		return
	}
	start := dateOnlyUTC(tStart)
	end := dateOnlyUTC(tEnd)
	totalDates := daysInclusive(start, end)

	// Permission: ต้องเป็น creator หรือมีแถวใน trip_members (จะ pending/accepted ก็ให้ดูของตัวเองได้)
	var allowed bool
	if err := h.db.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM trips WHERE id = $1 AND creator_id = $2)
		    OR EXISTS (SELECT 1 FROM trip_members WHERE trip_id = $1 AND user_id = $2)
	`, tripID, userID).Scan(&allowed); err != nil || !allowed {
		utils.WriteErrorResponse(w, http.StatusForbidden, "Forbidden", "Only trip members can view availability")
		return
	}

	// ดึงวันที่ที่ user ทำไว้
	rows, err := h.db.Query(ctx, `
		SELECT date
		  FROM availabilities
		 WHERE trip_id = $1 AND user_id = $2
		 ORDER BY date ASC
	`, tripID, userID)
	if err != nil {
		utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", err.Error())
		return
	}
	defer rows.Close()

	items := make([]dto.TripAvailabilityDateItem, 0, 32)
	for rows.Next() {
		var d time.Time
		if err := rows.Scan(&d); err != nil {
			utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", err.Error())
			return
		}
		items = append(items, dto.TripAvailabilityDateItem{
			Date: dateOnlyUTC(d).Format("2006-01-02"),
		})
	}
	if err := rows.Err(); err != nil {
		utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", err.Error())
		return
	}

	resp := dto.TripMyAvailabilityResponse{
		Availability: items,
		Summary: dto.TripAvailabilitySummary{
			TotalDates:     totalDates,
			SubmittedDates: len(items),
		},
	}
	utils.WriteJSONResponse(w, http.StatusOK, resp)
}

// GenerateAvailablePeriods handles POST /api/trips/{trip_id}/availability/generate-periods
// @Summary Generate continuous periods where members are available (and persist to available_periods)
// @Description คำนวณช่วงวันที่สมาชิกว่างตามเกณฑ์ แล้วลบข้อมูลเดิมและบันทึกของใหม่ลงตาราง available_periods ทันที
// @Tags availability
// @Accept json
// @Produce json
// @Param trip_id path string true "Trip ID"
// @Param min_days body int false "Minimum days for a period (default: 1)"
// @Param min_availability_member body int false "Minimum number of available members (default: 1)"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/trips/{trip_id}/availability/generate-periods [post]
func (h *TripsHandler) GenerateAvailablePeriods(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// auth
	_, ok := r.Context().Value("user_id").(uuid.UUID)
	if !ok {
		utils.WriteErrorResponse(w, http.StatusUnauthorized, "Unauthorized", "Invalid user context")
		return
	}

	// parse: /api/trips/{trip_id}/availability/generate-periods
	rest := strings.TrimPrefix(r.URL.Path, "/api/trips/")
	slash := strings.Index(rest, "/")
	if slash <= 0 || !strings.HasSuffix(r.URL.Path, "/availability/generate-periods") {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Invalid path", "missing or invalid trip_id")
		return
	}
	tripIDStr := rest[:slash]
	tripID, err := uuid.Parse(tripIDStr)
	if err != nil {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Invalid trip id", "trip_id must be UUID")
		return
	}

	// decode payload
	var in struct {
		MinDays               int `json:"min_days"`
		MinAvailabilityMember int `json:"min_availability_member"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Invalid request data", "Malformed JSON body")
		return
	}
	if in.MinDays <= 0 {
		in.MinDays = 1
	}
	if in.MinAvailabilityMember <= 0 {
		// หากไม่ส่งมา ให้ใช้ 1 เป็นขั้นต่ำ
		in.MinAvailabilityMember = 1
	}

	ctx := r.Context()

	// 1) โหลดช่วงทริป + นับจำนวนสมาชิกทั้งหมด (เอาเฉพาะสถานะ accepted เป็นสมาชิกจริง)
	var (
		tStart, tEnd time.Time
		totalMembers int
		tName        string
	)
	if err := h.db.QueryRow(ctx,
		`SELECT start_date, end_date, name FROM trips WHERE id = $1`, tripID,
	).Scan(&tStart, &tEnd, &tName); err != nil {
		utils.WriteErrorResponse(w, http.StatusNotFound, "Not Found", "Trip not found")
		return
	}
	if !tEnd.After(tStart) && !tEnd.Equal(tStart) {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Bad Request", "trip date range is invalid")
		return
	}
	if err := h.db.QueryRow(ctx,
		`SELECT COALESCE(COUNT(1),0) FROM trip_members WHERE trip_id = $1 AND status = 'accepted'`,
		tripID,
	).Scan(&totalMembers); err != nil {
		utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", err.Error())
		return
	}
	if totalMembers == 0 {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Bad Request", "no accepted members in this trip")
		return
	}

	// 2) ดึง free_count รายวันในช่วงทริป (เรียงตามวันที่)
	//    หมายเหตุ: สมมติ status = 'free' เป็นตัวบอกวันว่าง (ตามที่เราใช้ใน 2.2)
	rows, err := h.db.Query(ctx, `
		WITH d AS (
			SELECT generate_series($1::date, $2::date, interval '1 day')::date AS d
		),
		f AS (
			SELECT a.date AS d, COUNT(*)::int AS free_count
			FROM availabilities a
			JOIN trip_members tm ON tm.trip_id = a.trip_id AND tm.user_id = a.user_id AND tm.status = 'accepted'
			WHERE a.trip_id = $3 AND a.status = 'free'
			GROUP BY a.date
		)
		SELECT d.d, COALESCE(f.free_count, 0) AS free_count
		FROM d
		LEFT JOIN f ON f.d = d.d
		ORDER BY d.d ASC
	`, tStart, tEnd, tripID)
	if err != nil {
		utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", err.Error())
		return
	}
	defer rows.Close()

	type dayCount struct {
		Date      time.Time
		FreeCount int
	}
	daily := make([]dayCount, 0, 128)
	for rows.Next() {
		var dt time.Time
		var fc int
		if err := rows.Scan(&dt, &fc); err != nil {
			utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", err.Error())
			return
		}
		daily = append(daily, dayCount{Date: dt, FreeCount: fc})
	}
	if err := rows.Err(); err != nil {
		utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", err.Error())
		return
	}

	// 3) คัดวันผ่านเกณฑ์: free_count >= MinAvailabilityMember
	pass := make([]dayCount, 0, len(daily))
	for _, d := range daily {
		if d.FreeCount >= in.MinAvailabilityMember {
			pass = append(pass, d)
		}
	}

	// 4) จับกลุ่มวันติดกัน (gaps-and-islands)
	type period struct {
		Start    time.Time
		End      time.Time
		Duration int
		MinFree  int
		TotalM   int
		Percent  float64
	}
	periods := make([]period, 0)
	if len(pass) > 0 {
		curStart := pass[0].Date
		curEnd := pass[0].Date
		minFree := pass[0].FreeCount

		advance := func() {
			dur := int(curEnd.Sub(curStart).Hours()/24) + 1
			if dur >= in.MinDays {
				p := period{
					Start:    curStart,
					End:      curEnd,
					Duration: dur,
					MinFree:  minFree,
					TotalM:   totalMembers,
					Percent:  (float64(minFree) / float64(totalMembers)) * 100.0,
				}
				periods = append(periods, p)
			}
		}

		for i := 1; i < len(pass); i++ {
			expected := curEnd.AddDate(0, 0, 1) // next day
			if pass[i].Date.Equal(expected) {
				curEnd = pass[i].Date
				if pass[i].FreeCount < minFree {
					minFree = pass[i].FreeCount
				}
			} else {
				// ปิดช่วงเดิม
				advance()
				// เริ่มช่วงใหม่
				curStart = pass[i].Date
				curEnd = pass[i].Date
				minFree = pass[i].FreeCount
			}
		}
		// ปิดช่วงสุดท้าย
		advance()
	}

	// 5) จัดอันดับช่วง (min free สูง -> duration ยาว -> start เร็ว)
	sort.SliceStable(periods, func(i, j int) bool {
		if periods[i].MinFree != periods[j].MinFree {
			return periods[i].MinFree > periods[j].MinFree
		}
		if periods[i].Duration != periods[j].Duration {
			return periods[i].Duration > periods[j].Duration
		}
		return periods[i].Start.Before(periods[j].Start)
	})

	// สถิติ: กี่วันทีทุกคนว่าง (free_count == totalMembers)
	allMembersDays := 0
	for _, d := range daily {
		if d.FreeCount == totalMembers {
			allMembersDays++
		}
	}

	// 6) ลบของเก่า + insert ชุดใหม่ใน tx
	tx, err := h.db.Begin(ctx)
	if err != nil {
		utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", err.Error())
		return
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `DELETE FROM available_periods WHERE trip_id = $1`, tripID); err != nil {
		utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", err.Error())
		return
	}

	now := time.Now()
	for i, p := range periods {
		periodNo := i + 1
		// ไม่อ้างคอลัมน์ rank (เลี่ยง enum ปัญหา)
		_, err := tx.Exec(ctx, `
			INSERT INTO available_periods
			  (id, trip_id, period_number, start_date, end_date, duration_days,
			   free_count, flexible_count, total_members, availability_percentage, created_at)
			VALUES (gen_random_uuid(), $1, $2, $3, $4, $5,
			        $6, $7, $8, $9, $10)
		`,
			tripID, periodNo, p.Start, p.End, p.Duration,
			p.MinFree, 0 /* flexible_count */, p.TotalM, p.Percent, now,
		)
		if err != nil {
			utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", err.Error())
			return
		}
	}

	if err := tx.Commit(ctx); err != nil {
		utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", err.Error())
		return
	}

	// แจ้งสมาชิกที่ accepted ทุกคน ว่ามีช่วงเวลาที่แนะนำถูกสร้างใหม่
	{
		ctx := r.Context()
		rows, err := h.db.Query(ctx, `
		SELECT user_id
		FROM trip_members
		WHERE trip_id=$1 AND status='accepted'
	`, tripID)
		if err == nil {
			defer rows.Close()
			periodCount := len(periods)
			for rows.Next() {
				var uid uuid.UUID
				if err := rows.Scan(&uid); err == nil {
					msg := fmt.Sprintf("%d new suggested periods generated for %s", periodCount, tName)
					h.sendNoti(
						ctx,
						uid,
						TypeTripUpdate, // ใช้ประเภทอัปเดตทริป
						"Updated Avvailability Periods",
						&msg,
						map[string]any{
							"trip_id":          tripID.String(),
							"total_periods":    periodCount,
							"min_days":         in.MinDays,
							"min_availability": in.MinAvailabilityMember,
							"tripName":         tName,
						},
						h.tripURL(tripID),
					)
				}
			}
		}
	}

	// 7) ตอบกลับ (periods + stats)
	type outPeriod struct {
		PeriodNumber           int     `json:"period_number"`
		StartDate              string  `json:"start_date"`
		EndDate                string  `json:"end_date"`
		DurationDays           int     `json:"duration_days"`
		TotalMembers           int     `json:"total_members"`
		AvailabilityPercentage float64 `json:"availability_percentage"`
	}
	respPeriods := make([]outPeriod, 0, len(periods))
	for i, p := range periods {
		respPeriods = append(respPeriods, outPeriod{
			PeriodNumber:           i + 1,
			StartDate:              p.Start.Format("2006-01-02"),
			EndDate:                p.End.Format("2006-01-02"),
			DurationDays:           p.Duration,
			TotalMembers:           p.TotalM,
			AvailabilityPercentage: math.Round(p.Percent*100) / 100, // ปัดทศนิยม 2 ตำแหน่ง
		})
	}

	utils.WriteJSONResponse(w, http.StatusOK, map[string]interface{}{
		"message": "Periods generated successfully",
		"periods": respPeriods,
		"stats": map[string]interface{}{
			"total_periods":              len(periods),
			"all_members_available_days": allMembersDays,
			"total_members":              totalMembers,
			"trip":                       map[string]interface{}{"id": tripID.String(), "name": tName},
			"min_days":                   in.MinDays,
			"min_availability_member":    in.MinAvailabilityMember,
		},
	})
}

// 2.5 ดูช่วงเวลาที่ Generate แล้ว
// @Summary Get generated available periods of a trip
// @Description อ่านช่วงวันที่บันทึกไว้ในตาราง available_periods (กัน NULL ให้ปลอดภัย)
// @Tags availability
// @Produce json
// @Param trip_id path string true "Trip ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/trips/{trip_id}/available-periods [get]
func (h *TripsHandler) GetAvailablePeriods(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	// auth (ถ้าต้องการให้เฉพาะสมาชิกดู ให้เปิดส่วนนี้)
	if _, ok := r.Context().Value("user_id").(uuid.UUID); !ok {
		utils.WriteErrorResponse(w, http.StatusUnauthorized, "Unauthorized", "Invalid user context")
		return
	}

	// parse /api/trips/{trip_id}/available-periods
	rest := strings.TrimPrefix(r.URL.Path, "/api/trips/")
	slash := strings.Index(rest, "/")
	if slash <= 0 || !strings.HasSuffix(r.URL.Path, "/available-periods") {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Invalid path", "missing or invalid trip_id")
		return
	}
	tripIDStr := rest[:slash]
	tripID, err := uuid.Parse(tripIDStr)
	if err != nil {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Invalid trip id", "trip_id must be UUID")
		return
	}

	ctx := r.Context()

	// ตรวจว่าทริปมีจริง (ป้องกัน 404 สวย ๆ)
	var exists bool
	if err := h.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM trips WHERE id=$1)`, tripID).Scan(&exists); err != nil {
		utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", err.Error())
		return
	}
	if !exists {
		utils.WriteErrorResponse(w, http.StatusNotFound, "Not Found", "Trip not found")
		return
	}

	// อ่าน periods (กัน NULL ด้วย COALESCE และ/หรือ sql.Null*)
	rows, err := h.db.Query(ctx, `
		SELECT
			id,
			period_number,
			start_date,
			end_date,
			COALESCE(duration_days, 0)              AS duration_days,
			COALESCE(total_members, 0)              AS total_members,
			availability_percentage,                -- อาจเป็น NULL ถ้าเคย insert เก่า
			created_at
		FROM available_periods
		WHERE trip_id = $1
		ORDER BY period_number ASC, start_date ASC
	`, tripID)
	if err != nil {
		utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", err.Error())
		return
	}
	defer rows.Close()

	type periodDTO struct {
		ID                     string  `json:"id"`
		PeriodNumber           int     `json:"period_number"`
		StartDate              string  `json:"start_date"`
		EndDate                string  `json:"end_date"`
		DurationDays           int     `json:"duration_days"`
		TotalMembers           int     `json:"total_members"`
		AvailabilityPercentage float64 `json:"availability_percentage"`
		CreatedAt              string  `json:"created_at"`
	}

	list := make([]periodDTO, 0, 16)

	for rows.Next() {
		var (
			id           uuid.UUID
			periodNo     int
			startDate    time.Time
			endDate      time.Time
			durationDays int
			totalMembers int
			percNull     sql.NullFloat64
			createdAt    time.Time
		)
		if err := rows.Scan(
			&id,
			&periodNo,
			&startDate,
			&endDate,
			&durationDays,
			&totalMembers,
			&percNull,
			&createdAt,
		); err != nil {
			utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", err.Error())
			return
		}

		perc := 0.0
		if percNull.Valid {
			perc = percNull.Float64
		}

		list = append(list, periodDTO{
			ID:                     id.String(),
			PeriodNumber:           periodNo,
			StartDate:              startDate.Format("2006-01-02"),
			EndDate:                endDate.Format("2006-01-02"),
			DurationDays:           durationDays,
			TotalMembers:           totalMembers,
			AvailabilityPercentage: perc,
			CreatedAt:              createdAt.UTC().Format(time.RFC3339),
		})
	}
	if err := rows.Err(); err != nil {
		utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", err.Error())
		return
	}

	utils.WriteJSONResponse(w, http.StatusOK, map[string]interface{}{
		"periods": list,
	})
}

// ---------- helpers (ถ้ายังไม่มีในไฟล์นี้ ให้เพิ่ม) ----------

// getUserDisplayName ดึง display_name หรือ username จาก profile
// ถ้าไม่มี profile หรือไม่มี display_name จะใช้ username
// ถ้าไม่มี username จะใช้ user_id เป็น fallback
func (h *TripsHandler) getUserDisplayName(ctx context.Context, userID uuid.UUID) string {
	var displayName, username *string
	err := h.db.QueryRow(ctx, `
		SELECT p.display_name, p.username
		FROM profiles p
		WHERE p.user_id = $1
		LIMIT 1
	`, userID).Scan(&displayName, &username)

	if err != nil {
		// ถ้าไม่มี profile ให้ใช้ user_id
		return userID.String()
	}

	// ใช้ display_name ถ้ามี ถ้าไม่มีใช้ username ถ้าไม่มีทั้งคู่ใช้ user_id
	if displayName != nil && strings.TrimSpace(*displayName) != "" {
		return *displayName
	}
	if username != nil && strings.TrimSpace(*username) != "" {
		return *username
	}
	return userID.String()
}

func mathRound2(v float64) float64 {
	return math.Round(v*100) / 100
}

func dateOnlyUTC(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func daysInclusive(a, b time.Time) int {
	return int(b.Sub(a).Hours()/24) + 1
}

// pushNotification ส่งแจ้งเตือนเข้า table notifications โดยอิง DTO เดิมของโปรเจกต์
// หมายเหตุ: ถ้าใน dto/notification.go ของคุณใช้ชื่อฟิลด์/ชนิดต่างกัน ให้แก้ mapping ตรงนี้ได้เลย
func (h *TripsHandler) pushNotification(ctx context.Context, toUser uuid.UUID, tripID *uuid.UUID, nType, title, content string, meta map[string]interface{}) error {
	// marshaling metadata -> jsonb
	var metaJSON []byte
	var err error
	if meta != nil {
		metaJSON, err = json.Marshal(meta)
		if err != nil {
			return err
		}
	}

	// ตัวอย่าง SQL มาตรฐาน (ปรับ field ให้ตรง schema ของคุณ ถ้าแตกต่าง)
	// columns ที่พบใช้บ่อย: id (uuid gen), recipient_id, trip_id, type, title, content, metadata(jsonb), created_at, read_at
	if tripID != nil {
		_, err = h.db.Exec(ctx, `
			INSERT INTO notifications (recipient_id, trip_id, type, title, content, metadata)
			VALUES ($1, $2, $3, $4, $5, COALESCE($6::jsonb, '{}'::jsonb))
		`, toUser, *tripID, nType, title, content, metaJSON)
	} else {
		_, err = h.db.Exec(ctx, `
			INSERT INTO notifications (recipient_id, type, title, content, metadata)
			VALUES ($1, $2, $3, $4, COALESCE($5::jsonb, '{}'::jsonb))
		`, toUser, nType, title, content, metaJSON)
	}
	return err
}

// sendNoti: ห่อเรียก NotificationsService ให้สั้นลง
// Production-ready: includes proper context handling, error logging, and retry logic
func (h *TripsHandler) sendNoti(
	ctx context.Context,
	to uuid.UUID,
	typ Type,
	title string,
	message *string,
	data map[string]any,
	actionURL *string,
) {
	// Validate inputs before spawning goroutine
	if to == uuid.Nil {
		log.Printf("Warning: Attempted to send notification to nil user_id (type=%s, title=%s)",
			string(typ), title)
		return
	}
	if strings.TrimSpace(title) == "" {
		log.Printf("Warning: Attempted to send notification with empty title (user_id=%s, type=%s)",
			to.String(), string(typ))
		return
	}

	// fire-and-forget เพื่อไม่บล็อก request หลัก
	// ใช้ context.Background() แทน request context เพื่อไม่ให้ถูก cancel เมื่อ request เสร็จ
	go func() {
		// สร้าง context ใหม่ที่มี timeout เพื่อป้องกัน goroutine ค้าง
		bgCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Retry logic: retry once if first attempt fails
		maxRetries := 2
		var lastErr error
		for attempt := 1; attempt <= maxRetries; attempt++ {
			err := h.noti.Create(bgCtx, to, string(typ), title, message, data, actionURL)
			if err == nil {
				// Success - no need to retry
				return
			}

			lastErr = err
			// Don't retry on validation errors or context timeout
			if errors.Is(err, context.DeadlineExceeded) ||
				strings.Contains(err.Error(), "required") ||
				strings.Contains(err.Error(), "exceeds maximum") {
				break
			}

			// Wait before retry (exponential backoff)
			if attempt < maxRetries {
				waitTime := time.Duration(attempt) * 100 * time.Millisecond
				time.Sleep(waitTime)
				// Create new context for retry
				bgCtx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
			}
		}

		// Log error after all retries failed
		log.Printf("Failed to create notification after %d attempts: %v (user_id=%s, type=%s, title=%s)",
			maxRetries, lastErr, to.String(), string(typ), title)
	}()
}

// ช่วยสร้างลิงก์ไปหน้า trip ใน FE จาก FRONTEND_URL
func (h *TripsHandler) tripURL(tripID uuid.UUID) *string {
	base := os.Getenv("FRONTEND_URL")
	if base == "" {
		base = "http://localhost:8081"
	}
	u := fmt.Sprintf("%s/trips/%s", base, tripID.String())
	return &u
}
