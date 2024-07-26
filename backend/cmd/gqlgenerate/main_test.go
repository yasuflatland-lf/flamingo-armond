package main

import (
	"log"
	"os"
	"testing"

	"github.com/99designs/gqlgen/plugin/modelgen"
	"github.com/vektah/gqlparser/v2/ast"
)

func TestLoadGraphQLConfig(t *testing.T) {
	_, err := loadGraphQLConfig()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
}

func TestGenerateGraphQLCode(t *testing.T) {
	logger := log.New(os.Stdout, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile)
	cfg, err := loadGraphQLConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	err = generateGraphQLCode(cfg, logger)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
}

func TestValidationFieldHook(t *testing.T) {
	// Mock field definitions and directive
	fd := &ast.FieldDefinition{
		Directives: ast.DirectiveList{
			{
				Name: "validation",
				Arguments: ast.ArgumentList{
					{
						Name: "format",
						Value: &ast.Value{
							Raw:  "email",
							Kind: ast.StringValue,
						},
					},
				},
			},
		},
	}
	f := &modelgen.Field{}

	_, err := ValidationFieldHook(nil, fd, f)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	expectedTag := ` validate:"email"`
	if f.Tag != expectedTag {
		t.Fatalf("Expected tag %v, got %v", expectedTag, f.Tag)
	}
}
