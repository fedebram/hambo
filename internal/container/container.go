package container

import "time"

type Container struct {
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}
