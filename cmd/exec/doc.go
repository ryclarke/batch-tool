// Copyright 2018-2026 Ryan Clarke (ryclarke-github@rkc.aleeas.com)
//
// Licensed under the Apache License, Version 2.0

/*
Package exec provides the command implementation for running shell commands or
executable files across selected repositories.

The package builds Cobra command state, checks mutually exclusive execution
modes, and converts requested actions into call.Func values handled by call.Do.
*/
package exec
