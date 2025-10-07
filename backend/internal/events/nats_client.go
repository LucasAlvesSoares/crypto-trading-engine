package events

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/sirupsen/logrus"
)

// NATSClient wraps the NATS connection with helper methods
type NATSClient struct {
	conn   *nats.Conn
	logger *logrus.Logger
}

// NewNATSClient creates a new NATS client
func NewNATSClient(url string, logger *logrus.Logger) (*NATSClient, error) {
	opts := []nats.Option{
		nats.Name("crypto-trading-bot"),
		nats.MaxReconnects(-1), // Infinite reconnects
		nats.ReconnectWait(2 * time.Second),
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			if err != nil {
				logger.WithError(err).Warn("NATS disconnected")
			}
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			logger.Info("NATS reconnected")
		}),
		nats.ClosedHandler(func(nc *nats.Conn) {
			logger.Warn("NATS connection closed")
		}),
	}

	conn, err := nats.Connect(url, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	logger.Info("Connected to NATS")

	return &NATSClient{
		conn:   conn,
		logger: logger,
	}, nil
}

// Publish publishes an event to NATS
func (nc *NATSClient) Publish(eventType EventType, data interface{}) error {
	event, err := NewEvent(eventType, data)
	if err != nil {
		return fmt.Errorf("failed to create event: %w", err)
	}

	eventBytes, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	subject := string(eventType)
	if err := nc.conn.Publish(subject, eventBytes); err != nil {
		return fmt.Errorf("failed to publish event: %w", err)
	}

	nc.logger.WithFields(logrus.Fields{
		"event_id":   event.ID,
		"event_type": eventType,
		"subject":    subject,
	}).Debug("Published event")

	return nil
}

// Subscribe subscribes to a subject with a handler
func (nc *NATSClient) Subscribe(subject string, handler func(*Event) error) (*nats.Subscription, error) {
	sub, err := nc.conn.Subscribe(subject, func(msg *nats.Msg) {
		var event Event
		if err := json.Unmarshal(msg.Data, &event); err != nil {
			nc.logger.WithError(err).Error("Failed to unmarshal event")
			return
		}

		nc.logger.WithFields(logrus.Fields{
			"event_id":   event.ID,
			"event_type": event.Type,
			"subject":    msg.Subject,
		}).Debug("Received event")

		if err := handler(&event); err != nil {
			nc.logger.WithError(err).WithFields(logrus.Fields{
				"event_id":   event.ID,
				"event_type": event.Type,
			}).Error("Failed to handle event")
		}
	})

	if err != nil {
		return nil, fmt.Errorf("failed to subscribe: %w", err)
	}

	nc.logger.WithField("subject", subject).Info("Subscribed to subject")

	return sub, nil
}

// QueueSubscribe subscribes to a subject with a queue group
func (nc *NATSClient) QueueSubscribe(subject, queue string, handler func(*Event) error) (*nats.Subscription, error) {
	sub, err := nc.conn.QueueSubscribe(subject, queue, func(msg *nats.Msg) {
		var event Event
		if err := json.Unmarshal(msg.Data, &event); err != nil {
			nc.logger.WithError(err).Error("Failed to unmarshal event")
			return
		}

		nc.logger.WithFields(logrus.Fields{
			"event_id":   event.ID,
			"event_type": event.Type,
			"subject":    msg.Subject,
			"queue":      queue,
		}).Debug("Received event")

		if err := handler(&event); err != nil {
			nc.logger.WithError(err).WithFields(logrus.Fields{
				"event_id":   event.ID,
				"event_type": event.Type,
			}).Error("Failed to handle event")
		}
	})

	if err != nil {
		return nil, fmt.Errorf("failed to queue subscribe: %w", err)
	}

	nc.logger.WithFields(logrus.Fields{
		"subject": subject,
		"queue":   queue,
	}).Info("Subscribed to subject with queue")

	return sub, nil
}

// Request sends a request and waits for a response
func (nc *NATSClient) Request(subject string, data interface{}, timeout time.Duration) (*Event, error) {
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal data: %w", err)
	}

	msg, err := nc.conn.Request(subject, dataBytes, timeout)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	var event Event
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &event, nil
}

// Close closes the NATS connection
func (nc *NATSClient) Close() {
	if nc.conn != nil {
		nc.conn.Close()
		nc.logger.Info("NATS connection closed")
	}
}

// Drain drains the connection
func (nc *NATSClient) Drain() error {
	if nc.conn != nil {
		return nc.conn.Drain()
	}
	return nil
}

// IsConnected returns true if connected to NATS
func (nc *NATSClient) IsConnected() bool {
	return nc.conn != nil && nc.conn.IsConnected()
}

// WaitForConnection waits for the connection to be established
func (nc *NATSClient) WaitForConnection(ctx context.Context) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if nc.IsConnected() {
				return nil
			}
		}
	}
}
