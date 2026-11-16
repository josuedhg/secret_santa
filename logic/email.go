package logic

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/smtp"
	"strings"

	"secret_santa/config"
	"secret_santa/models"
)

type Mailer struct {
	cfg *config.Config
}

func NewMailer(cfg *config.Config) *Mailer {
	return &Mailer{cfg: cfg}
}

// SendMatchNotification sends the secret santa match email to a participant
func (m *Mailer) SendMatchNotification(event models.Event, giver, receiver models.Participant) error {
	revealURL := m.cfg.FormatRevealLink(giver.InviteCode)

	if !m.cfg.HasSMTP() {
		log.Printf("[MOCK EMAIL] To: %s (%s) | Your match is: %s | Reveal URL: %s",
			giver.Name, giver.Email, receiver.Name, revealURL)
		return nil
	}

	if giver.Email == "" {
		return fmt.Errorf("participant %s has no email address", giver.Name)
	}

	subject := fmt.Sprintf("🎁 Your Secret Santa Match is Here! - %s", event.Title)

	wishlistSection := ""
	wishlistSectionText := ""
	if strings.TrimSpace(receiver.Wishlist) != "" {
		wishlistSection = fmt.Sprintf(`
			<div style="background: #fdf2f2; border-left: 4px solid #dc2626; padding: 12px 16px; margin: 16px 0; border-radius: 4px;">
				<strong style="color: #991b1b;">Gift Hints & Wishlist from %s:</strong>
				<p style="margin: 6px 0 0 0; color: #4b5563;">%s</p>
			</div>
		`, receiver.Name, receiver.Wishlist)
		wishlistSectionText = fmt.Sprintf("\nGift Hints & Wishlist from %s:\n%s\n", receiver.Name, receiver.Wishlist)
	}

	budgetSection := ""
	budgetSectionText := ""
	if strings.TrimSpace(event.Budget) != "" {
		budgetSection = fmt.Sprintf("<p style='margin: 8px 0; color: #4b5563;'><strong>Budget / Spending Limit:</strong> %s</p>", event.Budget)
		budgetSectionText = fmt.Sprintf("Budget / Spending Limit: %s\n", event.Budget)
	}

	bodyHTML := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<style>
  body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif; background-color: #f3f4f6; margin: 0; padding: 24px; }
  .card { max-width: 580px; margin: 0 auto; background: #ffffff; border-radius: 12px; overflow: hidden; box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.1); border: 1px solid #e5e7eb; }
  .header { background: linear-gradient(135deg, #dc2626 0%%, #15803d 100%%); color: white; padding: 28px; text-align: center; }
  .header h1 { margin: 0 0 8px 0; font-size: 24px; font-weight: 700; }
  .header p { margin: 0; opacity: 0.95; font-size: 15px; }
  .content { padding: 32px 24px; }
  .match-box { background: #f0fdf4; border: 2px dashed #16a34a; border-radius: 8px; padding: 20px; text-align: center; margin: 24px 0; }
  .match-name { font-size: 26px; font-weight: 800; color: #15803d; margin: 8px 0; }
  .btn { display: inline-block; background-color: #dc2626; color: #ffffff !important; text-decoration: none; padding: 12px 28px; border-radius: 8px; font-weight: 600; margin-top: 16px; }
  .footer { font-size: 13px; color: #9ca3af; text-align: center; padding: 16px 24px 24px; }
</style>
</head>
<body>
  <div class="card">
    <div class="header">
      <h1>🎅 Ho Ho Ho! 🎁</h1>
      <p>%s</p>
    </div>
    <div class="content">
      <p style="font-size: 16px; color: #1f2937;">Hi <strong>%s</strong>,</p>
      <p style="color: #4b5563; line-height: 1.5;">The Secret Santa draw has taken place! You have been randomly assigned to give a holiday gift to:</p>
      
      <div class="match-box">
        <div style="font-size: 14px; text-transform: uppercase; letter-spacing: 1px; color: #166534; font-weight: 600;">You are Secret Santa for</div>
        <div class="match-name">🎁 %s 🎁</div>
      </div>

      %s
      %s

      <div style="text-align: center; margin-top: 24px;">
        <p style="color: #4b5563; font-size: 14px; margin-bottom: 8px;">Keep it a surprise! You can also view your assignment online:</p>
        <a href="%s" class="btn">View Your Match Online</a>
      </div>
    </div>
    <div class="footer">
      Remember: It's a secret! Don't tell %s that you drew their name! 🤫
    </div>
  </div>
</body>
</html>`,
		event.Title,
		giver.Name,
		receiver.Name,
		wishlistSection,
		budgetSection,
		revealURL,
		receiver.Name,
	)

	bodyPlain := fmt.Sprintf(`Ho Ho Ho! Secret Santa Draw Results

Hi %s,

The Secret Santa draw for "%s" has taken place!

You are Secret Santa for:
>>> %s <<<
%s%s
Keep it a surprise! You can also check your match anytime at:
%s

Happy Holidays!
`, giver.Name, event.Title, receiver.Name, wishlistSectionText, budgetSectionText, revealURL)

	return m.sendEmail(giver.Email, subject, bodyPlain, bodyHTML)
}

func (m *Mailer) sendEmail(to, subject, textBody, htmlBody string) error {
	addr := fmt.Sprintf("%s:%d", m.cfg.SMTPHost, m.cfg.SMTPPort)

	boundary := "---NextPart_SecretSanta_12345---"
	headers := make(map[string]string)
	headers["From"] = m.cfg.SMTPFrom
	headers["To"] = to
	headers["Subject"] = subject
	headers["MIME-Version"] = "1.0"
	headers["Content-Type"] = fmt.Sprintf("multipart/alternative; boundary=\"%s\"", boundary)

	var message strings.Builder
	for k, v := range headers {
		message.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}
	message.WriteString("\r\n")

	// Plain text section
	message.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	message.WriteString("Content-Type: text/plain; charset=\"utf-8\"\r\n")
	message.WriteString("Content-Transfer-Encoding: 7bit\r\n\r\n")
	message.WriteString(textBody)
	message.WriteString("\r\n\r\n")

	// HTML section
	message.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	message.WriteString("Content-Type: text/html; charset=\"utf-8\"\r\n")
	message.WriteString("Content-Transfer-Encoding: 7bit\r\n\r\n")
	message.WriteString(htmlBody)
	message.WriteString("\r\n\r\n")

	message.WriteString(fmt.Sprintf("--%s--\r\n", boundary))

	var auth smtp.Auth
	if m.cfg.SMTPUser != "" {
		auth = smtp.PlainAuth("", m.cfg.SMTPUser, m.cfg.SMTPPass, m.cfg.SMTPHost)
	}

	// Try STARTTLS on standard submission port or direct connection
	if m.cfg.SMTPPort == 465 {
		// SSL/TLS connection
		tlsConfig := &tls.Config{
			InsecureSkipVerify: false,
			ServerName:         m.cfg.SMTPHost,
		}
		conn, err := tls.Dial("tcp", addr, tlsConfig)
		if err != nil {
			return fmt.Errorf("tls dial failed: %w", err)
		}
		defer conn.Close()

		client, err := smtp.NewClient(conn, m.cfg.SMTPHost)
		if err != nil {
			return fmt.Errorf("smtp client failed: %w", err)
		}
		defer client.Close()

		if auth != nil {
			if ok, _ := client.Extension("AUTH"); ok {
				if err := client.Auth(auth); err != nil {
					return fmt.Errorf("smtp auth failed: %w", err)
				}
			}
		}

		if err := client.Mail(m.cfg.SMTPFrom); err != nil {
			return err
		}
		if err := client.Rcpt(to); err != nil {
			return err
		}
		w, err := client.Data()
		if err != nil {
			return err
		}
		if _, err := w.Write([]byte(message.String())); err != nil {
			return err
		}
		return w.Close()
	}

	// Standard smtp.SendMail with STARTTLS
	return smtp.SendMail(addr, auth, m.cfg.SMTPFrom, []string{to}, []byte(message.String()))
}
