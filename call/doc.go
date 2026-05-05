// Copyright 2018-2026 Ryan Clarke (ryclarke-github@rkc.aleeas.com)
//
// Licensed under the Apache License, Version 2.0

/*
Package call provides batching primitives for executing repository-scoped work
concurrently with shared cancellation and output semantics.

Command packages typically execute Do(...) with a Func implementation to apply
a unit of work across a batch of repositories; output for each repository is
streamed through the configured output.Channel.

When a workflow needs multiple sequential steps, Wrap(...) combines several
Func values into one atomically-callable implementation.

The Exec helper creates a Func for command execution, including stdout/stderr
streaming and exit-status error handling.
*/
package call
