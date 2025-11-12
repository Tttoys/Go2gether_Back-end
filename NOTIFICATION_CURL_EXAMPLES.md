# Notification API - cURL Examples

## Prerequisites
- Replace `YOUR_JWT_TOKEN` with your actual JWT token
- Replace `NOTIFICATION_ID` with actual notification UUID
- Base URL: `http://localhost:8080` (adjust if different)

---

## 1. List Notifications

### 1.1 Get all notifications (default: 20 items)
```bash
curl -X GET "http://localhost:8080/api/notifications" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json"
```

### 1.2 Get all notifications with pagination
```bash
curl -X GET "http://localhost:8080/api/notifications?limit=50&offset=0" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json"
```

### 1.3 Get only unread notifications
```bash
curl -X GET "http://localhost:8080/api/notifications?unread_only=true" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json"
```

### 1.4 Filter by notification type
```bash
# Filter by member_joined type
curl -X GET "http://localhost:8080/api/notifications?type=member_joined" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json"

# Filter by trip_update type
curl -X GET "http://localhost:8080/api/notifications?type=trip_update" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json"

# Filter by availability_updated type
curl -X GET "http://localhost:8080/api/notifications?type=availability_updated" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json"
```

### 1.5 Combined filters (unread + type + pagination)
```bash
curl -X GET "http://localhost:8080/api/notifications?unread_only=true&type=member_joined&limit=10&offset=0" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json"
```

### 1.6 Get maximum items (100)
```bash
curl -X GET "http://localhost:8080/api/notifications?limit=100" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json"
```

---

## 2. Mark Notification as Read

### 2.1 Mark single notification as read
```bash
curl -X POST "http://localhost:8080/api/notifications/NOTIFICATION_ID/read" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json"
```

**Example with actual UUID:**
```bash
curl -X POST "http://localhost:8080/api/notifications/509c0de9-162b-4dca-89c4-dda12ce5ec61/read" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json"
```

**Expected Response:**
```json
{
  "message": "Notification marked as read"
}
```

---

## 3. Mark All Notifications as Read

### 3.1 Mark all unread notifications as read
```bash
curl -X POST "http://localhost:8080/api/notifications/read-all" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json"
```

**Expected Response:**
```json
{
  "message": "All notifications marked as read",
  "updated_count": 5
}
```

---

## Notification Types

Valid notification types for filtering:
- `trip_invitation` - Trip invitation
- `invitation_accepted` - Invitation accepted
- `invitation_declined` - Invitation declined
- `trip_update` - Trip update
- `availability_updated` - Availability updated
- `member_joined` - Member joined trip
- `member_left` - Member left trip

---

## Example Response: List Notifications

```json
{
  "notifications": [
    {
      "id": "509c0de9-162b-4dca-89c4-dda12ce5ec61",
      "type": "member_joined",
      "title": "มีสมาชิกเข้าร่วมทริป",
      "message": "ผู้ใช้ 123e4567-e89b-12d3-a456-426614174000 เข้าร่วมทริป ทริปไปเที่ยวเกาะสมุย แล้ว",
      "data": {
        "trip_id": "123e4567-e89b-12d3-a456-426614174000",
        "user_id": "123e4567-e89b-12d3-a456-426614174001",
        "role": "member",
        "tripName": "ทริปไปเที่ยวเกาะสมุย"
      },
      "action_url": "http://localhost:8081/trips/123e4567-e89b-12d3-a456-426614174000",
      "read": false,
      "created_at": "2025-11-12T13:42:10Z"
    }
  ],
  "pagination": {
    "total": 15,
    "unread_count": 8,
    "limit": 20,
    "offset": 0
  }
}
```

---

## Error Responses

### 401 Unauthorized
```json
{
  "error": "Unauthorized",
  "message": "Invalid user context"
}
```

### 400 Bad Request (Invalid limit)
```json
{
  "error": "Invalid limit",
  "message": "limit must be a positive integer"
}
```

### 400 Bad Request (Invalid type)
```json
{
  "error": "Invalid type",
  "message": "invalid notification type"
}
```

### 404 Not Found
```json
{
  "error": "Not Found",
  "message": "Notification not found"
}
```

### 403 Forbidden
```json
{
  "error": "Forbidden",
  "message": "Notification not found or already marked as read"
}
```

---

## Quick Test Script

```bash
#!/bin/bash

# Set your JWT token
JWT_TOKEN="YOUR_JWT_TOKEN"
BASE_URL="http://localhost:8080"

echo "=== 1. List all notifications ==="
curl -X GET "${BASE_URL}/api/notifications" \
  -H "Authorization: Bearer ${JWT_TOKEN}" \
  -H "Content-Type: application/json" | jq

echo -e "\n=== 2. List unread notifications ==="
curl -X GET "${BASE_URL}/api/notifications?unread_only=true" \
  -H "Authorization: Bearer ${JWT_TOKEN}" \
  -H "Content-Type: application/json" | jq

echo -e "\n=== 3. Mark all as read ==="
curl -X POST "${BASE_URL}/api/notifications/read-all" \
  -H "Authorization: Bearer ${JWT_TOKEN}" \
  -H "Content-Type: application/json" | jq
```

---

## Using with jq (for pretty JSON output)

```bash
# List notifications with pretty output
curl -X GET "http://localhost:8080/api/notifications" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" | jq

# List unread with pretty output
curl -X GET "http://localhost:8080/api/notifications?unread_only=true" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" | jq '.notifications[] | {id, type, title, read}'
```

---

## Notes

- All endpoints require JWT authentication via `Authorization: Bearer` header
- Default limit is 20, maximum is 100
- Default offset is 0
- `unread_only=true` filters only unread notifications
- Type filter is case-sensitive
- Mark read operations only affect notifications belonging to the authenticated user

