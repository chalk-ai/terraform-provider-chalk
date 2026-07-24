package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"

	"github.com/chalk-ai/terraform-provider-chalk/tools/internal/providerschema"
)

type historyExpectation struct {
	count int
	hash  string
}

func TestHistoricalSnapshotsRegression(t *testing.T) {
	t.Parallel()
	snapshots, err := loadSnapshots("snapshots")
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshots) < 14 {
		t.Fatalf("historical snapshot count = %d, want at least the 14 releases from v0.9.18 through v1.0.2", len(snapshots))
	}
	if snapshots[0].value.Version != "v0.9.18" {
		t.Fatalf("unexpected snapshot baseline %s", snapshots[0].value.Version)
	}

	// Add each reviewed release transition here. A new snapshot deliberately
	// fails this test with the exact count/hash entry that must be reviewed.
	expected := map[string]historyExpectation{
		"v0.9.18->v0.9.19": {count: 1, hash: "f57dc68fd2260110877f98bbb9bfc872acd0dfb47932117140568f2229a784a7"},
		"v0.9.19->v0.9.20": {count: 1, hash: "c9438a1311831dabfd7bb61945a1da8ebe5f92e72a9986ea36fd8b311d85399a"},
		"v0.9.20->v0.9.21": {count: 1, hash: "3588faa410ecb7a85c122a2ab9d94db9bc93556e2b77d20bc24c83c1b79df158"},
		"v0.9.21->v0.9.22": {count: 1, hash: "5a19e99c98bc905048f98b73b66c2dd6046033e8420de8c3d450cafb1d292f9b"},
		"v0.9.22->v0.9.23": {count: 2, hash: "becd78436dec0c24e1977c9e85b07d2e4e70961f173b59b2bba5dbafa8062003"},
		"v0.9.23->v0.9.24": {count: 16, hash: "22c57810a8e369dfcabda11c399f1b58f78052ebdf3fb12f0fde3c7bed7f4b67"},
		"v0.9.24->v0.9.25": {count: 3, hash: "5ef74c88eb46b41e5391265ff233773f296bf063122ff13dbc854b3e4be01568"},
		"v0.9.25->v0.9.26": {count: 4, hash: "187194e95359b0d2e725f7df94507b9b49fece8d3ecbf12a0e6bb6196e2055c4"},
		"v0.9.26->v0.9.27": {count: 34, hash: "a2c3a9d21e63e373bc1b45c9462f7ad54e65b8d89cc44f6773f802f8fea32222"},
		"v0.9.27->v0.9.28": {count: 1, hash: "19cb630ff105b24ee4dc34911bad0ab549bd0bacadd0a099995fa93759b68961"},
		"v0.9.28->v1.0.0":  {count: 4, hash: "613e723b8eeb6310dff5f723ffd6d6b87dad79271d1d020afc9fdaaeadd6894c"},
		"v1.0.0->v1.0.1":   {count: 6, hash: "7e658c4048fb983a210609a5910bf4be9b8a99268659325a5583092b53f422a4"},
		"v1.0.1->v1.0.2":   {count: 57, hash: "89cde7c5b8c52e2c5e7f34097082a99d54d4d6658536ddf9ff9f79787ac11c8b"},
	}
	for index := 1; index < len(snapshots); index++ {
		from := snapshots[index-1].value
		to := snapshots[index].value
		key := from.Version + "->" + to.Version
		changes := providerschema.Diff(from, to)
		data, err := json.Marshal(changes)
		if err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(data)
		got := historyExpectation{count: len(changes), hash: hex.EncodeToString(digest[:])}
		want, exists := expected[key]
		if !exists {
			t.Errorf("%q: {count: %d, hash: %q},", key, got.count, got.hash)
			continue
		}
		if got != want {
			t.Errorf("%s regression = %#v, want %#v", key, got, want)
		}
	}
}

func TestGeneratedChangelogMatchesHistoricalSnapshots(t *testing.T) {
	t.Parallel()
	snapshots, err := loadSnapshots("snapshots")
	if err != nil {
		t.Fatal(err)
	}
	latest := snapshots[len(snapshots)-1].value
	rendered, err := renderChangelog(latest, snapshots)
	if err != nil {
		t.Fatal(err)
	}
	committed, err := os.ReadFile("../../docs/guides/changelog.md")
	if err != nil {
		t.Fatal(err)
	}
	if string(rendered) != string(committed) {
		t.Fatal("generated changelog does not match the committed historical changelog")
	}
}
