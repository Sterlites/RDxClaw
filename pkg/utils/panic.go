package utils

import (
	"runtime/debug"

	"github.com/Sterlites/RDxClaw/pkg/logger"
)

// SafeGo runs a function in a new goroutine and recovers from panics.
func SafeGo(component string, fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.ErrorCF(component, "Recovered from panic in goroutine",
					map[string]any{
						"panic": r,
						"stack": string(debug.Stack()),
					})
			}
		}()
		fn()
	}()
}
