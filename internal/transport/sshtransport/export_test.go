package sshtransport

import "time"

// WithTimeout shortens a test connection's TCP and handshake deadline.
func WithTimeout(config Config, timeout time.Duration) Config {
	config.timeout = timeout
	return config
}
