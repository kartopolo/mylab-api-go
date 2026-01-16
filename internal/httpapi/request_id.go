package httpapi

import "context"

type requestIDKeyType struct{}

var requestIDKey = requestIDKeyType{}

func withRequestIDInContext(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey, requestID)
}

func requestIDFromContext(ctx context.Context) string {
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
