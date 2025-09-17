package unit

import (
	"testing"

	"github.com/berkunal/joblin/src/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestNewResourceSpec(t *testing.T) {
	spec := models.NewResourceSpec()

	assert.Equal(t, "100m", spec.CPU)
	assert.Equal(t, "128Mi", spec.Memory)
	assert.Equal(t, "1Gi", spec.EphemeralStorage)

	// Verify the defaults are valid
	err := spec.Validate()
	assert.NoError(t, err, "Default resource spec should be valid")
}

func TestResourceSpec_Validate_ValidSpecs(t *testing.T) {
	validSpecs := []struct {
		name    string
		cpu     string
		memory  string
		storage string
	}{
		{"minimum_valid", "10m", "64Mi", "100Mi"},
		{"small_resources", "50m", "128Mi", "500Mi"},
		{"medium_resources", "500m", "1Gi", "2Gi"},
		{"large_resources", "2", "8Gi", "5Gi"},
		{"maximum_valid", "16", "32Gi", "10Gi"},
		{"fractional_cpu", "0.5", "256Mi", "1Gi"},
		{"decimal_cpu", "1.5", "512Mi", "2Gi"},
		{"bytes_memory", "100m", "134217728", "1073741824"}, // 128Mi and 1Gi in bytes
		{"scientific_notation", "100m", "1e9", "1e10"},      // 1GB and 10GB
	}

	for _, tt := range validSpecs {
		t.Run(tt.name, func(t *testing.T) {
			spec := models.ResourceSpec{
				CPU:              tt.cpu,
				Memory:           tt.memory,
				EphemeralStorage: tt.storage,
			}

			err := spec.Validate()
			assert.NoError(t, err, "Spec should be valid: CPU=%s, Memory=%s, Storage=%s",
				tt.cpu, tt.memory, tt.storage)
		})
	}
}

func TestResourceSpec_Validate_InvalidCPU(t *testing.T) {
	invalidCPUSpecs := []struct {
		name        string
		cpu         string
		expectedErr string
	}{
		{"empty_cpu", "", "CPU cannot be empty"},
		{"invalid_format", "abc", "invalid CPU format"},
		{"negative_cpu", "-100m", "CPU must be positive"},
		{"zero_cpu", "0", "CPU must be positive"},
		{"below_minimum", "5m", "CPU must be at least 10m"},
		{"above_maximum", "17", "CPU cannot exceed 16 cores"},
		{"invalid_unit", "100x", "invalid CPU format"},
		{"invalid_number", "1.2.3", "invalid CPU format"},
		{"special_chars", "100m@", "invalid CPU format"},
	}

	for _, tt := range invalidCPUSpecs {
		t.Run(tt.name, func(t *testing.T) {
			spec := models.ResourceSpec{
				CPU:              tt.cpu,
				Memory:           "128Mi",
				EphemeralStorage: "1Gi",
			}

			err := spec.Validate()
			require.Error(t, err, "CPU %s should be invalid", tt.cpu)
			assert.Contains(t, err.Error(), tt.expectedErr)
		})
	}
}

func TestResourceSpec_Validate_InvalidMemory(t *testing.T) {
	invalidMemorySpecs := []struct {
		name        string
		memory      string
		expectedErr string
	}{
		{"empty_memory", "", "memory cannot be empty"},
		{"invalid_format", "xyz", "invalid memory format"},
		{"negative_memory", "-128Mi", "memory must be positive"},
		{"zero_memory", "0", "memory must be positive"},
		{"below_minimum", "32Mi", "memory must be at least 64Mi"},
		{"above_maximum", "64Gi", "memory cannot exceed 32Gi"},
		{"invalid_unit", "128Mx", "invalid memory format"},
		{"invalid_number", "1.2.3Mi", "invalid memory format"},
		{"special_chars", "128Mi@", "invalid memory format"},
	}

	for _, tt := range invalidMemorySpecs {
		t.Run(tt.name, func(t *testing.T) {
			spec := models.ResourceSpec{
				CPU:              "100m",
				Memory:           tt.memory,
				EphemeralStorage: "1Gi",
			}

			err := spec.Validate()
			require.Error(t, err, "Memory %s should be invalid", tt.memory)
			assert.Contains(t, err.Error(), tt.expectedErr)
		})
	}
}

