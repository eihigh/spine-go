package spine

import (
	"os"
	"testing"
)

// Excerpt of examples/spineboy/export/spineboy-pma.atlas (Spine 4.2).
const spineboyAtlas = `spineboy-pma.png
	size: 1024, 256
	filter: Linear, Linear
	pma: true
	scale: 0.5
crosshair
	bounds: 352, 7, 45, 45
front-shin
	bounds: 665, 128, 41, 92
	rotate: 90
head
	bounds: 2, 27, 136, 149
`

func TestAtlasParse(t *testing.T) {
	atlas, err := NewTextureAtlas([]byte(spineboyAtlas))
	if err != nil {
		t.Fatal(err)
	}
	if len(atlas.Pages) != 1 {
		t.Fatalf("pages = %d, want 1", len(atlas.Pages))
	}
	page := atlas.Pages[0]
	if page.Name != "spineboy-pma.png" {
		t.Errorf("page name = %q", page.Name)
	}
	if page.Width != 1024 || page.Height != 256 {
		t.Errorf("page size = %dx%d, want 1024x256", page.Width, page.Height)
	}
	if !page.PMA {
		t.Errorf("page pma = false, want true")
	}
	if page.MinFilter != TextureFilterLinear || page.MagFilter != TextureFilterLinear {
		t.Errorf("page filter = %v, %v, want Linear, Linear", page.MinFilter, page.MagFilter)
	}
	if page.UWrap != TextureWrapClampToEdge || page.VWrap != TextureWrapClampToEdge {
		t.Errorf("page wrap = %v, %v, want ClampToEdge", page.UWrap, page.VWrap)
	}
	if len(atlas.Regions) != 3 {
		t.Fatalf("regions = %d, want 3", len(atlas.Regions))
	}

	// Non-rotated: bounds: 352, 7, 45, 45.
	r := atlas.FindRegion("crosshair")
	if r == nil {
		t.Fatal("crosshair not found")
	}
	if r.Page != page || r.X != 352 || r.Y != 7 || r.Degrees != 0 {
		t.Errorf("crosshair x,y,degrees = %d,%d,%d", r.X, r.Y, r.Degrees)
	}
	if r.Width != 45 || r.Height != 45 || r.OriginalWidth != 45 || r.OriginalHeight != 45 {
		t.Errorf("crosshair dims = %v,%v orig %v,%v", r.Width, r.Height, r.OriginalWidth, r.OriginalHeight)
	}
	if r.OffsetX != 0 || r.OffsetY != 0 {
		t.Errorf("crosshair offsets = %v,%v", r.OffsetX, r.OffsetY)
	}
	if r.PageWidth != 1024 || r.PageHeight != 256 {
		t.Errorf("crosshair page dims = %v,%v", r.PageWidth, r.PageHeight)
	}
	wantUV(t, "crosshair", &r.TextureRegion, 352.0/1024, 7.0/256, 397.0/1024, 52.0/256)

	// Rotated 90: bounds: 665, 128, 41, 92 — on-page rect is 92x41 pixels;
	// after parsing, Width/Height hold the on-page dims (92, 41) exactly as
	// C# Atlas.cs stores packedWidth/packedHeight, while OriginalWidth/Height
	// keep the unrotated dims (41, 92).
	r = atlas.FindRegion("front-shin")
	if r == nil {
		t.Fatal("front-shin not found")
	}
	if r.Degrees != 90 || !r.Rotate() {
		t.Errorf("front-shin degrees = %d", r.Degrees)
	}
	if r.Width != 92 || r.Height != 41 {
		t.Errorf("front-shin packed dims = %v,%v, want 92,41", r.Width, r.Height)
	}
	if r.OriginalWidth != 41 || r.OriginalHeight != 92 {
		t.Errorf("front-shin orig dims = %v,%v, want 41,92", r.OriginalWidth, r.OriginalHeight)
	}
	wantUV(t, "front-shin", &r.TextureRegion, 665.0/1024, 128.0/256, 757.0/1024, 169.0/256)

	if atlas.FindRegion("nope") != nil {
		t.Error("FindRegion(nope) != nil")
	}
}

func wantUV(t *testing.T, name string, r *TextureRegion, u, v, u2, v2 float32) {
	t.Helper()
	if r.U != u || r.V != v || r.U2 != u2 || r.V2 != v2 {
		t.Errorf("%s UVs = %v,%v,%v,%v, want %v,%v,%v,%v", name, r.U, r.V, r.U2, r.V2, u, v, u2, v2)
	}
}

func TestAtlasParseFile(t *testing.T) {
	const path = "/tmp/claude-0/-home-user-spine-go/de884c2e-6a12-5a6a-8c51-7d1cd185a96b/scratchpad/spine-runtimes/examples/spineboy/export/spineboy-pma.atlas"
	if _, err := os.Stat(path); err != nil {
		t.Skipf("atlas file not available: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	atlas, err := NewTextureAtlas(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(atlas.Pages) != 1 || len(atlas.Regions) != 40 {
		t.Fatalf("pages = %d, regions = %d, want 1, 40", len(atlas.Pages), len(atlas.Regions))
	}
	// muzzle03: bounds: 956, 171, 83, 53, rotate: 90. The on-page rect must
	// stay within the 1024x256 page (956+53=1009, 171+83=254).
	r := atlas.FindRegion("muzzle03")
	if r == nil {
		t.Fatal("muzzle03 not found")
	}
	wantUV(t, "muzzle03", &r.TextureRegion, 956.0/1024, 171.0/256, 1009.0/1024, 254.0/256)
	if r.Width != 53 || r.Height != 83 {
		t.Errorf("muzzle03 packed dims = %v,%v, want 53,83", r.Width, r.Height)
	}
}

func TestAtlasAttachmentLoader(t *testing.T) {
	atlas, err := NewTextureAtlas([]byte(spineboyAtlas))
	if err != nil {
		t.Fatal(err)
	}
	loader := NewAtlasAttachmentLoader(atlas)
	attachment, err := loader.NewRegionAttachment(nil, "gun", "head", nil)
	if err != nil {
		t.Fatal(err)
	}
	if attachment.Region() != &atlas.FindRegion("head").TextureRegion {
		t.Error("attachment region not set from atlas")
	}
	if _, err = loader.NewRegionAttachment(nil, "gun", "missing", nil); err == nil {
		t.Error("expected error for missing region")
	}
}
