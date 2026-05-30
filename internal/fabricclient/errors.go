package fabricclient

import (
	"errors"
	"fmt"
	"io"
	"net/http"

	openapi "github.com/Testbed-IAC/fabric-orchestrator-go-client"
)

var (
	ErrNotFound     = errors.New("fabricclient: resource not found (404)")
	ErrUnauthorized = errors.New("fabricclient: unauthorized — check your FABRIC token (401)")
	ErrForbidden    = errors.New("fabricclient: forbidden: check project permissions (403)")
	ErrBadRequest   = errors.New("fabricclient: bad request — GraphML or parameters rejected by orchestrator (400)")
	ErrServerError  = errors.New("fabricclient: orchestrator internal server error (500)")
)

// mapHTTPErr converts an orchestrator API error into a meaningful provider error.
// It extracts the HTTP status code and response body from GenericOpenAPIError
// so users see what the orchestrator actually said.
func mapHTTPErr(httpResp *http.Response, err error) error {
	if err == nil {
		return nil
	}

	body := ""
	var apiErr *openapi.GenericOpenAPIError
	if errors.As(err, &apiErr) {
		body = string(apiErr.Body())
	}
	if body == "" && httpResp != nil && httpResp.Body != nil {
		respBody, readErr := io.ReadAll(httpResp.Body)
		if readErr == nil {
			body = string(respBody)
		}
	}

	if httpResp != nil {
		switch httpResp.StatusCode {
		case http.StatusUnauthorized:
			return fmt.Errorf("%w: %s", ErrUnauthorized, truncate(body, 300))
		case http.StatusForbidden:
			return fmt.Errorf("%w: %s", ErrForbidden, truncate(body, 300))
		case http.StatusNotFound:
			return fmt.Errorf("%w: %s", ErrNotFound, truncate(body, 300))
		case http.StatusBadRequest:
			return fmt.Errorf("%w: %s", ErrBadRequest, truncate(body, 300))
		case http.StatusInternalServerError:
			return fmt.Errorf("%w: %s", ErrServerError, truncate(body, 300))
		default:
			if httpResp.StatusCode >= 300 {
				return fmt.Errorf("orchestrator returned HTTP %d: %s",
					httpResp.StatusCode, truncate(body, 300))
			}
		}
	}

	if err.Error() == "undefined response type" {
		if httpResp == nil {
			return fmt.Errorf("orchestrator client could not parse response — "+
				"this usually means the request never reached the orchestrator "+
				"(check orchestrator_url) or auth failed before a response was sent: %w", err)
		}
		contentType := httpResp.Header.Get("Content-Type")
		bodyPart := "<empty body>"
		if body != "" {
			bodyPart = truncate(body, 500)
		}
		return fmt.Errorf("orchestrator returned a response the generated client "+
			"could not decode (HTTP %d, Content-Type %q). The body usually tells "+
			"you what happened — an HTML login page means the token expired or "+
			"orchestrator_url is wrong; a plain-text gateway error means a proxy/LB "+
			"is in between; a JSON shape the client does not know means the API "+
			"schema drifted. Body: %s",
			httpResp.StatusCode, contentType, bodyPart)
	}

	return fmt.Errorf("orchestrator error: %w", err)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "...(truncated)"
}
