package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// this is the API gateway base URL/Endpoint
const WSO2_URL = "http://localhost:9090/api/management/v0.9"

// this struct represents a WSO2 connection (every call to the gateway)
type WSO2Client struct {
	BaseURL  string
	Username string
	Password string
}

// this is a helper function to make HTTP requests to the WSO2 API Gateway
func (c *WSO2Client) request(
	method string, // ex/- GET, POST, PUT, DELETE
	path string, // ex/- /rest-apis
	body string,
	contentType string,
) (string, error) {

	// this part creates the request body

	var reader io.Reader

	// if the body has data, convert is into an HTTP readable stream
	if body != "" {
		reader = bytes.NewBufferString(body) // convert string to bytes
	}

	// puts everything together and creates the HTTP request with the given method, path, and body
	req, err := http.NewRequest(
		method,
		c.BaseURL+path,
		reader,
	)

	if err != nil {
		return "", err
	}

	// Adds the basic authentication header to the request using the provided username and password
	req.SetBasicAuth(
		c.Username,
		c.Password,
	)

	// sets the content type header if its provided (here it can be application/json or application/yaml)
	if contentType != "" {
		req.Header.Set(
			"Content-Type",
			contentType,
		)
	}

	// This Sends the HTTP request to the WSO2 API Gateway and returns the response or an error if it occurs
	resp, err := http.DefaultClient.Do(req)

	if err != nil {
		return "", err
	}

	defer resp.Body.Close()

	// Reads the response body
	data, _ := io.ReadAll(resp.Body) // converts the byte response into data

	return string(data), nil
}

// this represents an API endpoint
type Operation struct {
	Method string `json:"method"` // ex/- GET
	Path   string `json:"path"`   // ex/- /books
}

// -------------------------
// MCP TOOLS
// -------------------------

func listAPIs(
	ctx context.Context,
	req *mcp.CallToolRequest,

) (*mcp.CallToolResult, error) {

	client := &WSO2Client{
		BaseURL:  WSO2_URL,
		Username: "admin",
		Password: "admin",
	}

	result, err := client.request(
		"GET",
		"/rest-apis",
		"",
		"",
	)

	if err != nil {
		return nil, err
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: result,
			},
		},
	}, nil
}

func main() {

	server :=
		mcp.NewServer(
			&mcp.Implementation{
				Name:    "wso2-api-mcp",
				Version: "1.0.0",
			},
			nil,
		)

	server.AddTool(
		&mcp.Tool{
			Name:        "list_apis",
			Description: "List APIs from WSO2 Gateway",

			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
		listAPIs,
	)

	if err :=
		server.Run(
			context.Background(),
			&mcp.StdioTransport{},
		); err != nil {

		fmt.Println(err)
	}

}
