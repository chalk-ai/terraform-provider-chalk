// genchangelog generates the Registry changelog from schema snapshots.
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
	snapshotVersion := flags.String("snapshot", "", "write a snapshot for vMAJOR.MINOR.PATCH")
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
		return writeSnapshot(snapshotsDir, snapshot)
	}

	snapshots, err := loadSnapshots(snapshotsDir)
	if err != nil {
		return err
	}
	if len(snapshots) == 0 {
		return fmt.Errorf("no snapshots found in %s", snapshotsDir)
	}

	live, err := liveSnapshot(ctx, snapshots[len(snapshots)-1].Version)
	if err != nil {
		return err
	}
	rendered := renderChangelog(live, snapshots)

	outputPath := *output
	if !filepath.IsAbs(outputPath) {
		outputPath = filepath.Join(root, outputPath)
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(outputPath, rendered, 0o644)
}
