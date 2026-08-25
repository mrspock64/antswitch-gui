package main

// Version is the running build's version, in the form "vX.Y.Z". Overridden
// at build time via:
//
//	go build -ldflags "-X main.Version=v1.2.3"
//
// "dev" (the default) means a local, non-release build.
var Version = "dev"
