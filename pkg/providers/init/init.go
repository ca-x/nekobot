// Package init registers all provider adaptors.
// Import this package to ensure all adaptors are registered.
package init

import (
	// Import adaptors to trigger their init() functions
	_ "nekobot/pkg/providers/adaptor/claude"
	_ "nekobot/pkg/providers/adaptor/gemini"
	_ "nekobot/pkg/providers/adaptor/generic"
	_ "nekobot/pkg/providers/adaptor/openai"
)
