package mailer

import (
	"context"
	"fmt"
	"log/slog"

	"vitamins-backend_2/pkg/rabbitmq"
)

// eventPublisher is the minimal publisher contract we need, so we don't depend
// on the concrete *rabbitmq.Publisher type and can mock it in tests.
type eventPublisher interface {
	Publish(ctx context.Context, exchange, routingKey string, msg any) error
}

// PublisherMailer implements service.Mailer by publishing a message to
// RabbitMQ instead of delivering it synchronously. The actual e-mail delivery
// is performed by the notifier worker that consumes these events.
type PublisherMailer struct {
	pub    eventPublisher
	logger *slog.Logger
}

func NewPublisherMailer(pub eventPublisher, logger *slog.Logger) *PublisherMailer {
	return &PublisherMailer{pub: pub, logger: logger}
}

func (m *PublisherMailer) SendOneTimeCode(ctx context.Context, toEmail, subject, code string) error {
	if m == nil || m.pub == nil {
		return fmt.Errorf("rabbitmq publisher not configured")
	}

	routingKey := routingKeyForSubject(subject)
	evt := rabbitmq.PasswordResetEvent{
		Email:   toEmail,
		Subject: subject,
		Code:    code,
	}

	if err := m.pub.Publish(ctx, rabbitmq.ExchangeEvents, routingKey, evt); err != nil {
		return fmt.Errorf("publish mail event: %w", err)
	}
	return nil
}

func routingKeyForSubject(subject string) string {
	switch subject {
	case "Password change code":
		return rabbitmq.RoutingKeyPasswordChangeRequested
	default:
		return rabbitmq.RoutingKeyPasswordResetRequested
	}
}
