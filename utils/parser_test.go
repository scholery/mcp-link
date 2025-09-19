package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	jsref "github.com/lestrrat-go/jsref"
	"sigs.k8s.io/yaml"
)

func Test_ParseYamlToJson(t *testing.T) {
	b, err := os.ReadFile("../example-fal-text2image.yaml")
	if err != nil {
		t.Fatalf("Error reading YAML file: %v", err)
	}

	jsonString, err := yaml.YAMLToJSON(b)
	if err != nil {
		t.Fatalf("Error converting YAML to JSON: %v", err)
	}

	// Parse JSON into interface{}
	var v interface{}
	if err := json.Unmarshal(jsonString, &v); err != nil {
		t.Fatalf("Error unmarshaling JSON: %v", err)
	}

	// Create a new resolver
	resolver := jsref.New()

	// Resolve all references recursively
	resolved, err := resolver.Resolve(v, "#", jsref.WithRecursiveResolution(true))
	if err != nil {
		t.Fatalf("Error resolving references: %v", err)
	}

	// Convert back to JSON for OpenAPI processing
	resolvedJSON, err := json.Marshal(resolved)
	if err != nil {
		t.Fatalf("Error marshaling resolved JSON: %v", err)
	}

	// Load into OpenAPI document
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true

	doc, err := loader.LoadFromData(resolvedJSON)
	if err != nil {
		t.Fatalf("Error loading JSON: %v", err)
	}

	// Validate the document
	if err := doc.Validate(context.Background()); err != nil {
		t.Fatalf("Error validating JSON: %v", err)
	}

	// Format the final result
	prettyJSON, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatalf("Error marshaling JSON: %v", err)
	}

	fmt.Println(string(prettyJSON))
}

func Test_ParseRef(t *testing.T){
	rawBytes, err := os.ReadFile("../ossm-test.json")
	if err != nil {
		fmt.Printf("failed to load schema: %v", err)
		return
	}
	parser, parseErr := ParseOpenAPIFromJSON(rawBytes)
	if parseErr != nil {
		fmt.Printf("failed to parse schema: %v", parseErr)
		return
	}
	apis := parser.APIs()
	fmt.Printf("apis: %v", apis)

	NewMCPFromCustomParser("http://test/", nil, parser)
}

