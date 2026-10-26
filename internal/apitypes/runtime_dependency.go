package apitypes

// RuntimeDependency declares a tool already expected in the execution image.
// It is a requirement, never an installation command.
type RuntimeDependency struct {
	Name       string `json:"name" toml:"name"`
	MinVersion string `json:"min_version,omitempty" toml:"min_version,omitempty"`
	Optional   bool   `json:"optional,omitempty" toml:"optional,omitempty"`
}
