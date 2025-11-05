package handlers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"GO2GETHER_BACK-END/internal/config"
	"GO2GETHER_BACK-END/internal/dto"
	"GO2GETHER_BACK-END/internal/models"
	"GO2GETHER_BACK-END/internal/utils"
)

// TripsHandler manages trip-related endpoints
type TripsHandler struct {
	db     *pgxpool.Pool
	config *config.Config
}

// ===== Helper: strict JSON decoder =====
func decodeStrictJSON(r *http.Request, v interface{}) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(v)
}

// NewTripsHandler creates a new TripsHandler
func NewTripsHandler(db *pgxpool.Pool, cfg *config.Config) *TripsHandler {
	return &TripsHandler{db: db, config: cfg}
}

// Trips dispatches by HTTP method for /api/trips
func (h *TripsHandler) Trips(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		h.CreateTrip(w, r)
	case http.MethodGet:
		// If path has an ID suffix, treat as detail
		if strings.HasPrefix(r.URL.Path, "/api/trips/") && len(r.URL.Path) > len("/api/trips/") {
			h.TripDetail(w, r)
			return
		}
		h.ListTrips(w, r)
	case http.MethodPut, http.MethodPatch:
		h.UpdateTrip(w, r)
	case http.MethodDelete:
		h.DeleteTrip(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// CreateTrip handles POST /api/trips
// @Summary Create a new trip
// @Tags trips
// @Accept json
// @Produce json
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

	// Extract authenticated user id from context
	uid := r.Context().Value("user_id")
	userID, ok := uid.(uuid.UUID)
	if !ok {
		utils.WriteErrorResponse(w, http.StatusUnauthorized, "Unauthorized", "Invalid user context")
		return
	}

	var req dto.CreateTripRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Invalid request data", "Malformed JSON body")
		return
	}

	// Basic validation
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

	// Parse dates (support YYYY-MM-DD and RFC3339)
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

	// Defaults for budget fields
	currency := strings.ToUpper(strings.TrimSpace(req.Currency))
	if currency == "" {
		currency = "THB"
	}
	totalBudget := req.TotalBudget
	if totalBudget < 0 {
		totalBudget = 0
	}

	// Insert trip with budget fields
	// id, name, destination, start_date, end_date, description, status, total_budget, currency, creator_id, created_at, updated_at
	_, err = h.db.Exec(context.Background(),
		`INSERT INTO trips (id, name, destination, start_date, end_date, description, status, total_budget, currency, creator_id, created_at, updated_at)
         VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
		newID, req.Name, req.Destination, startAt, endAt, req.Description, req.Status, totalBudget, currency, userID, now, now,
	)
	if err != nil {
		utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", err.Error())
		return
	}

	// Auto-join creator as trip member (role: creator, status: accepted)
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
	}}

	utils.WriteJSONResponse(w, http.StatusCreated, resp)
}

// ListTrips handles GET /api/trips with filters and pagination
// @Summary List trips
// @Tags trips
// @Produce json
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

	// Ensure authorized (context populated by middleware)
	if _, ok := r.Context().Value("user_id").(uuid.UUID); !ok {
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

	// Query total
	var total int
	if status == "all" {
		if err := h.db.QueryRow(context.Background(), `SELECT COUNT(1) FROM trips`).Scan(&total); err != nil {
			utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", err.Error())
			return
		}
	} else {
		if err := h.db.QueryRow(context.Background(), `SELECT COUNT(1) FROM trips WHERE status = $1`, status).Scan(&total); err != nil {
			utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", err.Error())
			return
		}
	}

	// Query page of trips with member_count via subquery (avoids GROUP BY pitfalls)
	rows, err := h.db.Query(context.Background(),
		`SELECT t.id, t.name, t.destination, t.start_date, t.end_date, t.status, t.total_budget, t.currency, t.creator_id, t.created_at,
                COALESCE((SELECT COUNT(DISTINCT tm.user_id) FROM trip_members tm WHERE tm.trip_id = t.id), 0) AS member_count
           FROM trips t
          WHERE ($1 = 'all' OR t.status = $1)
          ORDER BY t.created_at DESC
          LIMIT $2 OFFSET $3`, status, limit, offset)
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

	// Ensure authorized
	requesterID, ok := r.Context().Value("user_id").(uuid.UUID)
	if !ok {
		utils.WriteErrorResponse(w, http.StatusUnauthorized, "Unauthorized", "Invalid user context")
		return
	}

	// Extract id from path
	path := r.URL.Path
	idStr := strings.TrimPrefix(path, "/api/trips/")
	tripID, err := uuid.Parse(idStr)
	if err != nil {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Invalid trip id", "trip_id must be UUID")
		return
	}

	// Load trip
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

	// Members (optional user profile fields may be unavailable, fallback to blanks)
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

	// Stats
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

	// Permissions
	// Fallback exists-check in DB in case list didn't include requester row
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

	// Compose response
	resp := dto.TripDetailResponse{
		Trip: dto.TripDetailTrip{
			ID:          t.ID.String(),
			Name:        t.Name,
			Destination: t.Destination,
			Description: t.Description,
			StartMonth:  time.Date(t.StartDate.Year(), t.StartDate.Month(), 1, 0, 0, 0, 0, t.StartDate.Location()).Format("2006-01-02"),
			EndMonth:    time.Date(t.EndDate.Year(), t.EndDate.Month(), 1, 0, 0, 0, 0, t.EndDate.Location()).Format("2006-01-02"),
			TotalBudget: t.TotalBudget,
			Currency:    t.Currency,
			Status:      t.Status,
			CreatorID:   t.CreatorID.String(),
			CreatedAt:   t.CreatedAt.Format(time.RFC3339),
			UpdatedAt:   t.UpdatedAt.Format(time.RFC3339),
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

	// Extract trip ID from path
	idStr := strings.TrimPrefix(r.URL.Path, "/api/trips/")
	tripID, err := uuid.Parse(idStr)
	if err != nil {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Invalid trip id", "trip_id must be UUID")
		return
	}

	// Load current trip
	var cur models.Trip
	err = h.db.QueryRow(context.Background(),
		`SELECT id, name, destination, start_date, end_date, description, status, total_budget, currency, creator_id, created_at, updated_at
           FROM trips WHERE id = $1`, tripID).Scan(
		&cur.ID, &cur.Name, &cur.Destination, &cur.StartDate, &cur.EndDate, &cur.Description, &cur.Status, &cur.TotalBudget, &cur.Currency, &cur.CreatorID, &cur.CreatedAt, &cur.UpdatedAt,
	)
	if err != nil {
		utils.WriteErrorResponse(w, http.StatusNotFound, "Not Found", "Trip not found")
		return
	}

	// Permission: only creator can update
	if requesterID != cur.CreatorID {
		// As a fallback also allow if member role is creator
		var exists bool
		if err := h.db.QueryRow(context.Background(),
			`SELECT EXISTS(SELECT 1 FROM trip_members WHERE trip_id = $1 AND user_id = $2 AND LOWER(role) = 'creator')`,
			cur.ID, requesterID,
		).Scan(&exists); err != nil || !exists {
			utils.WriteErrorResponse(w, http.StatusForbidden, "Forbidden", "Only creator can update this trip")
			return
		}
	}

	var req dto.UpdateTripRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Invalid request data", "Malformed JSON body")
		return
	}

	// Prepare new values, default to current if nil
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

	// Parse months to first day of month
	startDate := cur.StartDate
	if req.StartMonth != nil {
		sm := strings.TrimSpace(*req.StartMonth)
		t, err := time.Parse("2006-01", sm)
		if err != nil {
			utils.WriteErrorResponse(w, http.StatusBadRequest, "Validation error", "start_month must be YYYY-MM")
			return
		}
		startDate = time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
	}
	endDate := cur.EndDate
	if req.EndMonth != nil {
		em := strings.TrimSpace(*req.EndMonth)
		t, err := time.Parse("2006-01", em)
		if err != nil {
			utils.WriteErrorResponse(w, http.StatusBadRequest, "Validation error", "end_month must be YYYY-MM")
			return
		}
		endDate = time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
	}
	if endDate.Before(startDate) {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Validation error", "end_month cannot be before start_month")
		return
	}

	totalBudget := cur.TotalBudget
	if req.TotalBudget != nil {
		if *req.TotalBudget < 0 {
			utils.WriteErrorResponse(w, http.StatusBadRequest, "Validation error", "total_budget cannot be negative")
			return
		}
		totalBudget = *req.TotalBudget
	}

	// Update
	now := time.Now()
	_, err = h.db.Exec(context.Background(),
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
		name, destination, description, startDate, endDate, status, totalBudget, now, cur.ID,
	)
	if err != nil {
		utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", err.Error())
		return
	}

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

	utils.WriteJSONResponse(w, http.StatusOK, dto.CreateTripResponse{Trip: updated})
}

// DeleteTrip handles DELETE /api/trips/{trip_id}
// @Summary Delete a trip
// @Tags trips
// @Produce json
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

	idStr := strings.TrimPrefix(r.URL.Path, "/api/trips/")
	tripID, err := uuid.Parse(idStr)
	if err != nil {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Invalid trip id", "trip_id must be UUID")
		return
	}

	// Ensure exists and permission
	var creatorID uuid.UUID
	if err := h.db.QueryRow(context.Background(), `SELECT creator_id FROM trips WHERE id = $1`, tripID).Scan(&creatorID); err != nil {
		utils.WriteErrorResponse(w, http.StatusNotFound, "Not Found", "Trip not found")
		return
	}

	if requesterID != creatorID {
		// Fallback allow if requester is creator member
		var exists bool
		if err := h.db.QueryRow(context.Background(),
			`SELECT EXISTS(SELECT 1 FROM trip_members WHERE trip_id = $1 AND user_id = $2 AND LOWER(role) = 'creator')`,
			tripID, requesterID,
		).Scan(&exists); err != nil || !exists {
			utils.WriteErrorResponse(w, http.StatusForbidden, "Forbidden", "Only creator can delete this trip")
			return
		}
	}

	// Delete trip (CASCADE will remove members if FK is set)
	if _, err := h.db.Exec(context.Background(), `DELETE FROM trips WHERE id = $1`, tripID); err != nil {
		utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", err.Error())
		return
	}

	utils.WriteJSONResponse(w, http.StatusOK, map[string]string{"message": "Trip deleted successfully"})
}

// InviteMembers handles POST /api/trips/{trip_id}/invitations
// @Summary      Invite members to a trip
// @Description  FR3.1 เชิญสมาชิกเข้าทริป (เฉพาะ creator/creator-role member)
// @Tags         trips
// @Accept       json
// @Produce      json
// @Param        trip_id  path      string                true  "Trip ID"
// @Param        payload  body      dto.TripInviteRequest true  "Invitation payload"
// @Success      200      {object}  dto.TripInviteResponse
// @Failure      400      {object}  utils.ErrorResponse
// @Failure      401      {object}  utils.ErrorResponse
// @Failure      403      {object}  utils.ErrorResponse
// @Failure      404      {object}  utils.ErrorResponse
// @Failure      500      {object}  utils.ErrorResponse
// @Router       /api/trips/{trip_id}/invitations [post]
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

	// parse /api/trips/{trip_id}/invitations
	rest := strings.TrimPrefix(r.URL.Path, "/api/trips/")
	i := strings.Index(rest, "/")
	if i <= 0 || !strings.HasSuffix(r.URL.Path, "/invitations") {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Invalid path", "missing or invalid trip_id")
		return
	}
	tripID, err := uuid.Parse(rest[:i])
	if err != nil {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Invalid trip id", "trip_id must be UUID")
		return
	}

	// permission: creator or creator-role member
	var creatorID uuid.UUID
	if err := h.db.QueryRow(r.Context(), `SELECT creator_id FROM trips WHERE id=$1`, tripID).Scan(&creatorID); err != nil {
		utils.WriteErrorResponse(w, http.StatusNotFound, "Not Found", "Trip not found")
		return
	}
	if requesterID != creatorID {
		var okCreator bool
		if err := h.db.QueryRow(r.Context(),
			`SELECT EXISTS(SELECT 1 FROM trip_members WHERE trip_id=$1 AND user_id=$2 AND LOWER(role)='creator')`,
			tripID, requesterID,
		).Scan(&okCreator); err != nil || !okCreator {
			utils.WriteErrorResponse(w, http.StatusForbidden, "Forbidden", "Only creator can invite members")
			return
		}
	}

	var req dto.TripInviteRequest
	if err := decodeStrictJSON(r, &req); err != nil {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Invalid request data", "Malformed JSON body or unknown fields")
		return
	}
	if len(req.UserIDs) == 0 {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Validation error", "user_ids is required and must not be empty")
		return
	}

	// normalize UUIDs, skip self & duplicates
	seen := map[uuid.UUID]struct{}{}
	cands := make([]uuid.UUID, 0, len(req.UserIDs))
	for _, s := range req.UserIDs {
		id, err := uuid.Parse(strings.TrimSpace(s))
		if err != nil {
			utils.WriteErrorResponse(w, http.StatusBadRequest, "Validation error", "user_ids must be UUIDs")
			return
		}
		if id == requesterID {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		cands = append(cands, id)
	}
	if len(cands) == 0 {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Validation error", "no valid user to invite")
		return
	}

	now := time.Now()
	tx, err := h.db.Begin(r.Context())
	if err != nil {
		utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", err.Error())
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()

	// validate user exists
	valid := make([]uuid.UUID, 0, len(cands))
	for _, uid := range cands {
		var exists bool
		if err := tx.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM users WHERE id=$1)`, uid).Scan(&exists); err != nil {
			utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", err.Error())
			return
		}
		if exists {
			valid = append(valid, uid)
		}
	}
	if len(valid) == 0 {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Validation error", "no valid user in database")
		return
	}

	type invitedRow struct {
		userID    uuid.UUID
		username  *string
		invitedAt time.Time
	}
	outRows := make([]invitedRow, 0, len(valid))

	for _, uid := range valid {
		// read status if exists
		var cur *string
		if err := tx.QueryRow(r.Context(),
			`SELECT status FROM trip_members WHERE trip_id=$1 AND user_id=$2`,
			tripID, uid,
		).Scan(&cur); err != nil && err.Error() != "no rows in result set" {
			utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", err.Error())
			return
		}

		if cur == nil {
			// insert pending
			if _, err := tx.Exec(r.Context(),
				`INSERT INTO trip_members (trip_id, user_id, role, status, invited_by, invited_at, availability_submitted)
				 VALUES ($1,$2,'member','pending',$3,$4,FALSE)
				 ON CONFLICT (trip_id,user_id) DO NOTHING`,
				tripID, uid, requesterID, now,
			); err != nil {
				utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", err.Error())
				return
			}
			outRows = append(outRows, invitedRow{userID: uid, invitedAt: now})
		} else {
			switch strings.ToLower(*cur) {
			case "accepted", "pending":
				// skip
				continue
			default:
				// reset to pending
				if _, err := tx.Exec(r.Context(),
					`UPDATE trip_members
					   SET status='pending', invited_by=$3, invited_at=$4
					 WHERE trip_id=$1 AND user_id=$2`,
					tripID, uid, requesterID, now,
				); err != nil {
					utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", err.Error())
					return
				}
				outRows = append(outRows, invitedRow{userID: uid, invitedAt: now})
			}
		}
	}

	// attach username
	for i := range outRows {
		var u *string
		_ = tx.QueryRow(r.Context(), `SELECT username FROM profiles WHERE user_id=$1`, outRows[i].userID).Scan(&u)
		outRows[i].username = u
	}

	if err := tx.Commit(r.Context()); err != nil {
		utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", err.Error())
		return
	}

	items := make([]dto.TripInviteItem, 0, len(outRows))
	for _, row := range outRows {
		items = append(items, dto.TripInviteItem{
			TripID:    tripID.String(),
			UserID:    row.userID.String(),
			Username:  row.username,
			Status:    "pending",
			InvitedAt: row.invitedAt.UTC().Format(time.RFC3339),
		})
	}
	utils.WriteJSONResponse(w, http.StatusOK, dto.TripInviteResponse{
		Invitations:       items,
		NotificationsSent: len(items),
	})
}

