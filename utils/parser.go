package utils

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/lestrrat-go/jsref"
	"gopkg.in/yaml.v3"
)

// OpenAPIParser provides a simple interface for parsing OpenAPI specifications
type OpenAPIParser interface {
	// Info returns basic information about the API
	Servers() []Server
	Info() APIInfo
	// APIs returns information about all API endpoints
	APIs() []APIEndpoint
}

// Server represents a server in the OpenAPI specification
type Server struct {
	URL         string `json:"url,omitempty"`
	Description string `json:"description,omitempty"`
}

// APIInfo contains basic information about the API
type APIInfo struct {
	Title       string `json:"title,omitempty"`
	Version     string `json:"version,omitempty"`
	Description string `json:"description,omitempty"`
}

// APIEndpoint represents a single API endpoint
type APIEndpoint struct {
	Path        string              `json:"path,omitempty"`
	Method      string              `json:"method,omitempty"`
	Summary     string              `json:"summary,omitempty"`
	Description string              `json:"description,omitempty"`
	OperationID string              `json:"operationId,omitempty"`
	Parameters  []Parameter         `json:"parameters,omitempty"`
	RequestBody *RequestBody        `json:"requestBody,omitempty"`
	Responses   map[string]Response `json:"responses,omitempty"`
	Security 	map[string]Security `json:"securitySchemes,omitempty"`
}

// Parameter represents an API parameter
type Parameter struct {
	Name        string  `json:"name,omitempty"`
	In          string  `json:"in,omitempty"`
	Required    bool    `json:"required,omitempty"`
	Description string  `json:"description,omitempty"`
	Schema      *Schema `json:"schema,omitempty"`
}

// RequestBody represents the request body of an API endpoint
type RequestBody struct {
	Required bool                 `json:"required,omitempty"`
	Content  map[string]MediaType `json:"content,omitempty"`
}

// MediaType represents a media type of a request or response
type MediaType struct {
	Schema *Schema `json:"schema,omitempty"`
}

// Response represents an API response
type Response struct {
	Description string               `json:"description,omitempty"`
	Content     map[string]MediaType `json:"content,omitempty"`
}

// Schema represents a JSON schema
type Schema struct {
	Type        string            `json:"type,omitempty"`
	Format      string            `json:"format,omitempty"`
	Description string            `json:"description,omitempty"`
	Default     interface{}       `json:"default,omitempty"`
	Enum        []interface{}     `json:"enum,omitempty"`
	Properties  map[string]Schema `json:"properties,omitempty"`
	Items       *Schema           `json:"items,omitempty"`
	Required    []string          `json:"required,omitempty"`
	Ref         string
}

type Component struct {
	Schemas         map[string]Schema 		`json:"schemas,omitempty"`
	Responses		map[string]Response 	`json:"responses,omitempty"`
	Parameters		map[string]Parameter 	`json:"parameters,omitempty"`
	// Examples		string `json:"examples,omitempty"`
	RequestBodies	map[string]RequestBody 	`json:"requestBodies,omitempty"`
	Headers			map[string]Header		`json:"headers,omitempty"`
	SecuritySchemes	map[string]Security 	`json:"securitySchemes,omitempty"`
	// Links			string `json:"links,omitempty"`
	// Callbacks		string `json:"callbacks,omitempty"`
}

type Header struct{
	Required    bool    `json:"required,omitempty"`
	Description string  `json:"description,omitempty"`
	Schema      *Schema `json:"schema,omitempty"`
}

type Security struct{
	Type        		string            `json:"type,omitempty"`  			//"apiKey", "http", "oauth2", "openIdConnect"
	Description 		string            `json:"description,omitempty"` 
	Name     			string       	  `json:"name,omitempty"` 			//apiKey
	In     				string       	  `json:"in,omitempty"` 			//apiKey
	Scheme     			string       	  `json:"scheme,omitempty"`			//http
	BearerFormat    	string       	  `json:"bearerFormat,omitempty"`	//http ("bearer")
	// Flows        		[]interface{}     `json:"flows,omitempty"`			//oauth2
	// OpenIdConnectUrl  	string 			  `json:"openIdConnectUrl,omitempty"`	//openIdConnect
}

// SimpleOpenAPIParser is a simple parser for OpenAPI specifications
type SimpleOpenAPIParser struct {
	document map[string]interface{}
	security map[string]interface{}
	component Component

}

