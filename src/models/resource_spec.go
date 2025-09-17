package models

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/resource"
)

// ResourceSpec defines resource limits for a job (CPU, memory, storage)
type ResourceSpec struct {
	CPU              string `json:"cpu"`
	Memory           string `json:"memory"`
	EphemeralStorage string `json:"ephemeral_storage"`
}

// NewResourceSpec creates a new ResourceSpec with default values
func NewResourceSpec() ResourceSpec {
	return ResourceSpec{
		CPU:              "100m",
		Memory:           "128Mi",
		EphemeralStorage: "1Gi",
	}
}

// Validate checks if the ResourceSpec has valid field values
func (r *ResourceSpec) Validate() error {
	if err := r.validateCPU(); err != nil {
		return fmt.Errorf("invalid CPU specification: %w", err)
	}

	if err := r.validateMemory(); err != nil {
		return fmt.Errorf("invalid memory specification: %w", err)
	}

	if err := r.validateEphemeralStorage(); err != nil {
		return fmt.Errorf("invalid ephemeral storage specification: %w", err)
	}

	return nil
}

func (r *ResourceSpec) validateCPU() error {
	if r.CPU == "" {
		return fmt.Errorf("CPU cannot be empty")
	}

	quantity, err := resource.ParseQuantity(r.CPU)
	if err != nil {
		return fmt.Errorf("invalid CPU format: %w", err)
	}

	// Convert to millicores for comparison
	milliCPU := quantity.MilliValue()

	// Minimum: 10m (10 millicores)
	if milliCPU <= 0 {
		return fmt.Errorf("CPU must be positive")
	}
	if milliCPU < 10 {
		return fmt.Errorf("CPU must be at least 10m, got %s", r.CPU)
	}

	// Maximum: 16 cores (16000 millicores)
	if milliCPU > 16000 {
		return fmt.Errorf("CPU cannot exceed 16 cores, got %s", r.CPU)
	}

	return nil
}

func (r *ResourceSpec) validateMemory() error {
	if r.Memory == "" {
		return fmt.Errorf("memory cannot be empty")
	}

	quantity, err := resource.ParseQuantity(r.Memory)
	if err != nil {
		return fmt.Errorf("invalid memory format: %w", err)
	}

	// Convert to bytes for comparison
	bytes := quantity.Value()

	// Check for zero/negative memory
	if bytes <= 0 {
		return fmt.Errorf("memory must be positive")
	}

	// Minimum: 64Mi
	minMemory := resource.MustParse("64Mi")
	if bytes < minMemory.Value() {
		return fmt.Errorf("memory must be at least 64Mi, got %s", r.Memory)
	}

	// Maximum: 32Gi
	maxMemory := resource.MustParse("32Gi")
	if bytes > maxMemory.Value() {
		return fmt.Errorf("memory cannot exceed 32Gi, got %s", r.Memory)
	}

	return nil
}

func (r *ResourceSpec) validateEphemeralStorage() error {
	if r.EphemeralStorage == "" {
		return fmt.Errorf("ephemeral storage cannot be empty")
	}

	quantity, err := resource.ParseQuantity(r.EphemeralStorage)
	if err != nil {
		return fmt.Errorf("invalid ephemeral storage quantity format: %w", err)
	}

	// Convert to bytes for comparison
	bytes := quantity.Value()

	// Minimum: 100Mi
	minStorage := resource.MustParse("100Mi")
	if bytes < minStorage.Value() {
		return fmt.Errorf("ephemeral storage must be at least 100Mi, got %s", r.EphemeralStorage)
	}

	// Maximum: 10Gi
	maxStorage := resource.MustParse("10Gi")
	if bytes > maxStorage.Value() {
		return fmt.Errorf("ephemeral storage cannot exceed 10Gi, got %s", r.EphemeralStorage)
	}

	return nil
}

// ToKubernetesResourceRequirements converts the ResourceSpec to Kubernetes resource requirements
func (r *ResourceSpec) ToKubernetesResourceRequirements() map[string]resource.Quantity {
	requirements := make(map[string]resource.Quantity)

	if cpu, err := resource.ParseQuantity(r.CPU); err == nil {
		requirements["cpu"] = cpu
	}

	if memory, err := resource.ParseQuantity(r.Memory); err == nil {
		requirements["memory"] = memory
	}

	if storage, err := resource.ParseQuantity(r.EphemeralStorage); err == nil {
		requirements["ephemeral-storage"] = storage
	}

	return requirements
}

// SetDefaults sets default values for all resource fields
func (r *ResourceSpec) SetDefaults() {
	if r.CPU == "" {
		r.CPU = "100m"
	}
	if r.Memory == "" {
		r.Memory = "128Mi"
	}
	if r.EphemeralStorage == "" {
		r.EphemeralStorage = "1Gi"
	}
}

// Clone creates a deep copy of the ResourceSpec
func (r *ResourceSpec) Clone() ResourceSpec {
	return ResourceSpec{
		CPU:              r.CPU,
		Memory:           r.Memory,
		EphemeralStorage: r.EphemeralStorage,
	}
}

func (r *ResourceSpec) String() string {
	return fmt.Sprintf("CPU: %s, Memory: %s, Storage: %s", r.CPU, r.Memory, r.EphemeralStorage)
}

// ValidateKubernetesQuantity validates that a string is a valid Kubernetes resource quantity
func ValidateKubernetesQuantity(quantity string) error {
	_, err := resource.ParseQuantity(quantity)
	return err
}

// CompareQuantities compares two Kubernetes resource quantities
func CompareQuantities(a, b string) (int, error) {
	qA, err := resource.ParseQuantity(a)
	if err != nil {
		return 0, fmt.Errorf("invalid quantity A: %w", err)
	}

	qB, err := resource.ParseQuantity(b)
	if err != nil {
		return 0, fmt.Errorf("invalid quantity B: %w", err)
	}

	return qA.Cmp(qB), nil
}
