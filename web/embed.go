package web

import "embed"

// TemplatesFS embeds HTML templates into the compiled Go binary.
//
//go:embed templates/*.html templates/partials/*.html
var TemplatesFS embed.FS

// StaticFS embeds optimized static assets (CSS, JS, fonts, images, PDFs) into the compiled Go binary.
//
//go:embed static
var StaticFS embed.FS
