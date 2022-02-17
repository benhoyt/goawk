// Interpreter.ExecuteContext is enabled only if the "goawk_context" build tag
// is set.

//go:build goawk_context
// +build goawk_context

package interp

import (
	"context"
)

const checkContextOps = 100 // for efficiency, only check context every few instructions

type ctxInfo struct {
	ctx context.Context
	ops int
}

// ExecuteContext is like Execute, but supports timeout and cancellation using
// a context. For efficiency, the context is only tested for cancellation or
// timeout every few instructions.
func (p *Interpreter) ExecuteContext(ctx context.Context, config *Config) (int, error) {
	p.interp.resetCore()
	p.interp.ctxInfo.ctx = ctx

	err := p.interp.setExecuteConfig(config)
	if err != nil {
		return 0, err
	}

	return p.interp.executeAll()
}

func (p *interp) resetContext() {
	p.ctxInfo.ctx = context.Background()
	p.ctxInfo.ops = 0
}

func (p *interp) checkContext() error {
	p.ctxInfo.ops++
	if p.ctxInfo.ops < checkContextOps {
		return nil
	}
	p.ctxInfo.ops = 0
	select {
	case <-p.ctxInfo.ctx.Done():
		return p.ctxInfo.ctx.Err()
	default:
		return nil
	}
}
