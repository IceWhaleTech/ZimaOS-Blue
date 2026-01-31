package builtin

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/smtp"
	"strings"
	"sync"

	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/skill"
)

// EmailConfig holds email configuration
type EmailConfig struct {
	SMTPHost     string `json:"smtp_host"`
	SMTPPort     int    `json:"smtp_port"`
	SMTPUser     string `json:"smtp_user"`
	SMTPPassword string `json:"smtp_password"`
	IMAPHost     string `json:"imap_host"`
	IMAPPort     int    `json:"imap_port"`
	IMAPUser     string `json:"imap_user"`
	IMAPPassword string `json:"imap_password"`
	FromAddress  string `json:"from_address"`
	UseTLS       bool   `json:"use_tls"`
}

// Email is a built-in email skill
type Email struct {
	manifest *skill.Manifest
	config   *EmailConfig
	mu       sync.RWMutex
}

// NewEmail creates a new email skill
func NewEmail(config *EmailConfig) *Email {
	return &Email{
		manifest: &skill.Manifest{
			ID:          "email",
			Name:        "Email",
			Version:     "1.0.0",
			Description: "Send and manage emails via SMTP/IMAP",
			Category:    "communication",
			Icon:        "email",
			Tags:        []string{"email", "smtp", "imap", "communication"},
			Inputs: []skill.Parameter{
				{
					Name:        "action",
					Type:        "string",
					Description: "Action: send, list, read, delete, config",
					Required:    true,
				},
				{
					Name:        "to",
					Type:        "string",
					Description: "Recipient email address (for send)",
					Required:    false,
				},
				{
					Name:        "subject",
					Type:        "string",
					Description: "Email subject (for send)",
					Required:    false,
				},
				{
					Name:        "body",
					Type:        "string",
					Description: "Email body (for send)",
					Required:    false,
				},
				{
					Name:        "cc",
					Type:        "string",
					Description: "CC recipients (comma-separated)",
					Required:    false,
				},
				{
					Name:        "bcc",
					Type:        "string",
					Description: "BCC recipients (comma-separated)",
					Required:    false,
				},
				{
					Name:        "folder",
					Type:        "string",
					Description: "Email folder (for list/read)",
					Required:    false,
					Default:     "INBOX",
				},
				{
					Name:        "limit",
					Type:        "number",
					Description: "Number of emails to list",
					Required:    false,
					Default:     10,
				},
				{
					Name:        "id",
					Type:        "string",
					Description: "Email ID (for read/delete)",
					Required:    false,
				},
			},
			Outputs: []skill.Parameter{
				{
					Name:        "sent",
					Type:        "boolean",
					Description: "Whether email was sent",
				},
				{
					Name:        "emails",
					Type:        "array",
					Description: "List of emails",
				},
				{
					Name:        "email",
					Type:        "object",
					Description: "Email details",
				},
			},
			Permissions: []string{"email.send", "email.read"},
		},
		config: config,
	}
}

// Manifest returns the skill manifest
func (e *Email) Manifest() *skill.Manifest {
	return e.manifest
}

// Validate validates the input parameters
func (e *Email) Validate(input map[string]any) error {
	action, ok := input["action"]
	if !ok {
		return fmt.Errorf("action is required")
	}

	actionStr, ok := action.(string)
	if !ok {
		return fmt.Errorf("action must be a string")
	}

	validActions := map[string]bool{
		"send": true, "list": true, "read": true, "delete": true, "config": true,
	}
	if !validActions[actionStr] {
		return fmt.Errorf("invalid action: %s", actionStr)
	}

	if actionStr == "send" {
		if _, ok := input["to"]; !ok {
			return fmt.Errorf("to is required for send action")
		}
		if _, ok := input["subject"]; !ok {
			return fmt.Errorf("subject is required for send action")
		}
		if _, ok := input["body"]; !ok {
			return fmt.Errorf("body is required for send action")
		}
	}

	if actionStr == "read" || actionStr == "delete" {
		if _, ok := input["id"]; !ok {
			return fmt.Errorf("id is required for %s action", actionStr)
		}
	}

	return nil
}

// Execute executes the email skill
func (e *Email) Execute(ctx context.Context, input map[string]any) (*skill.Result, error) {
	action := input["action"].(string)

	switch action {
	case "send":
		return e.sendEmail(ctx, input)
	case "list":
		return e.listEmails(ctx, input)
	case "read":
		return e.readEmail(ctx, input)
	case "delete":
		return e.deleteEmail(ctx, input)
	case "config":
		return e.getConfig()
	}

	return skill.NewErrorResult(fmt.Errorf("unknown action: %s", action)), nil
}