// RespondInvitation handles POST /api/trips/{trip_id}/invitations/respond
// @Summary      Respond to a trip invitation
// @Description  FR3.2 เพื่อนตอบรับคำเชิญ หรือยกเลิก (accept|decline)
// @Tags         trips
// @Accept       json
// @Produce      json
// @Param        trip_id  path      string                             true  "Trip ID"
// @Param        payload  body      dto.TripInvitationRespondRequest   true  "Respond payload"
// @Success      200      {object}  dto.TripInvitationRespondResponse
// @Failure      400      {object}  utils.ErrorResponse
// @Failure      401      {object}  utils.ErrorResponse
// @Failure      404      {object}  utils.ErrorResponse
// @Failure      409      {object}  utils.ErrorResponse
// @Failure      500      {object}  utils.ErrorResponse
// @Router       /api/trips/{trip_id}/invitations/respond [post]
func (h *TripsHandler) RespondInvitation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	userID, ok := r.Context().Value("user_id").(uuid.UUID)
	if !ok {
		utils.WriteErrorResponse(w, http.StatusUnauthorized, "Unauthorized", "Invalid user context")
		return
	}

	// parse /api/trips/{trip_id}/invitations/respond
	rest := strings.TrimPrefix(r.URL.Path, "/api/trips/")
	i := strings.Index(rest, "/")
	if i <= 0 || !strings.HasSuffix(r.URL.Path, "/invitations/respond") {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Invalid path", "missing or invalid trip_id")
		return
	}
	tripID, err := uuid.Parse(rest[:i])
	if err != nil {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Invalid trip id", "trip_id must be UUID")
		return
	}

	var req dto.TripInvitationRespondRequest
	if err := decodeStrictJSON(r, &req); err != nil {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Invalid request data", "Malformed JSON body or unknown fields")
		return
	}
	action := strings.ToLower(strings.TrimSpace(req.Response))
	if action != "accept" && action != "decline" {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Validation error", "response must be 'accept' or 'decline'")
		return
	}

	// basic trip data
	var tID uuid.UUID
	var tName, tDest string
	if err := h.db.QueryRow(r.Context(),
		`SELECT id,name,destination FROM trips WHERE id=$1`, tripID,
	).Scan(&tID, &tName, &tDest); err != nil {
		utils.WriteErrorResponse(w, http.StatusNotFound, "Not Found", "Trip not found")
		return
	}

	// invitation row
	var role, status string
	var invitedAt, joinedAt *time.Time
	if err := h.db.QueryRow(r.Context(),
		`SELECT role,status,invited_at,joined_at FROM trip_members WHERE trip_id=$1 AND user_id=$2`,
		tripID, userID,
	).Scan(&role, &status, &invitedAt, &joinedAt); err != nil {
		utils.WriteErrorResponse(w, http.StatusNotFound, "Not Found", "Invitation not found")
		return
	}
	if strings.ToLower(status) != "pending" {
		switch strings.ToLower(status) {
		case "accepted":
			utils.WriteErrorResponse(w, http.StatusConflict, "Conflict", "already accepted")
		case "declined":
			utils.WriteErrorResponse(w, http.StatusConflict, "Conflict", "already declined")
		default:
			utils.WriteErrorResponse(w, http.StatusConflict, "Conflict", "cannot respond in current status")
		}
		return
	}

	now := time.Now()
	if action == "accept" {
		if _, err := h.db.Exec(r.Context(),
			`UPDATE trip_members SET status='accepted', joined_at=$3 WHERE trip_id=$1 AND user_id=$2`,
			tripID, userID, now,
		); err != nil {
			utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", err.Error())
			return
		}
		joinedAt = &now
		status = "accepted"
	} else {
		if _, err := h.db.Exec(r.Context(),
			`UPDATE trip_members SET status='declined' WHERE trip_id=$1 AND user_id=$2`,
			tripID, userID,
		); err != nil {
			utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", err.Error())
			return
		}
		joinedAt = nil
		status = "declined"
	}

	var joinedAtStr *string
	if joinedAt != nil {
		s := joinedAt.UTC().Format(time.RFC3339)
		joinedAtStr = &s
	}
	utils.WriteJSONResponse(w, http.StatusOK, dto.TripInvitationRespondResponse{
		Message: map[string]string{"accepted": "Invitation accepted successfully", "declined": "Invitation declined successfully"}[status],
		Trip: dto.TripInvitationRespondTrip{
			ID:          tID.String(),
			Name:        tName,
			Destination: tDest,
		},
		Member: dto.TripInvitationRespondMember{
			UserID:   userID.String(),
			Role:     role,
			Status:   status,
			JoinedAt: joinedAtStr,
		},
	})
}

