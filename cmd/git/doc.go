// Copyright 2018-2026 Ryan Clarke (ryclarke-github@rkc.aleeas.com)
//
// Licensed under the Apache License, Version 2.0

/*
Package git provides git-focused subcommands for branch, status, commit, push,
diff, stash, and update operations across repository batches.

Commands in this package build call.Func handlers that execute git CLI commands
within each selected repository while preserving shared output semantics.
*/
package git
