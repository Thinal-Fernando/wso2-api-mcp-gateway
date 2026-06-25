package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// this is the API gateway base URL/Endpoint
const WSO2_URL = "http://localhost:9090/api/management/v0.9"

// this is for the policy hub
const POLICY_HUB_URL = "https://db720294-98fd-40f4-85a1-cc6a3b65bc9a-prod.e1-us-east-azure.choreoapis.dev/api-platform/policy-hub-api/policy-hub-public/v1.0/policies"

// this struct represents a WSO2 connection (every call to the gateway)
type WSO2Client struct {
	BaseURL  string
	Username string
	Password string
}

type policyListResponse struct {
	Data []Policy `json:"data"`
}

type Policy struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
}

type PolicyDocsResponse struct {
	Data struct {
		Policy      string `json:"policy"`
		Version     string `json:"version"`
		Entry       string `json:"entry"`
		StoragePath string `json:"storagePath"`
	}
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

func httpGet(url string) (string, error) {

	req, err := http.NewRequest("GET", url, nil)

	if err != nil {
		return "", err
	}

	resp, err := http.DefaultClient.Do(req)

	if err != nil {
		return "", err
	}

	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)

	if err != nil {
		return "", nil
	}

	if resp.StatusCode >= 400 {
		return "",
			fmt.Errorf(
				"HTTP %d: %s",
				resp.StatusCode,
				string(data),
			)
	}

	return string(data), nil
}

func httpGetJSON(url string, target any) error {

	body, err := httpGet(url)

	if err != nil {
		return err
	}

	return json.Unmarshal(
		[]byte(body),
		target,
	)
}

// -------------------------
// MCP TOOLS
// -------------------------

