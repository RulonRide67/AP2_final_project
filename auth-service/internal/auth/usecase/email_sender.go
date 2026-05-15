package usecase

import "context"

// EmailSender delivers transactional emails.
type EmailSender interface {
	SendVerificationEmail(ctx context.Context, to, verifyURL string) error
	SendPasswordResetEmail(ctx context.Context, to, resetURL string) error
}
