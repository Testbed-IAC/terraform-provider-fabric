package fabricclient

import (
	"errors"
	"net/http"
)

var (
	ErrNotFound         = errors.New("fabricclient: resource not found")
	ErrPermissionDenied = errors.New("fabricclient: permission denied")
)

func mapHTTPErr(resp *http.Response, err error) error {
	if resp == nil {
		return err
	}
	switch resp.StatusCode {
	case http.StatusNotFound:
		return ErrNotFound
	case http.StatusForbidden, http.StatusUnauthorized:
		return ErrPermissionDenied
	default:
		return err
	}
}