func listPolicies(
	ctx context.Context, req *mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {

	result, err := httpGet(POLICY_HUB_URL)

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

func getPolicy(
	ctx context.Context,
	req *mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {

	var args map[string]any

	if err := json.Unmarshal(
		req.Params.Arguments,
		&args,
	); err != nil {
		return nil, err
	}

	name, ok :=
		args["policy"].(string)

	if !ok {
		return nil,
			fmt.Errorf("missing policy")
	}

	url :=
		fmt.Sprintf(
			"%s/%s/versions/1.0/docs",
			POLICY_HUB_URL,
			name,
		)

	result, err :=
		httpGet(url)

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

func getPolicyMarkdown(
	ctx context.Context,
	req *mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {

	var args map[string]any

	if err := json.Unmarshal(
		req.Params.Arguments,
		&args,
	); err != nil {
		return nil, err
	}

	name, ok :=
		args["policy"].(string)

	if !ok {
		return nil,
			fmt.Errorf("missing policy")
	}

	var docs PolicyDocsResponse

	err := httpGetJSON(
		fmt.Sprintf(
			"%s/%s/versions/1.0/docs",
			POLICY_HUB_URL,
			name,
		),
		&docs,
	)

	if err != nil {
		return nil, err
	}

	mdURL :=
		docs.Data.StoragePath +
			"/" +
			docs.Data.Entry

	markdown, err :=
		httpGet(mdURL)

	if err != nil {
		return nil, err
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: markdown,
			},
		},
	}, nil
}

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

func createAPI(
	ctx context.Context, req *mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {

	var args map[string]any

	if err := json.Unmarshal(
		req.Params.Arguments, &args,
	); err != nil {
		return nil, err
	}

	yamlContent, ok := args["yaml"].(string)

	if !ok || yamlContent == "" {
		return nil, fmt.Errorf("yaml is required")
	}

	client := &WSO2Client{
		BaseURL:  WSO2_URL,
		Username: "admin",
		Password: "admin",
	}

	result, err := client.request(
		"POST",
		"/rest-apis",
		yamlContent,
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

func updateAPI(
	ctx context.Context,
	req *mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {

	var args map[string]any

	if err := json.Unmarshal(
		req.Params.Arguments,
		&args,
	); err != nil {
		return nil, err
	}

	id, ok := args["id"].(string)

	if !ok || id == "" {
		return nil, fmt.Errorf("id is required")
	}

	yamlContent, ok := args["yaml"].(string)

	if !ok || yamlContent == "" {
		return nil, fmt.Errorf("yaml is required")
	}

	client := &WSO2Client{
		BaseURL:  WSO2_URL,
		Username: "admin",
		Password: "admin",
	}

	result, err := client.request(
		"PUT",
		"/rest-apis/"+id,
		yamlContent,
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

func getPolicyRequirements(
	ctx context.Context,
	req *mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {

	var args map[string]any

	if err := json.Unmarshal(
		req.Params.Arguments,
		&args,
	); err != nil {
		return nil, err
	}

	name, ok :=
		args["policy"].(string)

	if !ok || name == "" {
		return nil,
			fmt.Errorf("missing policy")
	}

	var docs PolicyDocsResponse

	err := httpGetJSON(
		fmt.Sprintf(
			"%s/%s/versions/1.0/docs",
			POLICY_HUB_URL,
			name,
		),
		&docs,
	)

	if err != nil {
		return nil, err
	}

	mdURL :=
		docs.Data.StoragePath +
			"/" +
			docs.Data.Entry

	markdown, err :=
		httpGet(mdURL)

	if err != nil {
		return nil, err
	}

	result := map[string]any{
		"policy": name,
		"instructions": `
Read the documentation and identify:

1. Required parameters
2. Optional parameters
3. Parameter types
4. Example values

If required parameters are missing,
ask the user before applying the policy.
`,
		"documentation": markdown,
	}

	jsonData, err :=
		json.MarshalIndent(
			result,
			"",
			"  ",
		)

	if err != nil {
		return nil, err
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: string(jsonData),
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
			Name:        "list_policies",
			Description: "Lit all available WSO2 Policy Hub policies",

			InputSchema: map[string]any{
				"type": "object",

				"properties": map[string]any{},
			},
		}, listPolicies,
	)

	server.AddTool(
		&mcp.Tool{
			Name: "get_policy",

			Description: "Get policy metadata",

			InputSchema: map[string]any{
				"type": "object",

				"properties": map[string]any{
					"policy": map[string]any{
						"type": "string",
					},
				},

				"required": []string{
					"policy",
				},
			},
		},
		getPolicy,
	)

	server.AddTool(
		&mcp.Tool{
			Name: "get_policy_requirements",

			Description: `
				Retrieve policy documentation and determine
				what configuration values are required.

				Use this tool before applying a policy.

				The returned documentation should be inspected
				to identify required parameters, optional
				parameters, parameter types, and example values.

				If required values are missing, ask the user
				for them before calling create_api or update_api.
				`,

			InputSchema: map[string]any{
				"type": "object",

				"properties": map[string]any{
					"policy": map[string]any{
						"type":        "string",
						"description": "Policy name",
					},
				},

				"required": []string{
					"policy",
				},
			},
		},
		getPolicyRequirements,
	)

	server.AddTool(
		&mcp.Tool{
			Name: "get_policy_markdown",

			Description: `
				Retrieve the full markdown documentation for a policy.

				Use this tool before applying a policy.

				The markdown may contain:

				- required parameters
				- optional parameters
				- configuration examples
				- policy YAML snippets

				When a user requests a policy, inspect this documentation
				to determine what configuration values are required.

				If required values are missing, ask the user for them before
				calling create_api or update_api.
				`,

			InputSchema: map[string]any{
				"type": "object",

				"properties": map[string]any{
					"policy": map[string]any{
						"type": "string",
					},
				},

				"required": []string{
					"policy",
				},
			},
		},
		getPolicyMarkdown,
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
			Name: "create_api",
			Description: `
				Create a REST API in WSO2 API Gateway.

				The yaml field must contain a complete RestApi YAML manifest.

				Typical structure:

				apiVersion: gateway.api-platform.wso2.com/v1alpha1
				kind: RestApi

				metadata:
					name: api-name

				spec:
					displayName: API Name
			    	version: 1.0.0
					context: /api

				upstream:
					main:
					  url: https://backend.example.com

				operations:
					- method: GET
					  path: /items

				Generate all required fields.

				The YAML may contain:

					- operations
					- authentication
					- policies
					- rate limits
					- observability

				When a policy is requested:
					1. Use get_policy_requirements.
					2. Determine required configuration values.
					3. Ask the user for missing values.
					4. Generate the final YAML.
				`,

			InputSchema: map[string]any{
				"type": "object",

				"properties": map[string]any{
					"yaml": map[string]any{
						"type":        "string",
						"description": "Complete WSO2 RestApi YAML manifest",
					},
				},

				"required": []string{
					"yaml",
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
			Name: "update_api",
			Description: `
				Update an existing REST API.

				The yaml field must contain the complete updated RestApi manifest.

				Before updating:

				1. Use get_api to inspect the current API.
				2.  Use get_policy_requirements if a policy is being added.
				3. Determine required policy parameters.
				4. Ask the user for any missing values.
				5. Generate the updated YAML.
				`,
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"id": map[string]any{
						"type": "string",
					},
					"yaml": map[string]any{
						"type": "string",
					},
				},
				"required": []string{
					"id",
					"yaml",
				},
			},
		},
		updateAPI,
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
