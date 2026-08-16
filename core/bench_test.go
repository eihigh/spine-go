package spine

import (
	"os"
	"path/filepath"
	"testing"
)

func loadForBench(b *testing.B) (*SkeletonData, *Skeleton, *AnimationState) {
	b.Helper()
	atlasData, err := os.ReadFile(filepath.Join("../testdata", "spineboy-pma.atlas"))
	if err != nil {
		b.Fatal(err)
	}
	atlas, err := NewTextureAtlas(atlasData)
	if err != nil {
		b.Fatal(err)
	}
	jsonData, err := os.ReadFile(filepath.Join("../testdata", "spineboy-pro.json"))
	if err != nil {
		b.Fatal(err)
	}
	sj := NewSkeletonJson(NewAtlasAttachmentLoader(atlas))
	sd, err := sj.ReadSkeletonData(jsonData)
	if err != nil {
		b.Fatal(err)
	}
	skeleton := NewSkeleton(sd)
	state := NewAnimationState(NewAnimationStateData(sd))
	state.SetAnimationByName(0, "walk", true)
	return sd, skeleton, state
}

// BenchmarkUpdateWorldTransform measures the per-frame cost of the pose
// pipeline. The goal is zero allocations per frame after warmup.
func BenchmarkUpdateWorldTransform(b *testing.B) {
	_, skeleton, state := loadForBench(b)
	const dt = 1.0 / 60
	// Warmup so lazily grown buffers reach steady state.
	for i := 0; i < 120; i++ {
		state.Update(dt)
		state.Apply(skeleton)
		skeleton.Update(dt)
		skeleton.UpdateWorldTransform(PhysicsUpdate)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		state.Update(dt)
		state.Apply(skeleton)
		skeleton.Update(dt)
		skeleton.UpdateWorldTransform(PhysicsUpdate)
	}
}

// BenchmarkAnimationStateMix measures crossfading two animations.
func BenchmarkAnimationStateMix(b *testing.B) {
	sd, skeleton, state := loadForBench(b)
	data := state.Data
	data.DefaultMix = 0.25
	_ = sd
	const dt = 1.0 / 60
	names := []string{"walk", "run"}
	for i := 0; i < 120; i++ {
		state.Update(dt)
		state.Apply(skeleton)
		skeleton.UpdateWorldTransform(PhysicsUpdate)
	}
	b.ReportAllocs()
	b.ResetTimer()
	frame := 0
	for i := 0; i < b.N; i++ {
		if frame%30 == 0 {
			state.SetAnimationByName(0, names[(frame/30)%2], true)
		}
		frame++
		state.Update(dt)
		state.Apply(skeleton)
		skeleton.UpdateWorldTransform(PhysicsUpdate)
	}
}
