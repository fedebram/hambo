package client

import (
	"errors"
	"net/url"
)

func ParseServerURL(value string) (*url.URL, error) {
	u, err := url.Parse(value)
	if err != nil {
		return nil, errors.New("invalid URL")
	}
	if u.Scheme != "https" || u.Hostname() == "" {
		return nil, errors.New("must use HTTPS and include a hostname")
	}
	if u.User != nil {
		return nil, errors.New("must not contain credentials")
	}
	return u, nil
}
