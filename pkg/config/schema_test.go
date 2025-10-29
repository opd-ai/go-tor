package config

import (
	"encoding/json"
	"testing"
	"time"
)

func TestGenerateJSONSchema(t *testing.T) {
	schema, err := GenerateJSONSchema()
	if err != nil {
		t.Fatalf("GenerateJSONSchema() error = %v", err)
	}

	if schema == nil {
		t.Fatal("GenerateJSONSchema() returned nil schema")
	}

	// Validate schema structure
	if schema.Schema != "http://json-schema.org/draft-07/schema#" {
		t.Errorf("Schema field = %v, want http://json-schema.org/draft-07/schema#", schema.Schema)
	}

	if schema.Title == "" {
		t.Error("Schema title is empty")
	}

	if schema.Type != "object" {
		t.Errorf("Schema type = %v, want object", schema.Type)
	}

	// Check that key properties are present
	requiredProps := []string{
		"SocksPort",
		"ControlPort",
		"DataDirectory",
		"LogLevel",
		"CircuitBuildTimeout",
		"NumEntryGuards",
	}

	for _, prop := range requiredProps {
		if _, exists := schema.Properties[prop]; !exists {
			t.Errorf("Schema missing required property: %s", prop)
		}
	}

	// Validate OnionServiceConfig definition
	if _, exists := schema.Definitions["OnionServiceConfig"]; !exists {
		t.Error("Schema missing OnionServiceConfig definition")
	}
}

func TestJSONSchemaToJSON(t *testing.T) {
	schema, err := GenerateJSONSchema()
	if err != nil {
		t.Fatalf("GenerateJSONSchema() error = %v", err)
	}

	jsonData, err := schema.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON() error = %v", err)
	}

	if len(jsonData) == 0 {
		t.Fatal("ToJSON() returned empty data")
	}

	// Validate JSON can be parsed
	var parsed map[string]interface{}
	if err := json.Unmarshal(jsonData, &parsed); err != nil {
		t.Fatalf("Generated JSON is invalid: %v", err)
	}

	// Check structure
	if parsed["$schema"] != "http://json-schema.org/draft-07/schema#" {
		t.Error("JSON schema $schema field incorrect")
	}

	if parsed["type"] != "object" {
		t.Error("JSON schema type field incorrect")
	}
}

func TestValidateDetailed(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		wantValid   bool
		wantErrors  int
		wantWarnings int
	}{
		{
			name:        "valid config",
			config:      DefaultConfig(),
			wantValid:   true,
			wantErrors:  0,
			wantWarnings: 0,
		},
		{
			name: "invalid port",
			config: &Config{
				SocksPort:           99999,
				ControlPort:         9051,
				CircuitBuildTimeout: 60 * time.Second,
				MaxCircuitDirtiness: 10 * time.Minute,
				NumEntryGuards:      3,
				ConnLimit:           1000,
				LogLevel:            "info",
				CircuitPoolMinSize:  2,
				CircuitPoolMaxSize:  10,
				IsolationLevel:      "none",
			},
			wantValid:  false,
			wantErrors: 1,
		},
		{
			name: "port conflict",
			config: &Config{
				SocksPort:           9050,
				ControlPort:         9050, // Same as SOCKS
				CircuitBuildTimeout: 60 * time.Second,
				MaxCircuitDirtiness: 10 * time.Minute,
				NumEntryGuards:      3,
				ConnLimit:           1000,
				LogLevel:            "info",
				CircuitPoolMinSize:  2,
				CircuitPoolMaxSize:  10,
				IsolationLevel:      "none",
			},
			wantValid:  false,
			wantErrors: 1,
		},
		{
			name: "invalid log level",
			config: &Config{
				SocksPort:           9050,
				ControlPort:         9051,
				CircuitBuildTimeout: 60 * time.Second,
				MaxCircuitDirtiness: 10 * time.Minute,
				NumEntryGuards:      3,
				ConnLimit:           1000,
				LogLevel:            "invalid",
				CircuitPoolMinSize:  2,
				CircuitPoolMaxSize:  10,
				IsolationLevel:      "none",
			},
			wantValid:  false,
			wantErrors: 1,
		},
		{
			name: "privileged port warning",
			config: &Config{
				SocksPort:           80, // Privileged port
				ControlPort:         9051,
				CircuitBuildTimeout: 60 * time.Second,
				MaxCircuitDirtiness: 10 * time.Minute,
				NumEntryGuards:      3,
				ConnLimit:           1000,
				LogLevel:            "info",
				CircuitPoolMinSize:  2,
				CircuitPoolMaxSize:  10,
				IsolationLevel:      "none",
			},
			wantValid:    true,
			wantErrors:   0,
			wantWarnings: 1,
		},
		{
			name: "circuit pool size mismatch",
			config: &Config{
				SocksPort:           9050,
				ControlPort:         9051,
				CircuitBuildTimeout: 60 * time.Second,
				MaxCircuitDirtiness: 10 * time.Minute,
				NumEntryGuards:      3,
				ConnLimit:           1000,
				LogLevel:            "info",
				CircuitPoolMinSize:  10,
				CircuitPoolMaxSize:  5, // Less than min
				IsolationLevel:      "none",
			},
			wantValid:  false,
			wantErrors: 1,
		},
		{
			name: "invalid isolation level",
			config: &Config{
				SocksPort:           9050,
				ControlPort:         9051,
				CircuitBuildTimeout: 60 * time.Second,
				MaxCircuitDirtiness: 10 * time.Minute,
				NumEntryGuards:      3,
				ConnLimit:           1000,
				LogLevel:            "info",
				CircuitPoolMinSize:  2,
				CircuitPoolMaxSize:  10,
				IsolationLevel:      "invalid",
			},
			wantValid:  false,
			wantErrors: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.config.ValidateDetailed()

			if result.Valid != tt.wantValid {
				t.Errorf("ValidateDetailed().Valid = %v, want %v", result.Valid, tt.wantValid)
			}

			if len(result.Errors) != tt.wantErrors {
				t.Errorf("ValidateDetailed() errors = %d, want %d", len(result.Errors), tt.wantErrors)
				for _, err := range result.Errors {
					t.Logf("  Error: %v", err)
				}
			}

			if len(result.Warnings) != tt.wantWarnings {
				t.Errorf("ValidateDetailed() warnings = %d, want %d", len(result.Warnings), tt.wantWarnings)
				for _, warn := range result.Warnings {
					t.Logf("  Warning: %v", warn)
				}
			}
		})
	}
}

