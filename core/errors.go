package core

import "context"

type ErrorReporter interface {
	ReportError(ctx context.Context, err error)
}
