package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	// Subcommand dispatch: `versioned-docs fetch-aggregate ...` adapts a
	// docs-aggregate corpus into this tool's input layout (see fetch.go).
	// Without a subcommand the binary runs the site generator.
	if len(os.Args) > 1 && os.Args[1] == "fetch-aggregate" {
		if err := runFetchAggregate(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		return
	}

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

	if err := RunContentCommand(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "error running content command: %v\n", err)
		os.Exit(1)
	}


	softwareVersions, err := LoadSoftwareVersions(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading software versions: %v\n", err)
		os.Exit(1)
	}

	documentedVersions, err := DiscoverDocumentedVersions(*contentDir, softwareVersions)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error discovering documented versions: %v\n", err)
		os.Exit(1)
	}

	vmap := ResolveVersionMap(softwareVersions, documentedVersions)

	fmt.Println("Version mapping:")
	for _, sv := range softwareVersions {
		dv := vmap[sv]
		marker := ""
		if sv == dv {
			marker = " (authored)"
		}
		fmt.Printf("  %s -> docs/%s%s\n", sv, dv, marker)
	}

	gen := &Generator{
		Config:             cfg,
		SoftwareVersions:   softwareVersions,
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
