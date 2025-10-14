# Email Setup Guide

## การตั้งค่าสำหรับการส่ง Email

### 1. Gmail SMTP Setup

#### วิธีที่ 1: ใช้ Gmail App Password (แนะนำ)

1. เปิด 2-Factor Authentication ใน Gmail
2. ไปที่ Google Account Settings > Security > App passwords
3. สร้าง App Password สำหรับ "Mail"
4. ใช้ App Password นี้แทนรหัสผ่านปกติ

#### วิธีที่ 2: ใช้ OAuth2 (สำหรับ Production)

1. สร้าง Google Cloud Project
2. เปิด Gmail API
3. สร้าง OAuth2 credentials
4. ใช้ OAuth2 flow สำหรับการส่ง email

### 2. Environment Variables

สร้างไฟล์ `.env` ในโฟลเดอร์ root และเพิ่มการตั้งค่าต่อไปนี้:

```env
# Email Configuration
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USERNAME=your-email@gmail.com
SMTP_PASSWORD=your-app-password
EMAIL_FROM=your-email@gmail.com
EMAIL_FROM_NAME=Go2gether Team
SMTP_USE_TLS=true
SMTP_USE_SSL=false
```

### 3. การทดสอบ Email

#### วิธีที่ 1: ใช้ API Endpoint

```bash
# ส่ง forgot password request
curl -X POST http://localhost:8080/api/auth/forgot-password \
  -H "Content-Type: application/json" \
  -d '{"email": "test@example.com"}'
```

#### วิธีที่ 2: ตรวจสอบ Logs

เมื่อ email ไม่ได้ตั้งค่า ระบบจะแสดง verification code ใน console:

```
Verification code for test@example.com: 123456 (expires in 3 minutes)
```

### 4. Alternative Email Services

#### SendGrid

```env
# ใช้ SendGrid แทน SMTP
SENDGRID_API_KEY=your-sendgrid-api-key
EMAIL_FROM=your-email@yourdomain.com
```

#### Mailgun

```env
# ใช้ Mailgun
MAILGUN_API_KEY=your-mailgun-api-key
MAILGUN_DOMAIN=your-domain.com
EMAIL_FROM=your-email@yourdomain.com
```

### 5. Troubleshooting

#### ปัญหาที่พบบ่อย

1. **"email credentials not configured"**
   - ตรวจสอบว่า `SMTP_USERNAME` และ `SMTP_PASSWORD` ถูกตั้งค่าแล้ว

2. **"failed to send email: 535 Authentication failed"**
   - ตรวจสอบว่าใช้ App Password แทนรหัสผ่านปกติ
   - ตรวจสอบว่า 2FA เปิดใช้งานแล้ว

3. **"connection refused"**
   - ตรวจสอบ `SMTP_HOST` และ `SMTP_PORT`
   - ตรวจสอบการเชื่อมต่ออินเทอร์เน็ต

4. **"timeout"**
   - ตรวจสอบ firewall settings
   - ลองเปลี่ยน port เป็น 465 (SSL) หรือ 587 (TLS)

#### การ Debug

เพิ่ม debug logging ใน `email_service.go`:

```go
// เพิ่มใน sendEmail function
log.Printf("Sending email from %s to %s via %s:%s", fromEmail, to, e.config.SMTPHost, e.config.SMTPPort)
```

### 6. Production Considerations

1. **ใช้ Environment Variables** แทน hardcode
2. **ใช้ Secret Management** (AWS Secrets Manager, HashiCorp Vault)
3. **ตั้งค่า Rate Limiting** เพื่อป้องกัน spam
4. **ใช้ Email Templates** สำหรับ HTML emails
5. **ตั้งค่า Monitoring** สำหรับ email delivery status

### 7. Security Best Practices

1. **ไม่เก็บ credentials ใน code**
2. **ใช้ HTTPS** สำหรับ production
3. **ตั้งค่า SPF, DKIM, DMARC** records
4. **ใช้ dedicated email service** สำหรับ production
5. **ตั้งค่า email validation** และ rate limiting
