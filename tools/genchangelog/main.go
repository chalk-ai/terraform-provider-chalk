// genchangelog generates the Registry changelog from a current schema snapshot and release diffs.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "genchangelog:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("genchangelog", flag.ContinueOnError)
	providerDir := flags.String("provider-dir", ".", "provider repository root")
	output := flags.String("output", "docs/guides/changelog.md", "changelog output path")
	snapshotVersion := flags.String("snapshot", "", "capture schema changes for vMAJOR.MINOR.PATCH")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected arguments: %v", flags.Args())
	}

	root, err := filepath.Abs(*providerDir)
	if err != nil {
		return err
	}
	snapshotsDir := filepath.Join(root, "tools", "genchangelog", "snapshots")

	if *snapshotVersion != "" {
		snapshot, err := liveSnapshot(ctx, *snapshotVersion)
		if err != nil {
			return err
		}
		return captureRelease(snapshotsDir, snapshot)
	}

	current, err := loadCurrentSnapshot(snapshotsDir)
	if err != nil {
		return err
	}
	releasesDir := filepath.Join(snapshotsDir, "releases")
	releases, err := loadReleases(releasesDir)
	if err != nil {
		return err
	}
	if len(releases) == 0 {
		return fmt.Errorf("no releases found in %s", releasesDir)
	}
	if releases[len(releases)-1].Version != current.Version {
		return fmt.Errorf("current snapshot version %s does not match latest release %s", current.Version, releases[len(releases)-1].Version)
	}

	live, err := liveSnapshot(ctx, current.Version)
	if err != nil {
		return err
	}
	rendered := renderChangelog(live, current, releases)

	outputPath := *output
	if !filepath.IsAbs(outputPath) {
		outputPath = filepath.Join(root, outputPath)
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(outputPath, rendered, 0o644)
}
