package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"GO2GETHER_BACK-END/internal/dto"
	"GO2GETHER_BACK-END/internal/utils"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ProfileHandler struct {
	pool *pgxpool.Pool
}

func NewProfileHandler(pool *pgxpool.Pool) *ProfileHandler {
	return &ProfileHandler{pool: pool}
}

// Create godoc
// @Summary      Create user profile
// @Description  6.1 เพิ่มโปรไฟล์ (ต้องมี Bearer JWT)
// @Tags         profile
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        payload  body      dto.ProfileCreateRequest  true  "Profile payload"
// @Success      200      {object}  dto.ProfileCreateResponse
// @Failure      400      {object}  dto.ErrorResponse
// @Failure      401      {object}  dto.ErrorResponse
// @Failure      409      {object}  dto.ErrorResponse
// @Failure      500      {object}  dto.ErrorResponse
// @Router       /api/profile [post]
func (h *ProfileHandler) Create(w http.ResponseWriter, r *http.Request) {
	// 1) ต้องผ่าน AuthMiddleware: ดึง userID จาก context
	userID, ok := userIDFromContext(r.Context())
	if !ok {
		utils.WriteErrorResponse(w, http.StatusUnauthorized, "Unauthorized", "missing user in context")
		return
	}

	// 2) decode body
	var req dto.ProfileCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}
	if req.Username == "" {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Invalid request body", "username is required")
		return
	}

	// 3) parse birth_date (optional) — รองรับ "YYYY-MM-DD" และ RFC3339
	var birthDatePtr *time.Time
	if req.BirthDate != nil && *req.BirthDate != "" {
		if t, err := time.Parse("2006-01-02", *req.BirthDate); err == nil {
			birthDatePtr = &t
		} else if t2, err2 := time.Parse(time.RFC3339, *req.BirthDate); err2 == nil {
			tt := t2
			birthDatePtr = &tt
		} else {
			utils.WriteErrorResponse(w, http.StatusBadRequest, "Invalid request body", "birth_date must be ISO 8601 date or datetime")
			return
		}
	}

	ctx := r.Context()

	// 4) ป้องกัน user เดิมมีโปรไฟล์แล้ว
	const qHas = `select 1 from public.profiles where user_id = $1 limit 1`
	{
		var one int
		err := h.pool.QueryRow(ctx, qHas, userID).Scan(&one)
		if err == nil {
			utils.WriteErrorResponse(w, http.StatusBadRequest, "Bad Request", "Profile already exists for this user")
			return
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			utils.WriteErrorResponse(w, http.StatusInternalServerError, "Internal Server Error", err.Error())
			return
		}
	}

	// 5) insert โปรไฟล์
	const qIns = `
insert into public.profiles(
	user_id, username, first_name, last_name, display_name, avatar_url, phone, bio,
	birth_date, food_preferences, chronic_disease, allergic_food, allergic_drugs, emergency_contact
) values (
	$1, $2, $3, $4, $5,
	nullif($6,''), nullif($7,''), $8,
	$9, $10, $11, $12, $13, $14
)
returning username;
`
	var username string
	err := h.pool.QueryRow(
		ctx, qIns,
		userID, req.Username,
		nullable(req.FirstName), nullable(req.LastName), nullable(req.DisplayName),
		nullable(req.AvatarURL), nullable(req.Phone), nullable(req.Bio),
		birthDatePtr,
		nullable(req.FoodPreferences), nullable(req.ChronicDisease),
		nullable(req.AllergicFood), nullable(req.AllergicDrugs),
		nullable(req.EmergencyContact),
	).Scan(&username)
	if err != nil {
		// แยกเคส unique violation: username ซ้ำ หรือ user_id ซ้ำ
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			if pgErr.ConstraintName == "profiles_username_key" {
				utils.WriteErrorResponse(w, http.StatusConflict, "Conflict", "username already taken")
				return
			}
			// profiles_user_id_key หรืออื่น ๆ
			utils.WriteErrorResponse(w, http.StatusBadRequest, "Bad Request", "Profile already exists for this user")
			return
		}
		utils.WriteErrorResponse(w, http.StatusInternalServerError, "Internal Server Error", err.Error())
		return
	}

	// 6) success — ตามสเปค
	var resp dto.ProfileCreateResponse
	resp.User.Username = username
	resp.Message = "Profile create successfully"
	utils.WriteJSONResponse(w, http.StatusOK, resp)
}

