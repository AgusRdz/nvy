package main

import "github.com/AgusRdz/nvy/cmd"

var version = "dev"

func main() {
	cmd.SetVersion(version)
	cmd.Execute()
}
