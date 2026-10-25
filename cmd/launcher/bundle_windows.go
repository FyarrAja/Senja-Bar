//go:build bundled

package main

import _ "embed"

//go:embed payload.zip
var payload []byte
