# CURL Examples สำหรับ Go2gether Backend API

## Authentication

### 1. Register (สมัครสมาชิก)
```bash
curl -X POST http://localhost:8080/api/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "user@example.com",
    "password": "password123"
  }'
```

### 2. Login (เข้าสู่ระบบ)
```bash
curl -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "user@example.com",
    "password": "password123"
  }'
```

**Response จะได้ JWT token:**
```json
{
  "user": {
    "id": "...",
    "email": "user@example.com",
    ...
  },
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```

**บันทึก token ไว้ใช้ในคำสั่งต่อไป:**
```bash
export TOKEN="eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
```

---

## Profile

### 3. สร้างโปรไฟล์ (Create Profile)
```bash
curl -X POST http://localhost:8080/api/profile \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "username": "johndoe",
    "first_name": "John",
    "last_name": "Doe",
    "display_name": "Johnny",
    "bio": "Travel enthusiast",
    "phone": "+66123456789"
  }'
```

### 4. ดูโปรไฟล์ตัวเอง (Get My Profile)
```bash
curl -X GET http://localhost:8080/api/profile \
  -H "Authorization: Bearer $TOKEN"
```

### 5. อัปเดตโปรไฟล์ (Update Profile)
```bash
curl -X PUT http://localhost:8080/api/profile \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "display_name": "Johnny Updated",
    "bio": "Updated bio"
  }'
```

---

## Trips

### 6. สร้างทริป (Create Trip)
```bash
curl -X POST http://localhost:8080/api/trips \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "name": "Trip to Japan",
    "destination": "Tokyo, Japan",
    "start_date": "2025-12-01",
    "end_date": "2025-12-10",
    "description": "Amazing trip to Japan",
    "status": "published",
    "total_budget": 50000,
    "currency": "THB"
  }'
```

**บันทึก trip_id ไว้:**
```bash
export TRIP_ID="<trip_id_from_response>"
```

### 7. ดูรายการทริป (List Trips)
```bash
# ดูทริปทั้งหมด
curl -X GET "http://localhost:8080/api/trips" \
  -H "Authorization: Bearer $TOKEN"

# ดูทริปตาม status
curl -X GET "http://localhost:8080/api/trips?status=published&limit=10&offset=0" \
  -H "Authorization: Bearer $TOKEN"
```

### 8. ดูรายละเอียดทริป (Get Trip Detail)
```bash
curl -X GET "http://localhost:8080/api/trips/$TRIP_ID" \
  -H "Authorization: Bearer $TOKEN"
```

### 9. อัปเดตทริป (Update Trip)
```bash
curl -X PUT "http://localhost:8080/api/trips/$TRIP_ID" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "name": "Updated Trip to Japan",
    "start_month": "2025-12",
    "end_month": "2025-12",
    "total_budget": 60000
  }'
```

### 10. ลบทริป (Delete Trip)
```bash
curl -X DELETE "http://localhost:8080/api/trips/$TRIP_ID" \
  -H "Authorization: Bearer $TOKEN"
```

---

## Invitations (ลิงก์เชิญ)

### 11. สร้างลิงก์เชิญ (Generate Invitation Link)
```bash
curl -X POST "http://localhost:8080/api/trips/$TRIP_ID/invitations" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{}'
```

**Response:**
```json
{
  "invitation_link": "http://localhost:8081/trips/{trip_id}/join?token=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "expires_at": "2025-11-26T10:00:00Z",
  "message": "Invitation link generated successfully. Share this link to invite members to your trip."
}
```

**บันทึก invitation_token:**
```bash
export INVITATION_TOKEN="<token_from_link>"
```

### 12. เข้าร่วมทริปผ่านลิงก์ (Join Trip via Link)
```bash
curl -X POST http://localhost:8080/api/trips/join \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "invitation_token": "$INVITATION_TOKEN"
  }'
```

**Response:**
```json
{
  "message": "Successfully joined the trip",
  "trip": {
    "id": "...",
    "name": "Trip to Japan",
    "destination": "Tokyo, Japan"
  },
  "member": {
    "user_id": "...",
    "role": "member",
    "status": "accepted",
    "joined_at": "2025-10-27T10:00:00Z"
  }
}
```

---

## Invitations

### 13. ดูรายการคำเชิญ (List Invitations) - เฉพาะ creator
```bash
curl -X GET "http://localhost:8080/api/trips/$TRIP_ID/invitations" \
  -H "Authorization: Bearer $TOKEN"
```

