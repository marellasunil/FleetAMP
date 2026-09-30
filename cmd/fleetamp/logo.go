package main

import _ "embed"

// fleetAMPLogoPNG is the approved FleetAMP horizontal product mark supplied
// by the project owner. Embedding keeps packaged binaries self-contained.
//
//go:embed assets/fleetamp-logo.png
var fleetAMPLogoPNG []byte
