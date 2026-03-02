package postgres

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joshjon/kit/log"

	"verve/internal/task"
)

const pgChannel = "task_events"

// EventNotifier implements task.Notifier using PostgreSQL LISTEN/NOTIFY.
type EventNotifier struct {
	pool   *pgxpool.Pool
	logger log.Logger
}

// NewEventNotifier creates a new EventNotifier backed by the given pgx pool.
func NewEventNotifier(pool *pgxpool.Pool, logger log.Logger) *EventNotifier {
	return &EventNotifier{pool: pool, logger: logger}
}

// Notify sends a payload via PG NOTIFY.
func (n *EventNotifier) Notify(ctx context.Context, payload []byte) error {
	_, err := n.pool.Exec(ctx, "SELECT pg_notify($1, $2)", pgChannel, string(payload))
	return err
}

// Listen blocks and listens for PG notifications on the task_events channel,
// calling broker.Receive for each event. It reconnects automatically on
// connection errors.
func (n *EventNotifier) Listen(ctx context.Context, broker *task.Broker) {
	for {
		if err := n.listen(ctx, broker); err != nil {
			if ctx.Err() != nil {
				return
			}
			n.logger.Error("pg listen error, reconnecting", "error", err)
			select {
			case <-time.After(time.Second):
			case <-ctx.Done():
				return
			}
		}
	}
}

func (n *EventNotifier) listen(ctx context.Context, broker *task.Broker) error {
	conn, err := n.pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, "LISTEN "+pgChannel); err != nil {
		return err
	}

	n.logger.Info("pg listen started", "pg.channel", pgChannel)

	for {
		notification, err := conn.Conn().WaitForNotification(ctx)
		if err != nil {
			return err
		}

		var event task.Event
		if err := json.Unmarshal([]byte(notification.Payload), &event); err != nil {
			n.logger.Error("unmarshal pg notification", "error", err)
			continue
		}

		broker.Receive(event)
	}
}
