package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type Publisher struct {
	conn   *amqp.Connection
	ch     *amqp.Channel
	mu     sync.Mutex
	url    string
	logger *slog.Logger
}

func NewPublisher(url string, logger *slog.Logger) (*Publisher, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("rabbitmq dial: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("rabbitmq channel: %w", err)
	}

	if err := ch.ExchangeDeclare(ExchangeEvents, "topic", true, false, false, false, nil); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, fmt.Errorf("declare exchange: %w", err)
	}

	return &Publisher{
		conn:   conn,
		ch:     ch,
		url:    url,
		logger: logger,
	}, nil
}

func (p *Publisher) Publish(ctx context.Context, exchange, routingKey string, msg any) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	err = p.ch.PublishWithContext(ctx, exchange, routingKey, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Body:         body,
		Timestamp:    time.Now(),
	})
	if err != nil {
		return fmt.Errorf("publish message: %w", err)
	}

	p.logger.Info("published message",
		slog.String("exchange", exchange),
		slog.String("routing_key", routingKey),
	)

	return nil
}

func (p *Publisher) Close() error {
	if err := p.ch.Close(); err != nil {
		return fmt.Errorf("close channel: %w", err)
	}
	if err := p.conn.Close(); err != nil {
		return fmt.Errorf("close connection: %w", err)
	}
	return nil
}
