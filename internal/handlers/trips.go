package handlers

import (
	"context"
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
	userID, ok := utils.GetUserIDFromContext(r.Context())
	if !ok {
		utils.WriteErrorResponse(w, http.StatusUnauthorized, "Unauthorized", "Invalid user context")
		return
	}

	var req dto.CreateTripRequest
	if err := utils.DecodeJSONRequest(w, r, &req); err != nil {
		return // Error already handled by DecodeJSONRequest
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

	// Parse dates (ISO 8601 format: YYYY-MM-DD or RFC3339)
	startAt, err := utils.ParseDate(req.StartDate)
	if err != nil {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Validation error", "start_date must be ISO 8601 format (YYYY-MM-DD or RFC3339)")
		return
	}
	endAt, err := utils.ParseDate(req.EndDate)
	if err != nil {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Validation error", "end_date must be ISO 8601 format (YYYY-MM-DD or RFC3339)")
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
		StartDate:   utils.FormatDate(trip.StartDate),
		EndDate:     utils.FormatDate(trip.EndDate),
		Description: trip.Description,
		Status:      trip.Status,
		TotalBudget: trip.TotalBudget,
		Currency:    trip.Currency,
		CreatorID:   trip.CreatorID.String(),
		CreatedAt:   utils.FormatTimestamp(trip.CreatedAt),
		UpdatedAt:   utils.FormatTimestamp(trip.UpdatedAt),
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
	if _, ok := utils.GetUserIDFromContext(r.Context()); !ok {
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
			StartDate:   utils.FormatDate(startAt),
			EndDate:     utils.FormatDate(endAt),
			Status:      st,
			TotalBudget: totalBudget,
			Currency:    currency,
			CreatorID:   creatorID.String(),
			MemberCount: memberCount,
			CreatedAt:   utils.FormatTimestamp(createdAt),
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
	requesterID, ok := utils.GetUserIDFromContext(r.Context())
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
			m.InvitedAt = utils.FormatTimestamp(*invitedAt)
		}
		if joinedAt != nil {
			m.JoinedAt = utils.FormatTimestamp(*joinedAt)
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
			StartMonth:  utils.FormatMonth(t.StartDate),
			EndMonth:    utils.FormatMonth(t.EndDate),
			TotalBudget: t.TotalBudget,
			Currency:    t.Currency,
			Status:      t.Status,
			CreatorID:   t.CreatorID.String(),
			CreatedAt:   utils.FormatTimestamp(t.CreatedAt),
			UpdatedAt:   utils.FormatTimestamp(t.UpdatedAt),
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

	requesterID, ok := utils.GetUserIDFromContext(r.Context())
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
	if err := utils.DecodeJSONRequest(w, r, &req); err != nil {
		return // Error already handled by DecodeJSONRequest
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
		t, err := utils.ParseMonth(sm)
		if err != nil {
			utils.WriteErrorResponse(w, http.StatusBadRequest, "Validation error", "start_month must be ISO 8601 format (YYYY-MM)")
			return
		}
		startDate = t
	}
	endDate := cur.EndDate
	if req.EndMonth != nil {
		em := strings.TrimSpace(*req.EndMonth)
		t, err := utils.ParseMonth(em)
		if err != nil {
			utils.WriteErrorResponse(w, http.StatusBadRequest, "Validation error", "end_month must be ISO 8601 format (YYYY-MM)")
			return
		}
		endDate = t
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
		StartDate:   utils.FormatDate(startDate),
		EndDate:     utils.FormatDate(endDate),
		Description: description,
		Status:      status,
		TotalBudget: totalBudget,
		Currency:    cur.Currency,
		CreatorID:   cur.CreatorID.String(),
		CreatedAt:   utils.FormatTimestamp(cur.CreatedAt),
		UpdatedAt:   utils.FormatTimestamp(now),
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

	requesterID, ok := utils.GetUserIDFromContext(r.Context())
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
