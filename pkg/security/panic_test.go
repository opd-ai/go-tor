package security

import (
	"fmt"
	"strings"
	"testing"
)

func TestSafePanicMessage_RuntimeError(t *testing.T) {
	// Capture a real runtime.Error via a controlled nil dereference.
	var runtimeErr interface{}
	func() {
		defer func() { runtimeErr = recover() }()
		var p *int
		_ = *p // triggers runtime error: nil pointer dereference
	}()

	msg := SafePanicMessage(runtimeErr)
	if !strings.Contains(msg, "nil pointer dereference") {
		t.Errorf("expected runtime error description, got: %q", msg)
	}
}

func TestSafePanicMessage_ErrorValue(t *testing.T) {
	err := fmt.Errorf("secret_key=abc123")
	msg := SafePanicMessage(err)

	// Value must not appear in the log message.
	if strings.Contains(msg, "secret_key") {
		t.Errorf("sensitive error value leaked into panic message: %q", msg)
	}
	// Type name must appear.
	if !strings.Contains(msg, "type=") {
		t.Errorf("expected type= prefix, got: %q", msg)
	}
}

func TestSafePanicMessage_StringValue(t *testing.T) {
	msg := SafePanicMessage("password=hunter2")

	if strings.Contains(msg, "hunter2") {
		t.Errorf("sensitive string value leaked into panic message: %q", msg)
	}
	if !strings.Contains(msg, "type=") {
		t.Errorf("expected type= prefix, got: %q", msg)
	}
}

func TestSafePanicMessage_IntValue(t *testing.T) {
	msg := SafePanicMessage(42)

	// Integer value should not be logged directly.
	if strings.Contains(msg, "42") {
		t.Errorf("numeric value leaked into panic message: %q", msg)
	}
	if !strings.Contains(msg, "type=") {
		t.Errorf("expected type= prefix, got: %q", msg)
	}
}

func TestSafePanicMessage_NilInterface(t *testing.T) {
	// Passing a nil interface is an edge case; it should not panic the helper.
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("SafePanicMessage panicked on nil input: %v", r)
		}
	}()
	_ = SafePanicMessage(nil)
}
