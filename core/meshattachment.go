package spine

// MeshAttachment is an attachment that displays a textured mesh. A mesh has
// hull vertices and internal vertices within the hull. Holes are not
// supported. Each vertex has UVs (texture coordinates) and triangles are used
// to map an image on to the mesh.
type MeshAttachment struct {
	VertexAttachment

	region *TextureRegion

	// Path is the name of the texture region for this attachment.
	Path string

	// RegionUVs is the UV pair for each vertex, normalized within the texture
	// region.
	RegionUVs []float32

	// UVs is the UV pair for each vertex, normalized within the entire
	// texture. See UpdateRegion.
	UVs []float32

	// Triangles are triplets of vertex indices which describe the mesh's
	// triangulation.
	Triangles []uint16

	// Color to tint the mesh.
	Color Color

	// HullLength is the number of entries at the beginning of Vertices that
	// make up the mesh hull.
	HullLength int

	parentMesh *MeshAttachment

	// Sequence, or nil.
	Sequence *Sequence

	// Nonessential.

	// Edges are vertex index pairs describing edges for controlling
	// triangulation, or nil if nonessential data was not exported. Mesh
	// triangles will never cross edges. Triangulation is not performed at
	// runtime.
	Edges []uint16

	// Width and Height of the mesh's image, or zero if nonessential data was
	// not exported.
	Width, Height float32
}

// NewMeshAttachment creates a mesh attachment.
func NewMeshAttachment(name string) *MeshAttachment {
	return &MeshAttachment{
		VertexAttachment: newVertexAttachment(name),
		Color:            Color{1, 1, 1, 1},
	}
}

// Region returns the texture region, or nil.
func (a *MeshAttachment) Region() *TextureRegion { return a.region }

// SetRegion sets the texture region. UpdateRegion must be called for the
// change to take effect.
func (a *MeshAttachment) SetRegion(region *TextureRegion) {
	if region == nil {
		panic("spine: region cannot be nil")
	}
	a.region = region
}

// UpdateRegion calculates UVs using the RegionUVs and region. Must be called
// if the region, the region's properties, or the RegionUVs are changed.
func (a *MeshAttachment) UpdateRegion() {
	regionUVs := a.RegionUVs
	if a.UVs == nil || len(a.UVs) != len(regionUVs) {
		a.UVs = make([]float32, len(regionUVs))
	}
	uvs := a.UVs
	n := len(uvs)
	var u, v, width, height float32
	region := a.region
	if region != nil && region.PageWidth > 0 && region.OriginalWidth != 0 {
		u = region.U
		v = region.V
		textureWidth, textureHeight := region.PageWidth, region.PageHeight
		switch region.Degrees {
		case 90:
			u -= (region.OriginalHeight - region.OffsetY - region.Width) / textureWidth
			v -= (region.OriginalWidth - region.OffsetX - region.Height) / textureHeight
			width = region.OriginalHeight / textureWidth
			height = region.OriginalWidth / textureHeight
			for i := 0; i < n; i += 2 {
				uvs[i] = u + regionUVs[i+1]*width
				uvs[i+1] = v + (1-regionUVs[i])*height
			}
			return
		case 180:
			u -= (region.OriginalWidth - region.OffsetX - region.Width) / textureWidth
			v -= region.OffsetY / textureHeight
			width = region.OriginalWidth / textureWidth
			height = region.OriginalHeight / textureHeight
			for i := 0; i < n; i += 2 {
				uvs[i] = u + (1-regionUVs[i])*width
				uvs[i+1] = v + (1-regionUVs[i+1])*height
			}
			return
		case 270:
			u -= region.OffsetY / textureWidth
			v -= region.OffsetX / textureHeight
			width = region.OriginalHeight / textureWidth
			height = region.OriginalWidth / textureHeight
			for i := 0; i < n; i += 2 {
				uvs[i] = u + (1-regionUVs[i+1])*width
				uvs[i+1] = v + regionUVs[i]*height
			}
			return
		}
		u -= region.OffsetX / textureWidth
		v -= (region.OriginalHeight - region.OffsetY - region.Height) / textureHeight
		width = region.OriginalWidth / textureWidth
		height = region.OriginalHeight / textureHeight
	} else if region == nil {
		u, v = 0, 0
		width, height = 1, 1
	} else {
		u = region.U
		v = region.V
		width = region.U2 - u
		height = region.V2 - v
	}
	for i := 0; i < n; i += 2 {
		uvs[i] = u + regionUVs[i]*width
		uvs[i+1] = v + regionUVs[i+1]*height
	}
}