// วางใน struct/ไฟล์เดิม (ข้างๆ Create)
func (h *ProfileHandler) Handle(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		h.Create(w, r)
	case http.MethodGet:
		h.GetMe(w, r)
	case http.MethodPut: // <-- เพิ่ม
		h.Update(w, r)
	default:
		utils.WriteErrorResponse(w, http.StatusMethodNotAllowed, "Method Not Allowed", "only GET, POST are allowed")
	}
}

// GetMe godoc
// @Summary      Get my profile
// @Description  6.2 ดูโปรไฟล์ของตัวเอง (ต้องมี Bearer JWT)
// @Tags         profile
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  dto.ProfileGetResponse
// @Failure      401  {object}  dto.ErrorResponse
// @Failure      404  {object}  dto.ErrorResponse
// @Failure      500  {object}  dto.ErrorResponse
// @Router       /api/profile [get]
func (h *ProfileHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	// 1) auth
	userID, ok := userIDFromContext(r.Context())
	if !ok {
		utils.WriteErrorResponse(w, http.StatusUnauthorized, "Unauthorized", "missing user in context")
		return
	}

	// 2) query: join users + profiles
	const q = `
select
	u.id::text,
	p.username,
	u.email,
	p.first_name,
	p.last_name,
	p.display_name,
	p.avatar_url,
	p.phone,
	p.bio,
	p.birth_date, -- date
	p.food_preferences,
	p.chronic_disease,
	p.allergic_food,
	p.allergic_drugs,
	p.emergency_contact,
	u.role,
	u.created_at,
	u.updated_at
from public.users u
join public.profiles p on p.user_id = u.id
where u.id = $1
limit 1;
`
	ctx := r.Context()

	var (
		id, username, email, role       string
		firstName, lastName             *string
		displayName, avatarURL, phone   *string
		bio                             *string
		birthDateNullable               *time.Time
		foodPref, chronic, allergicFood *string
		allergicDrugs, emergencyContact *string
		createdAt, updatedAt            time.Time
	)

	err := h.pool.QueryRow(ctx, q, userID).Scan(
		&id,
		&username,
		&email,
		&firstName,
		&lastName,
		&displayName,
		&avatarURL,
		&phone,
		&bio,
		&birthDateNullable,
		&foodPref,
		&chronic,
		&allergicFood,
		&allergicDrugs,
		&emergencyContact,
		&role,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			utils.WriteErrorResponse(w, http.StatusNotFound, "Not Found", "Profile not found")
			return
		}
		utils.WriteErrorResponse(w, http.StatusInternalServerError, "Internal Server Error", err.Error())
		return
	}

	// 3) map -> DTO
	var resp dto.ProfileGetResponse
	resp.User.ID = id
	resp.User.Username = username
	resp.User.Email = email
	resp.User.FirstName = firstName
	resp.User.LastName = lastName
	resp.User.DisplayName = displayName
	resp.User.AvatarURL = avatarURL
	resp.User.Phone = phone
	resp.User.Bio = bio
	if birthDateNullable != nil {
		// ส่งเป็น "YYYY-MM-DD" (ตามตัวอย่าง)
		bd := birthDateNullable.Format("2006-01-02")
		resp.User.BirthDate = &bd
	}
	resp.User.FoodPreferences = foodPref
	resp.User.ChronicDisease = chronic
	resp.User.AllergicFood = allergicFood
	resp.User.AllergicDrugs = allergicDrugs
	resp.User.EmergencyContact = emergencyContact
	resp.User.Role = role
	resp.User.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	resp.User.UpdatedAt = updatedAt.UTC().Format(time.RFC3339)

	utils.WriteJSONResponse(w, http.StatusOK, resp)
}