// NewSimpleOpenAPIParser creates a new OpenAPI parser
func NewSimpleOpenAPIParser(data []byte) (*SimpleOpenAPIParser, error) {
	jsonString := string(data)

	// Parse JSON into interface{}
	var v interface{}
	if err := json.Unmarshal([]byte(jsonString), &v); err != nil {
		fmt.Println("Error unmarshaling JSON:", err)
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	// Create a new resolver
	resolver := jsref.New()

	resolved, err := resolver.Resolve(v, "#", jsref.WithRecursiveResolution(false))
	if err != nil {
		return nil, fmt.Errorf("failed to resolve references: %w", err)
	}

	resolvedMap, ok := resolved.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("failed to convert resolved to map[string]interface{}")
	}
	parser := &SimpleOpenAPIParser{
		document: resolvedMap,
	}

	return parser, nil
}

// Servers returns the servers in the OpenAPI specification
func (p *SimpleOpenAPIParser) Servers() []Server {
	servers := []Server{}

	if serversObj, ok := p.document["servers"].([]interface{}); ok {
		for _, server := range serversObj {
			serverObj, ok := server.(map[string]interface{})
			if !ok {
				continue
			}

			server := Server{}

			if url, ok := serverObj["url"].(string); ok {
				server.URL = url
			}

			if description, ok := serverObj["description"].(string); ok {
				server.Description = description
			}

			servers = append(servers, server)
		}
	}

	return servers
}

// Info returns basic information about the API
func (p *SimpleOpenAPIParser) Info() APIInfo {
	info := APIInfo{}

	if infoObj, ok := p.document["info"].(map[string]interface{}); ok {
		if title, ok := infoObj["title"].(string); ok {
			info.Title = title
		}

		if version, ok := infoObj["version"].(string); ok {
			info.Version = version
		}

		if description, ok := infoObj["description"].(string); ok {
			info.Description = description
		}
	}

	return info
}

// APIs returns information about all API endpoints
func (p *SimpleOpenAPIParser) APIs() []APIEndpoint {
	var endpoints []APIEndpoint

	paths, ok := p.document["paths"].(map[string]interface{})
	if !ok {
		return endpoints
	}
	p.component = p.Components()
	p.security = p.Security()

	securities := map[string]Security{}
	for name, _ := range p.security {
		if sec,ok := p.component.SecuritySchemes[name]; ok{
			securities[name] = sec
		}
	}

	for path, pathItem := range paths {
		pathItemObj, ok := pathItem.(map[string]interface{})
		if !ok {
			continue
		}

		for method, operation := range pathItemObj {
			// Skip non-HTTP method fields
			if !isHTTPMethod(method) {
				continue
			}

			operationObj, ok := operation.(map[string]interface{})
			if !ok {
				continue
			}

			endpoint := APIEndpoint{
				Path:      path,
				Method:    strings.ToUpper(method),
				Responses: make(map[string]Response),
			}

			if summary, ok := operationObj["summary"].(string); ok {
				endpoint.Summary = summary
			}

			if description, ok := operationObj["description"].(string); ok {
				endpoint.Description = description
			}

			if operationId, ok := operationObj["operationId"].(string); ok {
				endpoint.OperationID = operationId
			}

			// Parse parameters
			if parameters, ok := operationObj["parameters"].([]interface{}); ok {
				for _, param := range parameters {
					paramObj, ok := param.(map[string]interface{})
					if !ok {
						continue
					}
					endpoint.Parameters = append(endpoint.Parameters, p.parseParameter(paramObj))
				}
			}

			// Parse request body
			if requestBodyObj, ok := operationObj["requestBody"].(map[string]interface{}); ok {
				requestBody := p.parseRequestBody(requestBodyObj)
				endpoint.RequestBody = &requestBody
			}

			// Parse responses
			if responsesObj, ok := operationObj["responses"].(map[string]interface{}); ok {
				for statusCode, responseObj := range responsesObj {
					if responseMap, ok := responseObj.(map[string]interface{}); ok {
						endpoint.Responses[statusCode] = p.parseResponse(responseMap)
					}
				}
			}

			// Parse security,use global
			endpoint.Security = securities

			endpoints = append(endpoints, endpoint)
		}
	}

	return endpoints
}

func (p *SimpleOpenAPIParser) Security() map[string]interface{} {
	var security map[string]interface{}
	if securityObj, ok := p.document["security"].(map[string]interface{}); ok {
		security = securityObj
	}
	return security
}

