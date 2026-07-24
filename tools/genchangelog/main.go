// genchangelog creates the Terraform Registry provider changelog from
// versioned schema snapshots and the live provider schema.
package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/chalk-ai/terraform-provider-chalk/tools/internal/providerschema"
)

type config struct {
	providerDir     string
	output          string
	snapshotsDir    string
	snapshotVersion string
	checkVersion    string
	terraformSchema string
	checkOutput     bool
}

type versionedSnapshot struct {
	version providerschema.Version
	value   *providerschema.Snapshot
}

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "genchangelog:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	cfg, err := parseFlags(args)
	if err != nil {
		return err
	}
	if cfg.snapshotVersion != "" && cfg.checkVersion != "" {
		return errors.New("--snapshot and --check-snapshot cannot be used together")
	}
	if cfg.terraformSchema != "" && cfg.snapshotVersion == "" {
		return errors.New("--terraform-schema requires --snapshot")
	}

	snapshots, err := loadSnapshots(cfg.snapshotsDir)
	if err != nil {
		return err
	}

	if cfg.snapshotVersion != "" {
		snapshot, err := snapshotForVersion(ctx, cfg)
		if err != nil {
			return err
		}
		if err := writeNewSnapshot(cfg.snapshotsDir, snapshot, snapshots); err != nil {
			return err
		}
		snapshots, err = loadSnapshots(cfg.snapshotsDir)
		if err != nil {
			return err
		}
	}

	if len(snapshots) == 0 {
		return errors.New("no snapshots found")
	}
	latest := snapshots[len(snapshots)-1].value
	live, err := providerschema.LiveSnapshot(ctx, latest.Version)
	if err != nil {
		return fmt.Errorf("extracting live provider schema: %w", err)
	}

	if cfg.checkVersion != "" {
		return checkSnapshot(cfg.checkVersion, live, snapshots)
	}

	rendered, err := renderChangelog(live, snapshots)
	if err != nil {
		return err
	}
	if cfg.checkOutput {
		current, err := os.ReadFile(cfg.output)
		if err != nil {
			return fmt.Errorf("reading generated changelog: %w", err)
		}
		if !bytes.Equal(current, rendered) {
			return fmt.Errorf("%s is stale; run make docs", cfg.output)
		}
		return nil
	}
	return writeFileAtomically(cfg.output, rendered, 0o644)
}

func parseFlags(args []string) (config, error) {
	var cfg config
	flags := flag.NewFlagSet("genchangelog", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	flags.StringVar(&cfg.providerDir, "provider-dir", ".", "provider repository root")
	flags.StringVar(&cfg.output, "output", "docs/guides/changelog.md", "generated changelog path")
	flags.StringVar(&cfg.snapshotsDir, "snapshots-dir", "tools/genchangelog/snapshots", "snapshot directory")
	flags.StringVar(&cfg.snapshotVersion, "snapshot", "", "write a snapshot for vMAJOR.MINOR.PATCH")
	flags.StringVar(&cfg.checkVersion, "check-snapshot", "", "verify a release snapshot matches the live provider")
	flags.StringVar(&cfg.terraformSchema, "terraform-schema", "", "Terraform providers schema JSON used for historical snapshotting")
	flags.BoolVar(&cfg.checkOutput, "check", false, "check generated output without writing it")
	if err := flags.Parse(args); err != nil {
		return config{}, err
	}
	if flags.NArg() != 0 {
		return config{}, fmt.Errorf("unexpected arguments: %s", strings.Join(flags.Args(), " "))
	}

	root, err := filepath.Abs(cfg.providerDir)
	if err != nil {
		return config{}, fmt.Errorf("resolving provider directory: %w", err)
	}
	cfg.providerDir = root
	cfg.output = resolvePath(root, cfg.output)
	cfg.snapshotsDir = resolvePath(root, cfg.snapshotsDir)
	if cfg.terraformSchema != "" {
		cfg.terraformSchema = resolvePath(root, cfg.terraformSchema)
	}
	return cfg, nil
}

func resolvePath(root, path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Join(root, path)
}

func loadSnapshots(directory string) ([]versionedSnapshot, error) {
	paths, err := filepath.Glob(filepath.Join(directory, "v*.json"))
	if err != nil {
		return nil, fmt.Errorf("finding snapshots: %w", err)
	}
	snapshots := make([]versionedSnapshot, 0, len(paths))
	seen := map[string]bool{}
	for _, path := range paths {
		snapshot, err := providerschema.LoadSnapshot(path)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		filenameVersion := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		if filenameVersion != snapshot.Version {
			return nil, fmt.Errorf("%s contains version %s", path, snapshot.Version)
		}
		if seen[snapshot.Version] {
			return nil, fmt.Errorf("duplicate snapshot version %s", snapshot.Version)
		}
		seen[snapshot.Version] = true
		version, err := providerschema.ParseVersion(snapshot.Version)
		if err != nil {
			return nil, err
		}
		snapshots = append(snapshots, versionedSnapshot{version: version, value: snapshot})
	}
	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].version.Compare(snapshots[j].version) < 0
	})
	return snapshots, nil
}

