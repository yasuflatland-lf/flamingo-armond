package main

import (
	"backend/pkg/logger"
	"fmt"
	"github.com/99designs/gqlgen/api"
	"github.com/99designs/gqlgen/codegen/config"
	"github.com/99designs/gqlgen/plugin/modelgen"
	"github.com/vektah/gqlparser/v2/ast"
)

func main() {

	// Load the GraphQL configuration
	cfg, err := loadGraphQLConfig()
	if err != nil {
		logger.Logger.Error("Error loading GraphQL config: %v", err)
	}

	// Generate the GraphQL server code
	err = generateGraphQLCode(cfg)
	if err != nil {
		logger.Logger.Error("Error generating GraphQL code: %v", err)
	}
}

// loadGraphQLConfig loads the GraphQL configuration from default locations
func loadGraphQLConfig() (*config.Config, error) {
	cfg, err := config.LoadConfigFromDefaultLocations()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	return cfg, nil
}

// generateGraphQLCode generates the GraphQL server code using the provided config
func generateGraphQLCode(cfg *config.Config) error {
	// Attaching the mutation function onto modelgen plugin
	p := modelgen.Plugin{
		FieldHook: ValidationFieldHook,
	}

	// Generate the code using the API
	err := api.Generate(cfg, api.ReplacePlugin(&p))
	if err != nil {
		return fmt.Errorf("code generation failed: %w", err)
	}
	logger.Logger.Info("GraphQL code generation successful")
	return nil
}

// ValidationFieldHook is a custom hook for adding validation tags to fields based on directives
func ValidationFieldHook(td *ast.Definition, fd *ast.FieldDefinition, f *modelgen.Field) (*modelgen.Field, error) {
	// Look for the "validation" directive on the field
	c := fd.Directives.ForName("validation")
	if c != nil {
		// Add validation tag based on the "format" argument in the directive
		formatConstraint := c.Arguments.ForName("format")
		if formatConstraint != nil {
			// Use a format that avoids double quoting
			f.Tag += fmt.Sprintf(` validate:"%s"`, formatConstraint.Value.Raw)
		}
	}
	return f, nil
}
