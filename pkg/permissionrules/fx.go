package permissionrules

import "go.uber.org/fx"

// Module provides database-backed permission rule storage.
var Module = fx.Module("permissionrules",
	fx.Provide(NewManager),
)