func snapshotForVersion(ctx context.Context, cfg config) (*providerschema.Snapshot, error) {
	if err := providerschema.ValidateReleaseVersion(cfg.snapshotVersion); err != nil {
		return nil, err
	}
	if cfg.terraformSchema == "" {
		return providerschema.LiveSnapshot(ctx, cfg.snapshotVersion)
	}
	data, err := os.ReadFile(cfg.terraformSchema)
	if err != nil {
		return nil, fmt.Errorf("reading Terraform schema: %w", err)
	}
	return providerschema.SnapshotFromTerraformJSON(data, cfg.snapshotVersion)
}

func writeNewSnapshot(directory string, snapshot *providerschema.Snapshot, existing []versionedSnapshot) error {
	if err := snapshot.Validate(); err != nil {
		return err
	}
	targetVersion, err := providerschema.ParseVersion(snapshot.Version)
	if err != nil {
		return err
	}
	target := filepath.Join(directory, snapshot.Version+".json")
	for _, candidate := range existing {
		if candidate.value.Version == snapshot.Version {
			if providerschema.ContractEqual(candidate.value, snapshot) {
				return nil
			}
			return fmt.Errorf("refusing to overwrite conflicting snapshot %s", target)
		}
	}
	if len(existing) > 0 && targetVersion.Compare(existing[len(existing)-1].version) <= 0 {
		return fmt.Errorf("snapshot %s must be newer than latest snapshot %s", snapshot.Version, existing[len(existing)-1].value.Version)
	}
	data, err := providerschema.MarshalSnapshot(snapshot)
	if err != nil {
		return err
	}
	return writeFileAtomically(target, data, 0o644)
}

func checkSnapshot(version string, live *providerschema.Snapshot, snapshots []versionedSnapshot) error {
	if err := providerschema.ValidateReleaseVersion(version); err != nil {
		return err
	}
	var target *providerschema.Snapshot
	for _, snapshot := range snapshots {
		if snapshot.value.Version == version {
			target = snapshot.value
			break
		}
	}
	if target == nil {
		return fmt.Errorf("snapshot %s does not exist", version)
	}
	if snapshots[len(snapshots)-1].value.Version != version {
		return fmt.Errorf("snapshot %s is not the latest snapshot", version)
	}
	if !providerschema.ContractEqual(target, live) {
		return fmt.Errorf("snapshot %s does not match the live provider", version)
	}
	return nil
}

func writeFileAtomically(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}
	temp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*")
	if err != nil {
		return fmt.Errorf("creating temporary output: %w", err)
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return fmt.Errorf("writing temporary output: %w", err)
	}
	if err := temp.Chmod(mode); err != nil {
		temp.Close()
		return fmt.Errorf("setting output permissions: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("closing temporary output: %w", err)
	}
	if err := os.Rename(tempName, path); err != nil {
		return fmt.Errorf("replacing output: %w", err)
	}
	return nil
}
