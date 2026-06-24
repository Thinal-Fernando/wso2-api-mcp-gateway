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

// this is for the policy hub
const POLICY_HUB_URL = "https://db720294-98fd-40f4-85a1-cc6a3b65bc9a-prod.e1-us-east-azure.choreoapis.dev/api-platform/policy-hub-api/policy-hub-public/v1.0/policies?limit=100"

const POLICY_HUB_BASE = "https://db720294-98fd-40f4-85a1-cc6a3b65bc9a-prod.e1-us-east-azure.choreoapis.dev/api-platform/policy-hub-api/policy-hub-public/v1.0/policies"

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

type PolicySchema struct {
	Name       string            `json:"name"`
	Parameters map[string]string `json:"parameters"`
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

// helper function to convert Go values -> Yaml-like format
func toYAML(v any, indent int) string {
	space := strings.Repeat(" ", indent)

	switch val := v.(type) {

	case string:
		return fmt.Sprintf("\"%s\"", val)

	case float64, int, bool:
		return fmt.Sprintf("%v", val)

	case map[string]any:
		var out strings.Builder
		for k, vv := range val {
			out.WriteString(fmt.Sprintf("\n%s%s: %s", space, k, toYAML(vv, indent+2)))
		}
		return out.String()

	case []any:
		var out strings.Builder
		for _, item := range val {
			out.WriteString(fmt.Sprintf("\n%s- %s", space, toYAML(item, indent+2)))
		}
		return out.String()

	default:
		return fmt.Sprintf("\"%v\"", val)
	}
}

// this function builds the YAML definition for a REST API (Infrastructure as code)
func buildAPIYaml(
	name string,
	version string,
	context string,
	upstream string,
	operations []Operation,
	policy string,
	policyConfig map[string]any,

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
	if policy != "" {
		yaml += `
  policies:
`
		yaml += fmt.Sprintf(`    - name: %s
`, policy)

		if len(policyConfig) > 0 {
			yaml += `      params: 
`
			for k, v := range policyConfig {
				yaml += fmt.Sprintf("        %s: %s\n", k, toYAML(v, 10))
			}
		}
	}

	return strings.TrimSpace(yaml)
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

func GETPolicySchema(
	ctx context.Context, req *mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {

	var args map[string]any
	if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
		return nil, err
	}

	name, ok := args["policy"].(string)
	if !ok {
		return nil, fmt.Errorf("missing policy")
	}

	var docs PolicyDocsResponse
	err := httpGetJSON(
		fmt.Sprintf("%s/%s/versions/1.0/docs", POLICY_HUB_BASE, name),
		&docs,
	)
	if err != nil {
		return nil, err
	}

	raw, err := httpGet(docs.Data.StoragePath + "/" + docs.Data.Entry)
	if err != nil {
		return nil, err
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: raw,
			},
		},
	}, nil

}

func attachPolicyToAPI(
	ctx context.Context,
	req *mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {

	var args map[string]any
	if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
		return nil, err
	}

	apiID, _ := args["apiId"].(string)
	policy, _ := args["policy"].(string)

	config, _ := args["config"].(map[string]any)

	client := &WSO2Client{
		BaseURL:  WSO2_URL,
		Username: "admin",
		Password: "admin",
	}

	payload := map[string]any{
		"policy": policy,
		"config": config,
	}

	body, _ := json.Marshal(payload)

	result, err := client.request(
		"POST",
		fmt.Sprintf("/rest-apis/%s/policies", apiID),
		string(body),
		"application/json",
	)

	if err != nil {
		return nil, err
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: result},
		},
	}, nil
}

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
			POLICY_HUB_BASE,
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
			POLICY_HUB_BASE,
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

	policy, _ := args["policy"].(string)
	policyConfig, _ := args["policyConfig"].(map[string]any)

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
			policy,
			policyConfig,
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
			Name: "get_policy_markdown",

			Description: "Download policy documentation",

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
			Name:        "get_policy_schema",
			Description: "Get raw policy schema/documentation for parameter extraction",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"policy": map[string]any{
						"type": "string",
					},
				},
				"required": []string{"policy"},
			},
		},
		GETPolicySchema,
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
					"policy": map[string]any{
						"type":        "string",
						"description": "Optional policy to attach",
					},
					"policyConfig": map[string]any{
						"type":        "object",
						"description": "Policy configuration values",
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
