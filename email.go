package main

import (
	"crypto/tls"
	"fmt"
	"net/smtp"
)

func sendActivationEmail(toEmail, code string) error {
	smtpHost := AppConfig.SMTPHost
	smtpPort := AppConfig.SMTPPort
	fromEmail := AppConfig.FromEmail

	subject := "Subject: Your Activation Code\r\n"
	mime := "MIME-version: 1.0;\nContent-Type: text/html; charset=\"UTF-8\";\n\n"

	body := fmt.Sprintf(`
		<html>
		<body style="font-family: sans-serif; line-height: 1.6; color: #333;">
			<div style="max-width: 600px; margin: 0 auto; padding: 20px; border: 1px solid #eaeaea; border-radius: 10px;">
				<img src="https://%s/assets/usermount.svg" alt="Usermount Logo" style="height: 32px; margin-bottom: 16px; display: block;" />
				<h2 style="color: #000; margin-top: 0;">Welcome to %s!</h2>
				<p>Your activation code is:</p>
				<div style="font-size: 24px; font-weight: bold; padding: 15px; background: #f5f5f5; border-radius: 8px; text-align: center; letter-spacing: 2px;">
					%s
				</div>
				<p style="margin-top: 20px;">Please enter this code at the activation portal to setup your account.</p>
				<a href="%s">Activation Page</a>
				<p style="font-size: 12px; color: #999;">Note: This activation code will expire in 10 minutes.</p>
			</div>
		</body>
		</html>
		`, AppConfig.Domain, AppConfig.Name, code, fmt.Sprintf("https://%s/activate", AppConfig.Domain))

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
