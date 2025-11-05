package dto

// CreateTripRequest represents the payload to create a trip
type CreateTripRequest struct {
	Name        string  `json:"name"`
	Destination string  `json:"destination"`
	StartDate   string  `json:"start_date"` // ISO 8601 format: YYYY-MM-DD or RFC3339
	EndDate     string  `json:"end_date"`   // ISO 8601 format: YYYY-MM-DD or RFC3339
	Description string  `json:"description"`
	Status      string  `json:"status"` // draft | published | cancelled
	TotalBudget float64 `json:"total_budget"`
	Currency    string  `json:"currency"`
}

// UpdateTripRequest represents fields allowed to update a trip
// All fields are optional; only provided ones will be updated
type UpdateTripRequest struct {
	Name        *string  `json:"name"`
	Destination *string  `json:"destination"`
	Description *string  `json:"description"`
	StartMonth  *string  `json:"start_month"` // YYYY-MM
	EndMonth    *string  `json:"end_month"`   // YYYY-MM
	TotalBudget *float64 `json:"total_budget"`
	Status      *string  `json:"status"` // draft | published | cancelled
}

// TripResponse represents a trip object in responses
type TripResponse struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Destination string  `json:"destination"`
	StartDate   string  `json:"start_date"`
	EndDate     string  `json:"end_date"`
	Description string  `json:"description"`
	Status      string  `json:"status"`
	TotalBudget float64 `json:"total_budget"`
	Currency    string  `json:"currency"`
	CreatorID   string  `json:"creator_id"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

// CreateTripResponse envelope
type CreateTripResponse struct {
	Trip TripResponse `json:"trip"`
}

// TripListItem minimal list item
type TripListItem struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Destination string  `json:"destination"`
	StartDate   string  `json:"start_date"`
	EndDate     string  `json:"end_date"`
	Status      string  `json:"status"`
	TotalBudget float64 `json:"total_budget"`
	Currency    string  `json:"currency"`
	CreatorID   string  `json:"creator_id"`
	MemberCount int     `json:"member_count"`
	CreatedAt   string  `json:"created_at"`
}

// Pagination info
type Pagination struct {
	Total  int `json:"total"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

// TripListResponse envelope
type TripListResponse struct {
	Trips      []TripListItem `json:"trips"`
	Pagination Pagination     `json:"pagination"`
}

// TripMember item in trip detail
type TripMember struct {
	UserID                string `json:"user_id"`
	Username              string `json:"username"`
	DisplayName           string `json:"display_name"`
	FirstName             string `json:"first_name"`
	LastName              string `json:"last_name"`
	AvatarURL             string `json:"avatar_url"`
	Role                  string `json:"role"`
	Status                string `json:"status"`
	AvailabilitySubmitted bool   `json:"availability_submitted"`
	InvitedAt             string `json:"invited_at"`
	JoinedAt              string `json:"joined_at"`
}

// TripPermissions for detail
type TripPermissions struct {
	CanEdit         bool `json:"can_edit"`
	CanDelete       bool `json:"can_delete"`
	CanInvite       bool `json:"can_invite"`
	CanManageBudget bool `json:"can_manage_budget"`
}

// TripStats for detail
type TripStats struct {
	TotalMembers            int `json:"total_members"`
	AcceptedMembers         int `json:"accepted_members"`
	PendingInvitations      int `json:"pending_invitations"`
	MembersWithAvailability int `json:"members_with_availability"`
}

// TripDetailTrip encapsulates extra fields (start_month, end_month, etc.)
type TripDetailTrip struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Destination string  `json:"destination"`
	Description string  `json:"description"`
	StartMonth  string  `json:"start_month"`
	EndMonth    string  `json:"end_month"`
	TotalBudget float64 `json:"total_budget"`
	Currency    string  `json:"currency"`
	Status      string  `json:"status"`
	CreatorID   string  `json:"creator_id"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

// TripDetailResponse envelope
type TripDetailResponse struct {
	Trip        TripDetailTrip  `json:"trip"`
	Members     []TripMember    `json:"members"`
	Permissions TripPermissions `json:"permissions"`
	Stats       TripStats       `json:"stats"`
}
