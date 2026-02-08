package main

import _ "embed"

//go:embed static/style.css
var defaultCSS []byte

//go:embed static/script.js
var defaultJS []byte

//go:embed templates/page.html
var defaultPageTemplateBytes []byte

//go:embed templates/index.html
var defaultIndexTemplateBytes []byte
