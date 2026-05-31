package main

import "testing"

func TestFindAsset(t *testing.T) {
	assets := []ghAsset{
		{Name: "hef-linux-amd64"},
		{Name: "hef-darwin-arm64"},
		{Name: "hef-windows-amd64.exe"},
	}

	got := findAsset(assets)
	if got == nil {
		t.Fatal("findAsset should find an asset for the current platform")
	}

	if got := findAsset(nil); got != nil {
		t.Fatal("findAsset should return nil for empty assets")
	}

	if got := findAsset([]ghAsset{{Name: "hef-plan9-mips"}}); got != nil {
		t.Fatal("findAsset should return nil when no platform asset matches")
	}
}
