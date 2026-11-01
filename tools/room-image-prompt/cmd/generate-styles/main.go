// generate-styles derives room-image-prompt style assets from canonical room-art-direction Markdown.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/kani3camp/youtube-study-space/tools/room-image-prompt/internal/stylegen"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	check := flag.Bool("check", false, "生成済みstyleがcanonical Markdownと一致することだけを確認")
	flag.Parse()

	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}
	repoRoot, err := stylegen.FindRepoRoot(wd)
	if err != nil {
		return fmt.Errorf("find repository root: %w", err)
	}
	artifacts, err := stylegen.Generate(repoRoot)
	if err != nil {
		return fmt.Errorf("generate styles: %w", err)
	}

	if *check {
		if err := stylegen.Check(repoRoot, artifacts); err != nil {
			return fmt.Errorf("check generated styles: %w", err)
		}
		fmt.Printf("generated styles are up-to-date (%d)\n", len(artifacts))
		return nil
	}

	if err := stylegen.Write(repoRoot, artifacts); err != nil {
		return fmt.Errorf("write generated styles: %w", err)
	}
	for _, artifact := range artifacts {
		fmt.Printf("%s <- %s\n", artifact.OutputPath, artifact.SourcePath)
	}
	return nil
}
