package containerd

import "time"

const defaultStopGracePeriod = 10 * time.Second

type RuntimeOption func(*Runtime)

func WithStopGracePeriod(period time.Duration) RuntimeOption {
	if period <= 0 {
		panic("containerd runtime: stop grace period must be positive")
	}

	return func(r *Runtime) {
		r.stopGracePeriod = period
	}
}
