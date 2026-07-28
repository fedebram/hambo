package container

import "time"

// ServiceOption represents a function that modifies or extends the service.
type ServiceOption func(*Service)

// WithClock configures the function used by the service to get the current time.
func WithClock(now func() time.Time) ServiceOption {
	if now == nil {
		panic("service: clock cannot be nil")
	}

	return func(s *Service) {
		s.now = now
	}
}
