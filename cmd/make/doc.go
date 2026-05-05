// Copyright 2018-2026 Ryan Clarke (ryclarke-github@rkc.aleeas.com)
//
// Licensed under the Apache License, Version 2.0

/*
Package make provides command handlers for executing one or more make targets
across a selected set of repositories.

The package converts command flags into batched execution steps and runs
command-level safety checks before dispatching work through call.Do.
*/
package make
