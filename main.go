package main

import "github.com/ahlfors/gh-queued-job/cmd"

var version = "dev"

func main() {
	cmd.Execute(version)
}
