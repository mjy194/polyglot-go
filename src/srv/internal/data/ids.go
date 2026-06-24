package data

import "github.com/google/uuid"

const (
	idPrefixUser         = "user"
	idPrefixRole         = "role"
	idPrefixAPIKey       = "key"
	idPrefixAdminSession = "sess"
	idPrefixProvider     = "prov"
	idPrefixModelMapping = "map"
	idPrefixProxy        = "proxy"
)

func newID(prefix string) string {
	return prefix + "_" + uuid.NewString()
}
