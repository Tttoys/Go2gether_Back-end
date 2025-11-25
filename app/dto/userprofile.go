package dto

// รับ Body จาก POST /api/profile
type ProfileCreateRequest struct {
	Username         string  `json:"username"`
	FirstName        *string `json:"first_name"`
	LastName         *string `json:"last_name"`
	DisplayName      *string `json:"display_name"`
	AvatarURL        *string `json:"avatar_url"`
	Phone            *string `json:"phone"`
	Bio              *string `json:"bio"`
	BirthDate        *string `json:"birth_date"` // "YYYY-MM-DD" หรือ RFC3339
	FoodPreferences  *string `json:"food_preferences"`
	ChronicDisease   *string `json:"chronic_disease"`
	AllergicFood     *string `json:"allergic_food"`
	AllergicDrugs    *string `json:"allergic_drugs"`
	EmergencyContact *string `json:"emergency_contact"`
}

// ตอบกลับตามสเปค
type ProfileCreateResponse struct {
	User struct {
		Username string `json:"username"`
	} `json:"user"`
	Message string `json:"message"`
}

// ตอบกลับจาก GET /api/profile
type ProfileGetResponse struct {
	User struct {
		ID               string  `json:"id"`
		Username         string  `json:"username"`
		Email            string  `json:"email"`
		FirstName        *string `json:"first_name"`
		LastName         *string `json:"last_name"`
		DisplayName      *string `json:"display_name"`
		AvatarURL        *string `json:"avatar_url"`
		Phone            *string `json:"phone"`
		Bio              *string `json:"bio"`
		BirthDate        *string `json:"birth_date"` // จะส่งเป็น "YYYY-MM-DD"
		FoodPreferences  *string `json:"food_preferences"`
		ChronicDisease   *string `json:"chronic_disease"`
		AllergicFood     *string `json:"allergic_food"`
		AllergicDrugs    *string `json:"allergic_drugs"`
		EmergencyContact *string `json:"emergency_contact"`
		Role             string  `json:"role"`
		CreatedAt        string  `json:"created_at"` // RFC3339
		UpdatedAt        string  `json:"updated_at"` // RFC3339 (ใช้ users.updated_at)
	} `json:"user"`
}

type ProfileUpdateRequest struct {
	Username         *string `json:"username"`
	FirstName        *string `json:"first_name"`
	LastName         *string `json:"last_name"`
	DisplayName      *string `json:"display_name"`
	AvatarURL        *string `json:"avatar_url"`        // "" => NULL
	Phone            *string `json:"phone"`             // "" => NULL
	Bio              *string `json:"bio"`               // "" => NULL
	BirthDate        *string `json:"birth_date"`        // "" => NULL, else "YYYY-MM-DD" or RFC3339
	FoodPreferences  *string `json:"food_preferences"`  // "" => NULL
	ChronicDisease   *string `json:"chronic_disease"`   // "" => NULL
	AllergicFood     *string `json:"allergic_food"`     // "" => NULL
	AllergicDrugs    *string `json:"allergic_drugs"`    // "" => NULL
	EmergencyContact *string `json:"emergency_contact"` // "" => NULL
}

// ProfileCheckResponse สำหรับ GET /api/profile/check
type ProfileCheckResponse struct {
	Exists  bool   `json:"exists"`
	Message string `json:"message"`
}