### 14. ออกจากทริป (Leave Trip)
```bash
curl -X POST "http://localhost:8080/api/trips/$TRIP_ID/leave" \
  -H "Authorization: Bearer $TOKEN"
```

### 17. ลบสมาชิก (Remove Member) - เฉพาะ creator
```bash
export USER_ID_TO_REMOVE="<user_id>"
curl -X DELETE "http://localhost:8080/api/trips/$TRIP_ID/members/$USER_ID_TO_REMOVE" \
  -H "Authorization: Bearer $TOKEN"
```

---

## Password Reset

### 18. ขอรีเซ็ตรหัสผ่าน (Forgot Password)
```bash
curl -X POST http://localhost:8080/api/auth/forgot-password \
  -H "Content-Type: application/json" \
  -d '{
    "email": "user@example.com"
  }'
```

### 19. ตรวจสอบ OTP (Verify OTP)
```bash
curl -X POST http://localhost:8080/api/auth/verify-otp \
  -H "Content-Type: application/json" \
  -d '{
    "email": "user@example.com",
    "code": "123456"
  }'
```

### 20. รีเซ็ตรหัสผ่าน (Reset Password)
```bash
export RESET_TOKEN="<reset_token_from_verify_otp>"
curl -X POST http://localhost:8080/api/auth/reset-password \
  -H "Content-Type: application/json" \
  -d '{
    "reset_token": "$RESET_TOKEN",
    "new_password": "newPassword123"
  }'
```

### 21. ดู OTP (Get OTP) - สำหรับ development
```bash
curl -X POST http://localhost:8080/api/auth/get-otp \
  -H "Content-Type: application/json" \
  -d '{
    "email": "user@example.com"
  }'
```

---

## Health Checks

### 22. Health Check
```bash
curl -X GET http://localhost:8080/healthz
```

### 23. Liveness Check
```bash
curl -X GET http://localhost:8080/livez
```

### 24. Readiness Check
```bash
curl -X GET http://localhost:8080/readyz
```

---

## ตัวอย่างการใช้งานแบบเต็ม (Full Flow)

### สร้าง User 1 (Creator)
```bash
# 1. Register
curl -X POST http://localhost:8080/api/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email": "creator@example.com", "password": "password123"}'

# 2. Login และบันทึก token
export CREATOR_TOKEN="<token_from_response>"

# 3. สร้างโปรไฟล์
curl -X POST http://localhost:8080/api/profile \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $CREATOR_TOKEN" \
  -d '{"username": "creator", "first_name": "Creator", "last_name": "User"}'

# 4. สร้างทริป
curl -X POST http://localhost:8080/api/trips \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $CREATOR_TOKEN" \
  -d '{
    "name": "Amazing Trip",
    "destination": "Bangkok",
    "start_date": "2025-12-01",
    "end_date": "2025-12-10",
    "status": "published"
  }'

# 5. บันทึก trip_id
export TRIP_ID="<trip_id_from_response>"

# 6. สร้างลิงก์เชิญ
curl -X POST "http://localhost:8080/api/trips/$TRIP_ID/invitations" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $CREATOR_TOKEN" \
  -d '{}'
```

### สร้าง User 2 (Member) และเข้าร่วมผ่านลิงก์
```bash
# 1. Register
curl -X POST http://localhost:8080/api/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email": "member@example.com", "password": "password123"}'

# 2. Login และบันทึก token
export MEMBER_TOKEN="<token_from_response>"

# 3. สร้างโปรไฟล์
curl -X POST http://localhost:8080/api/profile \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $MEMBER_TOKEN" \
  -d '{"username": "member", "first_name": "Member", "last_name": "User"}'

# 4. เข้าร่วมทริปผ่านลิงก์ (ใช้ invitation_token จากลิงก์)
curl -X POST http://localhost:8080/api/trips/join \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $MEMBER_TOKEN" \
  -d "{\"invitation_token\": \"$INVITATION_TOKEN\"}"
```

---

## หมายเหตุ

1. **JWT Token**: ทุก endpoint ที่ต้อง authentication จะต้องมี `Authorization: Bearer <token>` ใน header
2. **FRONTEND_URL**: ตั้งค่า environment variable `FRONTEND_URL` สำหรับลิงก์เชิญ (default: `http://localhost:8081`)
3. **Invitation Token**: หมดอายุใน 30 วัน
4. **Base URL**: เปลี่ยน `http://localhost:8080` เป็น URL ของ server จริงถ้าจำเป็น

