package spineebiten

import (
	"github.com/hajimehoshi/ebiten/v2"

	spine "github.com/eihigh/spine-go/core"
)

var quadTriangles = []uint16{0, 1, 2, 2, 3, 0}

// twoColorShaderSrc implements Spine's two color (tint black) formula in
// premultiplied alpha space. The light color arrives as the regular vertex
// color (premultiplied), the dark color rgb (premultiplied by the same alpha)
// in Custom0..2:
//
//	out.a   = tex.a * light.a
//	out.rgb = (tex.a - tex.rgb) * dark.rgb + tex.rgb * light.rgb
const twoColorShaderSrc = `//kage:unit pixels
package main

func Fragment(dst vec4, src vec2, color vec4, custom vec4) vec4 {
	tex := imageSrc0At(src)
	var out vec4
	out.a = tex.a * color.a
	out.rgb = (vec3(tex.a) - tex.rgb) * custom.rgb + tex.rgb * color.rgb
	return out
}
`

var (
	// Blend factors assume premultiplied alpha sources (always true with
	// Ebitengine textures). They mirror spine-libgdx BlendMode's PMA factors:
	// setBlendFunctionSeparate(sourcePMA, destColor, sourceAlpha, destColor).
	blendNormal = ebiten.Blend{
		BlendFactorSourceRGB:        ebiten.BlendFactorOne,
		BlendFactorDestinationRGB:   ebiten.BlendFactorOneMinusSourceAlpha,
		BlendFactorSourceAlpha:      ebiten.BlendFactorOne,
		BlendFactorDestinationAlpha: ebiten.BlendFactorOneMinusSourceAlpha,
		BlendOperationRGB:           ebiten.BlendOperationAdd,
		BlendOperationAlpha:         ebiten.BlendOperationAdd,
	}
	blendAdditive = ebiten.Blend{
		BlendFactorSourceRGB:        ebiten.BlendFactorOne,
		BlendFactorDestinationRGB:   ebiten.BlendFactorOne,
		BlendFactorSourceAlpha:      ebiten.BlendFactorOne,
		BlendFactorDestinationAlpha: ebiten.BlendFactorOne,
		BlendOperationRGB:           ebiten.BlendOperationAdd,
		BlendOperationAlpha:         ebiten.BlendOperationAdd,
	}
	blendMultiply = ebiten.Blend{
		BlendFactorSourceRGB:        ebiten.BlendFactorDestinationColor,
		BlendFactorDestinationRGB:   ebiten.BlendFactorOneMinusSourceAlpha,
		BlendFactorSourceAlpha:      ebiten.BlendFactorOneMinusSourceAlpha,
		BlendFactorDestinationAlpha: ebiten.BlendFactorOneMinusSourceAlpha,
		BlendOperationRGB:           ebiten.BlendOperationAdd,
		BlendOperationAlpha:         ebiten.BlendOperationAdd,
	}
	blendScreen = ebiten.Blend{
		BlendFactorSourceRGB:        ebiten.BlendFactorOne,
		BlendFactorDestinationRGB:   ebiten.BlendFactorOneMinusSourceColor,
		BlendFactorSourceAlpha:      ebiten.BlendFactorOneMinusSourceColor,
		BlendFactorDestinationAlpha: ebiten.BlendFactorOneMinusSourceColor,
		BlendOperationRGB:           ebiten.BlendOperationAdd,
		BlendOperationAlpha:         ebiten.BlendOperationAdd,
	}
)

func blendFor(mode spine.BlendMode) ebiten.Blend {
	switch mode {
	case spine.BlendModeAdditive:
		return blendAdditive
	case spine.BlendModeMultiply:
		return blendMultiply
	case spine.BlendModeScreen:
		return blendScreen
	default:
		return blendNormal
	}
}

