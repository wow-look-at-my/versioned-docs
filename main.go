package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config.yaml")
	outputDir := flag.String("out", "site", "output directory")
	contentDir := flag.String("content", "content", "content directory with versioned docs")
	templateDir := flag.String("templates", "templates", "template directory")
	baseURL := flag.String("base-url", "", "base URL for generated site (e.g. /somesoftware)")
	flag.Parse()

	cfg, err := LoadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading config: %v\n", err)
		os.Exit(1)
	}

	documentedVersions, err := DiscoverDocumentedVersions(*contentDir, cfg.SoftwareVersions)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error discovering documented versions: %v\n", err)
		os.Exit(1)
	}

	vmap := ResolveVersionMap(cfg.SoftwareVersions, documentedVersions)

	fmt.Println("Version mapping:")
	for _, sv := range cfg.SoftwareVersions {
		dv := vmap[sv]
		marker := ""
		if sv == dv {
			marker = " (authored)"
		}
		fmt.Printf("  %s -> docs/%s%s\n", sv, dv, marker)
	}

	gen := &Generator{
		Config:             cfg,
		VersionMap:         vmap,
		DocumentedVersions: documentedVersions,
		ContentDir:         *contentDir,
		TemplateDir:        *templateDir,
		OutputDir:          *outputDir,
		BaseURL:            *baseURL,
	}

	if err := gen.Generate(); err != nil {
		fmt.Fprintf(os.Stderr, "error generating site: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nSite generated in %s/\n", *outputDir)
}
