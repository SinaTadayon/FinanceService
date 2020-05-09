package utils

type ContextKey string

const (
	CtxUserID        ContextKey = "userId"
	CtxAuthToken     ContextKey = "authorization"
	CtxUserACL       ContextKey = "userAcl"
	CtxRealIp        ContextKey = "real-ip"
	CtxUserAgent     ContextKey = "user-agent"
	CtxForwardedHost ContextKey = "forwarded-host"
	CtxTrackingId    ContextKey = "tracking-id"
)
