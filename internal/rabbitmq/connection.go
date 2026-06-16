package rabbitmq

import (
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
)

const (
	reconnectDelay = 5 * time.Second
	maxRetries     = 5
)

// Connection wraps an AMQP connection with reconnect logic.
type Connection struct {
	conn    *amqp.Connection
	url     string
	log     *zap.Logger
}

// NewConnection dials RabbitMQ and returns a managed connection.
func NewConnection(url string, log *zap.Logger) (*Connection, error) {
	c := &Connection{url: url, log: log}
	if err := c.connect(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Connection) connect() error {
	var err error
	for i := 0; i < maxRetries; i++ {
		c.conn, err = amqp.Dial(c.url)
		if err == nil {
			c.log.Info("connected to RabbitMQ")
			return nil
		}
		c.log.Warn("rabbitmq connection failed, retrying",
			zap.Int("attempt", i+1),
			zap.Error(err),
		)
		time.Sleep(reconnectDelay)
	}
	return fmt.Errorf("rabbitmq: failed to connect after %d attempts: %w", maxRetries, err)
}

// Channel opens a new AMQP channel.
func (c *Connection) Channel() (*amqp.Channel, error) {
	if c.conn == nil || c.conn.IsClosed() {
		if err := c.connect(); err != nil {
			return nil, err
		}
	}
	return c.conn.Channel()
}

// Close gracefully closes the underlying connection.
func (c *Connection) Close() error {
	if c.conn != nil && !c.conn.IsClosed() {
		return c.conn.Close()
	}
	return nil
}
