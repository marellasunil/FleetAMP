package main

import _ "embed"

// otelPlaneLogoPNG is the approved OTelPlane horizontal product mark supplied
// by the project owner. Embedding keeps packaged binaries self-contained.
//
//go:embed assets/otelplane-logo-transparent.png
var otelPlaneLogoPNG []byte
