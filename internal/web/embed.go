// Package web bundles the single-page frontend into the binary via
// go:embed so azdo-ward ships as one self-contained executable — no
// separate asset directory to deploy.
package web

import "embed"

// Static holds index.html, app.js and styles.css.
//
//go:embed static
var Static embed.FS
