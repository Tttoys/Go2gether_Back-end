package dto

// 2.1TripDatesTrip holds minimal trip info returned with date range
type TripDatesTrip struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	StartDate string `json:"start_date"` // from trips.start_date (YYYY-MM-DD)
	EndDate   string `json:"end_date"`   // from trips.end_date   (YYYY-MM-DD)
}

type TripDateRange struct {
	StartDate  string `json:"start_date"`  // normalized to first day of start month
	EndDate    string `json:"end_date"`    // normalized to last day of end month
	TotalDates int    `json:"total_dates"` // inclusive count
}

type TripDatesResponse struct {
	Trip      TripDatesTrip `json:"trip"`
	DateRange TripDateRange `json:"date_range"`
}

// 2.2 Save availability
type TripAvailabilityRequest struct {
	Dates []string `json:"dates"` // array of "YYYY-MM-DD"
}

type TripAvailabilitySummary struct {
	TotalDates     int `json:"total_dates"`
	SubmittedDates int `json:"submitted_dates"`
}

type TripAvailabilityResponse struct {
	Message string                  `json:"message"`
	Summary TripAvailabilitySummary `json:"summary"`
}

// 2.3 Get my availability
type TripAvailabilityDateItem struct {
	Date string `json:"date"` // YYYY-MM-DD
}

type TripMyAvailabilityResponse struct {
	Availability []TripAvailabilityDateItem `json:"availability"`
	Summary      TripAvailabilitySummary    `json:"summary"`
}

// 2.4 Generate periods (request/response)
type TripGeneratePeriodsRequest struct {
	MinDays               int `json:"min_days"`                // ขั้นต่ำความยาวช่วง (วัน)
	MinAvailabilityMember int `json:"min_availability_member"` // จำนวนสมาชิกขั้นต่ำที่ต้องว่าง "ทุกวัน" ในช่วง
}

type TripGeneratedPeriod struct {
	PeriodNumber           int     `json:"period_number"`
	StartDate              string  `json:"start_date"` // YYYY-MM-DD
	EndDate                string  `json:"end_date"`   // YYYY-MM-DD
	DurationDays           int     `json:"duration_days"`
	TotalMembers           int     `json:"total_members"`
	AvailabilityPercentage float64 `json:"availability_percentage"` // min free% ภายในช่วง (เช่น 80.00)
}

type TripGeneratePeriodsStats struct {
	TotalPeriods            int `json:"total_periods"`
	AllMembersAvailableDays int `json:"all_members_available_days"` // จำนวนวันที่ free_count = total_members
}

type TripGeneratePeriodsResponse struct {
	Message string                   `json:"message"`
	Periods []TripGeneratedPeriod    `json:"periods"`
	Stats   TripGeneratePeriodsStats `json:"stats"`
}

// 2.5 List generated available periods
type TripAvailablePeriodItem struct {
	ID                     string  `json:"id"`
	PeriodNumber           int     `json:"period_number"`
	StartDate              string  `json:"start_date"` // YYYY-MM-DD
	EndDate                string  `json:"end_date"`   // YYYY-MM-DD
	DurationDays           int     `json:"duration_days"`
	FreeCount              int     `json:"free_count"`
	FlexibleCount          int     `json:"flexible_count"`
	TotalMembers           int     `json:"total_members"`
	AvailabilityPercentage float64 `json:"availability_percentage"` // e.g., 100.00
	Rank                   string  `json:"rank"`                    // period_rank enum as text
	CreatedAt              string  `json:"created_at"`              // RFC3339
}

type TripAvailablePeriodsResponse struct {
	Periods []TripAvailablePeriodItem `json:"periods"`
}
