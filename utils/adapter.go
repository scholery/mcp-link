package utils

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func sanitizeToolName(name string) string {
	// Convert to lowercase and replace common separators with underscore
	s := strings.ToLower(name)

	// Replace special characters
	s = strings.ReplaceAll(s, " ", "_")
	s = strings.ReplaceAll(s, "-", "_")
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ReplaceAll(s, ".", "_")
	s = strings.ReplaceAll(s, "{", "")
	s = strings.ReplaceAll(s, "}", "")
	s = strings.ReplaceAll(s, ":", "_")
	s = strings.ReplaceAll(s, "?", "")
	s = strings.ReplaceAll(s, "&", "and")
	s = strings.ReplaceAll(s, "=", "_eq_")
	s = strings.ReplaceAll(s, "%", "_pct_")

	// Remove consecutive underscores
	for strings.Contains(s, "__") {
		s = strings.ReplaceAll(s, "__", "_")
	}

	// Remove trailing underscore
	s = strings.TrimSuffix(s, "_")

	// Remove leading underscore
	s = strings.TrimPrefix(s, "_")

	// Ensure the name is not empty
	if s == "" {
		return "unnamed_tool"
	}

	return s
}

func NewToolHandler(method string, url string, extraHeaders map[string]string) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Extract parameters from the request
		params := request.Params.Arguments

		// Create maps for different parameter types
		headerParams := make(map[string]interface{})
		pathParams := make(map[string]interface{})
		queryParams := make(map[string]interface{})
		bodyParams := make(map[string]interface{})

		// Extract specific parameter groups
		if headerParamsMap, ok := params["requestHeader"].(map[string]interface{}); ok {
			headerParams = headerParamsMap
		}

		if pathParamsMap, ok := params["pathNames"].(map[string]interface{}); ok {
			pathParams = pathParamsMap
		}

		if urlParamsMap, ok := params["searchParams"].(map[string]interface{}); ok {
			queryParams = urlParamsMap
		}

		if requestBodyMap, ok := params["requestBody"].(map[string]interface{}); ok {
			bodyParams = requestBodyMap
		}

		// If structured params aren't found, use flat params for backward compatibility
		if len(pathParams) == 0 && len(queryParams) == 0 && len(bodyParams) == 0 {
			// Process all params without structured separation (legacy approach)
			for paramName, paramValue := range params {
				placeholder := fmt.Sprintf("{%s}", paramName)
				if strings.Contains(url, placeholder) {
					pathParams[paramName] = paramValue
				} else {
					// Put in body by default
					bodyParams[paramName] = paramValue
				}
			}
		}

		// Create a copy of the URL for path parameter substitution
		finalURL := url

		// Process URL path parameters - replace {param_name} with the value from pathParams
		for paramName, paramValue := range pathParams {
			placeholder := fmt.Sprintf("{%s}", paramName)
			if strings.Contains(finalURL, placeholder) {
				// Convert the param value to string
				var strValue string
				switch v := paramValue.(type) {
				case string:
					strValue = v
				case nil:
					// Use empty string for nil path parameters
					strValue = ""
				default:
					// Convert other types to string
					strValue = fmt.Sprintf("%v", v)
				}

				// Replace the placeholder in the URL
				finalURL = strings.ReplaceAll(finalURL, placeholder, strValue)
			}
		}
		// Add query parameters to the URL
		if len(queryParams) > 0 {
			// Parse the URL to add query parameters properly
			parsedURL, err := neturl.Parse(finalURL)
			if err != nil {
				return mcp.NewToolResultText(fmt.Sprintf("Error parsing URL: %v", err)), nil
			}

			// Get existing query values or create new ones
			q := parsedURL.Query()

			// Add all query parameters
			for paramName, paramValue := range queryParams {
				// Convert the param value to string
				var strValue string
				switch v := paramValue.(type) {
				case string:
					strValue = v
				case nil:
					continue
				default:
					// Convert other types to string
					strValue = fmt.Sprintf("%v", v)
				}

				q.Add(paramName, strValue)
			}

			// Set the updated query string back to the URL
			parsedURL.RawQuery = q.Encode()
			finalURL = parsedURL.String()
		}

		// Convert body parameters to JSON for the HTTP request body
		var reqBody io.Reader = nil
		if len(bodyParams) > 0 {
			jsonParams, err := json.Marshal(bodyParams)
			if err != nil {
				return mcp.NewToolResultText(fmt.Sprintf("Error marshaling body parameters: %v", err)), nil
			}
			reqBody = bytes.NewBuffer(jsonParams)
		}

		// Create HTTP request with the processed URL
		req, err := http.NewRequestWithContext(ctx, method, finalURL, reqBody)
		if err != nil {
			return mcp.NewToolResultText(fmt.Sprintf("Error creating request: %v", err)), nil
		}

		// Set headers
		if reqBody != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		for key, value := range extraHeaders {
			req.Header.Set(key, value)
		}

		for key, value := range headerParams{
			var strValue string
			switch v := value.(type) {
			case string:
				strValue = v
			case nil:
				continue
			default:
				// Convert other types to string
				strValue = fmt.Sprintf("%v", v)
			}
			req.Header.Set(key, strValue)
		}

		// Execute the request
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			return mcp.NewToolResultText(fmt.Sprintf("Error executing request: %v", err)), nil
		}
		defer resp.Body.Close()

		// Read response body
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return mcp.NewToolResultText(fmt.Sprintf("Error reading response: %v", err)), nil
		}

		// TODO: handle image response
		// if strings.HasPrefix(resp.Header.Get("Content-Type"), "image/") {
		// return mcp.NewToolResultImage("", base64.StdEncoding.EncodeToString(body), resp.Header.Get("Content-Type")), nil
		// }

		return mcp.NewToolResultText(string(body)), nil
	}
}

