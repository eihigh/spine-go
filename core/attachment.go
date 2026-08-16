package spine

import "sync/atomic"

// Attachment is the interface for all attachments.
type Attachment interface {
	// Name returns the attachment's name.
	Name() string
	// Copy returns a copy of the attachment.
	Copy() Attachment
}

type baseAttachment struct {
	name string
}

func newBaseAttachment(name string) baseAttachment {
	if name == "" {
		panic("spine: name cannot be empty")
	}
	return baseAttachment{name: name}
}

func (a *baseAttachment) Name() string { return a.name }

var nextAttachmentID int64

func nextID() int {
	return int(atomic.AddInt64(&nextAttachmentID, 1) - 1)
}

// VertexAttachment is the base for an attachment with vertices that are
// transformed by one or more bones and can be deformed by a slot's Deform.
type VertexAttachment struct {
	baseAttachment

	id int

	// TimelineAttachment: timelines for the timeline attachment are also
	// applied to this attachment. May be nil if no attachment-specific
	// timelines should be applied.
	TimelineAttachment Attachment

	// Bones are the bones which affect the Vertices. The array entries are,
	// for each vertex, the number of bones affecting the vertex followed by
	// that many bone indices, which is the index of the bone in
	// Skeleton.Bones. Will be nil if this attachment has no weights.
	Bones []int32

	// Vertices are the vertex positions in the bone's coordinate system. For
	// a non-weighted attachment, the values are x,y entries for each vertex.
	// For a weighted attachment, the values are x,y,weight entries for each
	// bone affecting each vertex.
	Vertices []float32

	// WorldVerticesLength is the maximum number of world vertex values that
	// can be output by ComputeWorldVertices using the count parameter.
	WorldVerticesLength int
}

func newVertexAttachment(name string) VertexAttachment {
	v := VertexAttachment{baseAttachment: newBaseAttachment(name), id: nextID()}
	return v
}

// ID returns a unique ID for this attachment.
func (v *VertexAttachment) ID() int { return v.id }

func (v *VertexAttachment) vertexAttachment() *VertexAttachment { return v }

// hasVertexAttachment is implemented by all attachments embedding VertexAttachment.
type hasVertexAttachment interface {
	vertexAttachment() *VertexAttachment
}

// AsVertexAttachment returns the VertexAttachment embedded in a, or nil if a
// is not a vertex attachment.
func AsVertexAttachment(a Attachment) *VertexAttachment {
	if va, ok := a.(hasVertexAttachment); ok {
		return va.vertexAttachment()
	}
	return nil
}

func (v *VertexAttachment) copyTo(to *VertexAttachment) {
	to.TimelineAttachment = v.TimelineAttachment
	if v.Bones != nil {
		to.Bones = make([]int32, len(v.Bones))
		copy(to.Bones, v.Bones)
	} else {
		to.Bones = nil
	}
	if v.Vertices != nil {
		to.Vertices = make([]float32, len(v.Vertices))
		copy(to.Vertices, v.Vertices)
	} else {
		to.Vertices = nil
	}
	to.WorldVerticesLength = v.WorldVerticesLength
}

// ComputeWorldVertices transforms the attachment's local Vertices to world
// coordinates. If the slot's Deform is not empty, it is used to deform the
// vertices.
//
// start is the index of the first Vertices value to transform. Each vertex has
// 2 values, x and y. count is the number of world vertex values to output;
// must be <= WorldVerticesLength - start. worldVertices is the output world
// vertices; must have a length >= offset + count*stride/2. offset is the
// worldVertices index to begin writing values. stride is the number of
// worldVertices entries between the value pairs written.
func (v *VertexAttachment) ComputeWorldVertices(slot *Slot, start, count int, worldVertices []float32, offset, stride int) {
	count = offset + (count>>1)*stride
	deformArray := slot.Deform
	vertices := v.Vertices
	bones := v.Bones
	if bones == nil {
		if len(deformArray) > 0 {
			vertices = deformArray
		}
		bone := slot.Bone
		x, y := bone.WorldX, bone.WorldY
		a, b, c, d := bone.A, bone.B, bone.C, bone.D
		for vi, w := start, offset; w < count; vi, w = vi+2, w+stride {
			vx, vy := vertices[vi], vertices[vi+1]
			worldVertices[w] = vx*a + vy*b + x
			worldVertices[w+1] = vx*c + vy*d + y
		}
		return
	}
	vi, skip := 0, 0
	for i := 0; i < start; i += 2 {
		n := int(bones[vi])
		vi += n + 1
		skip += n
	}
	skeletonBones := slot.Bone.Skeleton.Bones
	if len(deformArray) == 0 {
		for w, b := offset, skip*3; w < count; w += stride {
			var wx, wy float32
			n := int(bones[vi])
			vi++
			n += vi
			for ; vi < n; vi, b = vi+1, b+3 {
				bone := skeletonBones[bones[vi]]
				vx, vy, weight := vertices[b], vertices[b+1], vertices[b+2]
				wx += (vx*bone.A + vy*bone.B + bone.WorldX) * weight
				wy += (vx*bone.C + vy*bone.D + bone.WorldY) * weight
			}
			worldVertices[w] = wx
			worldVertices[w+1] = wy
		}
	} else {
		deform := deformArray
		for w, b, f := offset, skip*3, skip<<1; w < count; w += stride {
			var wx, wy float32
			n := int(bones[vi])
			vi++
			n += vi
			for ; vi < n; vi, b, f = vi+1, b+3, f+2 {
				bone := skeletonBones[bones[vi]]
				vx, vy, weight := vertices[b]+deform[f], vertices[b+1]+deform[f+1], vertices[b+2]
				wx += (vx*bone.A + vy*bone.B + bone.WorldX) * weight
				wy += (vx*bone.C + vy*bone.D + bone.WorldY) * weight
			}
			worldVertices[w] = wx
			worldVertices[w+1] = wy
		}
	}
}

