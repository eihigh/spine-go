package spine

// Vertex index constants for RegionAttachment offsets and UVs.
const (
	BLX, BLY = 0, 1
	ULX, ULY = 2, 3
	URX, URY = 4, 5
	BRX, BRY = 6, 7
)

// RegionAttachment is an attachment that displays a textured quadrilateral.
type RegionAttachment struct {
	baseAttachment

	region *TextureRegion

	// Path is the name of the texture region for this attachment.
	Path string

	// X, Y is the local translation, Rotation the local rotation.
	X, Y, Rotation float32

	// ScaleX and ScaleY are the local scale.
	ScaleX, ScaleY float32

	// Width and Height of the region attachment in Spine.
	Width, Height float32

	// UVs holds the computed texture coordinates for the 4 vertices.
	UVs [8]float32

	// Offset holds, for each of the 4 vertices, a pair of x,y values that is
	// the local position of the vertex. See UpdateRegion.
	Offset [8]float32

	// Color to tint the region.
	Color Color

	// Sequence, or nil.
	Sequence *Sequence
}

// NewRegionAttachment creates a region attachment.
func NewRegionAttachment(name string) *RegionAttachment {
	return &RegionAttachment{
		baseAttachment: newBaseAttachment(name),
		ScaleX:         1,
		ScaleY:         1,
		Color:          Color{1, 1, 1, 1},
	}
}

// Region returns the texture region, or nil.
func (a *RegionAttachment) Region() *TextureRegion { return a.region }

// SetRegion sets the texture region. UpdateRegion must be called for the
// change to take effect.
func (a *RegionAttachment) SetRegion(region *TextureRegion) {
	if region == nil {
		panic("spine: region cannot be nil")
	}
	a.region = region
}

// UpdateRegion calculates the Offset and UVs using the region and the
// attachment's transform. Must be called if the region, the region's
// properties, or the transform are changed.
func (a *RegionAttachment) UpdateRegion() {
	region := a.region
	if region == nil {
		a.UVs[BLX] = 0
		a.UVs[BLY] = 0
		a.UVs[ULX] = 0
		a.UVs[ULY] = 1
		a.UVs[URX] = 1
		a.UVs[URY] = 1
		a.UVs[BRX] = 1
		a.UVs[BRY] = 0
		return
	}

	width, height := a.Width, a.Height
	localX2 := width / 2
	localY2 := height / 2
	localX := -localX2
	localY := -localY2
	rotated := false
	if region.OriginalWidth != 0 { // Atlas region with whitespace stripping info.
		localX += region.OffsetX / region.OriginalWidth * width
		localY += region.OffsetY / region.OriginalHeight * height
		if region.Degrees == 90 {
			rotated = true
			localX2 -= (region.OriginalWidth - region.OffsetX - region.Height) / region.OriginalWidth * width
			localY2 -= (region.OriginalHeight - region.OffsetY - region.Width) / region.OriginalHeight * height
		} else {
			localX2 -= (region.OriginalWidth - region.OffsetX - region.Width) / region.OriginalWidth * width
			localY2 -= (region.OriginalHeight - region.OffsetY - region.Height) / region.OriginalHeight * height
		}
	}
	scaleX, scaleY := a.ScaleX, a.ScaleY
	localX *= scaleX
	localY *= scaleY
	localX2 *= scaleX
	localY2 *= scaleY
	r := a.Rotation * degRad
	cosr, sinr := cos(r), sin(r)
	x, y := a.X, a.Y
	localXCos := localX*cosr + x
	localXSin := localX * sinr
	localYCos := localY*cosr + y
	localYSin := localY * sinr
	localX2Cos := localX2*cosr + x
	localX2Sin := localX2 * sinr
	localY2Cos := localY2*cosr + y
	localY2Sin := localY2 * sinr
	offset := &a.Offset
	offset[BLX] = localXCos - localYSin
	offset[BLY] = localYCos + localXSin
	offset[ULX] = localXCos - localY2Sin
	offset[ULY] = localY2Cos + localXSin
	offset[URX] = localX2Cos - localY2Sin
	offset[URY] = localY2Cos + localX2Sin
	offset[BRX] = localX2Cos - localYSin
	offset[BRY] = localYCos + localX2Sin

	uvs := &a.UVs
	if rotated {
		uvs[BLX] = region.U2
		uvs[BLY] = region.V
		uvs[ULX] = region.U2
		uvs[ULY] = region.V2
		uvs[URX] = region.U
		uvs[URY] = region.V2
		uvs[BRX] = region.U
		uvs[BRY] = region.V
	} else {
		uvs[BLX] = region.U2
		uvs[BLY] = region.V2
		uvs[ULX] = region.U
		uvs[ULY] = region.V2
		uvs[URX] = region.U
		uvs[URY] = region.V
		uvs[BRX] = region.U2
		uvs[BRY] = region.V
	}
}

// ComputeWorldVertices transforms the attachment's four vertices to world
// coordinates. If the attachment has a Sequence, the region may be changed.
//
// worldVertices must have a length >= offset + 8. offset is the worldVertices
// index to begin writing values. stride is the number of worldVertices entries
// between the value pairs written.
func (a *RegionAttachment) ComputeWorldVertices(slot *Slot, worldVertices []float32, offset, stride int) {
	if a.Sequence != nil {
		a.Sequence.Apply(slot, a)
	}

	vertexOffset := &a.Offset
	bone := slot.Bone
	x, y := bone.WorldX, bone.WorldY
	wa, wb, wc, wd := bone.A, bone.B, bone.C, bone.D

	offsetX, offsetY := vertexOffset[BRX], vertexOffset[BRY]
	worldVertices[offset] = offsetX*wa + offsetY*wb + x // br
	worldVertices[offset+1] = offsetX*wc + offsetY*wd + y
	offset += stride

	offsetX, offsetY = vertexOffset[BLX], vertexOffset[BLY]
	worldVertices[offset] = offsetX*wa + offsetY*wb + x // bl
	worldVertices[offset+1] = offsetX*wc + offsetY*wd + y
	offset += stride

	offsetX, offsetY = vertexOffset[ULX], vertexOffset[ULY]
	worldVertices[offset] = offsetX*wa + offsetY*wb + x // ul
	worldVertices[offset+1] = offsetX*wc + offsetY*wd + y
	offset += stride

	offsetX, offsetY = vertexOffset[URX], vertexOffset[URY]
	worldVertices[offset] = offsetX*wa + offsetY*wb + x // ur
	worldVertices[offset+1] = offsetX*wc + offsetY*wd + y
}

// Copy implements Attachment.
func (a *RegionAttachment) Copy() Attachment {
	c := NewRegionAttachment(a.name)
	c.region = a.region
	c.Path = a.Path
	c.X = a.X
	c.Y = a.Y
	c.ScaleX = a.ScaleX
	c.ScaleY = a.ScaleY
	c.Rotation = a.Rotation
	c.Width = a.Width
	c.Height = a.Height
	c.UVs = a.UVs
	c.Offset = a.Offset
	c.Color = a.Color
	if a.Sequence != nil {
		c.Sequence = a.Sequence.Copy()
	}
	return c
}
