// Command example renders the official spineboy skeleton with Ebitengine.
//
// Controls: press Space to cycle animations, arrow keys to move the aim
// target IK (demonstrating runtime IK control).
package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	spine "github.com/eihigh/spine-go/core"
	spineebiten "github.com/eihigh/spine-go/ebiten"
)

const (
	screenW = 960
	screenH = 640
)

type game struct {
	skeleton *spine.Skeleton
	state    *spine.AnimationState
	renderer *spineebiten.Renderer

	animations []string
	current    int

	world *ebiten.Image
}

func newGame(dir string) (*game, error) {
	atlasBytes, err := os.ReadFile(filepath.Join(dir, "spineboy-pma.atlas"))
	if err != nil {
		return nil, err
	}
	atlas, err := spine.NewTextureAtlas(atlasBytes)
	if err != nil {
		return nil, err
	}
	if err := spineebiten.LoadTextures(atlas, dir); err != nil {
		return nil, err
	}

	jsonBytes, err := os.ReadFile(filepath.Join(dir, "spineboy-pro.json"))
	if err != nil {
		return nil, err
	}
	loader := spine.NewAtlasAttachmentLoader(atlas)
	skeletonJson := spine.NewSkeletonJson(loader)
	skeletonData, err := skeletonJson.ReadSkeletonData(jsonBytes)
	if err != nil {
		return nil, err
	}

	skeleton := spine.NewSkeleton(skeletonData)
	skeleton.SetPosition(screenW/2, screenH-60)

	stateData := spine.NewAnimationStateData(skeletonData)
	stateData.DefaultMix = 0.2
	state := spine.NewAnimationState(stateData)

	g := &game{
		skeleton: skeleton,
		state:    state,
		renderer: spineebiten.NewRenderer(),
	}
	for _, anim := range skeletonData.Animations {
		g.animations = append(g.animations, anim.Name)
	}
	state.SetAnimationByName(0, g.animations[g.current], true)
	return g, nil
}

func (g *game) Update() error {
	if inpututil.IsKeyJustPressed(ebiten.KeySpace) {
		g.current = (g.current + 1) % len(g.animations)
		g.state.SetAnimationByName(0, g.animations[g.current], true)
	}

	const delta = 1.0 / 60

	// Runtime IK control demo: drive the aim constraint target with arrows.
	if aim := g.skeleton.FindBone("crosshair"); aim != nil {
		dx, dy := float32(0), float32(0)
		if ebiten.IsKeyPressed(ebiten.KeyArrowLeft) {
			dx = -4
		}
		if ebiten.IsKeyPressed(ebiten.KeyArrowRight) {
			dx = 4
		}
		if ebiten.IsKeyPressed(ebiten.KeyArrowUp) {
			dy = 4
		}
		if ebiten.IsKeyPressed(ebiten.KeyArrowDown) {
			dy = -4
		}
		aim.X += dx
		aim.Y += dy
	}

	g.state.Update(delta)
	g.state.Apply(g.skeleton)
	g.skeleton.Update(delta)
	g.skeleton.UpdateWorldTransform(spine.PhysicsUpdate)
	return nil
}

func (g *game) Draw(screen *ebiten.Image) {
	// Spine uses a y-up coordinate system; flip the world for rendering.
	if g.world == nil {
		g.world = ebiten.NewImage(screenW, screenH)
	}
	g.world.Clear()
	g.renderer.Draw(g.world, g.skeleton)

	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(1, -1)
	op.GeoM.Translate(0, screenH)
	screen.DrawImage(g.world, op)

	ebitenutil.DebugPrint(screen, fmt.Sprintf(
		"space: next animation (%s)\nTPS: %0.1f  FPS: %0.1f",
		g.animations[g.current], ebiten.ActualTPS(), ebiten.ActualFPS()))
}

func (g *game) Layout(w, h int) (int, int) { return screenW, screenH }

func main() {
	dir := "data"
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}
	g, err := newGame(dir)
	if err != nil {
		log.Fatal(err)
	}
	ebiten.SetWindowSize(screenW, screenH)
	ebiten.SetWindowTitle("spine-go: spineboy")
	if err := ebiten.RunGame(g); err != nil {
		log.Fatal(err)
	}
}
