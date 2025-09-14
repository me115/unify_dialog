package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

// ParameterResolver 处理来自各种源的动态参数解析
type ParameterResolver struct {
	state *ExecutionState
}

// NewParameterResolver 创建新的参数解析器
func NewParameterResolver(state *ExecutionState) *ParameterResolver {
	return &ParameterResolver{
		state: state,
	}
}

// ResolveParameters 解析步骤中的所有参数，用实际值替换引用
func (r *ParameterResolver) ResolveParameters(ctx context.Context, step *ExecutionStep) (map[string]interface{}, error) {
	resolved := make(map[string]interface{})

	for key, value := range step.Parameters {
		resolvedValue, err := r.resolveParameter(ctx, value)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve parameter '%s': %w", key, err)
		}
		resolved[key] = resolvedValue
	}

	return resolved, nil
}

// resolveParameter recursively resolves a single parameter value
func (r *ParameterResolver) resolveParameter(ctx context.Context, value interface{}) (interface{}, error) {
	switch v := value.(type) {
	case map[string]interface{}:
		// Check if this is a reference object
		if ref, exists := v["$ref"]; exists {
			return r.resolveReference(ctx, ref.(string))
		}

		// Check if this is a parameter reference object
		if _, exists := v["type"]; exists {
			return r.resolveParameterReference(ctx, v)
		}

		// Otherwise, recursively resolve each field
		resolved := make(map[string]interface{})
		for k, val := range v {
			resolvedVal, err := r.resolveParameter(ctx, val)
			if err != nil {
				return nil, err
			}
			resolved[k] = resolvedVal
		}
		return resolved, nil

	case []interface{}:
		// Resolve each element in the array
		resolved := make([]interface{}, len(v))
		for i, val := range v {
			resolvedVal, err := r.resolveParameter(ctx, val)
			if err != nil {
				return nil, err
			}
			resolved[i] = resolvedVal
		}
		return resolved, nil

	case string:
		// Check if the string contains template variables
		return r.resolveStringTemplate(ctx, v)

	default:
		// Return static values as-is
		return value, nil
	}
}

// resolveReference resolves a simple $ref reference
func (r *ParameterResolver) resolveReference(ctx context.Context, reference string) (interface{}, error) {
	if strings.HasPrefix(reference, "steps.") {
		return r.resolveStepReference(reference)
	}

	if strings.HasPrefix(reference, "inputs.") {
		return r.resolveInputReference(reference)
	}

	if strings.HasPrefix(reference, "env.") {
		return r.resolveEnvironmentReference(reference)
	}

	return nil, fmt.Errorf("unknown reference type: %s", reference)
}

// resolveParameterReference resolves a structured parameter reference
func (r *ParameterResolver) resolveParameterReference(ctx context.Context, refObj map[string]interface{}) (interface{}, error) {
	refType, ok := refObj["type"].(string)
	if !ok {
		return nil, fmt.Errorf("parameter reference missing type field")
	}

	reference, ok := refObj["reference"].(string)
	if !ok {
		return nil, fmt.Errorf("parameter reference missing reference field")
	}

	var value interface{}
	var err error

	switch ReferenceType(refType) {
	case ReferenceStepResult:
		value, err = r.resolveStepReference("steps." + reference)
	case ReferenceUserInput:
		value, err = r.resolveInputReference("inputs." + reference)
	case ReferenceDataStore:
		value, err = r.resolveDataStoreReference(reference)
	case ReferenceEnvironment:
		value, err = r.resolveEnvironmentReference("env." + reference)
	case ReferenceStatic:
		value = refObj["value"]
	default:
		return nil, fmt.Errorf("unknown reference type: %s", refType)
	}

	if err != nil {
		// Check for default value
		if defaultValue, hasDefault := refObj["default"]; hasDefault {
			value = defaultValue
			err = nil
		} else {
			return nil, err
		}
	}

	// Apply path extraction if specified
	if path, hasPath := refObj["path"].(string); hasPath && path != "" {
		value, err = r.extractValueByPath(value, path)
		if err != nil {
			return nil, err
		}
	}

	// Apply transformation if specified
	if transform, hasTransform := refObj["transform"].(string); hasTransform && transform != "" {
		value, err = r.applyTransformation(value, transform)
		if err != nil {
			return nil, err
		}
	}

	return value, nil
}

