package adapters

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"time"

	"github.com/combi/pkg/domain/integration"
	"github.com/combi/pkg/domain/notification"
	"github.com/combi/pkg/log"
)

var _ notification.EmailProvider = (*SMTPAdapter)(nil)

type SMTPAdapter struct {
	logger   log.Logger
	host     string
	port     string
	username string
	password string
	auth     smtp.Auth
}

func NewSMTPAdapter(credentials map[string]string, logger log.Logger) (*SMTPAdapter, error) {
	host, ok := credentials["host"]
	if !ok || host == "" {
		return nil, fmt.Errorf("%w: SMTP adapter requires 'host'", integration.ErrMissingCredentials)
	}
	port, ok := credentials["port"]
	if !ok || port == "" {
		return nil, fmt.Errorf("%w: SMTP adapter requires 'port'", integration.ErrMissingCredentials)
	}
	username, ok := credentials["username"]
	if !ok {
		return nil, fmt.Errorf("%w: SMTP adapter requires 'username'", integration.ErrMissingCredentials)
	}
	password, ok := credentials["password"]
	if !ok {
		return nil, fmt.Errorf("%w: SMTP adapter requires 'password'", integration.ErrMissingCredentials)
	}

	return &SMTPAdapter{
		logger:   logger.With(log.String("adapter", "smtp")),
		host:     host,
		port:     port,
		username: username,
		password: password,
		auth:     smtp.PlainAuth("", username, password, host),
	}, nil
}

func (a *SMTPAdapter) Send(ctx context.Context, req notification.SendRequest) (*notification.SendResult, error) {
	toList := make([]string, len(req.To))
	for i, to := range req.To {
		toList[i] = to.Email.String()
	}

	msg := a.buildMessage(req, toList)
	addr := net.JoinHostPort(a.host, a.port)

	// Standard Go SMTP handling.
	// Note: net/smtp.SendMail handles the connection, HELLO, STARTTLS, AUTH, MAIL, RCPT, DATA, QUIT sequence.

	// Handle ports: 587 (STARTTLS), 465 (Implicit TLS), 25 (Unencrypted/STARTTLS)
	var err error
	if a.port == "465" {
		err = a.sendMailTLS(addr, req.From.Email.String(), toList, []byte(msg))
	} else {
		// Default behavior (works for 587 and 25 with STARTTLS supported by server)
		err = smtp.SendMail(addr, a.auth, req.From.Email.String(), toList, []byte(msg))
	}

	if err != nil {
		return nil, fmt.Errorf("%w: %w", integration.ErrProviderError, err)
	}

	// SMTP is fire-and-forget regarding message IDs in the standard protocol return.
	// We generate a local ID or parse the server response if we implemented a custom client.
	// For simplicity with standard lib, we assume success implies queued at server.
	providerID := fmt.Sprintf("smtp-%d", time.Now().UnixNano())

	return &notification.SendResult{
		ProviderID: providerID,
	}, nil
}

func (a *SMTPAdapter) buildMessage(req notification.SendRequest, toList []string) string {
	// Basic MIME construction
	var msg strings.Builder
	msg.WriteString(fmt.Sprintf("From: %s <%s>\r\n", req.From.Name, req.From.Email.String()))
	msg.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(toList, ",")))
	msg.WriteString(fmt.Sprintf("Subject: %s\r\n", req.Subject))
	msg.WriteString("MIME-version: 1.0;\n")
	msg.WriteString("Content-Type: text/html; charset=\"UTF-8\";\n\r\n")
	msg.WriteString(req.HTMLBody)
	msg.WriteString("\r\n")
	return msg.String()
}

// sendMailTLS implements SMTPS (Implicit TLS on port 465) which standard lib doesn't support directly via SendMail
func (a *SMTPAdapter) sendMailTLS(addr string, from string, to []string, msg []byte) error {
	// Explicitly using ServerName in config is safer
	tlsConfig := &tls.Config{ServerName: a.host}
	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		return err
	}

	c, err := smtp.NewClient(conn, a.host)
	if err != nil {
		_ = conn.Close()
		return err
	}
	defer func() { _ = c.Close() }()

	if err = c.Auth(a.auth); err != nil {
		return err
	}
	if err = c.Mail(from); err != nil {
		return err
	}
	for _, addr := range to {
		if err = c.Rcpt(addr); err != nil {
			return err
		}
	}
	w, err := c.Data()
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
	return c.Quit()
}

func (a *SMTPAdapter) TestConnection(ctx context.Context) error {
	addr := net.JoinHostPort(a.host, a.port)
	timeout := 5 * time.Second

	var conn net.Conn
	var err error

	// Handle connection type based on port
	if a.port == "465" {
		// Implicit TLS
		conn, err = tls.DialWithDialer(&net.Dialer{Timeout: timeout}, "tcp", addr, &tls.Config{ServerName: a.host})
	} else {
		// Plain TCP (will upgrade to STARTTLS if supported)
		conn, err = net.DialTimeout("tcp", addr, timeout)
	}

	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}

	// Establish SMTP Client
	c, err := smtp.NewClient(conn, a.host)
	if err != nil {
		_ = conn.Close()
		return err
	}
	defer func() { _ = c.Close() }()

	// If not implicit TLS, try STARTTLS upgrade
	if a.port != "465" {
		// Hello is usually called automatically by NewClient, but required before StartTLS if we want to ensure state
		if err := c.Hello("localhost"); err != nil {
			return err
		}
		if ok, _ := c.Extension("STARTTLS"); ok {
			if err := c.StartTLS(&tls.Config{ServerName: a.host}); err != nil {
				return fmt.Errorf("starttls failed: %w", err)
			}
		}
	}

	if err := c.Auth(a.auth); err != nil {
		return fmt.Errorf("auth failed: %w", err)
	}

	return c.Quit()
}
