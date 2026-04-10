package policy

// Policy defines what an agent/runtime is allowed to do.
type Policy struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	Filesystem  FSPolicy    `json:"filesystem,omitempty"`
	Network     NetPolicy   `json:"network,omitempty"`
	Tools       ToolsPolicy `json:"tools,omitempty"`
}

type FSPolicy struct {
	AllowRead  []string `json:"allow_read,omitempty"`
	AllowWrite []string `json:"allow_write,omitempty"`
	DenyRead   []string `json:"deny_read,omitempty"`
	DenyWrite  []string `json:"deny_write,omitempty"`
}

type NetPolicy struct {
	Mode     string    `json:"mode,omitempty"` // none, allowlist, permissive
	Outbound []NetRule `json:"outbound,omitempty"`
}

type NetRule struct {
	Host    string   `json:"host"`
	Ports   []int    `json:"ports,omitempty"`
	Methods []string `json:"methods,omitempty"`
	Paths   []string `json:"paths,omitempty"`
}

type ToolsPolicy struct {
	Allow []string `json:"allow,omitempty"`
	Deny  []string `json:"deny,omitempty"`
}
