package spine

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func loadAtlasT(t *testing.T, name string) *TextureAtlas {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("../testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	atlas, err := NewTextureAtlas(data)
	if err != nil {
		t.Fatal(err)
	}
	return atlas
}

func loadJSONT(t *testing.T, jsonName, atlasName string) *SkeletonData {
	t.Helper()
	atlas := loadAtlasT(t, atlasName)
	data, err := os.ReadFile(filepath.Join("../testdata", jsonName))
	if err != nil {
		t.Fatal(err)
	}
	sj := NewSkeletonJson(NewAtlasAttachmentLoader(atlas))
	sd, err := sj.ReadSkeletonData(data)
	if err != nil {
		t.Fatal(err)
	}
	return sd
}

func loadBinaryT(t *testing.T, skelName, atlasName string) *SkeletonData {
	t.Helper()
	atlas := loadAtlasT(t, atlasName)
	data, err := os.ReadFile(filepath.Join("../testdata", skelName))
	if err != nil {
		t.Fatal(err)
	}
	sb := NewSkeletonBinary(NewAtlasAttachmentLoader(atlas))
	sd, err := sb.ReadSkeletonData(data)
	if err != nil {
		t.Fatal(err)
	}
	return sd
}

var testSets = []struct {
	json, skel, atlas string
}{
	{"spineboy-pro.json", "spineboy-pro.skel", "spineboy-pma.atlas"},
	{"raptor-pro.json", "raptor-pro.skel", "raptor-pma.atlas"},
	{"coin-pro.json", "coin-pro.skel", "coin-pma.atlas"},
}

func TestLoadJSON(t *testing.T) {
	for _, set := range testSets {
		t.Run(set.json, func(t *testing.T) {
			sd := loadJSONT(t, set.json, set.atlas)
			if len(sd.Bones) == 0 {
				t.Fatal("no bones")
			}
			if len(sd.Slots) == 0 {
				t.Fatal("no slots")
			}
			if len(sd.Animations) == 0 {
				t.Fatal("no animations")
			}
			if !strings.HasPrefix(sd.Version, "4.2") {
				t.Fatalf("unexpected version %q", sd.Version)
			}
		})
	}
}

func TestSetupPoseFinite(t *testing.T) {
	for _, set := range testSets {
		t.Run(set.json, func(t *testing.T) {
			sd := loadJSONT(t, set.json, set.atlas)
			skeleton := NewSkeleton(sd)
			skeleton.SetToSetupPose()
			skeleton.UpdateWorldTransform(PhysicsUpdate)
			checkFinite(t, skeleton)
		})
	}
}

// TestAnimationSampling plays every animation of every test skeleton through
// an AnimationState and verifies the resulting world transforms stay finite.
func TestAnimationSampling(t *testing.T) {
	for _, set := range testSets {
		t.Run(set.json, func(t *testing.T) {
			sd := loadJSONT(t, set.json, set.atlas)
			for _, anim := range sd.Animations {
				skeleton := NewSkeleton(sd)
				state := NewAnimationState(NewAnimationStateData(sd))
				state.SetAnimation(0, anim, true)
				const dt = 1.0 / 30
				steps := int(anim.Duration/dt) + 10
				if steps > 400 {
					steps = 400
				}
				for i := 0; i < steps; i++ {
					state.Update(dt)
					state.Apply(skeleton)
					skeleton.Update(dt)
					skeleton.UpdateWorldTransform(PhysicsUpdate)
				}
				checkFinite(t, skeleton)
			}
		})
	}
}

func checkFinite(t *testing.T, skeleton *Skeleton) {
	t.Helper()
	for _, bone := range skeleton.Bones {
		for i, v := range []float32{bone.A, bone.B, bone.C, bone.D, bone.WorldX, bone.WorldY} {
			f := float64(v)
			if math.IsNaN(f) || math.IsInf(f, 0) {
				t.Fatalf("bone %s: non-finite world transform component %d: %v", bone.Data.Name, i, v)
			}
		}
	}
}

// TestJSONBinaryEquivalence verifies the independently implemented JSON and
// binary loaders produce equivalent skeleton data, both structurally and when
// sampled through animations.
func TestJSONBinaryEquivalence(t *testing.T) {
	for _, set := range testSets {
		t.Run(set.skel, func(t *testing.T) {
			jd := loadJSONT(t, set.json, set.atlas)
			bd := loadBinaryT(t, set.skel, set.atlas)

			if len(jd.Bones) != len(bd.Bones) {
				t.Fatalf("bone count: json %d != binary %d", len(jd.Bones), len(bd.Bones))
			}
			for i := range jd.Bones {
				jb, bb := jd.Bones[i], bd.Bones[i]
				if jb.Name != bb.Name || jb.Inherit != bb.Inherit {
					t.Fatalf("bone %d: %q/%v != %q/%v", i, jb.Name, jb.Inherit, bb.Name, bb.Inherit)
				}
				assertClose(t, "bone "+jb.Name+" x", jb.X, bb.X)
				assertClose(t, "bone "+jb.Name+" y", jb.Y, bb.Y)
				assertClose(t, "bone "+jb.Name+" rotation", jb.Rotation, bb.Rotation)
				assertClose(t, "bone "+jb.Name+" scaleX", jb.ScaleX, bb.ScaleX)
				assertClose(t, "bone "+jb.Name+" length", jb.Length, bb.Length)
			}
			if len(jd.Slots) != len(bd.Slots) {
				t.Fatalf("slot count: json %d != binary %d", len(jd.Slots), len(bd.Slots))
			}
			for i := range jd.Slots {
				js, bs := jd.Slots[i], bd.Slots[i]
				if js.Name != bs.Name || js.AttachmentName != bs.AttachmentName || js.BlendMode != bs.BlendMode {
					t.Fatalf("slot %d differs: %+v vs %+v", i, js, bs)
				}
			}
			if len(jd.Animations) != len(bd.Animations) {
				t.Fatalf("animation count: json %d != binary %d", len(jd.Animations), len(bd.Animations))
			}
			for i := range jd.Animations {
				ja, ba := jd.Animations[i], bd.Animations[i]
				if ja.Name != ba.Name {
					t.Fatalf("animation %d: %q != %q", i, ja.Name, ba.Name)
				}
				assertClose(t, "duration of "+ja.Name, ja.Duration, ba.Duration)
			}

			// Compare every timeline's frame data (times and values). This is
			// the strong check: it verifies both loaders decode identical
			// keyframes up to JSON export rounding.
			for i := range jd.Animations {
				ja, ba := jd.Animations[i], bd.Animations[i]
				jt, bt := ja.Timelines(), ba.Timelines()
				if len(jt) != len(bt) {
					t.Fatalf("%s: timeline count %d != %d", ja.Name, len(jt), len(bt))
				}
				for ti := range jt {
					jf, bf := jt[ti].Frames(), bt[ti].Frames()
					if len(jf) != len(bf) {
						t.Fatalf("%s timeline %d (%T): frame count %d != %d", ja.Name, ti, jt[ti], len(jf), len(bf))
					}
					for k := range jf {
						assertClose(t, fmt.Sprintf("%s timeline %d (%T) frame value %d", ja.Name, ti, jt[ti], k), jf[k], bf[k])
					}
				}
			}

			// Sample every animation from both loaders and compare world
			// transforms.
			for i := range jd.Animations {
				compareSampled(t, jd, bd, jd.Animations[i].Name)
			}
		})
	}
}

func compareSampled(t *testing.T, jd, bd *SkeletonData, animation string) {
	t.Helper()
	js := NewSkeleton(jd)
	bs := NewSkeleton(bd)
	jstate := NewAnimationState(NewAnimationStateData(jd))
	bstate := NewAnimationState(NewAnimationStateData(bd))
	// Not looping: the JSON duration is a rounded decimal (e.g. 0.6667 vs the
	// binary's 0.6666667), so at a loop boundary the two skeletons wrap on
	// different steps and briefly sample opposite ends of the animation.
	jstate.SetAnimation(0, jd.FindAnimation(animation), false)
	bstate.SetAnimation(0, bd.FindAnimation(animation), false)
	// dt deliberately does not divide the 1/30s keyframe grid: stepped curves
	// jump at keyframe times, and because JSON stores rounded times a sample
	// landing exactly on a keyframe would see the jump one float-epsilon
	// apart between the two loaders.
	const dt = 0.0161803
	duration := jd.FindAnimation(animation).Duration
	steps := int(duration/dt) - 1
	if steps > 200 {
		steps = 200
	}
	for i := 0; i < steps; i++ {
		jstate.Update(dt)
		jstate.Apply(js)
		js.Update(dt)
		js.UpdateWorldTransform(PhysicsUpdate)

		bstate.Update(dt)
		bstate.Apply(bs)
		bs.Update(dt)
		bs.UpdateWorldTransform(PhysicsUpdate)

		for bi := range js.Bones {
			jb, bb := js.Bones[bi], bs.Bones[bi]
			ctx := fmt.Sprintf("%s step %d bone %s", animation, i, jb.Data.Name)
			// JSON exports store rounded decimals while .skel stores exact
			// IEEE floats, so sampled poses agree only to export precision
			// (amplified along bone chains) — hence the looser tolerance
			// than for structural data.
			assertCloseTol(t, ctx+" a", jb.A, bb.A, sampledTol)
			assertCloseTol(t, ctx+" b", jb.B, bb.B, sampledTol)
			assertCloseTol(t, ctx+" c", jb.C, bb.C, sampledTol)
			assertCloseTol(t, ctx+" d", jb.D, bb.D, sampledTol)
			assertCloseTol(t, ctx+" worldX", jb.WorldX, bb.WorldX, sampledTol)
			assertCloseTol(t, ctx+" worldY", jb.WorldY, bb.WorldY, sampledTol)
		}
	}
}

func assertClose(t *testing.T, what string, a, b float32) {
	t.Helper()
	assertCloseTol(t, what, a, b, func(m float64) float64 { return 0.005 + 0.0005*m })
}

// sampledTol allows for JSON decimal rounding amplified by animation speed:
// keyframe times are exported with 3-4 decimals, so a bone moving ~2500
// units/s (raptor's jump) legitimately deviates by over a unit at a sample
// point. Gross loader bugs produce differences orders of magnitude larger
// (and are caught exactly by the frame-data comparison above).
func sampledTol(m float64) float64 { return 2 + 0.002*m }

func assertCloseTol(t *testing.T, what string, a, b float32, tolFn func(magnitude float64) float64) {
	t.Helper()
	diff := float64(a - b)
	if diff < 0 {
		diff = -diff
	}
	tol := tolFn(math.Max(math.Abs(float64(a)), math.Abs(float64(b))))
	if diff > tol {
		t.Fatalf("%s: %v != %v (diff %v)", what, a, b, diff)
	}
}

// TestSkinAttachments verifies attachments resolve through skins.
func TestSkinAttachments(t *testing.T) {
	sd := loadJSONT(t, "spineboy-pro.json", "spineboy-pma.atlas")
	skeleton := NewSkeleton(sd)
	found := 0
	for _, slot := range skeleton.Slots {
		if slot.Attachment() != nil {
			found++
		}
	}
	if found == 0 {
		t.Fatal("no attachments resolved in setup pose")
	}
}