// NewMCPFromCustomParser creates an MCP server from our custom OpenAPIParser
func NewMCPFromCustomParser(baseURL string, extraHeaders map[string]string, parser OpenAPIParser) (*server.MCPServer, error) {
	// Create a new MCP server
	apiInfo := parser.Info()
	prefix := "mcplink_" + sanitizeToolName(apiInfo.Title)

	s := server.NewMCPServer(
		prefix,
		apiInfo.Version,
		server.WithResourceCapabilities(true, true),
		server.WithLogging(),
	)

	// Add all API endpoints as tools
	for _, api := range parser.APIs() {
		// Create tool name from API path and method
		name := sanitizeToolName(fmt.Sprintf("%s_%s_%s", prefix, strings.ToLower(api.Method), api.Path))

		// Define tool options
		opts := []mcp.ToolOption{
			mcp.WithDescription(api.OperationID + " " + api.Summary + " " + api.Description),
		}

		// Add parameters
		// paramsOpts := mcp.WithObject("searchParams", mcp.Description("url parameters for the tool"))
		query_props := map[string]interface{}{}
		path_props := map[string]interface{}{}

		for _, param := range api.Parameters {
			if param.In == "query" {
				prop := map[string]interface{}{}
				query_props[param.Name] = prop
				prop["type"] = param.Schema.Type
				if param.Schema.Enum != nil {
					prop["enum"] = param.Schema.Enum
				}
				if param.Schema.Format != "" {
					prop["format"] = param.Schema.Format
				}
				if param.Schema.Default != nil {
					prop["default"] = param.Schema.Default
				}
				if param.Schema.Description != "" {
					prop["description"] = param.Schema.Description
				}
				if param.Schema.Items != nil {
					prop["items"] = param.Schema.Items
				}
				if param.Schema.Properties != nil {
					prop["properties"] = param.Schema.Properties
				}
			} else if param.In == "path" {
				prop := map[string]interface{}{}
				path_props[param.Name] = prop
				prop["type"] = param.Schema.Type
				if param.Schema.Enum != nil {
					prop["enum"] = param.Schema.Enum
				}
				if param.Schema.Format != "" {
					prop["format"] = param.Schema.Format
				}
				if param.Schema.Default != nil {
					prop["default"] = param.Schema.Default
				}
				if param.Schema.Description != "" {
					prop["description"] = param.Schema.Description
				}
				if param.Schema.Items != nil {
					prop["items"] = param.Schema.Items
				}
			}
		}

		if len(query_props) > 0 {
			opts = append(opts, mcp.WithObject("searchParams", mcp.Description("url parameters for the tool"), mcp.Properties(query_props)))
		}
		if len(path_props) > 0 {
			opts = append(opts, mcp.WithObject("pathNames", mcp.Description("path parameters for the tool"), mcp.Properties(path_props)))
		}

		// Handle request body parameters if present
		props := map[string]interface{}{}
		if api.RequestBody != nil && len(api.RequestBody.Content) > 0 {
			for _, mediaType := range api.RequestBody.Content {
				if mediaType.Schema != nil {
					for propName, propSchema := range mediaType.Schema.Properties {
						prop := map[string]interface{}{}
						props[propName] = prop
						prop["type"] = propSchema.Type
						if propSchema.Enum != nil {
							prop["enum"] = propSchema.Enum
						}
						if propSchema.Format != "" {
							prop["format"] = propSchema.Format
						}
						if propSchema.Default != nil {
							prop["default"] = propSchema.Default
						}
						if propSchema.Description != "" {
							prop["description"] = propSchema.Description
						}
						if propSchema.Items != nil {
							prop["items"] = propSchema.Items
						}
						if propSchema.Properties != nil {
							prop["properties"] = propSchema.Properties
						}
					}
				}
			}
			opts = append(opts, mcp.WithObject("requestBody", mcp.Description("request body for the tool"), mcp.Properties(props)))
		}

		// parse header
		if len(api.Security) > 0 {
			securities := map[string]interface{}{}
			for name, param := range api.Security {
				prop := map[string]interface{}{}
				securities[name] = prop
				if param.Name != "" {
					prop["name"] = param.Name
				}
				if param.Type != "" {
					prop["type"] = param.Type
				}
				if param.Scheme != "" {
					prop["scheme"] = param.Scheme
				}
				if param.In != "" {
					prop["in"] = param.In
				}
				if param.Description != "" {
					prop["description"] = param.Description
				}
				if param.BearerFormat != "" {
					prop["bearerFormat"] = param.BearerFormat
				}
			}
			opts = append(opts, mcp.WithObject("requestHeader", mcp.Description("request header for the tool"), mcp.Properties(securities)))
		}

		// Create the tool and handler
		tool := mcp.NewTool(name, opts...)
		handler := NewToolHandler(api.Method, baseURL+api.Path, extraHeaders)

		// Add the tool to the server
		s.AddTool(tool, handler)
	}

	return s, nil
}