// ListInvitations handles GET /api/trips/{trip_id}/invitations
// @Summary      List invitations of a trip (creator only)
// @Description  FR3.3 ดูรายการคำเชิญของทริป (creator เห็นสถิติด้วย)
// @Tags         trips
// @Produce      json
// @Param        trip_id  path      string  true  "Trip ID"
// @Success      200      {object}  dto.TripInvitationsListResponse
// @Failure      400      {object}  utils.ErrorResponse
// @Failure      401      {object}  utils.ErrorResponse
// @Failure      403      {object}  utils.ErrorResponse
// @Failure      404      {object}  utils.ErrorResponse
// @Failure      500      {object}  utils.ErrorResponse
// @Router       /api/trips/{trip_id}/invitations [get]
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

	// parse /api/trips/{trip_id}/invitations
	rest := strings.TrimPrefix(r.URL.Path, "/api/trips/")
	i := strings.Index(rest, "/")
	if i <= 0 || !strings.HasSuffix(r.URL.Path, "/invitations") {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Invalid path", "missing or invalid trip_id")
		return
	}
	tripID, err := uuid.Parse(rest[:i])
	if err != nil {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Invalid trip id", "trip_id must be UUID")
		return
	}

	// permission: creator/creator-role
	var creatorID uuid.UUID
	if err := h.db.QueryRow(r.Context(), `SELECT creator_id FROM trips WHERE id=$1`, tripID).Scan(&creatorID); err != nil {
		utils.WriteErrorResponse(w, http.StatusNotFound, "Not Found", "Trip not found")
		return
	}
	if requesterID != creatorID {
		var okCreator bool
		if err := h.db.QueryRow(r.Context(),
			`SELECT EXISTS(SELECT 1 FROM trip_members WHERE trip_id=$1 AND user_id=$2 AND LOWER(role)='creator')`,
			tripID, requesterID,
		).Scan(&okCreator); err != nil || !okCreator {
			utils.WriteErrorResponse(w, http.StatusForbidden, "Forbidden", "Only creator can view invitations")
			return
		}
	}

	rows, err := h.db.Query(r.Context(), `
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
			status, invitedBy                string
			invitedAt                        *time.Time
		)
		if err := rows.Scan(&uid, &username, &displayName, &avatarURL, &status, &invitedBy, &invitedAt); err != nil {
			utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", err.Error())
			return
		}
		var invitedAtStr *string
		if invitedAt != nil {
			s := invitedAt.UTC().Format(time.RFC3339)
			invitedAtStr = &s
		}
		if invitedBy == "" {
			invitedBy = creatorID.String()
		}
		invites = append(invites, dto.TripInvitationListItem{
			UserID:      uid.String(),
			Username:    username,
			DisplayName: displayName,
			AvatarURL:   avatarURL,
			Status:      status,
			InvitedBy:   invitedBy,
			InvitedAt:   invitedAtStr,
		})
	}
	if err := rows.Err(); err != nil {
		utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", err.Error())
		return
	}

	var pending, accepted, declined int
	if err := h.db.QueryRow(r.Context(),
		`SELECT 
			COALESCE(SUM(CASE WHEN status='pending'  THEN 1 ELSE 0 END),0),
			COALESCE(SUM(CASE WHEN status='accepted' THEN 1 ELSE 0 END),0),
			COALESCE(SUM(CASE WHEN status='declined' THEN 1 ELSE 0 END),0)
		 FROM trip_members WHERE trip_id=$1`, tripID,
	).Scan(&pending, &accepted, &declined); err != nil {
		utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", err.Error())
		return
	}

	utils.WriteJSONResponse(w, http.StatusOK, dto.TripInvitationsListResponse{
		Invitations: invites,
		Stats: dto.TripInvitationsStats{
			Total:    pending + accepted + declined,
			Pending:  pending,
			Accepted: accepted,
			Declined: declined,
		},
	})
}

// CancelInvitation handles DELETE /api/trips/{trip_id}/invitations/{user_id}
// @Summary      Cancel a pending invitation (creator only)
// @Description  FR3.4 ยกเลิกคำเชิญ (ลบแถวที่ status='pending')
// @Tags         trips
// @Produce      json
// @Param        trip_id  path      string  true  "Trip ID"
// @Param        user_id  path      string  true  "User ID"
// @Success      200      {object}  map[string]string
// @Failure      400      {object}  utils.ErrorResponse
// @Failure      401      {object}  utils.ErrorResponse
// @Failure      403      {object}  utils.ErrorResponse
// @Failure      404      {object}  utils.ErrorResponse
// @Failure      409      {object}  utils.ErrorResponse
// @Failure      500      {object}  utils.ErrorResponse
// @Router       /api/trips/{trip_id}/invitations/{user_id} [delete]
func (h *TripsHandler) CancelInvitation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	requesterID, ok := r.Context().Value("user_id").(uuid.UUID)
	if !ok {
		utils.WriteErrorResponse(w, http.StatusUnauthorized, "Unauthorized", "Invalid user context")
		return
	}

	// parse /api/trips/{trip_id}/invitations/{user_id}
	rest := strings.TrimPrefix(r.URL.Path, "/api/trips/")
	slash := strings.Index(rest, "/")
	if slash <= 0 {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Invalid path", "missing trip_id")
		return
	}
	tripIDStr := rest[:slash]
	rest2 := rest[slash+1:]
	if !strings.HasPrefix(rest2, "invitations/") {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Invalid path", "missing invitations segment")
		return
	}
	userIDStr := strings.TrimPrefix(rest2, "invitations/")

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
	if targetUserID == requesterID {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Bad Request", "cannot cancel your own invitation")
		return
	}

	// permission
	var creatorID uuid.UUID
	if err := h.db.QueryRow(r.Context(), `SELECT creator_id FROM trips WHERE id=$1`, tripID).Scan(&creatorID); err != nil {
		utils.WriteErrorResponse(w, http.StatusNotFound, "Not Found", "Trip not found")
		return
	}
	if requesterID != creatorID {
		var okCreator bool
		if err := h.db.QueryRow(r.Context(),
			`SELECT EXISTS(SELECT 1 FROM trip_members WHERE trip_id=$1 AND user_id=$2 AND LOWER(role)='creator')`,
			tripID, requesterID,
		).Scan(&okCreator); err != nil || !okCreator {
			utils.WriteErrorResponse(w, http.StatusForbidden, "Forbidden", "Only creator can cancel invitations")
			return
		}
	}

	// must be pending
	var status string
	if err := h.db.QueryRow(r.Context(),
		`SELECT status FROM trip_members WHERE trip_id=$1 AND user_id=$2`,
		tripID, targetUserID,
	).Scan(&status); err != nil {
		utils.WriteErrorResponse(w, http.StatusNotFound, "Not Found", "Invitation not found")
		return
	}
	if strings.ToLower(status) != "pending" {
		utils.WriteErrorResponse(w, http.StatusConflict, "Conflict", "cannot cancel invitation in current status")
		return
	}

	// delete row
	cmd, err := h.db.Exec(r.Context(),
		`DELETE FROM trip_members WHERE trip_id=$1 AND user_id=$2 AND status='pending'`,
		tripID, targetUserID,
	)
	if err != nil {
		utils.WriteErrorResponse(w, http.StatusInternalServerError, "Database error", err.Error())
		return
	}
	if cmd.RowsAffected() == 0 {
		utils.WriteErrorResponse(w, http.StatusConflict, "Conflict", "invitation is no longer pending")
		return
	}
	utils.WriteJSONResponse(w, http.StatusOK, map[string]string{"message": "Invitation cancelled successfully"})
}

// LeaveTrip handles POST /api/trips/{trip_id}/leave
// @Summary      Leave a trip (for accepted members)
// @Description  FR3.5 ออกจากทริป (ลบแถวสมาชิกที่ accepted)
// @Tags         trips
// @Produce      json
// @Param        trip_id  path      string  true  "Trip ID"
// @Success      200      {object}  map[string]string
// @Failure      400      {object}  utils.ErrorResponse
// @Failure      401      {object}  utils.ErrorResponse
// @Failure      403      {object}  utils.ErrorResponse
// @Failure      404      {object}  utils.ErrorResponse
// @Failure      409      {object}  utils.ErrorResponse
// @Failure      500      {object}  utils.ErrorResponse
// @Router       /api/trips/{trip_id}/leave [post]
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

	// parse /api/trips/{trip_id}/leave
	rest := strings.TrimPrefix(r.URL.Path, "/api/trips/")
	i := strings.Index(rest, "/")
	if i <= 0 || !strings.HasSuffix(r.URL.Path, "/leave") {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Invalid path", "missing or invalid trip_id")
		return
	}
	tripID, err := uuid.Parse(rest[:i])
	if err != nil {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Invalid trip id", "trip_id must be UUID")
		return
	}

	// ensure trip + creator
	var creatorID uuid.UUID
	if err := h.db.QueryRow(r.Context(), `SELECT creator_id FROM trips WHERE id=$1`, tripID).Scan(&creatorID); err != nil {
		utils.WriteErrorResponse(w, http.StatusNotFound, "Not Found", "Trip not found")
		return
	}
	// creator cannot leave
	if userID == creatorID {
		utils.WriteErrorResponse(w, http.StatusForbidden, "Forbidden", "Creator cannot leave their own trip")
		return
	}

	// must be accepted member
	var role, status string
	if err := h.db.QueryRow(r.Context(),
		`SELECT role,status FROM trip_members WHERE trip_id=$1 AND user_id=$2`,
		tripID, userID,
	).Scan(&role, &status); err != nil {
		utils.WriteErrorResponse(w, http.StatusNotFound, "Not Found", "You are not invited to this trip")
		return
	}
	if strings.ToLower(status) != "accepted" {
		utils.WriteErrorResponse(w, http.StatusConflict, "Conflict", "You are not an active member of this trip")
		return
	}

	// delete membership row
	cmd, err := h.db.Exec(r.Context(),
		`DELETE FROM trip_members WHERE trip_id=$1 AND user_id=$2 AND status='accepted'`,
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
	utils.WriteJSONResponse(w, http.StatusOK, map[string]string{"message": "You have left the trip successfully"})
}

// RemoveMember handles DELETE /api/trips/{trip_id}/members/{user_id}
// @Summary      Remove a member from a trip (creator only)
// @Description  FR3.6 ลบสมาชิกออกจากทริป (ลบแถวสมาชิกที่มีอยู่ ไม่ตั้งสถานะ 'removed')
// @Tags         trips
// @Produce      json
// @Param        trip_id  path      string  true  "Trip ID"
// @Param        user_id  path      string  true  "User ID"
// @Success      200      {object}  map[string]string
// @Failure      400      {object}  utils.ErrorResponse
// @Failure      401      {object}  utils.ErrorResponse
// @Failure      403      {object}  utils.ErrorResponse
// @Failure      404      {object}  utils.ErrorResponse
// @Failure      409      {object}  utils.ErrorResponse
// @Failure      500      {object}  utils.ErrorResponse
// @Router       /api/trips/{trip_id}/members/{user_id} [delete]
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

	// parse /api/trips/{trip_id}/members/{user_id}
	rest := strings.TrimPrefix(r.URL.Path, "/api/trips/")
	slash := strings.Index(rest, "/")
	if slash <= 0 {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Invalid path", "missing trip_id")
		return
	}
	tripIDStr := rest[:slash]
	rest2 := rest[slash+1:]
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

	// ensure trip + permission
	var creatorID uuid.UUID
	if err := h.db.QueryRow(r.Context(), `SELECT creator_id FROM trips WHERE id=$1`, tripID).Scan(&creatorID); err != nil {
		utils.WriteErrorResponse(w, http.StatusNotFound, "Not Found", "Trip not found")
		return
	}
	if requesterID != creatorID {
		var okCreator bool
		if err := h.db.QueryRow(r.Context(),
			`SELECT EXISTS(SELECT 1 FROM trip_members WHERE trip_id=$1 AND user_id=$2 AND LOWER(role)='creator')`,
			tripID, requesterID,
		).Scan(&okCreator); err != nil || !okCreator {
			utils.WriteErrorResponse(w, http.StatusForbidden, "Forbidden", "Only creator can remove a member")
			return
		}
	}

	// cannot remove trip creator
	if targetUserID == creatorID {
		utils.WriteErrorResponse(w, http.StatusForbidden, "Forbidden", "Cannot remove the trip creator")
		return
	}

	// ensure the member exists in this trip
	var role, status string
	if err := h.db.QueryRow(r.Context(),
		`SELECT role,status FROM trip_members WHERE trip_id=$1 AND user_id=$2`,
		tripID, targetUserID,
	).Scan(&role, &status); err != nil {
		utils.WriteErrorResponse(w, http.StatusNotFound, "Not Found", "Member not found in this trip")
		return
	}

	// delete membership row
	cmd, err := h.db.Exec(r.Context(),
		`DELETE FROM trip_members WHERE trip_id=$1 AND user_id=$2`,
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
	utils.WriteJSONResponse(w, http.StatusOK, map[string]string{"message": "Member removed successfully"})
}
