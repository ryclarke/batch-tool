// Copyright 2018-2026 Ryan Clarke (ryclarke-github@rkc.aleeas.com)
//
// Licensed under the Apache License, Version 2.0

/*
Package config provides configuration defaults, key constants, context-scoped
viper configuration access, and environment-specific setup for the CLI.

Callers use config.SetViper/config.Viper to pass resolved settings through
context and keep direct viper imports localized to this package.
*/
package config
