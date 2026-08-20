package main

import (
	"crypto/tls"
	"fmt"
	"net/smtp"
	"strings"
)

func sendActivationEmail(toEmail, code string) error {
	smtpHost := AppConfig.SMTPHost
	smtpPort := AppConfig.SMTPPort
	fromEmail := AppConfig.FromEmail

	subject := "Subject: Your Activation Code\r\n"
	mime := "MIME-version: 1.0;\nContent-Type: text/html; charset=\"UTF-8\";\n\n"

	setupURL := fmt.Sprintf("https://%s/setup?code=%s", AppConfig.Domain, code)
	if strings.HasPrefix(AppConfig.Domain, "http://") || strings.HasPrefix(AppConfig.Domain, "https://") {
		setupURL = fmt.Sprintf("%s/setup?code=%s", AppConfig.Domain, code)
	}

	body := fmt.Sprintf(`
		<html>
		<body style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif; line-height: 1.6; color: #18181b; background-color: #fafafa; padding: 20px 0;">
			<div style="max-width: 520px; margin: 0 auto; padding: 32px; background: #ffffff; border: 1px solid #e4e4e7; border-radius: 12px; box-shadow: 0 1px 3px rgba(0,0,0,0.05);">
				<img src="https://%s/assets/usermount.svg" alt="Usermount Logo" style="height: 28px; margin-bottom: 24px; display: block;" />
				<h2 style="color: #09090b; font-size: 20px; font-weight: 600; margin: 0 0 12px 0;">Welcome to %s!</h2>
				<p style="color: #52525b; font-size: 14px; margin: 0 0 24px 0;">You have been invited to set up your account. Click the button below to complete your registration immediately:</p>
				
				<div style="text-align: center; margin: 28px 0;">
					<a href="%s" style="display: inline-block; background: #09090b; color: #ffffff; text-decoration: none; padding: 12px 28px; font-size: 14px; font-weight: 500; border-radius: 6px; box-shadow: 0 1px 2px rgba(0,0,0,0.08);">
						Set Up Account
					</a>
				</div>

				<div style="border-top: 1px solid #f4f4f5; margin: 24px 0; padding-top: 20px;">
					<p style="color: #71717a; font-size: 12px; margin: 0 0 8px 0;">Or manually enter this activation code at the activation portal:</p>
					<div style="font-size: 18px; font-family: monospace; font-weight: 600; color: #18181b; padding: 10px; background: #f4f4f5; border-radius: 6px; text-align: center; letter-spacing: 2px;">
						%s
					</div>
				</div>

				<p style="font-size: 12px; color: #a1a1aa; margin: 24px 0 0 0; text-align: center;">Note: This activation code and link will expire in 10 minutes.</p>
			</div>
		</body>
		</html>
		`, AppConfig.Domain, AppConfig.Name, setupURL, code)

	msg := []byte(subject + mime + body)
	addr := smtpHost + ":" + smtpPort

	c, err := smtp.Dial(addr)
	if err != nil {
		return fmt.Errorf("failed to connect to SMTP server: %w", err)
	}
	defer c.Close()

	if ok, _ := c.Extension("STARTTLS"); ok {
		config := &tls.Config{
			ServerName:         smtpHost,
			InsecureSkipVerify: AppConfig.SMTPSkipVerify,
		}
		if err = c.StartTLS(config); err != nil {
			return fmt.Errorf("failed to establish STARTTLS: %w", err)
		}
	}

	if AppConfig.SMTPUser != "" && AppConfig.SMTPPassword != "" {
		auth := smtp.PlainAuth("", AppConfig.SMTPUser, AppConfig.SMTPPassword, smtpHost)
		if err = c.Auth(auth); err != nil {
			return fmt.Errorf("failed to authenticate with SMTP server: %w", err)
		}
	}

	if err = c.Mail(fromEmail); err != nil {
		return fmt.Errorf("failed to set sender: %w", err)
	}
	if err = c.Rcpt(toEmail); err != nil {
		return fmt.Errorf("failed to set recipient: %w", err)
	}
	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("failed to initiate data transfer: %w", err)
	}
	_, err = w.Write(msg)
	if err != nil {
		return fmt.Errorf("failed to write message body: %w", err)
	}
	err = w.Close()
	if err != nil {
		return fmt.Errorf("failed to close data writer: %w", err)
	}
	_ = c.Quit()

	return nil
}
