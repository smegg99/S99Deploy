package main

// Config for the little site that hosts the s99deploy binary for curl.
#Config: {
	gin_mode:    "release" | "debug" | "test" | *"release" @go(GinMode)
	listen_addr: string | *"127.0.0.1:9250"                @go(ListenAddr)
	// The CLI binary served at /s99deploy, built by the deploy alongside the
	// site binary. Relative to the working directory (the app checkout).
	bin_path: string | *"./bin/s99deploy" @go(BinPath)
	// Whose X-Forwarded-Proto is believed when building the URLs printed by /
	// and /install.sh. Empty trusts nobody.
	trusted_proxies: [...string] | *["127.0.0.1", "::1"] @go(TrustedProxies,type=[]string)
}
