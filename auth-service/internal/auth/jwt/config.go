package jwt

import "time"

// Config holds JWT signing and token lifetime settings.
type Config struct {
	Secret     string
	Issuer     string
	AccessTTL  time.Duration
	RefreshTTL time.Duration
}
