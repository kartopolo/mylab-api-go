package httpapi

import "context"

type authInfoKeyType struct{}

var authInfoKey = authInfoKeyType{}

type AuthInfo struct {
	UserID    int64
	CompanyID int64
	Role      string
}

func withAuthInfoInContext(ctx context.Context, info AuthInfo) context.Context {
	return context.WithValue(ctx, authInfoKey, info)
}

func authInfoFromContext(ctx context.Context) (AuthInfo, bool) {
	val := ctx.Value(authInfoKey)
	if val == nil {
		return AuthInfo{}, false
	}
	info, ok := val.(AuthInfo)
	return info, ok
}
