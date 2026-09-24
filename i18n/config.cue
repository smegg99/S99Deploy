// i18n/config.cue

defaultLocale: "en"
locales: ["en", "pl"]

// Two things a person reads: the log, and what the two command lines print.
// Nothing here is served to an HTTP client, so there is no web target.
targets: ["backend"]
roots: ["locales"]

namespaces: {
	logs: {targets: ["backend"]}
	cli: {targets: ["backend"]}
}

scan: ["../internal"]

// go:embed cannot reach outside its own package, so the accessors and the
// bundle land in the package that embeds them.
outputs: {backend: "../internal/messages"}

go: {package: "messages"}
