// Copyright 2018-2026 Ryan Clarke (ryclarke-github@rkc.aleeas.com)
//
// Licensed under the Apache License, Version 2.0

/*
Package pr provides pull-request lifecycle commands such as create, edit, get,
and merge across repository batches.

The package maps CLI flags to scm provider operations through shared option
structures, then executes each PR action concurrently per repository.
*/
package pr
