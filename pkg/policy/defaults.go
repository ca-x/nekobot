package policy

func DefaultPolicy() Policy {
	return Policy{
		Name:        "permissive",
		Description: "Allows all operations by default",
		Network: NetPolicy{
			Mode: "permissive",
		},
		Tools: ToolsPolicy{
			Allow: []string{"*"},
		},
	}
}

func RestrictedPolicy() Policy {
	return Policy{
		Name:        "restricted",
		Description: "Denies all operations unless explicitly allowed",
		Filesystem: FSPolicy{
			DenyWrite: []string{"/etc/*", "/usr/*", "/bin/*", "/sbin/*", "/var/*"},
			DenyRead:  []string{"/etc/shadow", "/etc/passwd"},
		},
		Network: NetPolicy{
			Mode: "none",
		},
		Tools: ToolsPolicy{
			Deny: []string{"exec"},
		},
	}
}

func StandardPolicy() Policy {
	return Policy{
		Name:        "standard",
		Description: "Sensible defaults: deny system writes, allow permissive outbound by default",
		Filesystem: FSPolicy{
			DenyWrite: []string{"/etc/*", "/usr/*", "/bin/*", "/sbin/*"},
			DenyRead:  []string{"/etc/shadow"},
		},
		Network: NetPolicy{
			Mode: "permissive",
		},
		Tools: ToolsPolicy{
			Allow: []string{"*"},
		},
	}
}

func Presets() []Policy {
	return []Policy{
		DefaultPolicy(),
		StandardPolicy(),
		RestrictedPolicy(),
	}
}
