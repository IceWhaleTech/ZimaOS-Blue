package bootstrap

import "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/formfiller"

// NewRuntimeFormfillerHandler keeps formfiller disabled at runtime.
// Route registration falls back to the existing feature-disabled surface when
// this returns nil.
func NewRuntimeFormfillerHandler() *formfiller.Handler {
	return nil
}
