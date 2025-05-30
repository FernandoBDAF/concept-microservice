package domain

import (
	"fmt"
	"reflect"
	"strings"
)

// DocumentationGenerator generates documentation for models
type DocumentationGenerator struct {
	models map[reflect.Type]*ModelDocumentation
}

// ModelDocumentation represents documentation for a model
type ModelDocumentation struct {
	Name        string
	Description string
	Fields      []FieldDocumentation
	Examples    []ExampleDocumentation
}

// FieldDocumentation represents documentation for a model field
type FieldDocumentation struct {
	Name        string
	Type        string
	Description string
	Required    bool
	Validation  []string
	Example     interface{}
}

// ExampleDocumentation represents an example usage of a model
type ExampleDocumentation struct {
	Name        string
	Description string
	Value       interface{}
}

// NewDocumentationGenerator creates a new DocumentationGenerator
func NewDocumentationGenerator() *DocumentationGenerator {
	return &DocumentationGenerator{
		models: make(map[reflect.Type]*ModelDocumentation),
	}
}

// RegisterModel registers a model for documentation
func (g *DocumentationGenerator) RegisterModel(modelType reflect.Type, doc *ModelDocumentation) {
	g.models[modelType] = doc
}

// GenerateDocumentation generates documentation for all registered models
func (g *DocumentationGenerator) GenerateDocumentation() string {
	var sb strings.Builder

	sb.WriteString("# Model Documentation\n\n")

	for _, doc := range g.models {
		sb.WriteString(fmt.Sprintf("## %s\n\n", doc.Name))
		sb.WriteString(fmt.Sprintf("%s\n\n", doc.Description))

		sb.WriteString("### Fields\n\n")
		sb.WriteString("| Name | Type | Required | Description |\n")
		sb.WriteString("|------|------|----------|-------------|\n")

		for _, field := range doc.Fields {
			required := "No"
			if field.Required {
				required = "Yes"
			}
			sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n",
				field.Name, field.Type, required, field.Description))
		}
		sb.WriteString("\n")

		if len(doc.Examples) > 0 {
			sb.WriteString("### Examples\n\n")
			for _, example := range doc.Examples {
				sb.WriteString(fmt.Sprintf("#### %s\n\n", example.Name))
				sb.WriteString(fmt.Sprintf("%s\n\n", example.Description))
				sb.WriteString("```json\n")
				sb.WriteString(fmt.Sprintf("%+v\n", example.Value))
				sb.WriteString("```\n\n")
			}
		}
	}

	return sb.String()
}

// GenerateFieldDocumentation generates documentation for a model's fields
func (g *DocumentationGenerator) GenerateFieldDocumentation(modelType reflect.Type) []FieldDocumentation {
	var fields []FieldDocumentation

	for i := 0; i < modelType.NumField(); i++ {
		field := modelType.Field(i)
		fieldDoc := FieldDocumentation{
			Name:        field.Name,
			Type:        field.Type.String(),
			Description: field.Tag.Get("doc"),
			Required:    strings.Contains(field.Tag.Get("validate"), "required"),
		}

		// Parse validation rules
		validation := field.Tag.Get("validate")
		if validation != "" {
			fieldDoc.Validation = strings.Split(validation, ",")
		}

		// Get example value
		if example := field.Tag.Get("example"); example != "" {
			fieldDoc.Example = example
		}

		fields = append(fields, fieldDoc)
	}

	return fields
}

// GenerateExampleDocumentation generates example documentation for a model
func (g *DocumentationGenerator) GenerateExampleDocumentation(modelType reflect.Type, examples []interface{}) []ExampleDocumentation {
	var docs []ExampleDocumentation

	for i, example := range examples {
		doc := ExampleDocumentation{
			Name:  fmt.Sprintf("Example %d", i+1),
			Value: example,
		}

		// Try to get description from struct tags if available
		if t := reflect.TypeOf(example); t.Kind() == reflect.Struct {
			if field, ok := t.FieldByName("Description"); ok {
				doc.Description = field.Tag.Get("doc")
			}
		}

		docs = append(docs, doc)
	}

	return docs
}

// GenerateMarkdown generates markdown documentation for a model
func (g *DocumentationGenerator) GenerateMarkdown(modelType reflect.Type) string {
	doc, exists := g.models[modelType]
	if !exists {
		return fmt.Sprintf("No documentation found for type %s", modelType.Name())
	}

	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# %s\n\n", doc.Name))
	sb.WriteString(fmt.Sprintf("%s\n\n", doc.Description))

	sb.WriteString("## Fields\n\n")
	sb.WriteString("| Name | Type | Required | Description |\n")
	sb.WriteString("|------|------|----------|-------------|\n")

	for _, field := range doc.Fields {
		required := "No"
		if field.Required {
			required = "Yes"
		}
		sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n",
			field.Name, field.Type, required, field.Description))
	}

	if len(doc.Examples) > 0 {
		sb.WriteString("\n## Examples\n\n")
		for _, example := range doc.Examples {
			sb.WriteString(fmt.Sprintf("### %s\n\n", example.Name))
			sb.WriteString(fmt.Sprintf("%s\n\n", example.Description))
			sb.WriteString("```json\n")
			sb.WriteString(fmt.Sprintf("%+v\n", example.Value))
			sb.WriteString("```\n\n")
		}
	}

	return sb.String()
}
