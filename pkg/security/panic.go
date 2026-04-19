package security

import (
	"fmt"
	"runtime"
)

// SafePanicMessage returns a safe string representation of a recovered panic value.
// It logs only the type information for non-runtime panics to prevent potential
// sensitive state leakage through error logs. For runtime.Error values (nil pointer
// dereference, index out of range, etc.) the standardised error message is safe
// to include because it contains no application-level state.
func SafePanicMessage(r interface{}) string {
	if re, ok := r.(runtime.Error); ok {
		// runtime.Error messages are standardised OS/runtime descriptions and
		// do not contain application secrets or sensitive data.
		return re.Error()
	}
	// For all other panic values (explicit panic("msg"), panic(err), etc.)
	// we only emit the type to avoid leaking the value.
	return fmt.Sprintf("panic recovered (type=%T)", r)
}
