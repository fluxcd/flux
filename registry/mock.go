package registry

type MockRegistry struct{}

// NewMockRegistry creates a mock registry for use in testing.
func NewMockRegistry() *MockRegistry {
	return &MockRegistry{}
}
