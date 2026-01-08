package main

// These variables are set at build time via -ldflags
var (
	GitRepoURL    = "unknown"
	GitCommitHash = "unknown"
	GitCommitDate = "unknown"
)
