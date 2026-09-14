// Copyright 2018-2026 Ryan Clarke (ryclarke-github@rkc.aleeas.com)
//
// Licensed under the Apache License, Version 2.0

/*
Package auth resolves SCM credentials per repository owner through pluggable
backends, allowing separate accounts for each configured project.

Credentials are looked up by host and owner, cached in memory for the lifetime
of the process, and never written to viper or to disk. No configuration field
holds a credential directly: settings name an environment variable, an external
command, or a gh CLI account from which the credential is read.
*/
package auth
