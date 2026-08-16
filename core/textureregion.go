package spine

// TextureRegion describes a rectangular area in a texture page, along with the
// information needed to map whitespace-stripped and rotated atlas regions back
// to their original image space. The runtime core never touches texture
// pixels; the Texture field is an opaque handle for the renderer.
//
// It corresponds to libgdx's TextureAtlas.AtlasRegion as used by the Spine
// runtime.
type TextureRegion struct {
	// Texture is a renderer-specific handle for the texture page this region
	// belongs to (for the ebiten backend, the page's *ebiten.Image is looked
	// up through the AtlasPage).
	Texture any

	// PageWidth and PageHeight are the texture page dimensions in pixels.
	PageWidth, PageHeight float32

	// U, V, U2, V2 are the region's texture coordinates: U,V is the top left
	// corner and U2,V2 the bottom right corner in texture space (V axis
	// pointing down). For a region rotated 90 degrees, the on-page rectangle
	// spans (Height x Width) pixels.
	U, V, U2, V2 float32

	// Width and Height are the packed dimensions of the region in pixels
	// (packedWidth/packedHeight in the reference runtimes). For a region
	// rotated 90 degrees they are swapped relative to the atlas file's size
	// entry, i.e. Width is always the on-page horizontal span.
	Width, Height float32

	// Degrees is the number of degrees the region has been rotated on the
	// page, counter clockwise: 0, 90, 180 or 270.
	Degrees int

	// OffsetX and OffsetY are the number of whitespace pixels stripped from
	// the left and bottom edges of the original image.
	OffsetX, OffsetY float32

	// OriginalWidth and OriginalHeight are the dimensions of the original
	// image, before whitespace was stripped.
	OriginalWidth, OriginalHeight float32
}

// Rotate reports whether the region is rotated 90 degrees on the page.
func (r *TextureRegion) Rotate() bool { return r.Degrees == 90 }

// HasTextureRegion is implemented by attachments that display a texture
// region: RegionAttachment and MeshAttachment.
type HasTextureRegion interface {
	Attachment

	// Region returns the texture region, or nil.
	Region() *TextureRegion
	// SetRegion sets the texture region. UpdateRegion must be called for the
	// change to take effect.
	SetRegion(region *TextureRegion)
	// UpdateRegion updates the attachment's cached UV coordinates from the
	// region. Must be called if the region or its properties changed.
	UpdateRegion()
}
