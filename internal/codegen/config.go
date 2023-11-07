package codegen

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	PackageName string       `yaml:"package"`
	SpecPath    string       `yaml:"spec"`
	Output      OutputConfig `yaml:"output"`
	ServerName  string       `yaml:"server_name"`

	GenerateDirective string `yaml:"generate_directive"`

	IncludeTags    []string                  `yaml:"include_tags"`
	ExcludeTags    []string                  `yaml:"exclude_tags"`
	TypeMapping    map[string]string         `yaml:"type_mapping"`
	TypeMappingExt map[string]TypeMappingExt `yaml:"type_mapping_ext"`

	OutputOptions OutputOptions `yaml:"output_options"`

	// Deprecated: use Output and OutputOptions instead
	GenerateServer *bool `yaml:"generate_server"`
}

type OutputConfig struct {
	Single string
	Types  string
	Server string
	Client string
	Spec   string
}

type OutputOptions struct {
	SkipFmt        bool  `yaml:"skip_fmt"`
	GenerateServer *bool `yaml:"generate_server"`
	GenerateClient *bool `yaml:"generate_client"`

	// UnwrapEnvelope controls auto-detection and unwrapping of the ginx success
	// envelope {code,msg,data} in response schemas. nil (the default) means ON:
	// when a JSON success response schema is exactly {code:integer,msg:string,
	// data:any}, only the data sub-schema is used to generate the XxxRsp type,
	// preventing a double-wrapped wire body. Set false to keep response schemas
	// verbatim.
	UnwrapEnvelope *bool `yaml:"unwrap_envelope"`
}

type TypeMappingExt struct {
	Type   string `yaml:"type"`
	Import string `yaml:"import"`
}

func (o *OutputConfig) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode {
		o.Single = value.Value
		return nil
	}
	if value.Kind == yaml.MappingNode {
		var m struct {
			Types  string `yaml:"types"`
			Server string `yaml:"server"`
			Client string `yaml:"client"`
			Spec   string `yaml:"spec"`
		}
		if err := value.Decode(&m); err != nil {
			return err
		}
		o.Types = m.Types
		o.Server = m.Server
		o.Client = m.Client
		o.Spec = m.Spec
		return nil
	}
	return fmt.Errorf("output must be a string or a map")
}

func (o OutputConfig) MarshalYAML() (any, error) {
	if o.Single != "" {
		return o.Single, nil
	}
	m := make(map[string]string)
	if o.Types != "" {
		m["types"] = o.Types
	}
	if o.Server != "" {
		m["server"] = o.Server
	}
	if o.Client != "" {
		m["client"] = o.Client
	}
	if o.Spec != "" {
		m["spec"] = o.Spec
	}
	return m, nil
}

func (o *OutputConfig) IsMultiFile() bool {
	return o.Types != "" || o.Server != "" || o.Client != "" || o.Spec != ""
}

func (c *Config) ShouldGenerateServer() bool {
	if c.OutputOptions.GenerateServer != nil {
		return *c.OutputOptions.GenerateServer
	}
	if c.GenerateServer != nil {
		return *c.GenerateServer
	}
	return true
}

func (c *Config) ShouldGenerateClient() bool {
	if c.OutputOptions.GenerateClient != nil {
		return *c.OutputOptions.GenerateClient
	}
	return c.Output.Client != ""
}

// ShouldUnwrapEnvelope reports whether response schemas shaped exactly like the
// ginx success envelope {code,msg,data} should be unwrapped to their data
// sub-schema during code generation. Defaults to true (auto-detect ON).
func (c *Config) ShouldUnwrapEnvelope() bool {
	if c.OutputOptions.UnwrapEnvelope != nil {
		return *c.OutputOptions.UnwrapEnvelope
	}
	return true
}

func (c *Config) GetServerName() string {
	if c.ServerName == "" {
		return ""
	}
	return ToCamelCase(c.ServerName)
}

func (c *Config) GetOutputPath() string {
	if c.Output.Single != "" {
		return c.Output.Single
	}
	return ""
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return &cfg, nil
}

func (c *Config) ShouldIncludeOperation(tags []string) bool {
	if len(c.IncludeTags) == 0 && len(c.ExcludeTags) == 0 {
		return true
	}

	tagSet := make(map[string]bool, len(tags))
	for _, t := range tags {
		tagSet[t] = true
	}

	if len(c.ExcludeTags) > 0 {
		for _, t := range c.ExcludeTags {
			if tagSet[t] {
				return false
			}
		}
	}

	if len(c.IncludeTags) > 0 {
		for _, t := range c.IncludeTags {
			if tagSet[t] {
				return true
			}
		}
		return false
	}

	return true
}