func (p *SimpleOpenAPIParser) Components() Component {
	component := Component{
		Schemas: make(map[string]Schema),
		Responses: make(map[string]Response),
		Parameters: make(map[string]Parameter),
		RequestBodies: make(map[string]RequestBody),
		Headers: make(map[string]Header),
		SecuritySchemes: make(map[string]Security),
	}

	componentsObj, ok := p.document["components"].(map[string]interface{})
	if !ok {
		return component
	}

	if schemas, ok := componentsObj["schemas"].(map[string]interface{}); ok {
		for name, schema := range schemas {
			schemaObj, ok := schema.(map[string]interface{}) 
			if !ok {
				continue
			}
			component.Schemas[name] = p.parseSchema(schemaObj)
		}
	}
	if responses, ok := componentsObj["responses"].(map[string]interface{}); ok {
		for name, responseMap := range responses {
			responseObj, ok := responseMap.(map[string]interface{})
			if !ok {
				continue
			}
			component.Responses[name] = p.parseResponse(responseObj)
		}
	}
	if parameters, ok := componentsObj["parameters"].(map[string]interface{}); ok {
		for name, parameter := range parameters {
			parameterObj, ok := parameter.(map[string]interface{})
			if !ok {
				continue
			}
			component.Parameters[name] = p.parseParameter(parameterObj)
		}
	}
	if requestBodies, ok := componentsObj["requestBodies"].(map[string]interface{}); ok {
		for name, requestBodyData := range requestBodies {
			requestBodyObj, ok := requestBodyData.(map[string]interface{})
			if !ok {
				continue
			}
			component.RequestBodies[name] = p.parseRequestBody(requestBodyObj)
		}
	}
	if headers, ok := componentsObj["headers"].(map[string]interface{}); ok {
		for name, headerData := range headers {
			headerObj, ok := headerData.(map[string]interface{})
			if !ok {
				continue
			}
			header := Header{}
			if required, ok := headerObj["required"].(bool); ok {
				header.Required = required
			}
			if description, ok := headerObj["description"].(string); ok {
				header.Description = description
			}
			if schemaObj, ok := headerObj["schema"].(map[string]interface{}); ok {
				schema := p.parseSchema(schemaObj)
				header.Schema = &schema
			}
			component.Headers[name] = header
		}
	}
	if securitySchemes, ok := componentsObj["securitySchemes"].(map[string]interface{}); ok {
		for name, securityScheme := range securitySchemes {
			securitySchemeObj, ok := securityScheme.(map[string]interface{})
			if !ok {
				continue
			}
			security := Security{}
			if ty, ok := securitySchemeObj["type"].(string); ok {
				security.Type = ty
			}
			if description, ok := securitySchemeObj["description"].(string); ok {
				security.Description = description
			}
			if val, ok := securitySchemeObj["name"].(string); ok {
				security.Name = val
			}
			if in, ok := securitySchemeObj["in"].(string); ok {
				security.In = in
			}
			if scheme, ok := securitySchemeObj["scheme"].(string); ok {
				security.Scheme = scheme
			}
			if bearerFormat, ok := securitySchemeObj["bearerFormat"].(string); ok {
				security.BearerFormat = bearerFormat
			}
			component.SecuritySchemes[name] = security
		}
	}

	return component
}

// parseSchemaRef parses a JSON schema object
// ref:"#/components/schemas/TeamPerformanceQuery"
func (p *SimpleOpenAPIParser) parseSchemaRef(ref string) (*Schema,error) {
    // del #/components/
    newStr := strings.TrimPrefix(ref, "#/components/")
    // split by /
    parts := strings.Split(newStr, "/")
	if len(parts) < 2 || len(parts[0]) == 0{
		return nil,fmt.Errorf("failed to parse ref: %s", ref)
	}
	var obj map[string]Schema
	switch  parts[0]{
		case "schemas":
			obj = p.component.Schemas
		// case "responses":
		// 	obj = component.Responses
		// case "parameters":
		// 	obj = component.Parameters
		// case "requestBodies":
		// 	obj = component.RequestBodies
		// case "headers":
		// 	obj = component.Headers
		// case "securitySchemes":
		// 	obj = component.SecuritySchemes
	}
	subPart := parts[1]
	if schema,ok := obj[subPart]; ok{
		return &schema, nil
	}
	return nil, fmt.Errorf("failed to parse ref: %s", ref)
}

func (p *SimpleOpenAPIParser) parseResponse(responseMap map[string]interface{}) Response {
	response := Response{
		Content: make(map[string]MediaType),
	}

	if description, ok := responseMap["description"].(string); ok {
		response.Description = description
	}

	if contentObj, ok := responseMap["content"].(map[string]interface{}); ok {
		for mediaTypeName, mediaTypeObj := range contentObj {
			if mediaTypeMap, ok := mediaTypeObj.(map[string]interface{}); ok {
				mediaType := MediaType{}

				if schemaObj, ok := mediaTypeMap["schema"].(map[string]interface{}); ok {
					schema := p.parseSchema(schemaObj)
					mediaType.Schema = &schema
				}

				response.Content[mediaTypeName] = mediaType
			}
		}
	}
	return response
}

