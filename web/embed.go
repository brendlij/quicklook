package web

import "embed"

// Files contains the dependency-free web interface.
//
//go:embed index.html style.css app.js
var Files embed.FS