// Renderer draws skeletons to an *ebiten.Image. It accumulates vertices of
// consecutive slots that share a texture, blend mode and tint mode, and
// flushes them with a single DrawTriangles call, so a one-page atlas typically
// renders in one draw call.
//
// A Renderer may be reused for any number of skeletons; it holds only
// per-frame scratch buffers. It is not safe for concurrent use.
type Renderer struct {
	clipper *spine.SkeletonClipping

	worldVertices []float32

	// Batch state.
	vertices  []ebiten.Vertex
	indices   []uint16
	texture   *ebiten.Image
	blend     ebiten.Blend
	dark      bool
	hasBatch  bool
	shader    *ebiten.Shader
	shaderErr error
}

// NewRenderer creates a renderer.
func NewRenderer() *Renderer {
	r := &Renderer{clipper: spine.NewSkeletonClipping()}
	r.shader, r.shaderErr = ebiten.NewShader([]byte(twoColorShaderSrc))
	return r
}

// Clipper returns the SkeletonClipping used by this renderer, for use with
// e.g. Skeleton.Bounds.
func (r *Renderer) Clipper() *spine.SkeletonClipping { return r.clipper }

// Draw renders the skeleton's current pose to target.
func (r *Renderer) Draw(target *ebiten.Image, skeleton *spine.Skeleton) {
	clipper := r.clipper
	skeletonColor := skeleton.Color
	sr, sg, sb, sa := skeletonColor.R, skeletonColor.G, skeletonColor.B, skeletonColor.A

	for _, slot := range skeleton.DrawOrder {
		if !slot.Bone.IsActive() {
			clipper.ClipEndSlot(slot)
			continue
		}

		var texture *ebiten.Image
		var worldVertices, uvs []float32
		var triangles []uint16
		var attachmentColor spine.Color
		verticesCount := 0

		switch attachment := slot.Attachment().(type) {
		case *spine.RegionAttachment:
			worldVertices = ensure(&r.worldVertices, 8)
			attachment.ComputeWorldVertices(slot, worldVertices, 0, 2)
			verticesCount = 4
			triangles = quadTriangles
			uvs = attachment.UVs[:]
			attachmentColor = attachment.Color
			texture, _ = regionTexture(attachment.Region())
		case *spine.MeshAttachment:
			count := attachment.WorldVerticesLength
			worldVertices = ensure(&r.worldVertices, count)
			attachment.ComputeWorldVertices(slot, 0, count, worldVertices, 0, 2)
			verticesCount = count >> 1
			triangles = attachment.Triangles
			uvs = attachment.UVs
			attachmentColor = attachment.Color
			texture, _ = regionTexture(attachment.Region())
		case *spine.ClippingAttachment:
			clipper.ClipStart(slot, attachment)
			continue
		default:
			clipper.ClipEndSlot(slot)
			continue
		}

		if texture != nil {
			lightColor := slot.Color
			alpha := sa * lightColor.A * attachmentColor.A
			multiplier := alpha // Colors are premultiplied.

			// With premultiplied colors, additive blending is normal
			// blending with a vertex alpha of zero (matches spine-libgdx).
			blendMode := slot.Data.BlendMode
			vertexAlpha := alpha
			if blendMode == spine.BlendModeAdditive {
				blendMode = spine.BlendModeNormal
				vertexAlpha = 0
			}
			r.flushIfNeeded(target, texture, blendFor(blendMode), slot.DarkColor != nil)

			light := premulColor(sr, sg, sb, vertexAlpha, lightColor, attachmentColor, multiplier)
			dark := darkColor(sr, sg, sb, attachmentColor, slot.DarkColor, multiplier)

			if clipper.IsClipping() {
				clipper.ClipTrianglesUV(worldVertices, triangles, len(triangles), uvs)
				cv := clipper.ClippedVertices()
				cuv := clipper.ClippedUVs()
				ct := clipper.ClippedTriangles()
				r.appendSlot(cv, cuv, ct, len(cv)>>1, light, dark)
			} else {
				r.appendSlot(worldVertices, uvs, triangles, verticesCount, light, dark)
			}
		}

		clipper.ClipEndSlot(slot)
	}
	clipper.ClipEnd()
	r.Flush(target)
}

