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
	jstate.SetAnimation(0, jd.FindAnimation(animation), true)
	bstate.SetAnimation(0, bd.FindAnimation(animation), true)
	const dt = 1.0 / 30
	duration := jd.FindAnimation(animation).Duration
	steps := int(duration/dt) + 5
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
			assertClose(t, ctx+" a", jb.A, bb.A)
			assertClose(t, ctx+" b", jb.B, bb.B)
			assertClose(t, ctx+" c", jb.C, bb.C)
			assertClose(t, ctx+" d", jb.D, bb.D)
			assertClose(t, ctx+" worldX", jb.WorldX, bb.WorldX)
			assertClose(t, ctx+" worldY", jb.WorldY, bb.WorldY)
		}
	}
}

func assertClose(t *testing.T, what string, a, b float32) {
	t.Helper()
	// float32 epsilon comparison with a relative component for large values.
	diff := float64(a - b)
	if diff < 0 {
		diff = -diff
	}
	tol := 0.005 + 0.0005*math.Max(math.Abs(float64(a)), math.Abs(float64(b)))
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
