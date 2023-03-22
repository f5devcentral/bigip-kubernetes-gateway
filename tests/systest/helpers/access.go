package helpers

import "net/http"

// connection test helper for connection test.
type ConnectHelper struct {
	Client *http.Client
}
