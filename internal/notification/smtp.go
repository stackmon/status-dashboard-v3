package notification

import (
	"context"
	"fmt"

	mail "github.com/wneessen/go-mail"
)

// Sender delivers one rendered email to one recipient. It is an interface so the
// worker and tests can substitute a fake without touching a real SMTP server.
type Sender interface {
	Send(ctx context.Context, recipient string, email Email) error
}

// smtpSender sends mail directly to the configured (OTC) SMTP endpoint using
// github.com/wneessen/go-mail.
type smtpSender struct {
	client *mail.Client
	from   string
}

// NewSMTPSender builds a reusable SMTP sender from the parsed config.
func NewSMTPSender(cfg Config) (Sender, error) {
	opts := []mail.Option{
		mail.WithPort(cfg.Port),
		mail.WithTimeout(cfg.Timeout),
		mail.WithTLSPolicy(tlsPolicy(cfg.TLS)),
	}
	if cfg.User != "" {
		opts = append(opts,
			mail.WithSMTPAuth(mail.SMTPAuthPlain),
			mail.WithUsername(cfg.User),
			mail.WithPassword(cfg.Password),
		)
	} else {
		// Relay without credentials (e.g. a local catcher) must not negotiate AUTH.
		opts = append(opts, mail.WithSMTPAuth(mail.SMTPAuthNoAuth))
	}

	client, err := mail.NewClient(cfg.Host, opts...)
	if err != nil {
		return nil, fmt.Errorf("build smtp client: %w", err)
	}

	return &smtpSender{client: client, from: cfg.From}, nil
}

// Send composes and delivers a single message. The context bounds the whole
// dial+send so a hung server cannot exceed the lease.
func (s *smtpSender) Send(ctx context.Context, recipient string, email Email) error {
	msg := mail.NewMsg()
	if err := msg.From(s.from); err != nil {
		return fmt.Errorf("set from: %w", err)
	}
	if err := msg.To(recipient); err != nil {
		return fmt.Errorf("set recipient: %w", err)
	}
	msg.Subject(email.Subject)
	msg.SetBodyString(mail.TypeTextPlain, email.Body)

	if err := s.client.DialAndSendWithContext(ctx, msg); err != nil {
		return fmt.Errorf("send mail: %w", err)
	}
	return nil
}

// tlsPolicy selects mandatory TLS when configured, otherwise opportunistic.
func tlsPolicy(enabled bool) mail.TLSPolicy {
	if enabled {
		return mail.TLSMandatory
	}
	return mail.TLSOpportunistic
}