// resolveStepReference resolves references to step results
func (r *ParameterResolver) resolveStepReference(reference string) (interface{}, error) {
	parts := strings.Split(reference, ".")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid step reference: %s", reference)
	}

	stepID := parts[1]

	// Get step result from data store
	result, exists := r.state.DataStore[stepID]
	if !exists {
		return nil, fmt.Errorf("step '%s' not found or not executed", stepID)
	}

	// If there's a path, extract specific value
	if len(parts) > 2 {
		path := strings.Join(parts[2:], ".")
		return r.extractValueByPath(result, path)
	}

	return result, nil
}

// resolveInputReference resolves references to user input
func (r *ParameterResolver) resolveInputReference(reference string) (interface{}, error) {
	parts := strings.Split(reference, ".")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid input reference: %s", reference)
	}

	inputType := parts[1]

	switch inputType {
	case "all":
		return r.state.UserInput, nil
	case "latest":
		if len(r.state.UserInput) > 0 {
			return r.state.UserInput[len(r.state.UserInput)-1], nil
		}
		return nil, fmt.Errorf("no user input available")
	case "content":
		if len(r.state.UserInput) > 0 {
			return r.state.UserInput[len(r.state.UserInput)-1].Content, nil
		}
		return nil, fmt.Errorf("no user input available")
	default:
		// Try to parse as index
		if idx, err := strconv.Atoi(inputType); err == nil {
			if idx < 0 || idx >= len(r.state.UserInput) {
				return nil, fmt.Errorf("input index %d out of range", idx)
			}
			return r.state.UserInput[idx], nil
		}
		return nil, fmt.Errorf("unknown input type: %s", inputType)
	}
}

// resolveDataStoreReference resolves references to data store values
func (r *ParameterResolver) resolveDataStoreReference(reference string) (interface{}, error) {
	value, exists := r.state.DataStore[reference]
	if !exists {
		return nil, fmt.Errorf("data store key '%s' not found", reference)
	}
	return value, nil
}

// resolveEnvironmentReference resolves references to environment variables
func (r *ParameterResolver) resolveEnvironmentReference(reference string) (interface{}, error) {
	parts := strings.Split(reference, ".")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid environment reference: %s", reference)
	}

	envVar := parts[1]
	value := os.Getenv(envVar)
	if value == "" {
		return nil, fmt.Errorf("environment variable '%s' not found", envVar)
	}

	return value, nil
}

// resolveStringTemplate resolves template variables in strings (e.g., "${steps.step1.result}")
func (r *ParameterResolver) resolveStringTemplate(ctx context.Context, template string) (interface{}, error) {
	// Pattern to match ${reference} template variables
	re := regexp.MustCompile(`\$\{([^}]+)\}`)

	result := template
	matches := re.FindAllStringSubmatch(template, -1)

	for _, match := range matches {
		if len(match) != 2 {
			continue
		}

		placeholder := match[0] // ${...}
		reference := match[1]   // content inside {}

		// Resolve the reference
		value, err := r.resolveReference(ctx, reference)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve template variable '%s': %w", reference, err)
		}

		// Convert value to string for replacement
		valueStr := fmt.Sprintf("%v", value)
		result = strings.Replace(result, placeholder, valueStr, -1)
	}

	// If the entire string was a template and resolved to non-string, return the actual type
	if template == "${"+strings.TrimPrefix(strings.TrimSuffix(template, "}"), "${")+"}" {
		// This was a pure template, return the resolved value directly
		if len(matches) == 1 {
			return r.resolveReference(ctx, matches[0][1])
		}
	}

	return result, nil
}

// extractValueByPath extracts a value from an object using a JSONPath-like syntax
func (r *ParameterResolver) extractValueByPath(data interface{}, path string) (interface{}, error) {
	if path == "" {
		return data, nil
	}

	// Handle array indexing: field[0], field[1], etc.
	if strings.Contains(path, "[") {
		return r.extractArrayValue(data, path)
	}

	// Handle object field access: field1.field2.field3
	return r.extractObjectValue(data, path)
}

