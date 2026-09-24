// internal/site/schema.cue

package site

// Config for the little site that hosts the depl binary for curl.
#Config: {
	gin_mode:    "release" | "debug" | "test" | *"release" @go(GinMode)
	listen_addr: string | *"127.0.0.1:9250"                @go(ListenAddr)
	// The CLI binary served at /depl, relative to the working directory.
	bin_path: string | *"./bin/depl" @go(BinPath)
	// Peers trusted for X-Forwarded-Proto and gin ClientIP: addresses or CIDR blocks; empty trusts nobody.
	trusted_proxies: [...string] | *["127.0.0.1", "::1"] @go(TrustedProxies,type=[]string)
}
