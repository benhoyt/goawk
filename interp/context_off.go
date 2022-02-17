// Interpreter.ExecuteContext is disabled by default (to enable, set the
// "goawk_context" build tag).

//go:build !goawk_context
// +build !goawk_context

package interp

type ctxInfo struct{}

// Called from VM execute loop: this will be optimized away unless the
// "goawk_context" build tag is set and ExecuteContext is enabled.
func (p *interp) checkContext() error {
	return nil
}

func (p *interp) resetContext() {}
