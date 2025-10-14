// สร้างไฟล์ internal/utils/email_service.go
// ตัวอย่างการส่ง email ด้วย Gmail SMTP

package utils

import (
	"fmt"
	"net/smtp"

	"GO2GETHER_BACK-END/internal/config"
)

// EmailService handles email sending operations
type EmailService struct {
	config *config.EmailConfig
}

// NewEmailService creates a new email service instance
func NewEmailService(cfg *config.EmailConfig) *EmailService {
	return &EmailService{
		config: cfg,
	}
}

// SendVerificationCode sends verification code to user's email
func (e *EmailService) SendVerificationCode(to, code string) error {
	subject := "Password Reset Verification Code"
	body := fmt.Sprintf(`
Hello,

You requested to reset your password for Go2gether.

Your verification code is: %s

This code will expire in 3 minutes.

If you didn't request this, please ignore this email.

Best regards,
Go2gether Team
	`, code)

	return e.sendEmail(to, subject, body)
}

// sendEmail sends an email using SMTP
func (e *EmailService) sendEmail(to, subject, body string) error {
	// Check if credentials are set
	if e.config.SMTPUsername == "" || e.config.SMTPPassword == "" {
		return fmt.Errorf("email credentials not configured")
	}

	// Setup authentication
	auth := smtp.PlainAuth("", e.config.SMTPUsername, e.config.SMTPPassword, e.config.SMTPHost)

	// Compose message
	fromEmail := e.config.FromEmail
	if fromEmail == "" {
		fromEmail = e.config.SMTPUsername
	}

	message := []byte(fmt.Sprintf(
		"From: %s <%s>\r\n"+
			"To: %s\r\n"+
			"Subject: %s\r\n"+
			"\r\n"+
			"%s\r\n",
		e.config.FromName, fromEmail, to, subject, body))

	// Send email
	addr := e.config.SMTPHost + ":" + e.config.SMTPPort
	err := smtp.SendMail(addr, auth, fromEmail, []string{to}, message)
	if err != nil {
		return fmt.Errorf("failed to send email: %v", err)
	}

	return nil
}

// ===== Alternative: SendGrid Implementation =====

/*
import (
	"github.com/sendgrid/sendgrid-go"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
)

type SendGridEmailService struct {
	apiKey string
	from   string
}

func NewSendGridEmailService() *SendGridEmailService {
	return &SendGridEmailService{
		apiKey: os.Getenv("SENDGRID_API_KEY"),
		from:   os.Getenv("EMAIL_FROM"),
	}
}

func (s *SendGridEmailService) SendVerificationCode(to, code string) error {
	from := mail.NewEmail("Go2gether", s.from)
	subject := "Password Reset Verification Code"
	toEmail := mail.NewEmail("", to)

	plainTextContent := fmt.Sprintf(`
Hello,

You requested to reset your password for Go2gether.

Your verification code is: %s

This code will expire in 3 minutes.

If you didn't request this, please ignore this email.

Best regards,
Go2gether Team
	`, code)

	htmlContent := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<body style="font-family: Arial, sans-serif; line-height: 1.6; color: #333;">
	<div style="max-width: 600px; margin: 0 auto; padding: 20px;">
		<h2 style="color: #4CAF50;">Password Reset Request</h2>
		<p>Hello,</p>
		<p>You requested to reset your password for Go2gether.</p>
		<div style="background-color: #f4f4f4; padding: 20px; margin: 20px 0; border-radius: 5px; text-align: center;">
			<h1 style="color: #4CAF50; margin: 0; font-size: 32px; letter-spacing: 5px;">%s</h1>
			<p style="margin: 10px 0 0 0; color: #666; font-size: 14px;">Verification Code</p>
		</div>
		<p style="color: #d32f2f; font-weight: bold;">⏰ This code will expire in 3 minutes.</p>
		<p>If you didn't request this, please ignore this email.</p>
		<hr style="border: none; border-top: 1px solid #eee; margin: 30px 0;">
		<p style="color: #999; font-size: 12px;">Best regards,<br>Go2gether Team</p>
	</div>
</body>
</html>
	`, code)

	message := mail.NewSingleEmail(from, subject, toEmail, plainTextContent, htmlContent)
	client := sendgrid.NewSendClient(s.apiKey)

	response, err := client.Send(message)
	if err != nil {
		return fmt.Errorf("failed to send email: %v", err)
	}

	if response.StatusCode >= 400 {
		return fmt.Errorf("sendgrid returned error status: %d", response.StatusCode)
	}

	return nil
}
*/

// ===== How to use in forgot_password.go =====

/*
// In forgot_password.go, add email service to handler:

type ForgotPasswordHandler struct {
	db           *pgxpool.Pool
	emailService *utils.EmailService
}

func NewForgotPasswordHandler(db *pgxpool.Pool) *ForgotPasswordHandler {
	return &ForgotPasswordHandler{
		db:           db,
		emailService: utils.NewEmailService(),
	}
}

// Then in ForgotPassword function, replace the TODO:

// Send verification code via email
err = h.emailService.SendVerificationCode(req.Email, code)
if err != nil {
	utils.WriteErrorResponse(w, http.StatusInternalServerError, "Failed to send email", err.Error())
	return
}
*/
