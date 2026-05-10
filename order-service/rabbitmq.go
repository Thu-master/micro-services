package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	amqp "github.com/rabbitmq/amqp091-go"
)

// RabbitMQClient handles RabbitMQ operations
type RabbitMQClient struct {
	conn      *amqp.Connection
	channel   *amqp.Channel
	exchange  string
	queueName string
}

// NewRabbitMQClient creates a new RabbitMQ client
func NewRabbitMQClient(uri, exchange, queueName string) (*RabbitMQClient, error) {
	conn, err := amqp.Dial(uri)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to open channel: %w", err)
	}

	// Declare exchange
	err = ch.ExchangeDeclare(
		exchange,       // name
		"direct",       // type
		true,           // durable
		false,          // auto-deleted
		false,          // internal
		false,          // no-wait
		nil,            // arguments
	)
	if err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("failed to declare exchange: %w", err)
	}

	// Declare queue
	_, err = ch.QueueDeclare(
		queueName, // name
		true,      // durable
		false,     // delete when usused
		false,     // exclusive
		false,     // no-wait
		nil,       // arguments
	)
	if err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("failed to declare queue: %w", err)
	}

	return &RabbitMQClient{
		conn:      conn,
		channel:   ch,
		exchange:  exchange,
		queueName: queueName,
	}, nil
}

// Publish publishes a message to RabbitMQ
func (r *RabbitMQClient) Publish(ctx context.Context, routingKey string, message interface{}) error {
	body, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	return r.channel.PublishWithContext(
		ctx,
		r.exchange,    // exchange
		routingKey,    // routing key
		false,         // mandatory
		false,         // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
		},
	)
}

// BindQueue binds a queue to exchange
func (r *RabbitMQClient) BindQueue(routingKey string) error {
	return r.channel.QueueBind(
		r.queueName,  // queue name
		routingKey,   // routing key
		r.exchange,   // exchange
		false,        // no-wait
		nil,          // arguments
	)
}

// Consume starts consuming messages
func (r *RabbitMQClient) Consume(ctx context.Context, handler func([]byte) error) error {
	msgs, err := r.channel.Consume(
		r.queueName, // queue
		"",          // consumer
		false,       // auto-ack
		false,       // exclusive
		false,       // no-local
		false,       // no-wait
		nil,         // args
	)
	if err != nil {
		return fmt.Errorf("failed to consume: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case msg, ok := <-msgs:
			if !ok {
				return fmt.Errorf("channel closed")
			}
			if err := handler(msg.Body); err != nil {
				log.Printf("Handler error: %v", err)
				msg.Nack(false, true) // requeue
			} else {
				msg.Ack(false)
			}
		}
	}
}

// Close closes the RabbitMQ connection
func (r *RabbitMQClient) Close() error {
	if r.channel != nil {
		r.channel.Close()
	}
	if r.conn != nil {
		return r.conn.Close()
	}
	return nil
}