// Update godoc
// @Summary      Update user profile
// @Description  6.3 อัปเดตโปรไฟล์ (ต้องมี Bearer JWT)
// @Tags         profile
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        payload  body      dto.ProfileUpdateRequest  true  "Profile update payload"
// @Success      200      {object}  dto.ProfileGetResponse
// @Failure      400      {object}  dto.ErrorResponse
// @Failure      401      {object}  dto.ErrorResponse
// @Failure      404      {object}  dto.ErrorResponse
// @Failure      409      {object}  dto.ErrorResponse
// @Failure      500      {object}  dto.ErrorResponse
// @Router       /api/profile [put]
func (h *ProfileHandler) Update(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		utils.WriteErrorResponse(w, http.StatusMethodNotAllowed, "Method Not Allowed", "only PUT is allowed")
		return
	}

	userID, ok := userIDFromContext(r.Context())
	if !ok {
		utils.WriteErrorResponse(w, http.StatusUnauthorized, "Unauthorized", "missing user in context")
		return
	}

	var req dto.ProfileUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	// สร้างชุด SET แบบไดนามิก (อัปเดตเฉพาะฟิลด์ที่ถูกส่งมา)
	set := []string{}
	args := []any{}
	i := 1

	addStr := func(col string, p *string, nullIfEmpty bool) {
		if p == nil {
			return
		}
		var v any = *p
		if nullIfEmpty && *p == "" {
			v = nil
		}
		set = append(set, fmt.Sprintf("%s = $%d", col, i))
		args = append(args, v)
		i++
	}

	// username (unique) — nullIfEmpty = false (ไม่อนุญาตให้ลบ username)
	if req.Username != nil {
		addStr("username", req.Username, false)
	}
	addStr("first_name", req.FirstName, true)
	addStr("last_name", req.LastName, true)
	addStr("display_name", req.DisplayName, true)
	addStr("avatar_url", req.AvatarURL, true)
	addStr("phone", req.Phone, true)
	addStr("bio", req.Bio, true)
	addStr("food_preferences", req.FoodPreferences, true)
	addStr("chronic_disease", req.ChronicDisease, true)
	addStr("allergic_food", req.AllergicFood, true)
	addStr("allergic_drugs", req.AllergicDrugs, true)
	addStr("emergency_contact", req.EmergencyContact, true)

	// birth_date: แปลงเป็น *time.Time หรือ NULL
	if req.BirthDate != nil {
		if *req.BirthDate == "" {
			set = append(set, fmt.Sprintf("birth_date = $%d", i))
			args = append(args, nil)
			i++
		} else {
			if t, err := time.Parse("2006-01-02", *req.BirthDate); err == nil {
				set = append(set, fmt.Sprintf("birth_date = $%d", i))
				args = append(args, t)
				i++
			} else if t2, err2 := time.Parse(time.RFC3339, *req.BirthDate); err2 == nil {
				set = append(set, fmt.Sprintf("birth_date = $%d", i))
				args = append(args, t2)
				i++
			} else {
				utils.WriteErrorResponse(w, http.StatusBadRequest, "Invalid request body", "birth_date must be ISO 8601 date or datetime")
				return
			}
		}
	}

	if len(set) == 0 {
		// ไม่ได้ส่งฟิลด์ใดมา
		utils.WriteErrorResponse(w, http.StatusBadRequest, "Bad Request", "no fields to update")
		return
	}

	ctx := r.Context()

	// อัปเดตโปรไฟล์ — ถ้าไม่มีแถว แปลว่า user นี้ยังไม่มีโปรไฟล์
	qUpdate := fmt.Sprintf(`update public.profiles set %s where user_id = $%d`, strings.Join(set, ", "), i)
	args = append(args, userID)

	ct, err := h.pool.Exec(ctx, qUpdate, args...)
	if err != nil {
		// จับ unique violation เช่น username ซ้ำ
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			if pgErr.ConstraintName == "profiles_username_key" {
				utils.WriteErrorResponse(w, http.StatusConflict, "Conflict", "username already taken")
				return
			}
		}
		utils.WriteErrorResponse(w, http.StatusInternalServerError, "Internal Server Error", err.Error())
		return
	}
	if ct.RowsAffected() == 0 {
		utils.WriteErrorResponse(w, http.StatusNotFound, "Not Found", "Profile not found")
		return
	}

	// select โปรไฟล์ล่าสุดเหมือน GetMe (เพื่อสร้าง response)
	const q = `
select
	u.id::text,
	p.username,
	u.email,
	p.first_name,
	p.last_name,
	p.display_name,
	p.avatar_url,
	p.phone,
	p.bio,
	p.birth_date, -- date
	p.food_preferences,
	p.chronic_disease,
	p.allergic_food,
	p.allergic_drugs,
	p.emergency_contact,
	u.role,
	u.created_at,
	u.updated_at
from public.users u
join public.profiles p on p.user_id = u.id
where u.id = $1
limit 1;
`
	var (
		id, username, email, role       string
		firstName, lastName             *string
		displayName, avatarURL, phone   *string
		bio                             *string
		birthDateNullable               *time.Time
		foodPref, chronic, allergicFood *string
		allergicDrugs, emergencyContact *string
		createdAt, updatedAt            time.Time
	)
	err = h.pool.QueryRow(ctx, q, userID).Scan(
		&id,
		&username,
		&email,
		&firstName,
		&lastName,
		&displayName,
		&avatarURL,
		&phone,
		&bio,
		&birthDateNullable,
		&foodPref,
		&chronic,
		&allergicFood,
		&allergicDrugs,
		&emergencyContact,
		&role,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			utils.WriteErrorResponse(w, http.StatusNotFound, "Not Found", "Profile not found")
			return
		}
		utils.WriteErrorResponse(w, http.StatusInternalServerError, "Internal Server Error", err.Error())
		return
	}

	var res dto.ProfileGetResponse
	res.User.ID = id
	res.User.Username = username
	res.User.Email = email
	res.User.FirstName = firstName
	res.User.LastName = lastName
	res.User.DisplayName = displayName
	res.User.AvatarURL = avatarURL
	res.User.Phone = phone
	res.User.Bio = bio
	if birthDateNullable != nil {
		bd := birthDateNullable.Format("2006-01-02")
		res.User.BirthDate = &bd
	}
	res.User.FoodPreferences = foodPref
	res.User.ChronicDisease = chronic
	res.User.AllergicFood = allergicFood
	res.User.AllergicDrugs = allergicDrugs
	res.User.EmergencyContact = emergencyContact
	res.User.Role = role
	res.User.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	res.User.UpdatedAt = updatedAt.UTC().Format(time.RFC3339)

	utils.WriteJSONResponse(w, http.StatusOK, map[string]any{
		"user":    res.User,
		"message": "Profile updated successfully",
	})
}

// ---------- helpers ----------

func nullable(p *string) *string {
	if p == nil || *p == "" {
		return nil
	}
	return p
}

func userIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	// ปรับ key ให้ตรงกับ AuthMiddleware ของโปรเจ็กต์คุณ
	// รองรับทั้ง "userID" และ "user_id" (string หรือ uuid.UUID)
	if v := ctx.Value("userID"); v != nil {
		switch t := v.(type) {
		case uuid.UUID:
			return t, true
		case string:
			if id, err := uuid.Parse(t); err == nil {
				return id, true
			}
		}
	}
	if v := ctx.Value("user_id"); v != nil {
		switch t := v.(type) {
		case uuid.UUID:
			return t, true
		case string:
			if id, err := uuid.Parse(t); err == nil {
				return id, true
			}
		}
	}
	return uuid.Nil, false
}