func TestResourceSpec_Validate_InvalidEphemeralStorage(t *testing.T) {
	invalidStorageSpecs := []struct {
		name        string
		storage     string
		expectedErr string
	}{
		{"empty_storage", "", "ephemeral storage cannot be empty"},
		{"invalid_format", "abc", "invalid ephemeral storage quantity format"},
		{"below_minimum", "50Mi", "ephemeral storage must be at least 100Mi"},
		{"above_maximum", "20Gi", "ephemeral storage cannot exceed 10Gi"},
		{"invalid_unit", "1Gx", "invalid ephemeral storage quantity format"},
		{"invalid_number", "1.2.3Gi", "invalid ephemeral storage quantity format"},
		{"special_chars", "1Gi@", "invalid ephemeral storage quantity format"},
	}

	for _, tt := range invalidStorageSpecs {
		t.Run(tt.name, func(t *testing.T) {
			spec := models.ResourceSpec{
				CPU:              "100m",
				Memory:           "128Mi",
				EphemeralStorage: tt.storage,
			}

			err := spec.Validate()
			require.Error(t, err, "Storage %s should be invalid", tt.storage)
			assert.Contains(t, err.Error(), tt.expectedErr)
		})
	}
}

func TestResourceSpec_ToKubernetesResourceRequirements(t *testing.T) {
	spec := models.ResourceSpec{
		CPU:              "500m",
		Memory:           "1Gi",
		EphemeralStorage: "2Gi",
	}

	requirements := spec.ToKubernetesResourceRequirements()

	// Verify all resources are present
	assert.Len(t, requirements, 3)

	// Verify CPU
	cpu, exists := requirements["cpu"]
	assert.True(t, exists, "CPU should be present")
	assert.Equal(t, "500m", cpu.String())

	// Verify Memory
	memory, exists := requirements["memory"]
	assert.True(t, exists, "Memory should be present")
	assert.Equal(t, "1Gi", memory.String())

	// Verify Ephemeral Storage
	storage, exists := requirements["ephemeral-storage"]
	assert.True(t, exists, "Ephemeral storage should be present")
	assert.Equal(t, "2Gi", storage.String())
}

func TestResourceSpec_ToKubernetesResourceRequirements_InvalidQuantities(t *testing.T) {
	spec := models.ResourceSpec{
		CPU:              "invalid-cpu",
		Memory:           "invalid-memory",
		EphemeralStorage: "invalid-storage",
	}

	requirements := spec.ToKubernetesResourceRequirements()

	// Should return empty map when all quantities are invalid
	assert.Empty(t, requirements)

	// Test with mixed valid/invalid quantities
	spec.CPU = "100m" // Valid
	requirements = spec.ToKubernetesResourceRequirements()

	assert.Len(t, requirements, 1)
	_, exists := requirements["cpu"]
	assert.True(t, exists, "Valid CPU should be included")
}

func TestResourceSpec_SetDefaults(t *testing.T) {
	tests := []struct {
		name     string
		initial  models.ResourceSpec
		expected models.ResourceSpec
	}{
		{
			name:    "all_empty",
			initial: models.ResourceSpec{},
			expected: models.ResourceSpec{
				CPU:              "100m",
				Memory:           "128Mi",
				EphemeralStorage: "1Gi",
			},
		},
		{
			name: "partial_empty",
			initial: models.ResourceSpec{
				CPU:    "200m",
				Memory: "",
			},
			expected: models.ResourceSpec{
				CPU:              "200m",
				Memory:           "128Mi",
				EphemeralStorage: "1Gi",
			},
		},
		{
			name: "none_empty",
			initial: models.ResourceSpec{
				CPU:              "500m",
				Memory:           "1Gi",
				EphemeralStorage: "2Gi",
			},
			expected: models.ResourceSpec{
				CPU:              "500m",
				Memory:           "1Gi",
				EphemeralStorage: "2Gi",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := tt.initial
			spec.SetDefaults()
			assert.Equal(t, tt.expected, spec)
		})
	}
}

func TestResourceSpec_Clone(t *testing.T) {
	original := models.ResourceSpec{
		CPU:              "500m",
		Memory:           "1Gi",
		EphemeralStorage: "2Gi",
	}

	cloned := original.Clone()

	// Verify values are copied
	assert.Equal(t, original.CPU, cloned.CPU)
	assert.Equal(t, original.Memory, cloned.Memory)
	assert.Equal(t, original.EphemeralStorage, cloned.EphemeralStorage)

	// Verify it's a true copy (modifying one doesn't affect the other)
	cloned.CPU = "1000m"
	assert.NotEqual(t, original.CPU, cloned.CPU)
	assert.Equal(t, "500m", original.CPU)
	assert.Equal(t, "1000m", cloned.CPU)
}

