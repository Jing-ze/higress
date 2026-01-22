package common

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
	"github.com/mark3labs/mcp-go/mcp"
)

type RequestURL struct {
	Method     string
	Scheme     string
	Host       string
	Path       string
	ParsedURL  *url.URL
	InternalIP bool
}

func NewRequestURL(header api.RequestHeaderMap) *RequestURL {
	method, _ := header.Get(":method")
	scheme, _ := header.Get(":scheme")
	host, _ := header.Get(":authority")
	path, _ := header.Get(":path")
	internalIP, _ := header.Get("x-envoy-internal")
	fullURL := fmt.Sprintf("%s://%s%s", scheme, host, path)
	parsedURL, err := url.Parse(fullURL)
	if err != nil {
		api.LogWarnf("url parse fullURL:%s failed:%s", fullURL, err)
		return nil
	}
	api.LogDebugf("RequestURL: method=%s, scheme=%s, host=%s, path=%s", method, scheme, host, path)
	return &RequestURL{Method: method, Scheme: scheme, Host: host, Path: path, ParsedURL: parsedURL, InternalIP: internalIP == "true"}
}

// JSONRPCHTTPError is the error code for HTTP errors in JSON-RPC
// Uses -32001 from the implementation-defined server error range (-32000 to -32099)
const JSONRPCHTTPError = -32001

// IsJSONRPCResponse checks if the body is a valid JSON-RPC response
func IsJSONRPCResponse(body []byte) bool {
	if len(body) == 0 {
		return false
	}
	var msg struct {
		JSONRPC string `json:"jsonrpc"`
	}
	if err := json.Unmarshal(body, &msg); err != nil {
		return false
	}
	return msg.JSONRPC == mcp.JSONRPC_VERSION
}

// WrapHTTPResponseAsJSONRPC wraps a non-JSON-RPC HTTP response into JSON-RPC error format
// This is used to convert HTTP errors from upstream filters (like jwt-auth) into MCP-compatible format
func WrapHTTPResponseAsJSONRPC(statusCode int, body string) []byte {
	// Build message: "HTTP {statusCode} {statusText}: {body}"
	// e.g., "HTTP 401 Unauthorized: Jwt is missing"
	statusText := http.StatusText(statusCode)
	if statusText == "" {
		statusText = "Unknown"
	}

	message := fmt.Sprintf("HTTP %d %s", statusCode, statusText)
	if body != "" {
		message = fmt.Sprintf("%s: %s", message, body)
	}

	response := mcp.JSONRPCError{
		JSONRPC: mcp.JSONRPC_VERSION,
		ID:      nil, // No ID available for upstream errors
		Error: struct {
			Code    int         `json:"code"`
			Message string      `json:"message"`
			Data    interface{} `json:"data,omitempty"`
		}{
			Code:    JSONRPCHTTPError,
			Message: message,
			Data: map[string]interface{}{
				"httpStatus":     statusCode,
				"httpStatusText": statusText,
				"body":           body,
			},
		},
	}

	result, _ := json.Marshal(response)
	return result
}