func (p *SimpleOpenAPIParser) parseRequestBody(requestBodyObj map[string]interface{}) RequestBody {
	requestBody := RequestBody{
		Content: make(map[string]MediaType),
	}

	if required, ok := requestBodyObj["required"].(bool); ok {
		requestBody.Required = required
	}

	if contentObj, ok := requestBodyObj["content"].(map[string]interface{}); ok {
		for mediaTypeName, mediaTypeObj := range contentObj {
			if mediaTypeMap, ok := mediaTypeObj.(map[string]interface{}); ok {
				mediaType := MediaType{}

				if schemaObj, ok := mediaTypeMap["schema"].(map[string]interface{}); ok {
					schema := p.parseSchema(schemaObj)
					mediaType.Schema = &schema
				} 

				requestBody.Content[mediaTypeName] = mediaType
			}
		}
	}
	return requestBody
}

func (p *SimpleOpenAPIParser) parseParameter(paramObj map[string]interface{}) Parameter {
	parameter := Parameter{}

	if name, ok := paramObj["name"].(string); ok {
		parameter.Name = name
	}

	if in, ok := paramObj["in"].(string); ok {
		parameter.In = in
	}

	if required, ok := paramObj["required"].(bool); ok {
		parameter.Required = required
	}

	if description, ok := paramObj["description"].(string); ok {
		parameter.Description = description
	}

	if schemaObj, ok := paramObj["schema"].(map[string]interface{}); ok {
		schema := p.parseSchema(schemaObj)
		parameter.Schema = &schema
	}
	return parameter
}
// parseSchema parses a JSON schema object
func (p *SimpleOpenAPIParser) parseSchema(schemaObj map[string]interface{}) Schema {
	if ref, ok := schemaObj["$ref"].(string); ok {
		if schema,err := p.parseSchemaRef(ref); err == nil {
			return *schema
		}
	}

	schema := Schema{
		Properties: make(map[string]Schema),
	}

	if t, ok := schemaObj["type"].(string); ok {
		schema.Type = t
	}

	if format, ok := schemaObj["format"].(string); ok {
		schema.Format = format
	}

	if description, ok := schemaObj["description"].(string); ok {
		schema.Description = description
	}

	if defaultValue, ok := schemaObj["default"]; ok {
		schema.Default = defaultValue
	}

	if enum, ok := schemaObj["enum"].([]interface{}); ok {
		schema.Enum = enum
	}

	if required, ok := schemaObj["required"].([]interface{}); ok {
		for _, req := range required {
			if reqStr, ok := req.(string); ok {
				schema.Required = append(schema.Required, reqStr)
			}
		}
	}

	// Handle properties
	if properties, ok := schemaObj["properties"].(map[string]interface{}); ok {
		for propName, propObj := range properties {
			if propMap, ok := propObj.(map[string]interface{}); ok {
				propSchema := p.parseSchema(propMap)
				schema.Properties[propName] = propSchema
			}
		}
	}

	// Handle items for array type
	if items, ok := schemaObj["items"].(map[string]interface{}); ok {
		itemsSchema := p.parseSchema(items)
		schema.Items = &itemsSchema
	}

	return schema
}

func isHTTPMethod(method string) bool {
	method = strings.ToLower(method)
	return method == "get" || method == "post" || method == "put" ||
		method == "delete" || method == "options" || method == "head" ||
		method == "patch" || method == "trace"
}

// ParseOpenAPIFromYAML parses OpenAPI specification from YAML format
func ParseOpenAPIFromYAML(data []byte) (OpenAPIParser, error) {
	// Convert YAML to JSON for consistent processing
	var jsonData []byte
	var yamlObj interface{}

	// Unmarshal YAML to an interface
	if err := yaml.Unmarshal(data, &yamlObj); err != nil {
		return nil, fmt.Errorf("failed to unmarshal YAML: %w", err)
	}

	// Marshal the interface to JSON
	jsonData, err := json.Marshal(yamlObj)
	if err != nil {
		return nil, fmt.Errorf("failed to convert YAML to JSON: %w", err)
	}

	// Use the JSON data for parsing
	data = jsonData
	parser, err := NewSimpleOpenAPIParser(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse OpenAPI specification: %w", err)
	}
	return parser, nil
}

// ParseOpenAPIFromJSON parses an OpenAPI specification from JSON
func ParseOpenAPIFromJSON(data []byte) (OpenAPIParser, error) {
	return NewSimpleOpenAPIParser(data)
}
