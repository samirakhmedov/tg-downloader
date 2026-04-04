package repository

// IPOTokenChecker provides PO Token server health status.
// Implementations perform periodic TCP reachability checks against the
// bgutil-ytdlp-pot-provider server.
type IPOTokenChecker interface {
	// IsReachable returns true if the last health check found the server reachable.
	// Returns false if the server is unreachable or if PO Token checking is disabled
	// (providerUrl is nil/empty).
	IsReachable() bool

	// ProviderURL returns the configured PO Token server URL.
	// Returns empty string if PO Token checking is disabled.
	ProviderURL() string
}