// ComputeWorldVertices transforms the attachment's local vertices to world
// coordinates. If the attachment has a Sequence, the region may be changed.
func (a *MeshAttachment) ComputeWorldVertices(slot *Slot, start, count int, worldVertices []float32, offset, stride int) {
	if a.Sequence != nil {
		a.Sequence.Apply(slot, a)
	}
	a.VertexAttachment.ComputeWorldVertices(slot, start, count, worldVertices, offset, stride)
}

// ParentMesh returns the parent mesh if this is a linked mesh, else nil. A
// linked mesh shares the Bones, Vertices, RegionUVs, Triangles, HullLength,
// Edges, Width, and Height with the parent mesh, but may have a different
// Name or Path (and therefore a different texture).
func (a *MeshAttachment) ParentMesh() *MeshAttachment { return a.parentMesh }

// SetParentMesh sets the parent mesh and copies its shared data.
func (a *MeshAttachment) SetParentMesh(parentMesh *MeshAttachment) {
	a.parentMesh = parentMesh
	if parentMesh != nil {
		a.Bones = parentMesh.Bones
		a.Vertices = parentMesh.Vertices
		a.RegionUVs = parentMesh.RegionUVs
		a.Triangles = parentMesh.Triangles
		a.HullLength = parentMesh.HullLength
		a.WorldVerticesLength = parentMesh.WorldVerticesLength
		a.Edges = parentMesh.Edges
		a.Width = parentMesh.Width
		a.Height = parentMesh.Height
	}
}

// NewLinkedMesh returns a new mesh with the parent mesh set to this mesh's
// parent mesh, if any, else to this mesh.
func (a *MeshAttachment) NewLinkedMesh() *MeshAttachment {
	mesh := NewMeshAttachment(a.name)
	mesh.TimelineAttachment = a.TimelineAttachment
	mesh.region = a.region
	mesh.Path = a.Path
	mesh.Color = a.Color
	if a.parentMesh != nil {
		mesh.SetParentMesh(a.parentMesh)
	} else {
		mesh.SetParentMesh(a)
	}
	if mesh.region != nil {
		mesh.UpdateRegion()
	}
	return mesh
}

// Copy implements Attachment. Linked meshes are copied by creating a new
// linked mesh.
func (a *MeshAttachment) Copy() Attachment {
	if a.parentMesh != nil {
		return a.NewLinkedMesh()
	}
	c := NewMeshAttachment(a.name)
	a.copyTo(&c.VertexAttachment)
	c.region = a.region
	c.Path = a.Path
	c.Color = a.Color

	c.RegionUVs = make([]float32, len(a.RegionUVs))
	copy(c.RegionUVs, a.RegionUVs)

	c.UVs = make([]float32, len(a.UVs))
	copy(c.UVs, a.UVs)

	c.Triangles = make([]uint16, len(a.Triangles))
	copy(c.Triangles, a.Triangles)

	c.HullLength = a.HullLength
	if a.Sequence != nil {
		c.Sequence = a.Sequence.Copy()
	}

	if a.Edges != nil {
		c.Edges = make([]uint16, len(a.Edges))
		copy(c.Edges, a.Edges)
	}
	c.Width = a.Width
	c.Height = a.Height
	return c
}
