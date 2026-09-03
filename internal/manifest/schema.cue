package manifest

// The per-app deploy definition, read from deploy.json in the app repo root.
#Manifest: {
	// Service name: the systemd unit, the system user, and /opt/<name>.
	name: string & =~"^[a-z][a-z0-9]*(-[a-z0-9]+)*$"

	// Shell commands run in order as the service user in the app dir.
	build: [...string & !=""] | *[] @go(Build,type=[]string)

	// Argv exec'd by systemd through `s99deploy run`. argv[0] with a path
	// separator is resolved relative to the app dir; a bare name through PATH.
	run: [string & !="", ...string & !=""]

	// Extra environment for build and run (e.g. CONFIG_PATH).
	env: {[string]: string} | *{}

	// Vars that must be non-empty in the build environment (usually from .env).
	require_env: [...string & !=""] | *[] @go(RequireEnv,type=[]string)

	// Post-deploy health check on loopback.
	check: #Check
}

#Check: {
	port:            int & >0 & <=65535
	path:            string & =~"^/" | *"/"
	timeout_seconds: int & >0 & <=300 | *10 @go(TimeoutSeconds)
}