// extractArrayValue handles array indexing in paths
func (r *ParameterResolver) extractArrayValue(data interface{}, path string) (interface{}, error) {
	// Parse array access pattern: field[index].subfield
	re := regexp.MustCompile(`^([^[]+)\[(\d+)\](.*)$`)
	matches := re.FindStringSubmatch(path)

	if len(matches) != 4 {
		return nil, fmt.Errorf("invalid array path: %s", path)
	}

	fieldName := matches[1]
	indexStr := matches[2]
	remainingPath := strings.TrimPrefix(matches[3], ".")

	// Parse index
	index, err := strconv.Atoi(indexStr)
	if err != nil {
		return nil, fmt.Errorf("invalid array index: %s", indexStr)
	}

	// Get the field first
	var fieldValue interface{}
	if fieldName != "" {
		fieldValue, err = r.extractObjectValue(data, fieldName)
		if err != nil {
			return nil, err
		}
	} else {
		fieldValue = data
	}

	// Extract array element
	rv := reflect.ValueOf(fieldValue)
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}

	switch rv.Kind() {
	case reflect.Slice, reflect.Array:
		if index < 0 || index >= rv.Len() {
			return nil, fmt.Errorf("array index %d out of bounds (length: %d)", index, rv.Len())
		}
		arrayElement := rv.Index(index).Interface()

		// Continue with remaining path if any
		if remainingPath != "" {
			return r.extractValueByPath(arrayElement, remainingPath)
		}
		return arrayElement, nil

	default:
		return nil, fmt.Errorf("field '%s' is not an array", fieldName)
	}
}

// extractObjectValue handles object field access in paths
func (r *ParameterResolver) extractObjectValue(data interface{}, path string) (interface{}, error) {
	parts := strings.SplitN(path, ".", 2)
	fieldName := parts[0]

	// Handle map[string]interface{}
	if dataMap, ok := data.(map[string]interface{}); ok {
		value, exists := dataMap[fieldName]
		if !exists {
			return nil, fmt.Errorf("field '%s' not found", fieldName)
		}

		// Continue with remaining path if any
		if len(parts) > 1 {
			return r.extractValueByPath(value, parts[1])
		}
		return value, nil
	}

	// Handle struct using reflection
	rv := reflect.ValueOf(data)
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}

	if rv.Kind() != reflect.Struct {
		return nil, fmt.Errorf("cannot extract field '%s' from non-object type", fieldName)
	}

	// Try to find field by name (case-insensitive)
	rt := rv.Type()
	for i := 0; i < rv.NumField(); i++ {
		field := rt.Field(i)

		// Check field name or JSON tag
		jsonTag := field.Tag.Get("json")
		if jsonTag != "" {
			jsonTag = strings.Split(jsonTag, ",")[0] // Remove options like omitempty
		}

		if strings.EqualFold(field.Name, fieldName) || strings.EqualFold(jsonTag, fieldName) {
			fieldValue := rv.Field(i).Interface()

			// Continue with remaining path if any
			if len(parts) > 1 {
				return r.extractValueByPath(fieldValue, parts[1])
			}
			return fieldValue, nil
		}
	}

	return nil, fmt.Errorf("field '%s' not found in struct", fieldName)
}

// applyTransformation applies a named transformation to a value
func (r *ParameterResolver) applyTransformation(value interface{}, transformName string) (interface{}, error) {
	switch transformName {
	case "string":
		return fmt.Sprintf("%v", value), nil

	case "json":
		jsonBytes, err := json.Marshal(value)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal to JSON: %w", err)
		}
		return string(jsonBytes), nil

	case "upper":
		if str, ok := value.(string); ok {
			return strings.ToUpper(str), nil
		}
		return nil, fmt.Errorf("upper transformation can only be applied to strings")

	case "lower":
		if str, ok := value.(string); ok {
			return strings.ToLower(str), nil
		}
		return nil, fmt.Errorf("lower transformation can only be applied to strings")

	case "length":
		rv := reflect.ValueOf(value)
		switch rv.Kind() {
		case reflect.String, reflect.Slice, reflect.Array, reflect.Map:
			return rv.Len(), nil
		default:
			return nil, fmt.Errorf("length transformation can only be applied to strings, slices, arrays, or maps")
		}

	case "first":
		rv := reflect.ValueOf(value)
		if rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array {
			if rv.Len() > 0 {
				return rv.Index(0).Interface(), nil
			}
			return nil, fmt.Errorf("cannot get first element of empty collection")
		}
		return nil, fmt.Errorf("first transformation can only be applied to slices or arrays")

	case "last":
		rv := reflect.ValueOf(value)
		if rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array {
			if rv.Len() > 0 {
				return rv.Index(rv.Len() - 1).Interface(), nil
			}
			return nil, fmt.Errorf("cannot get last element of empty collection")
		}
		return nil, fmt.Errorf("last transformation can only be applied to slices or arrays")

	default:
		return nil, fmt.Errorf("unknown transformation: %s", transformName)
	}
}