func TestResourceSpec_String(t *testing.T) {
	spec := models.ResourceSpec{
		CPU:              "500m",
		Memory:           "1Gi",
		EphemeralStorage: "2Gi",
	}

	str := spec.String()
	assert.Contains(t, str, "500m")
	assert.Contains(t, str, "1Gi")
	assert.Contains(t, str, "2Gi")
	assert.Contains(t, str, "CPU:")
	assert.Contains(t, str, "Memory:")
	assert.Contains(t, str, "Storage:")
}

func TestValidateKubernetesQuantity(t *testing.T) {
	validQuantities := []string{
		"100m",
		"1",
		"1.5",
		"1Gi",
		"500Mi",
		"1000000", // bytes
		"1e6",     // scientific notation
		"0.1",
		"10Gi",
	}

	for _, quantity := range validQuantities {
		t.Run("valid_"+quantity, func(t *testing.T) {
			err := models.ValidateKubernetesQuantity(quantity)
			assert.NoError(t, err, "Quantity %s should be valid", quantity)
		})
	}

	invalidQuantities := []string{
		"abc",
		"1.2.3",
		"100x",
		"",
		"1Gx",
		"@123",
		"123@",
		"1 Gi", // space not allowed
	}

	for _, quantity := range invalidQuantities {
		t.Run("invalid_"+quantity, func(t *testing.T) {
			err := models.ValidateKubernetesQuantity(quantity)
			assert.Error(t, err, "Quantity %s should be invalid", quantity)
		})
	}
}

func TestCompareQuantities(t *testing.T) {
	tests := []struct {
		name     string
		a        string
		b        string
		expected int
		wantErr  bool
	}{
		{"equal_same_format", "100m", "100m", 0, false},
		{"equal_different_format", "1", "1000m", 0, false},
		{"a_greater", "200m", "100m", 1, false},
		{"a_less", "100m", "200m", -1, false},
		{"memory_equal", "1Gi", "1024Mi", 0, false},
		{"memory_a_greater", "2Gi", "1Gi", 1, false},
		{"memory_a_less", "512Mi", "1Gi", -1, false},
		{"invalid_a", "abc", "100m", 0, true},
		{"invalid_b", "100m", "xyz", 0, true},
		{"both_invalid", "abc", "xyz", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := models.CompareQuantities(tt.a, tt.b)

			if tt.wantErr {
				assert.Error(t, err, "Should return error for invalid quantities")
			} else {
				assert.NoError(t, err, "Should not return error for valid quantities")
				assert.Equal(t, tt.expected, result, "Comparison result should match expected")
			}
		})
	}
}

