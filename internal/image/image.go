package image

type Image struct {
	Name      string `json:"name"`
	Digest    string `json:"digest"`
	SizeBytes int64  `json:"size_bytes"`
}

type PullProgress struct {
	Event        string `json:"event"`
	Name         string `json:"name,omitempty"`
	Digest       string `json:"digest,omitempty"`
	CurrentBytes int64  `json:"current_bytes"`
	TotalBytes   int64  `json:"total_bytes"`
}

type PullProgressFunc func(PullProgress)