func (e *Email) sendEmail(ctx context.Context, input map[string]any) (*skill.Result, error) {
	e.mu.RLock()
	config := e.config
	e.mu.RUnlock()

	if config == nil || config.SMTPHost == "" {
		return skill.NewResult(map[string]any{
			"sent":    false,
			"message": "Email not configured. Please configure SMTP settings.",
		}), nil
	}

	to := input["to"].(string)
	subject := input["subject"].(string)
	body := input["body"].(string)

	// Build recipients list
	recipients := []string{to}
	if cc, ok := input["cc"].(string); ok && cc != "" {
		recipients = append(recipients, strings.Split(cc, ",")...)
	}
	if bcc, ok := input["bcc"].(string); ok && bcc != "" {
		recipients = append(recipients, strings.Split(bcc, ",")...)
	}

	// Build message
	from := config.FromAddress
	if from == "" {
		from = config.SMTPUser
	}

	msg := fmt.Sprintf("From: %s\r\n", from)
	msg += fmt.Sprintf("To: %s\r\n", to)
	if cc, ok := input["cc"].(string); ok && cc != "" {
		msg += fmt.Sprintf("Cc: %s\r\n", cc)
	}
	msg += fmt.Sprintf("Subject: %s\r\n", subject)
	msg += "MIME-Version: 1.0\r\n"
	msg += "Content-Type: text/plain; charset=\"utf-8\"\r\n"
	msg += "\r\n"
	msg += body

	// Send email
	addr := fmt.Sprintf("%s:%d", config.SMTPHost, config.SMTPPort)
	auth := smtp.PlainAuth("", config.SMTPUser, config.SMTPPassword, config.SMTPHost)

	var err error
	if config.UseTLS {
		err = e.sendWithTLS(addr, auth, from, recipients, []byte(msg), config)
	} else {
		err = smtp.SendMail(addr, auth, from, recipients, []byte(msg))
	}

	if err != nil {
		return skill.NewResult(map[string]any{
			"sent":    false,
			"error":   err.Error(),
			"message": fmt.Sprintf("Failed to send email: %v", err),
		}), nil
	}

	return skill.NewResult(map[string]any{
		"sent":    true,
		"to":      to,
		"subject": subject,
		"message": fmt.Sprintf("Email sent to %s", to),
	}), nil
}

func (e *Email) sendWithTLS(addr string, auth smtp.Auth, from string, to []string, msg []byte, config *EmailConfig) error {
	tlsConfig := &tls.Config{
		ServerName: config.SMTPHost,
	}

	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		return err
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, config.SMTPHost)
	if err != nil {
		return err
	}
	defer client.Close()

	if err = client.Auth(auth); err != nil {
		return err
	}

	if err = client.Mail(from); err != nil {
		return err
	}

	for _, recipient := range to {
		if err = client.Rcpt(strings.TrimSpace(recipient)); err != nil {
			return err
		}
	}

	w, err := client.Data()
	if err != nil {
		return err
	}

	_, err = w.Write(msg)
	if err != nil {
		return err
	}

	err = w.Close()
	if err != nil {
		return err
	}

	return client.Quit()
}

func (e *Email) listEmails(ctx context.Context, input map[string]any) (*skill.Result, error) {
	e.mu.RLock()
	config := e.config
	e.mu.RUnlock()

	if config == nil || config.IMAPHost == "" {
		return skill.NewResult(map[string]any{
			"emails":  []map[string]any{},
			"message": "IMAP not configured. Please configure IMAP settings to list emails.",
		}), nil
	}

	folder := "INBOX"
	if f, ok := input["folder"].(string); ok {
		folder = f
	}

	limit := 10
	if l, ok := input["limit"].(float64); ok {
		limit = int(l)
	}

	// Placeholder: In production, implement actual IMAP connection
	return skill.NewResult(map[string]any{
		"emails":  []map[string]any{},
		"folder":  folder,
		"limit":   limit,
		"message": "IMAP listing placeholder. Configure IMAP for actual email listing.",
	}), nil
}

func (e *Email) readEmail(ctx context.Context, input map[string]any) (*skill.Result, error) {
	id := input["id"].(string)

	// Placeholder: In production, implement actual IMAP read
	return skill.NewResult(map[string]any{
		"email":   nil,
		"id":      id,
		"message": "IMAP read placeholder. Configure IMAP for actual email reading.",
	}), nil
}

func (e *Email) deleteEmail(ctx context.Context, input map[string]any) (*skill.Result, error) {
	id := input["id"].(string)

	// Placeholder: In production, implement actual IMAP delete
	return skill.NewResult(map[string]any{
		"deleted": false,
		"id":      id,
		"message": "IMAP delete placeholder. Configure IMAP for actual email deletion.",
	}), nil
}

func (e *Email) getConfig() (*skill.Result, error) {
	e.mu.RLock()
	config := e.config
	e.mu.RUnlock()

	if config == nil {
		return skill.NewResult(map[string]any{
			"configured": false,
			"message":    "Email not configured",
		}), nil
	}

	return skill.NewResult(map[string]any{
		"configured":   true,
		"smtp_host":    config.SMTPHost,
		"smtp_port":    config.SMTPPort,
		"imap_host":    config.IMAPHost,
		"imap_port":    config.IMAPPort,
		"from_address": config.FromAddress,
		"use_tls":      config.UseTLS,
	}), nil
}

// SetConfig updates the email configuration
func (e *Email) SetConfig(config *EmailConfig) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.config = config
}