// BoundingBoxAttachment is an attachment with vertices that make up a polygon.
// Can be used for hit detection, creating physics bodies, spawning particle
// effects, and more.
type BoundingBoxAttachment struct {
	VertexAttachment

	// Nonessential.
	Color Color
}

// NewBoundingBoxAttachment creates a bounding box attachment.
func NewBoundingBoxAttachment(name string) *BoundingBoxAttachment {
	return &BoundingBoxAttachment{
		VertexAttachment: newVertexAttachment(name),
		Color:            Color{0.38, 0.94, 0, 1}, // 60f000ff
	}
}

// Copy implements Attachment.
func (a *BoundingBoxAttachment) Copy() Attachment {
	c := NewBoundingBoxAttachment(a.name)
	a.copyTo(&c.VertexAttachment)
	c.Color = a.Color
	return c
}

// PathAttachment is an attachment whose vertices make up a composite Bezier curve.
type PathAttachment struct {
	VertexAttachment

	// Lengths of each curve segment: the sum of all previous curve lengths.
	Lengths []float32

	// Closed is true if the last and first vertex are connected.
	Closed bool

	// ConstantSpeed is true if movement over the path uses constant speed.
	ConstantSpeed bool

	// Nonessential.
	Color Color
}

// NewPathAttachment creates a path attachment.
func NewPathAttachment(name string) *PathAttachment {
	return &PathAttachment{
		VertexAttachment: newVertexAttachment(name),
		Color:            Color{1, 0.5, 0, 1}, // ff7f00ff
	}
}

// Copy implements Attachment.
func (a *PathAttachment) Copy() Attachment {
	c := NewPathAttachment(a.name)
	a.copyTo(&c.VertexAttachment)
	c.Lengths = make([]float32, len(a.Lengths))
	copy(c.Lengths, a.Lengths)
	c.Closed = a.Closed
	c.ConstantSpeed = a.ConstantSpeed
	c.Color = a.Color
	return c
}

// ClippingAttachment is an attachment with vertices that make up a polygon
// used for clipping the rendering of other attachments.
type ClippingAttachment struct {
	VertexAttachment

	// EndSlot: clipping is performed between the slot containing this
	// attachment and the end slot. If nil, clipping is done until the end of
	// the skeleton's rendering.
	EndSlot *SlotData

	// Nonessential.
	Color Color
}

// NewClippingAttachment creates a clipping attachment.
func NewClippingAttachment(name string) *ClippingAttachment {
	return &ClippingAttachment{
		VertexAttachment: newVertexAttachment(name),
		Color:            Color{0.2275, 0.2275, 0.8078, 1}, // ce3a3aff
	}
}

// Copy implements Attachment.
func (a *ClippingAttachment) Copy() Attachment {
	c := NewClippingAttachment(a.name)
	a.copyTo(&c.VertexAttachment)
	c.EndSlot = a.EndSlot
	c.Color = a.Color
	return c
}

// PointAttachment is an attachment which is a single point and a rotation.
// This can be used to spawn projectiles, particles, etc. A bone can be used in
// similar ways, but a PointAttachment is slightly less expensive to compute
// and can be hidden, shown, and placed in a skin.
type PointAttachment struct {
	baseAttachment

	X, Y, Rotation float32

	// Nonessential.
	Color Color
}

// NewPointAttachment creates a point attachment.
func NewPointAttachment(name string) *PointAttachment {
	return &PointAttachment{
		baseAttachment: newBaseAttachment(name),
		Color:          Color{0.9451, 0.9451, 0, 1}, // f1f100ff
	}
}

// ComputeWorldPosition computes the world position of the point.
func (a *PointAttachment) ComputeWorldPosition(bone *Bone) (x, y float32) {
	x = a.X*bone.A + a.Y*bone.B + bone.WorldX
	y = a.X*bone.C + a.Y*bone.D + bone.WorldY
	return x, y
}

// ComputeWorldRotation computes the world rotation of the point, in degrees.
func (a *PointAttachment) ComputeWorldRotation(bone *Bone) float32 {
	r := a.Rotation * degRad
	cosr, sinr := cos(r), sin(r)
	x := cosr*bone.A + sinr*bone.B
	y := cosr*bone.C + sinr*bone.D
	return atan2Deg(y, x)
}

// Copy implements Attachment.
func (a *PointAttachment) Copy() Attachment {
	c := NewPointAttachment(a.name)
	c.X = a.X
	c.Y = a.Y
	c.Rotation = a.Rotation
	c.Color = a.Color
	return c
}
