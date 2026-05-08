package llm

import "context"

type usageRecorderKey struct{}

type UsageRecorder interface {
	AddUsage(Usage)
}

func WithUsageRecorder(ctx context.Context, recorder UsageRecorder) context.Context {
	if recorder == nil {
		return ctx
	}
	return context.WithValue(ctx, usageRecorderKey{}, recorder)
}

func RecordUsageFromContext(ctx context.Context, usage Usage) {
	recorder, ok := ctx.Value(usageRecorderKey{}).(UsageRecorder)
	if !ok {
		return
	}
	recorder.AddUsage(usage)
}
