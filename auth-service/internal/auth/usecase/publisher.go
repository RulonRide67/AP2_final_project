package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/madiyar/final-project/auth-service/internal/domain/entity"
)

// EventPublisher publishes domain events to NATS.
type EventPublisher interface {
	PublishUserCreated(ctx context.Context, user *entity.User) error
	PublishUserVerified(ctx context.Context, user *entity.User) error
}

// UserEvent is the JSON payload for auth user lifecycle events.
type UserEvent struct {
	UserID     string    `json:"user_id"`
	Email      string    `json:"email"`
	Username   string    `json:"username"`
	OccurredAt time.Time `json:"occurred_at"`
}

// NATSEventPublisher publishes events using a NATS connection.
type NATSEventPublisher struct {
	nc     *nats.Conn
	prefix string
}

// NewNATSEventPublisher creates a publisher with subjects like "{prefix}.user.created".
func NewNATSEventPublisher(nc *nats.Conn, subjectPrefix string) *NATSEventPublisher {
	return &NATSEventPublisher{nc: nc, prefix: subjectPrefix}
}

func (p *NATSEventPublisher) PublishUserCreated(ctx context.Context, user *entity.User) error {
	return p.publish(ctx, "user.created", user)
}

func (p *NATSEventPublisher) PublishUserVerified(ctx context.Context, user *entity.User) error {
	return p.publish(ctx, "user.verified", user)
}

func (p *NATSEventPublisher) publish(ctx context.Context, event string, user *entity.User) error {
	if p == nil || p.nc == nil {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	payload, err := json.Marshal(UserEvent{
		UserID:     user.ID.String(),
		Email:      user.Email,
		Username:   user.Username,
		OccurredAt: time.Now().UTC(),
	})
	if err != nil {
		return fmt.Errorf("marshal user event: %w", err)
	}

	subject := fmt.Sprintf("%s.%s", p.prefix, event)
	if err := p.nc.Publish(subject, payload); err != nil {
		return fmt.Errorf("publish %s: %w", subject, err)
	}
	return nil
}

// NoopEventPublisher discards events (useful in tests).
type NoopEventPublisher struct{}

func (NoopEventPublisher) PublishUserCreated(context.Context, *entity.User) error { return nil }
func (NoopEventPublisher) PublishUserVerified(context.Context, *entity.User) error { return nil }
