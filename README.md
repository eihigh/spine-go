# spine-go

A Go port of the official [Spine Runtimes](https://github.com/EsotericSoftware/spine-runtimes)
(version **4.2**), with a rendering backend for [Ebitengine](https://ebitengine.org/).

- `core/` — renderer-agnostic runtime (package `spine`): skeleton, animation
  state, IK / transform / path / physics constraints, JSON & binary loaders,
  atlas parser, clipping. Ported faithfully from `spine-libgdx` 4.2.
- `ebiten/` — Ebitengine rendering layer (package `spineebiten`): batching
  renderer with premultiplied-alpha handling, blend modes and two-color tint.
- `example/` — runnable examples using the official example skeletons.

Editor/runtime versions must be kept in lockstep: this runtime loads data
exported from **Spine 4.2.x** only.

## Usage

```go
import (
    spine "github.com/eihigh/spine-go/core"
    spineebiten "github.com/eihigh/spine-go/ebiten"
)

atlasData, _ := os.ReadFile("hero.atlas")
atlas, _ := spine.NewTextureAtlas(atlasData)
spineebiten.LoadTextures(atlas, "assetdir") // assigns *ebiten.Image pages

jsonData, _ := os.ReadFile("hero.json")
loader := spine.NewAtlasAttachmentLoader(atlas)
skeletonJson := spine.NewSkeletonJson(loader)
skeletonData, err := skeletonJson.ReadSkeletonData(jsonData)

skeleton := spine.NewSkeleton(skeletonData)
stateData := spine.NewAnimationStateData(skeletonData)
state := spine.NewAnimationState(stateData)
state.SetAnimationByName(0, "walk", true)

// Per frame:
state.Update(delta)
state.Apply(skeleton)
skeleton.Update(delta)
skeleton.UpdateWorldTransform(spine.PhysicsUpdate)
renderer.Draw(screen, skeleton) // spineebiten.Renderer
```

The public API mirrors the official runtimes (`skeleton.FindIkConstraint("aim")`,
`TrackEntry`, `AnimationState` listeners, …), so the
[Spine Runtimes Guide](https://esotericsoftware.com/spine-runtime-architecture)
and examples for other languages translate directly.

## License

This project is a derivative work of the Spine Runtimes and is licensed under
the [Spine Runtimes License Agreement](./LICENSE).

**Important:** integrating the Spine Runtimes (including this library) into
your application requires a valid
[Spine Editor license](https://esotericsoftware.com/spine-editor-license).
Each user of a product that integrates this library must obtain their own
Spine Editor license. Redistribution in any form must include the LICENSE file
and copyright notice. This is not legal advice; read the license text itself.