func TestValidationError(t *testing.T) {
	tests := []struct {
		name    string
		err     ValidationError
		wantMsg string
	}{
		{
			name: "with suggestion",
			err: ValidationError{
				Field:      "SocksPort",
				Value:      99999,
				Message:    "invalid port",
				Suggestion: "use port 9050",
				Severity:   "error",
			},
			wantMsg: "SocksPort: invalid port (suggestion: use port 9050)",
		},
		{
			name: "without suggestion",
			err: ValidationError{
				Field:    "LogLevel",
				Value:    "invalid",
				Message:  "invalid log level",
				Severity: "error",
			},
			wantMsg: "LogLevel: invalid log level",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.wantMsg {
				t.Errorf("ValidationError.Error() = %v, want %v", got, tt.wantMsg)
			}
		})
	}
}

func TestValidateDetailedWithOnionServices(t *testing.T) {
	cfg := DefaultConfig()
	cfg.OnionServices = []OnionServiceConfig{
		{
			ServiceDir:  "/var/lib/tor/service1",
			VirtualPort: 80,
			TargetAddr:  "localhost:8080",
			MaxStreams:  0,
		},
		{
			ServiceDir:  "", // Invalid: empty
			VirtualPort: 443,
			TargetAddr:  "localhost:8443",
		},
		{
			ServiceDir:  "/var/lib/tor/service3",
			VirtualPort: 99999, // Invalid: out of range
			TargetAddr:  "localhost:9000",
		},
	}

	result := cfg.ValidateDetailed()

	if result.Valid {
		t.Error("ValidateDetailed() should fail with invalid onion services")
	}

	if len(result.Errors) < 2 {
		t.Errorf("ValidateDetailed() should have at least 2 errors, got %d", len(result.Errors))
	}

	// Check that errors mention the onion service index
	foundServiceError := false
	for _, err := range result.Errors {
		if err.Field == "OnionServices[1].ServiceDir" || err.Field == "OnionServices[2].VirtualPort" {
			foundServiceError = true
			break
		}
	}

	if !foundServiceError {
		t.Error("ValidateDetailed() should report onion service field errors with index")
	}
}

func TestJSONSchemaPropertiesComplete(t *testing.T) {
	schema, err := GenerateJSONSchema()
	if err != nil {
		t.Fatalf("GenerateJSONSchema() error = %v", err)
	}

	// All Config fields should be in schema
	expectedFields := []string{
		"SocksPort", "ControlPort", "DataDirectory",
		"CircuitBuildTimeout", "MaxCircuitDirtiness", "NewCircuitPeriod",
		"NumEntryGuards", "UseEntryGuards", "UseBridges",
		"BridgeAddresses", "ExcludeNodes", "ExcludeExitNodes",
		"ConnLimit", "DormantTimeout", "OnionServices",
		"LogLevel", "MetricsPort", "EnableMetrics",
		"EnableConnectionPooling", "ConnectionPoolMaxIdle", "ConnectionPoolMaxLife",
		"EnableCircuitPrebuilding", "CircuitPoolMinSize", "CircuitPoolMaxSize",
		"EnableBufferPooling", "IsolationLevel", "IsolateDestinations",
		"IsolateSOCKSAuth", "IsolateClientPort", "IsolateClientProtocol",
	}

	for _, field := range expectedFields {
		if _, exists := schema.Properties[field]; !exists {
			t.Errorf("Schema missing field: %s", field)
		}
	}
}

func TestJSONSchemaEnumValidation(t *testing.T) {
	schema, err := GenerateJSONSchema()
	if err != nil {
		t.Fatalf("GenerateJSONSchema() error = %v", err)
	}

	// Check LogLevel enum
	logLevelProp := schema.Properties["LogLevel"]
	expectedLogLevels := []string{"debug", "info", "warn", "error"}
	if len(logLevelProp.Enum) != len(expectedLogLevels) {
		t.Errorf("LogLevel enum count = %d, want %d", len(logLevelProp.Enum), len(expectedLogLevels))
	}

	// Check IsolationLevel enum
	isolationProp := schema.Properties["IsolationLevel"]
	expectedIsolation := []string{"none", "destination", "credential", "port", "session"}
	if len(isolationProp.Enum) != len(expectedIsolation) {
		t.Errorf("IsolationLevel enum count = %d, want %d", len(isolationProp.Enum), len(expectedIsolation))
	}
}
