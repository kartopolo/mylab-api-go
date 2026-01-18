package shared

import "context"

type requestIDKeyType struct{}

var requestIDKey = requestIDKeyType{}

func WithRequestIDInContext(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey, requestID)
}

func RequestIDFromContext(ctx context.Context) string {
	val := ctx.Value(requestIDKey)
	if val == nil {
		return ""
	}
	s, ok := val.(string)
	if !ok {
		return ""
	}
	return s
}
