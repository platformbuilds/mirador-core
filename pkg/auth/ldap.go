package auth

// NOTE: LDAP authenticator removed.
// This package remains only for historical reasons; external authentication
// systems should provide LDAP or other auth upstream. The LDAP implementation
// used to be here, but it had tight coupling to config and tenant logic. This
// is intentionally a no-op stub so code that references the package will compile
// without bringing back the old behaviors.

// NewLDAPAuthenticator intentionally returns nil - LDAP support has been removed
// from MIRADOR-CORE core.
func NewLDAPAuthenticator() interface{} {
	return nil
}