func TestKubernetesQuantityParsing_EdgeCases(t *testing.T) {
	t.Run("zero_values", func(t *testing.T) {
		// Test that zero values are parsed correctly
		quantity, err := resource.ParseQuantity("0")
		require.NoError(t, err)
		assert.True(t, quantity.IsZero())

		// But validation should catch zero as invalid for our use case
		spec := models.ResourceSpec{
			CPU:              "0",
			Memory:           "128Mi",
			EphemeralStorage: "1Gi",
		}
		err = spec.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "CPU must be positive")
	})

	t.Run("very_large_values", func(t *testing.T) {
		// Test parsing very large values
		quantity, err := resource.ParseQuantity("999Gi")
		require.NoError(t, err)
		assert.False(t, quantity.IsZero())

		// But validation should catch values above our limits
		spec := models.ResourceSpec{
			CPU:              "100m",
			Memory:           "999Gi",
			EphemeralStorage: "1Gi",
		}
		err = spec.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "memory cannot exceed 32Gi")
	})

	t.Run("precision_handling", func(t *testing.T) {
		// Test that Kubernetes handles precision correctly
		q1, err := resource.ParseQuantity("0.1")
		require.NoError(t, err)

		q2, err := resource.ParseQuantity("100m")
		require.NoError(t, err)

		// These should be equal
		assert.Equal(t, 0, q1.Cmp(q2))
	})

	t.Run("binary_vs_decimal_units", func(t *testing.T) {
		// Test difference between binary (1024-based) and decimal (1000-based) units
		gib, err := resource.ParseQuantity("1Gi") // 1024^3 bytes
		require.NoError(t, err)

		gb, err := resource.ParseQuantity("1000000000") // 1000^3 bytes
		require.NoError(t, err)

		// 1Gi should be larger than 1GB
		assert.Equal(t, 1, gib.Cmp(gb))
	})

	t.Run("millicpu_conversion", func(t *testing.T) {
		// Test millicpu calculations are correct
		tests := []struct {
			input    string
			expected int64
		}{
			{"1", 1000},
			{"0.5", 500},
			{"100m", 100},
			{"1.5", 1500},
			{"2000m", 2000},
		}

		for _, tt := range tests {
			t.Run(tt.input, func(t *testing.T) {
				quantity, err := resource.ParseQuantity(tt.input)
				require.NoError(t, err)
				assert.Equal(t, tt.expected, quantity.MilliValue())
			})
		}
	})

	t.Run("memory_byte_conversion", func(t *testing.T) {
		// Test memory conversions are correct
		tests := []struct {
			input    string
			expected int64
		}{
			{"1", 1},
			{"1Ki", 1024},
			{"1Mi", 1024 * 1024},
			{"1Gi", 1024 * 1024 * 1024},
			{"1k", 1000},
			{"1M", 1000 * 1000},
			{"1G", 1000 * 1000 * 1000},
		}

		for _, tt := range tests {
			t.Run(tt.input, func(t *testing.T) {
				quantity, err := resource.ParseQuantity(tt.input)
				require.NoError(t, err)
				assert.Equal(t, tt.expected, quantity.Value())
			})
		}
	})
}

func TestResourceSpecValidation_BoundaryValues(t *testing.T) {
	t.Run("cpu_boundaries", func(t *testing.T) {
		// Test exact boundary values for CPU
		tests := []struct {
			cpu   string
			valid bool
		}{
			{"9m", false},     // Below minimum
			{"10m", true},     // Exact minimum
			{"11m", true},     // Above minimum
			{"15999m", true},  // Below maximum (15.999 cores)
			{"16", true},      // Exact maximum
			{"16000m", true},  // Exact maximum in millicores
			{"16001m", false}, // Above maximum
		}

		for _, tt := range tests {
			spec := models.ResourceSpec{
				CPU:              tt.cpu,
				Memory:           "128Mi",
				EphemeralStorage: "1Gi",
			}
			err := spec.Validate()
			if tt.valid {
				assert.NoError(t, err, "CPU %s should be valid", tt.cpu)
			} else {
				assert.Error(t, err, "CPU %s should be invalid", tt.cpu)
			}
		}
	})

	t.Run("memory_boundaries", func(t *testing.T) {
		// Test exact boundary values for memory
		tests := []struct {
			memory string
			valid  bool
		}{
			{"63Mi", false},   // Below minimum
			{"64Mi", true},    // Exact minimum
			{"65Mi", true},    // Above minimum
			{"32Gi", true},    // Exact maximum
			{"33Gi", false},   // Above maximum
			{"32768Mi", true}, // 32Gi in Mi
		}

		for _, tt := range tests {
			spec := models.ResourceSpec{
				CPU:              "100m",
				Memory:           tt.memory,
				EphemeralStorage: "1Gi",
			}
			err := spec.Validate()
			if tt.valid {
				assert.NoError(t, err, "Memory %s should be valid", tt.memory)
			} else {
				assert.Error(t, err, "Memory %s should be invalid", tt.memory)
			}
		}
	})

	t.Run("storage_boundaries", func(t *testing.T) {
		// Test exact boundary values for ephemeral storage
		tests := []struct {
			storage string
			valid   bool
		}{
			{"99Mi", false},   // Below minimum
			{"100Mi", true},   // Exact minimum
			{"101Mi", true},   // Above minimum
			{"10Gi", true},    // Exact maximum
			{"11Gi", false},   // Above maximum
			{"10240Mi", true}, // 10Gi in Mi
		}

		for _, tt := range tests {
			spec := models.ResourceSpec{
				CPU:              "100m",
				Memory:           "128Mi",
				EphemeralStorage: tt.storage,
			}
			err := spec.Validate()
			if tt.valid {
				assert.NoError(t, err, "Storage %s should be valid", tt.storage)
			} else {
				assert.Error(t, err, "Storage %s should be invalid", tt.storage)
			}
		}
	})
}
