package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

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

// this function builds the YAML definition for a REST API (Infrastructure as code)
func buildAPIYaml(
	name string,
	version string,
	context string,
	upstream string,
	operations []Operation,

) string {

	yaml := fmt.Sprintf(`
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi

metadata:
  name: %s

spec:
  displayName: %s
  version: %s
  context: %s

  upstream:
    main:
      url: "%s"

  operations:
`,
		name,
		name,
		version,
		context,
		upstream,
	)

	// This loop iterates over the provided operations and appends them to the YAML definition in the correct format
	for _, op := range operations {

		yaml += fmt.Sprintf(`
    - method: %s
      path: %s
`,
			op.Method,
			op.Path,
		)
	}

	return strings.TrimSpace(yaml)
}

// -------------------------
// MCP TOOLS
// -------------------------
func listResources(
	ctx context.Context,
	req *mcp.CallToolRequest,

) (*mcp.CallToolResult, error) {

	client := &WSO2Client{
		BaseURL:  WSO2_URL,
		Username: "admin",
		Password: "admin",
	}

	endpoints := map[string]string{

		"rest_apis": "/rest-apis",

		"webbroker_apis": "/webbroker-apis",

		"mcp_proxies": "/mcp-proxies",

		"llm_providers": "/llm-providers",

		"llm_proxies": "/llm-proxies",
	}

	resources := make(
		map[string]string,
	)

	// Loops through the endpoints and makes a GET request to each one, storing the result in the resources map
	for name, path := range endpoints { // name is the key (e.g., "rest_apis") and path is the value (e.g., "/rest-apis")

		result, err := // sends a GET request to the WSO2 API Gateway for the given path and returns the result or an error
			client.request( //request is the func described on the top
				"GET",
				path,
				"",
				"",
			)

			// if there is an error, store the error message in the resources map and continue to the next endpoint
		if err != nil {

			resources[name] =
				"ERROR: " + err.Error()

			continue
		}

		// if the request is successful, store the result in the resources map
		resources[name] =
			result
	}

	// Converts the resources map into a JSON string with indentation for better readability
	jsonData, err :=
		json.MarshalIndent(
			resources,
			"",
			"  ",
		)

	if err != nil {
		return nil, err
	}

	// These return the result of the MCP tool execution back to the MCP Client
	return &mcp.CallToolResult{

		Content: []mcp.Content{

			&mcp.TextContent{
				Text: string(jsonData), // this is the JSON string representation of the resources map being sent back to the MCP client
			},
		},
	}, nil
}

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

func getAPI(
	ctx context.Context,
	req *mcp.CallToolRequest,

) (*mcp.CallToolResult, error) {

	var args map[string]any

	json.Unmarshal(
		req.Params.Arguments,
		&args,
	)

	id, _ :=
		args["id"].(string)

	client := &WSO2Client{
		BaseURL:  WSO2_URL,
		Username: "admin",
		Password: "admin",
	}

	result, err :=
		client.request(
			"GET",
			"/rest-apis/"+id,
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

func createAPI(
	ctx context.Context,
	req *mcp.CallToolRequest,

) (*mcp.CallToolResult, error) {

	var args map[string]any

	err := json.Unmarshal(
		req.Params.Arguments,
		&args,
	)

	if err != nil {
		return nil, err
	}

	name, ok :=
		args["name"].(string)

	if !ok {
		return nil, fmt.Errorf("missing name")
	}

	version, ok :=
		args["version"].(string)

	if !ok {
		return nil, fmt.Errorf("missing version")
	}

	contextPath, ok :=
		args["context"].(string)

	if !ok {
		return nil, fmt.Errorf("missing context")
	}

	upstream, ok :=
		args["upstream"].(string)

	if !ok {
		return nil, fmt.Errorf("missing upstream")
	}

	rawOps, ok :=
		args["operations"].([]any)

	if !ok {
		return nil, fmt.Errorf(
			"operations required",
		)
	}

	operations :=
		[]Operation{}

	for _, item := range rawOps {

		obj, ok :=
			item.(map[string]any)

		if !ok {
			continue
		}

		method, ok :=
			obj["method"].(string)

		if !ok {
			continue
		}

		path, ok :=
			obj["path"].(string)

		if !ok {
			continue
		}

		operations = append(
			operations,
			Operation{
				Method: method,
				Path:   path,
			},
		)
	}

	yaml :=
		buildAPIYaml(
			name,
			version,
			contextPath,
			upstream,
			operations,
		)

	client := &WSO2Client{
		BaseURL:  WSO2_URL,
		Username: "admin",
		Password: "admin",
	}

	result, err :=
		client.request(
			"POST",
			"/rest-apis",
			yaml,
			"application/yaml",
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

func deleteAPI(
	ctx context.Context,
	req *mcp.CallToolRequest,

) (*mcp.CallToolResult, error) {

	var args map[string]any

	json.Unmarshal(
		req.Params.Arguments,
		&args,
	)

	id, _ :=
		args["id"].(string)

	client := &WSO2Client{
		BaseURL:  WSO2_URL,
		Username: "admin",
		Password: "admin",
	}

	result, err :=
		client.request(
			"DELETE",
			"/rest-apis/"+id,
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

			Name: "list_resources",

			Description: "List all WSO2 Gateway resources",

			InputSchema: map[string]any{

				"type": "object",

				"properties": map[string]any{},
			},
		},

		listResources,
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

	server.AddTool(

		&mcp.Tool{
			Name:        "create_api",
			Description: "Create API in WSO2 Gateway",

			InputSchema: map[string]any{

				"type": "object",

				"properties": map[string]any{

					"name": map[string]any{
						"type":        "string",
						"description": "API name",
					},

					"version": map[string]any{
						"type":        "string",
						"description": "API version",
					},

					"context": map[string]any{
						"type":        "string",
						"description": "API context path e.g /pets",
					},

					"upstream": map[string]any{
						"type":        "string",
						"description": "Backend URL",
					},

					"operations": map[string]any{

						"type": "array",

						"description": "API operations",

						"items": map[string]any{

							"type": "object",

							"properties": map[string]any{

								"method": map[string]any{
									"type": "string",
								},

								"path": map[string]any{
									"type": "string",
								},
							},

							"required": []string{
								"method",
								"path",
							},
						},
					},
				},
				"required": []string{
					"name",
					"version",
					"context",
					"upstream",
					"operations",
				},
			},
		},

		createAPI,
	)

	server.AddTool(

		&mcp.Tool{
			Name:        "get_api",
			Description: "Get API by ID",

			InputSchema: map[string]any{

				"type": "object",

				"properties": map[string]any{

					"id": map[string]any{
						"type":        "string",
						"description": "API ID",
					},
				},

				"required": []string{
					"id",
				},
			},
		},

		getAPI,
	)

	server.AddTool(

		&mcp.Tool{
			Name:        "delete_api",
			Description: "Delete API by ID",

			InputSchema: map[string]any{

				"type": "object",

				"properties": map[string]any{

					"id": map[string]any{
						"type":        "string",
						"description": "API ID to delete",
					},
				},

				"required": []string{
					"id",
				},
			},
		},

		deleteAPI,
	)

	if err :=
		server.Run(
			context.Background(),
			&mcp.StdioTransport{},
		); err != nil {

		fmt.Println(err)
	}

}
