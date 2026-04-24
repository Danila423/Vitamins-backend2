package rabbitmq

import (
	"context"
	"fmt"
	"log/slog"

	amqp "github.com/rabbitmq/amqp091-go"
)

type HandlerFunc func(ctx context.Context, body []byte) error

type Consumer struct {
	conn   *amqp.Connection
	ch     *amqp.Channel
	queue  string
	logger *slog.Logger
}

func NewConsumer(url, queue string, logger *slog.Logger) (*Consumer, error) {
	return NewConsumerWithBindings(url, queue, "", nil, logger)
}

func NewConsumerWithBindings(url, queue, exchange string, routingKeys []string, logger *slog.Logger) (*Consumer, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("rabbitmq dial: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("rabbitmq channel: %w", err)
	}

	if exchange != "" {
		if err := ch.ExchangeDeclare(exchange, "topic", true, false, false, false, nil); err != nil {
			ch.Close()
			conn.Close()
			return nil, fmt.Errorf("declare exchange: %w", err)
		}
	}

	if _, err = ch.QueueDeclare(queue, true, false, false, false, nil); err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("declare queue: %w", err)
	}

	for _, key := range routingKeys {
		if err := ch.QueueBind(queue, key, exchange, false, nil); err != nil {
			ch.Close()
			conn.Close()
			return nil, fmt.Errorf("bind queue %s to %s: %w", queue, key, err)
		}
	}

	return &Consumer{
		conn:   conn,
		ch:     ch,
		queue:  queue,
		logger: logger,
	}, nil
}

func (c *Consumer) Consume(ctx context.Context, handler HandlerFunc) error {
	if err := c.ch.Qos(10, 0, false); err != nil {
		return fmt.Errorf("set qos: %w", err)
	}

	msgs, err := c.ch.ConsumeWithContext(ctx, c.queue, "", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("consume: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case d, ok := <-msgs:
			if !ok {
				return nil
			}
			if err := handler(ctx, d.Body); err != nil {
				c.logger.Error("handler failed",
					slog.String("queue", c.queue),
					slog.String("error", err.Error()),
				)
				_ = d.Nack(false, true)
			} else {
				_ = d.Ack(false)
			}
		}
	}
}

func (c *Consumer) Close() error {
	if err := c.ch.Close(); err != nil {
		return fmt.Errorf("close channel: %w", err)
	}
	if err := c.conn.Close(); err != nil {
		return fmt.Errorf("close connection: %w", err)
	}
	return nil
}
