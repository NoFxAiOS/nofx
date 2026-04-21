// Package safe provides panic-recovery wrappers for goroutines.
// A panic in any bare goroutine tears down the entire process.
// Use safe.Go instead of `go func()` in long-running or critical paths.
package safe

import (
	"fmt"
	"nofx/logger"
	"runtime/debug"
)

// Go launches fn in a new goroutine with automatic panic recovery.
// If fn panics, the panic is logged (with stack trace) but the process
// continues running. An optional onPanic callback receives the recovered value.
func Go(fn func(), onPanic ...func(recovered interface{})) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				stack := string(debug.Stack())
				logger.Errorf("🔥 goroutine panic recovered: %v\n%s", r, stack)

				for _, cb := range onPanic {
					func() {
						defer func() {
							if r2 := recover(); r2 != nil {
								logger.Errorf("🔥 onPanic callback itself panicked: %v", r2)
							}
						}()
						cb(r)
					}()
				}
			}
		}()
		fn()
	}()
}

// GoNamed is like Go but tags the log line with a human-readable name.
func GoNamed(name string, fn func(), onPanic ...func(recovered interface{})) {
	Go(func() {
		fn()
	}, append([]func(interface{}){
		func(r interface{}) {
			logger.Errorf("🔥 [%s] goroutine panicked: %v", name, r)
		},
	}, onPanic...)...)
}

// Must converts a panic into an error. Useful inside goroutines where you
// want to handle panics as errors in the caller's recovery flow.
func Must(fn func()) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v\n%s", r, debug.Stack())
		}
	}()
	fn()
	return nil
}
