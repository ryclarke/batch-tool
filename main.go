// Copyright 2018-2026 Ryan Clarke (ryclarke-github@rkc.aleeas.com)
//
// Licensed under the Apache License, Version 2.0

/*
Package main is the entry point for the batch-tool application.

It simply executes the root command defined in the cmd package,
which handles all command-line parsing and execution logic.
*/
package main

import "github.com/ryclarke/batch-tool/cmd"

func main() {
	cmd.Execute()
}