type rgba struct{ r, g, b, a float32 }

// premulColor computes the premultiplied light color.
func premulColor(sr, sg, sb, alpha float32, slotColor, attachmentColor spine.Color, mul float32) rgba {
	return rgba{
		r: sr * slotColor.R * attachmentColor.R * mul,
		g: sg * slotColor.G * attachmentColor.G * mul,
		b: sb * slotColor.B * attachmentColor.B * mul,
		a: alpha,
	}
}

// darkColor computes the premultiplied dark color, or zero if none.
func darkColor(sr, sg, sb float32, attachmentColor spine.Color, dark *spine.Color, mul float32) rgba {
	if dark == nil {
		return rgba{}
	}
	return rgba{
		r: sr * attachmentColor.R * dark.R * mul,
		g: sg * attachmentColor.G * dark.G * mul,
		b: sb * attachmentColor.B * dark.B * mul,
	}
}

func regionTexture(region *spine.TextureRegion) (*ebiten.Image, bool) {
	if region == nil {
		return nil, false
	}
	img, ok := region.Texture.(*ebiten.Image)
	return img, ok
}

func (r *Renderer) flushIfNeeded(target *ebiten.Image, texture *ebiten.Image, blend ebiten.Blend, dark bool) {
	if r.hasBatch && texture == r.texture && blend == r.blend && dark == r.dark {
		return
	}
	r.Flush(target)
	r.texture = texture
	r.blend = blend
	r.dark = dark
	r.hasBatch = true
}

func (r *Renderer) appendSlot(positions, uvs []float32, triangles []uint16, verticesCount int, light, dark rgba) {
	base := uint16(len(r.vertices))
	tw, th := 1, 1
	if r.texture != nil {
		tw, th = r.texture.Bounds().Dx(), r.texture.Bounds().Dy()
	}
	fw, fh := float32(tw), float32(th)
	for i := 0; i < verticesCount; i++ {
		v := ebiten.Vertex{
			DstX:   positions[i*2],
			DstY:   positions[i*2+1],
			SrcX:   uvs[i*2] * fw,
			SrcY:   uvs[i*2+1] * fh,
			ColorR: light.r,
			ColorG: light.g,
			ColorB: light.b,
			ColorA: light.a,
		}
		if r.dark {
			v.Custom0 = dark.r
			v.Custom1 = dark.g
			v.Custom2 = dark.b
			v.Custom3 = 0
		}
		r.vertices = append(r.vertices, v)
	}
	for _, t := range triangles {
		r.indices = append(r.indices, base+t)
	}
}

// Flush draws any batched vertices. Called automatically by Draw; call it
// manually only if you interleave your own drawing with batching.
func (r *Renderer) Flush(target *ebiten.Image) {
	if len(r.indices) == 0 || r.texture == nil {
		r.vertices = r.vertices[:0]
		r.indices = r.indices[:0]
		return
	}
	if r.dark && r.shader != nil {
		op := &ebiten.DrawTrianglesShaderOptions{}
		op.Blend = r.blend
		op.Images[0] = r.texture
		target.DrawTrianglesShader(r.vertices, r.indices, r.shader, op)
	} else {
		op := &ebiten.DrawTrianglesOptions{}
		op.Blend = r.blend
		op.ColorScaleMode = ebiten.ColorScaleModePremultipliedAlpha
		target.DrawTriangles(r.vertices, r.indices, r.texture, op)
	}
	r.vertices = r.vertices[:0]
	r.indices = r.indices[:0]
}

func ensure(buf *[]float32, n int) []float32 {
	if cap(*buf) < n {
		*buf = make([]float32, n)
	}
	*buf = (*buf)[:n]
	return *buf
}
