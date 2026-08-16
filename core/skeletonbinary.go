package spine

import (
	"fmt"
	"io"
	"math"
	"strconv"
)

// Timeline type constants of the binary format.
const (
	boneRotate     = 0
	boneTranslate  = 1
	boneTranslateX = 2
	boneTranslateY = 3
	boneScale      = 4
	boneScaleX     = 5
	boneScaleY     = 6
	boneShear      = 7
	boneShearX     = 8
	boneShearY     = 9
	boneInherit    = 10

	slotAttachment = 0
	slotRGBA       = 1
	slotRGB        = 2
	slotRGBA2      = 3
	slotRGB2       = 4
	slotAlpha      = 5

	attachmentDeformBin   = 0
	attachmentSequenceBin = 1

	pathPosition = 0
	pathSpacing  = 1
	pathMix      = 2

	physicsInertia  = 0
	physicsStrength = 1
	physicsDamping  = 2
	physicsMass     = 4
	physicsWind     = 5
	physicsGravity  = 6
	physicsMix      = 7
	physicsReset    = 8

	curveLinearBin  = 0
	curveSteppedBin = 1
	curveBezierBin  = 2
)

// Attachment type ordinals of the binary format.
const (
	attachmentTypeRegion = iota
	attachmentTypeBoundingBox
	attachmentTypeMesh
	attachmentTypeLinkedMesh
	attachmentTypePath
	attachmentTypePoint
	attachmentTypeClipping
)

// SkeletonBinary loads skeleton data in the Spine binary (.skel) format.
type SkeletonBinary struct {
	attachmentLoader AttachmentLoader

	// Scale scales the loaded skeleton data. Useful when different sizes of
	// the same skeleton are needed at runtime. Must be != 0.
	Scale float32

	linkedMeshes []binaryLinkedMesh
}

type binaryLinkedMesh struct {
	parent               string
	skinIndex, slotIndex int
	mesh                 *MeshAttachment
	inheritTimelines     bool
}

// NewSkeletonBinary creates a binary loader that creates attachments with the
// given attachment loader.
func NewSkeletonBinary(attachmentLoader AttachmentLoader) *SkeletonBinary {
	if attachmentLoader == nil {
		panic("spine: attachmentLoader cannot be nil")
	}
	return &SkeletonBinary{attachmentLoader: attachmentLoader, Scale: 1}
}

type binaryError struct{ err error }

type skeletonInput struct {
	data    []byte
	pos     int
	strings []string
}

func (in *skeletonInput) fail(err error) {
	panic(binaryError{err})
}

func (in *skeletonInput) read() int {
	if in.pos >= len(in.data) {
		in.fail(io.ErrUnexpectedEOF)
	}
	b := in.data[in.pos]
	in.pos++
	return int(b)
}

func (in *skeletonInput) readByte() int { return int(int8(in.read())) }

func (in *skeletonInput) readUnsignedByte() int { return in.read() }

func (in *skeletonInput) readBoolean() bool { return in.read() != 0 }

func (in *skeletonInput) readInt32() int32 {
	return int32(uint32(in.read())<<24 | uint32(in.read())<<16 | uint32(in.read())<<8 | uint32(in.read()))
}

func (in *skeletonInput) readLong() int64 {
	return int64(in.readInt32())<<32 | int64(uint32(in.readInt32()))
}

func (in *skeletonInput) readFloat() float32 {
	return math.Float32frombits(uint32(in.readInt32()))
}

// readInt reads a 1-5 byte varint (libgdx DataInput.readInt(optimizePositive)).
func (in *skeletonInput) readInt(optimizePositive bool) int {
	b := in.read()
	result := b & 0x7F
	if b&0x80 != 0 {
		b = in.read()
		result |= (b & 0x7F) << 7
		if b&0x80 != 0 {
			b = in.read()
			result |= (b & 0x7F) << 14
			if b&0x80 != 0 {
				b = in.read()
				result |= (b & 0x7F) << 21
				if b&0x80 != 0 {
					b = in.read()
					result |= (b & 0x7F) << 28
				}
			}
		}
	}
	r := int32(result)
	if !optimizePositive {
		r = int32(uint32(r)>>1) ^ -(r & 1)
	}
	return int(r)
}

// readStringNullable reads a string; ok is false when the stored value was null.
func (in *skeletonInput) readStringNullable() (s string, ok bool) {
	byteCount := in.readInt(true)
	switch byteCount {
	case 0:
		return "", false
	case 1:
		return "", true
	}
	byteCount--
	if in.pos+byteCount > len(in.data) {
		in.fail(io.ErrUnexpectedEOF)
	}
	s = string(in.data[in.pos : in.pos+byteCount])
	in.pos += byteCount
	return s, true
}

func (in *skeletonInput) readString() string {
	s, _ := in.readStringNullable()
	return s
}

// readStringRef reads an index into the strings table; "" represents null.
func (in *skeletonInput) readStringRef() string {
	index := in.readInt(true)
	if index == 0 {
		return ""
	}
	return in.strings[index-1]
}

func rgba8888ToColor(v int32) Color {
	u := uint32(v)
	return Color{
		R: float32(u>>24&0xff) / 255,
		G: float32(u>>16&0xff) / 255,
		B: float32(u>>8&0xff) / 255,
		A: float32(u&0xff) / 255,
	}
}

func rgb888ToColor(v int32) Color {
	u := uint32(v)
	return Color{
		R: float32(u>>16&0xff) / 255,
		G: float32(u>>8&0xff) / 255,
		B: float32(u&0xff) / 255,
		A: 1,
	}
}

// ReadSkeletonDataReader reads skeleton data from r.
func (sb *SkeletonBinary) ReadSkeletonDataReader(r io.Reader) (*SkeletonData, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("spine: reading skeleton: %w", err)
	}
	return sb.ReadSkeletonData(data)
}

// ReadSkeletonData reads skeleton data in the Spine 4.2 binary format.
func (sb *SkeletonBinary) ReadSkeletonData(data []byte) (skeletonData *SkeletonData, err error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("spine: data cannot be empty")
	}
	if sb.Scale == 0 {
		panic("spine: scale cannot be 0")
	}
	defer func() {
		sb.linkedMeshes = sb.linkedMeshes[:0]
		if r := recover(); r != nil {
			if be, ok := r.(binaryError); ok {
				skeletonData = nil
				err = fmt.Errorf("spine: error reading skeleton file: %w", be.err)
				return
			}
			panic(r)
		}
	}()

	scale := sb.Scale
	input := &skeletonInput{data: data}
	sd := NewSkeletonData()

	hash := input.readLong()
	if hash != 0 {
		sd.Hash = strconv.FormatInt(hash, 10)
	}
	sd.Version = input.readString()
	sd.X = input.readFloat()
	sd.Y = input.readFloat()
	sd.Width = input.readFloat()
	sd.Height = input.readFloat()
	sd.ReferenceScale = input.readFloat() * scale

	nonessential := input.readBoolean()
	if nonessential {
		sd.FPS = input.readFloat()
		sd.ImagesPath = input.readString()
		sd.AudioPath = input.readString()
	}

	// Strings.
	n := input.readInt(true)
	input.strings = make([]string, n)
	for i := 0; i < n; i++ {
		input.strings[i] = input.readString()
	}

	// Bones.
	n = input.readInt(true)
	sd.Bones = make([]*BoneData, n)
	for i := 0; i < n; i++ {
		name := input.readString()
		var parent *BoneData
		if i != 0 {
			parent = sd.Bones[input.readInt(true)]
		}
		bd := NewBoneData(i, name, parent)
		bd.Rotation = input.readFloat()
		bd.X = input.readFloat() * scale
		bd.Y = input.readFloat() * scale
		bd.ScaleX = input.readFloat()
		bd.ScaleY = input.readFloat()
		bd.ShearX = input.readFloat()
		bd.ShearY = input.readFloat()
		bd.Length = input.readFloat() * scale
		bd.Inherit = Inherit(input.readByte())
		bd.SkinRequired = input.readBoolean()
		if nonessential {
			bd.Color = rgba8888ToColor(input.readInt32())
			bd.Icon = input.readString()
			bd.Visible = input.readBoolean()
		}
		sd.Bones[i] = bd
	}

	// Slots.
	n = input.readInt(true)
	sd.Slots = make([]*SlotData, n)
	for i := 0; i < n; i++ {
		slotName := input.readString()
		boneData := sd.Bones[input.readInt(true)]
		slotData := NewSlotData(i, slotName, boneData)
		slotData.Color = rgba8888ToColor(input.readInt32())

		darkColor := input.readInt32()
		if darkColor != -1 {
			c := rgb888ToColor(darkColor)
			slotData.DarkColor = &c
		}

		slotData.AttachmentName = input.readStringRef()
		slotData.BlendMode = BlendMode(input.readInt(true))
		if nonessential {
			slotData.Visible = input.readBoolean()
		}
		sd.Slots[i] = slotData
	}

	// IK constraints.
	n = input.readInt(true)
	sd.IkConstraints = make([]*IkConstraintData, n)
	for i := 0; i < n; i++ {
		ikData := NewIkConstraintData(input.readString())
		ikData.Order = input.readInt(true)
		nn := input.readInt(true)
		ikData.Bones = make([]*BoneData, nn)
		for ii := 0; ii < nn; ii++ {
			ikData.Bones[ii] = sd.Bones[input.readInt(true)]
		}
		ikData.Target = sd.Bones[input.readInt(true)]
		flags := input.read()
		ikData.SkinRequired = flags&1 != 0
		if flags&2 != 0 {
			ikData.BendDirection = 1
		} else {
			ikData.BendDirection = -1
		}
		ikData.Compress = flags&4 != 0
		ikData.Stretch = flags&8 != 0
		ikData.Uniform = flags&16 != 0
		if flags&32 != 0 {
			if flags&64 != 0 {
				ikData.Mix = input.readFloat()
			} else {
				ikData.Mix = 1
			}
		}
		if flags&128 != 0 {
			ikData.Softness = input.readFloat() * scale
		}
		sd.IkConstraints[i] = ikData
	}

	// Transform constraints.
	n = input.readInt(true)
	sd.TransformConstraints = make([]*TransformConstraintData, n)
	for i := 0; i < n; i++ {
		tData := NewTransformConstraintData(input.readString())
		tData.Order = input.readInt(true)
		nn := input.readInt(true)
		tData.Bones = make([]*BoneData, nn)
		for ii := 0; ii < nn; ii++ {
			tData.Bones[ii] = sd.Bones[input.readInt(true)]
		}
		tData.Target = sd.Bones[input.readInt(true)]
		flags := input.read()
		tData.SkinRequired = flags&1 != 0
		tData.Local = flags&2 != 0
		tData.Relative = flags&4 != 0
		if flags&8 != 0 {
			tData.OffsetRotation = input.readFloat()
		}
		if flags&16 != 0 {
			tData.OffsetX = input.readFloat() * scale
		}
		if flags&32 != 0 {
			tData.OffsetY = input.readFloat() * scale
		}
		if flags&64 != 0 {
			tData.OffsetScaleX = input.readFloat()
		}
		if flags&128 != 0 {
			tData.OffsetScaleY = input.readFloat()
		}
		flags = input.read()
		if flags&1 != 0 {
			tData.OffsetShearY = input.readFloat()
		}
		if flags&2 != 0 {
			tData.MixRotate = input.readFloat()
		}
		if flags&4 != 0 {
			tData.MixX = input.readFloat()
		}
		if flags&8 != 0 {
			tData.MixY = input.readFloat()
		}
		if flags&16 != 0 {
			tData.MixScaleX = input.readFloat()
		}
		if flags&32 != 0 {
			tData.MixScaleY = input.readFloat()
		}
		if flags&64 != 0 {
			tData.MixShearY = input.readFloat()
		}
		sd.TransformConstraints[i] = tData
	}

	// Path constraints.
	n = input.readInt(true)
	sd.PathConstraints = make([]*PathConstraintData, n)
	for i := 0; i < n; i++ {
		pData := NewPathConstraintData(input.readString())
		pData.Order = input.readInt(true)
		pData.SkinRequired = input.readBoolean()
		nn := input.readInt(true)
		pData.Bones = make([]*BoneData, nn)
		for ii := 0; ii < nn; ii++ {
			pData.Bones[ii] = sd.Bones[input.readInt(true)]
		}
		pData.Target = sd.Slots[input.readInt(true)]
		flags := input.read()
		pData.PositionMode = PositionMode(flags & 1)
		pData.SpacingMode = SpacingMode((flags >> 1) & 3)
		pData.RotateMode = RotateMode((flags >> 3) & 3)
		if flags&128 != 0 {
			pData.OffsetRotation = input.readFloat()
		}
		pData.Position = input.readFloat()
		if pData.PositionMode == PositionModeFixed {
			pData.Position *= scale
		}
		pData.Spacing = input.readFloat()
		if pData.SpacingMode == SpacingModeLength || pData.SpacingMode == SpacingModeFixed {
			pData.Spacing *= scale
		}
		pData.MixRotate = input.readFloat()
		pData.MixX = input.readFloat()
		pData.MixY = input.readFloat()
		sd.PathConstraints[i] = pData
	}

	// Physics constraints.
	n = input.readInt(true)
	sd.PhysicsConstraints = make([]*PhysicsConstraintData, n)
	for i := 0; i < n; i++ {
		phData := NewPhysicsConstraintData(input.readString())
		phData.Order = input.readInt(true)
		phData.Bone = sd.Bones[input.readInt(true)]
		flags := input.read()
		phData.SkinRequired = flags&1 != 0
		if flags&2 != 0 {
			phData.X = input.readFloat()
		}
		if flags&4 != 0 {
			phData.Y = input.readFloat()
		}
		if flags&8 != 0 {
			phData.Rotate = input.readFloat()
		}
		if flags&16 != 0 {
			phData.ScaleX = input.readFloat()
		}
		if flags&32 != 0 {
			phData.ShearX = input.readFloat()
		}
		if flags&64 != 0 {
			phData.Limit = input.readFloat() * scale
		} else {
			phData.Limit = 5000 * scale
		}
		phData.Step = 1 / float32(input.readUnsignedByte())
		phData.Inertia = input.readFloat()
		phData.Strength = input.readFloat()
		phData.Damping = input.readFloat()
		if flags&128 != 0 {
			phData.MassInverse = input.readFloat()
		} else {
			phData.MassInverse = 1
		}
		phData.Wind = input.readFloat()
		phData.Gravity = input.readFloat()
		flags = input.read()
		phData.InertiaGlobal = flags&1 != 0
		phData.StrengthGlobal = flags&2 != 0
		phData.DampingGlobal = flags&4 != 0
		phData.MassGlobal = flags&8 != 0
		phData.WindGlobal = flags&16 != 0
		phData.GravityGlobal = flags&32 != 0
		phData.MixGlobal = flags&64 != 0
		if flags&128 != 0 {
			phData.Mix = input.readFloat()
		} else {
			phData.Mix = 1
		}
		sd.PhysicsConstraints[i] = phData
	}

	// Default skin.
	defaultSkin, err2 := sb.readSkin(input, sd, true, nonessential)
	if err2 != nil {
		return nil, err2
	}
	if defaultSkin != nil {
		sd.DefaultSkin = defaultSkin
		sd.Skins = append(sd.Skins, defaultSkin)
	}

	// Skins.
	for i, nn := 0, input.readInt(true); i < nn; i++ {
		skin, err2 := sb.readSkin(input, sd, false, nonessential)
		if err2 != nil {
			return nil, err2
		}
		sd.Skins = append(sd.Skins, skin)
	}

	// Linked meshes.
	for _, lm := range sb.linkedMeshes {
		skin := sd.Skins[lm.skinIndex]
		parent := skin.Attachment(lm.slotIndex, lm.parent)
		if parent == nil {
			return nil, fmt.Errorf("spine: parent mesh not found: %s", lm.parent)
		}
		if lm.inheritTimelines {
			lm.mesh.TimelineAttachment = parent
		} else {
			lm.mesh.TimelineAttachment = lm.mesh
		}
		parentMesh, ok := parent.(*MeshAttachment)
		if !ok {
			return nil, fmt.Errorf("spine: parent mesh is not a mesh attachment: %s", lm.parent)
		}
		lm.mesh.SetParentMesh(parentMesh)
		if lm.mesh.Sequence == nil {
			lm.mesh.UpdateRegion()
		}
	}
	sb.linkedMeshes = sb.linkedMeshes[:0]

	// Events.
	n = input.readInt(true)
	sd.Events = make([]*EventData, n)
	for i := 0; i < n; i++ {
		eData := NewEventData(input.readString())
		eData.Int = input.readInt(false)
		eData.Float = input.readFloat()
		eData.String = input.readString()
		audioPath, hasAudio := input.readStringNullable()
		eData.AudioPath = audioPath
		if hasAudio {
			eData.Volume = input.readFloat()
			eData.Balance = input.readFloat()
		}
		sd.Events[i] = eData
	}

	// Animations.
	n = input.readInt(true)
	sd.Animations = make([]*Animation, n)
	for i := 0; i < n; i++ {
		anim, err2 := sb.readAnimation(input, input.readString(), sd)
		if err2 != nil {
			return nil, err2
		}
		sd.Animations[i] = anim
	}

	return sd, nil
}

func (sb *SkeletonBinary) readSkin(input *skeletonInput, skeletonData *SkeletonData, defaultSkin, nonessential bool) (*Skin, error) {
	var skin *Skin
	var slotCount int
	if defaultSkin {
		slotCount = input.readInt(true)
		if slotCount == 0 {
			return nil, nil
		}
		skin = NewSkin("default")
	} else {
		skin = NewSkin(input.readString())

		if nonessential {
			skin.Color = rgba8888ToColor(input.readInt32())
		}

		nn := input.readInt(true)
		skin.Bones = make([]*BoneData, nn)
		for i := 0; i < nn; i++ {
			skin.Bones[i] = skeletonData.Bones[input.readInt(true)]
		}

		for i, nn := 0, input.readInt(true); i < nn; i++ {
			skin.Constraints = append(skin.Constraints, skeletonData.IkConstraints[input.readInt(true)])
		}
		for i, nn := 0, input.readInt(true); i < nn; i++ {
			skin.Constraints = append(skin.Constraints, skeletonData.TransformConstraints[input.readInt(true)])
		}
		for i, nn := 0, input.readInt(true); i < nn; i++ {
			skin.Constraints = append(skin.Constraints, skeletonData.PathConstraints[input.readInt(true)])
		}
		for i, nn := 0, input.readInt(true); i < nn; i++ {
			skin.Constraints = append(skin.Constraints, skeletonData.PhysicsConstraints[input.readInt(true)])
		}

		slotCount = input.readInt(true)
	}

	for i := 0; i < slotCount; i++ {
		slotIndex := input.readInt(true)
		for ii, nn := 0, input.readInt(true); ii < nn; ii++ {
			name := input.readStringRef()
			attachment, err := sb.readAttachment(input, skeletonData, skin, slotIndex, name, nonessential)
			if err != nil {
				return nil, err
			}
			if attachment != nil {
				skin.SetAttachment(slotIndex, name, attachment)
			}
		}
	}
	return skin, nil
}

func (sb *SkeletonBinary) readAttachment(input *skeletonInput, skeletonData *SkeletonData, skin *Skin, slotIndex int,
	attachmentName string, nonessential bool) (Attachment, error) {
	scale := sb.Scale

	flags := input.readByte()
	name := attachmentName
	if flags&8 != 0 {
		name = input.readStringRef()
	}
	switch flags & 0b111 {
	case attachmentTypeRegion:
		var path string
		if flags&16 != 0 {
			path = input.readStringRef()
		}
		color := int32(-1) // 0xffffffff
		if flags&32 != 0 {
			color = input.readInt32()
		}
		var sequence *Sequence
		if flags&64 != 0 {
			sequence = readBinarySequence(input)
		}
		var rotation float32
		if flags&128 != 0 {
			rotation = input.readFloat()
		}
		x := input.readFloat()
		y := input.readFloat()
		scaleX := input.readFloat()
		scaleY := input.readFloat()
		width := input.readFloat()
		height := input.readFloat()

		if path == "" {
			path = name
		}
		region, err := sb.attachmentLoader.NewRegionAttachment(skin, name, path, sequence)
		if err != nil {
			return nil, err
		}
		if region == nil {
			return nil, nil
		}
		region.Path = path
		region.X = x * scale
		region.Y = y * scale
		region.ScaleX = scaleX
		region.ScaleY = scaleY
		region.Rotation = rotation
		region.Width = width * scale
		region.Height = height * scale
		region.Color = rgba8888ToColor(color)
		region.Sequence = sequence
		if sequence == nil {
			region.UpdateRegion()
		}
		return region, nil

	case attachmentTypeBoundingBox:
		vertices := readBinaryVertices(input, flags&16 != 0, scale)
		var color int32
		if nonessential {
			color = input.readInt32()
		}

		box, err := sb.attachmentLoader.NewBoundingBoxAttachment(skin, name)
		if err != nil {
			return nil, err
		}
		if box == nil {
			return nil, nil
		}
		box.WorldVerticesLength = vertices.length
		box.Vertices = vertices.vertices
		box.Bones = vertices.bones
		if nonessential {
			box.Color = rgba8888ToColor(color)
		}
		return box, nil

	case attachmentTypeMesh:
		path := name
		if flags&16 != 0 {
			path = input.readStringRef()
		}
		color := int32(-1)
		if flags&32 != 0 {
			color = input.readInt32()
		}
		var sequence *Sequence
		if flags&64 != 0 {
			sequence = readBinarySequence(input)
		}
		hullLength := input.readInt(true)
		vertices := readBinaryVertices(input, flags&128 != 0, scale)
		uvs := readBinaryFloatArray(input, vertices.length, 1)
		triangles := readBinaryShortArray(input, (vertices.length-hullLength-2)*3)

		var edges []uint16
		var width, height float32
		if nonessential {
			edges = readBinaryShortArray(input, input.readInt(true))
			width = input.readFloat()
			height = input.readFloat()
		}

		mesh, err := sb.attachmentLoader.NewMeshAttachment(skin, name, path, sequence)
		if err != nil {
			return nil, err
		}
		if mesh == nil {
			return nil, nil
		}
		mesh.Path = path
		mesh.Color = rgba8888ToColor(color)
		mesh.Bones = vertices.bones
		mesh.Vertices = vertices.vertices
		mesh.WorldVerticesLength = vertices.length
		mesh.Triangles = triangles
		mesh.RegionUVs = uvs
		if sequence == nil {
			mesh.UpdateRegion()
		}
		mesh.HullLength = hullLength << 1
		mesh.Sequence = sequence
		if nonessential {
			mesh.Edges = edges
			mesh.Width = width * scale
			mesh.Height = height * scale
		}
		return mesh, nil

	case attachmentTypeLinkedMesh:
		path := name
		if flags&16 != 0 {
			path = input.readStringRef()
		}
		color := int32(-1)
		if flags&32 != 0 {
			color = input.readInt32()
		}
		var sequence *Sequence
		if flags&64 != 0 {
			sequence = readBinarySequence(input)
		}
		inheritTimelines := flags&128 != 0
		skinIndex := input.readInt(true)
		parent := input.readStringRef()
		var width, height float32
		if nonessential {
			width = input.readFloat()
			height = input.readFloat()
		}

		mesh, err := sb.attachmentLoader.NewMeshAttachment(skin, name, path, sequence)
		if err != nil {
			return nil, err
		}
		if mesh == nil {
			return nil, nil
		}
		mesh.Path = path
		mesh.Color = rgba8888ToColor(color)
		mesh.Sequence = sequence
		if nonessential {
			mesh.Width = width * scale
			mesh.Height = height * scale
		}
		sb.linkedMeshes = append(sb.linkedMeshes, binaryLinkedMesh{
			parent: parent, skinIndex: skinIndex, slotIndex: slotIndex, mesh: mesh, inheritTimelines: inheritTimelines,
		})
		return mesh, nil

	case attachmentTypePath:
		closed := flags&16 != 0
		constantSpeed := flags&32 != 0
		vertices := readBinaryVertices(input, flags&64 != 0, scale)
		lengths := make([]float32, vertices.length/6)
		for i := range lengths {
			lengths[i] = input.readFloat() * scale
		}
		var color int32
		if nonessential {
			color = input.readInt32()
		}

		path, err := sb.attachmentLoader.NewPathAttachment(skin, name)
		if err != nil {
			return nil, err
		}
		if path == nil {
			return nil, nil
		}
		path.Closed = closed
		path.ConstantSpeed = constantSpeed
		path.WorldVerticesLength = vertices.length
		path.Vertices = vertices.vertices
		path.Bones = vertices.bones
		path.Lengths = lengths
		if nonessential {
			path.Color = rgba8888ToColor(color)
		}
		return path, nil

	case attachmentTypePoint:
		rotation := input.readFloat()
		x := input.readFloat()
		y := input.readFloat()
		var color int32
		if nonessential {
			color = input.readInt32()
		}

		point, err := sb.attachmentLoader.NewPointAttachment(skin, name)
		if err != nil {
			return nil, err
		}
		if point == nil {
			return nil, nil
		}
		point.X = x * scale
		point.Y = y * scale
		point.Rotation = rotation
		if nonessential {
			point.Color = rgba8888ToColor(color)
		}
		return point, nil

	case attachmentTypeClipping:
		endSlotIndex := input.readInt(true)
		vertices := readBinaryVertices(input, flags&16 != 0, scale)
		var color int32
		if nonessential {
			color = input.readInt32()
		}

		clip, err := sb.attachmentLoader.NewClippingAttachment(skin, name)
		if err != nil {
			return nil, err
		}
		if clip == nil {
			return nil, nil
		}
		clip.EndSlot = skeletonData.Slots[endSlotIndex]
		clip.WorldVerticesLength = vertices.length
		clip.Vertices = vertices.vertices
		clip.Bones = vertices.bones
		if nonessential {
			clip.Color = rgba8888ToColor(color)
		}
		return clip, nil
	}
	return nil, nil
}

func readBinarySequence(input *skeletonInput) *Sequence {
	sequence := NewSequence(input.readInt(true))
	sequence.Start = input.readInt(true)
	sequence.Digits = input.readInt(true)
	sequence.SetupIndex = input.readInt(true)
	return sequence
}

type binaryVertices struct {
	length   int
	bones    []int32
	vertices []float32
}

func readBinaryVertices(input *skeletonInput, weighted bool, scale float32) binaryVertices {
	vertexCount := input.readInt(true)
	var vertices binaryVertices
	vertices.length = vertexCount << 1
	if !weighted {
		vertices.vertices = readBinaryFloatArray(input, vertices.length, scale)
		return vertices
	}
	weights := make([]float32, 0, vertices.length*3*3)
	bonesArray := make([]int32, 0, vertices.length*3)
	for i := 0; i < vertexCount; i++ {
		boneCount := input.readInt(true)
		bonesArray = append(bonesArray, int32(boneCount))
		for ii := 0; ii < boneCount; ii++ {
			bonesArray = append(bonesArray, int32(input.readInt(true)))
			weights = append(weights, input.readFloat()*scale, input.readFloat()*scale, input.readFloat())
		}
	}
	vertices.vertices = weights
	vertices.bones = bonesArray
	return vertices
}

func readBinaryFloatArray(input *skeletonInput, n int, scale float32) []float32 {
	array := make([]float32, n)
	if scale == 1 {
		for i := 0; i < n; i++ {
			array[i] = input.readFloat()
		}
	} else {
		for i := 0; i < n; i++ {
			array[i] = input.readFloat() * scale
		}
	}
	return array
}

func readBinaryShortArray(input *skeletonInput, n int) []uint16 {
	array := make([]uint16, n)
	for i := 0; i < n; i++ {
		array[i] = uint16(input.readInt(true))
	}
	return array
}

func (sb *SkeletonBinary) readAnimation(input *skeletonInput, name string, skeletonData *SkeletonData) (*Animation, error) {
	timelines := make([]Timeline, 0, input.readInt(true))
	scale := sb.Scale

	// Slot timelines.
	for i, n := 0, input.readInt(true); i < n; i++ {
		slotIndex := input.readInt(true)
		for ii, nn := 0, input.readInt(true); ii < nn; ii++ {
			timelineType := input.readByte()
			frameCount := input.readInt(true)
			frameLast := frameCount - 1
			switch timelineType {
			case slotAttachment:
				timeline := NewAttachmentTimeline(frameCount, slotIndex)
				for frame := 0; frame < frameCount; frame++ {
					time := input.readFloat()
					timeline.SetFrame(frame, time, input.readStringRef())
				}
				timelines = append(timelines, timeline)

			case slotRGBA:
				timeline := NewRGBATimeline(frameCount, input.readInt(true), slotIndex)
				time := input.readFloat()
				r := float32(input.read()) / 255
				g := float32(input.read()) / 255
				b := float32(input.read()) / 255
				a := float32(input.read()) / 255
				for frame, bezier := 0, 0; ; frame++ {
					timeline.SetFrame(frame, time, r, g, b, a)
					if frame == frameLast {
						break
					}
					time2 := input.readFloat()
					r2 := float32(input.read()) / 255
					g2 := float32(input.read()) / 255
					b2 := float32(input.read()) / 255
					a2 := float32(input.read()) / 255
					switch input.readByte() {
					case curveSteppedBin:
						timeline.SetStepped(frame)
					case curveBezierBin:
						setBinaryBezier(input, timeline, bezier, frame, 0, time, time2, r, r2, 1)
						bezier++
						setBinaryBezier(input, timeline, bezier, frame, 1, time, time2, g, g2, 1)
						bezier++
						setBinaryBezier(input, timeline, bezier, frame, 2, time, time2, b, b2, 1)
						bezier++
						setBinaryBezier(input, timeline, bezier, frame, 3, time, time2, a, a2, 1)
						bezier++
					}
					time = time2
					r, g, b, a = r2, g2, b2, a2
				}
				timelines = append(timelines, timeline)

			case slotRGB:
				timeline := NewRGBTimeline(frameCount, input.readInt(true), slotIndex)
				time := input.readFloat()
				r := float32(input.read()) / 255
				g := float32(input.read()) / 255
				b := float32(input.read()) / 255
				for frame, bezier := 0, 0; ; frame++ {
					timeline.SetFrame(frame, time, r, g, b)
					if frame == frameLast {
						break
					}
					time2 := input.readFloat()
					r2 := float32(input.read()) / 255
					g2 := float32(input.read()) / 255
					b2 := float32(input.read()) / 255
					switch input.readByte() {
					case curveSteppedBin:
						timeline.SetStepped(frame)
					case curveBezierBin:
						setBinaryBezier(input, timeline, bezier, frame, 0, time, time2, r, r2, 1)
						bezier++
						setBinaryBezier(input, timeline, bezier, frame, 1, time, time2, g, g2, 1)
						bezier++
						setBinaryBezier(input, timeline, bezier, frame, 2, time, time2, b, b2, 1)
						bezier++
					}
					time = time2
					r, g, b = r2, g2, b2
				}
				timelines = append(timelines, timeline)

			case slotRGBA2:
				timeline := NewRGBA2Timeline(frameCount, input.readInt(true), slotIndex)
				time := input.readFloat()
				r := float32(input.read()) / 255
				g := float32(input.read()) / 255
				b := float32(input.read()) / 255
				a := float32(input.read()) / 255
				r2 := float32(input.read()) / 255
				g2 := float32(input.read()) / 255
				b2 := float32(input.read()) / 255
				for frame, bezier := 0, 0; ; frame++ {
					timeline.SetFrame(frame, time, r, g, b, a, r2, g2, b2)
					if frame == frameLast {
						break
					}
					time2 := input.readFloat()
					nr := float32(input.read()) / 255
					ng := float32(input.read()) / 255
					nb := float32(input.read()) / 255
					na := float32(input.read()) / 255
					nr2 := float32(input.read()) / 255
					ng2 := float32(input.read()) / 255
					nb2 := float32(input.read()) / 255
					switch input.readByte() {
					case curveSteppedBin:
						timeline.SetStepped(frame)
					case curveBezierBin:
						setBinaryBezier(input, timeline, bezier, frame, 0, time, time2, r, nr, 1)
						bezier++
						setBinaryBezier(input, timeline, bezier, frame, 1, time, time2, g, ng, 1)
						bezier++
						setBinaryBezier(input, timeline, bezier, frame, 2, time, time2, b, nb, 1)
						bezier++
						setBinaryBezier(input, timeline, bezier, frame, 3, time, time2, a, na, 1)
						bezier++
						setBinaryBezier(input, timeline, bezier, frame, 4, time, time2, r2, nr2, 1)
						bezier++
						setBinaryBezier(input, timeline, bezier, frame, 5, time, time2, g2, ng2, 1)
						bezier++
						setBinaryBezier(input, timeline, bezier, frame, 6, time, time2, b2, nb2, 1)
						bezier++
					}
					time = time2
					r, g, b, a = nr, ng, nb, na
					r2, g2, b2 = nr2, ng2, nb2
				}
				timelines = append(timelines, timeline)

			case slotRGB2:
				timeline := NewRGB2Timeline(frameCount, input.readInt(true), slotIndex)
				time := input.readFloat()
				r := float32(input.read()) / 255
				g := float32(input.read()) / 255
				b := float32(input.read()) / 255
				r2 := float32(input.read()) / 255
				g2 := float32(input.read()) / 255
				b2 := float32(input.read()) / 255
				for frame, bezier := 0, 0; ; frame++ {
					timeline.SetFrame(frame, time, r, g, b, r2, g2, b2)
					if frame == frameLast {
						break
					}
					time2 := input.readFloat()
					nr := float32(input.read()) / 255
					ng := float32(input.read()) / 255
					nb := float32(input.read()) / 255
					nr2 := float32(input.read()) / 255
					ng2 := float32(input.read()) / 255
					nb2 := float32(input.read()) / 255
					switch input.readByte() {
					case curveSteppedBin:
						timeline.SetStepped(frame)
					case curveBezierBin:
						setBinaryBezier(input, timeline, bezier, frame, 0, time, time2, r, nr, 1)
						bezier++
						setBinaryBezier(input, timeline, bezier, frame, 1, time, time2, g, ng, 1)
						bezier++
						setBinaryBezier(input, timeline, bezier, frame, 2, time, time2, b, nb, 1)
						bezier++
						setBinaryBezier(input, timeline, bezier, frame, 3, time, time2, r2, nr2, 1)
						bezier++
						setBinaryBezier(input, timeline, bezier, frame, 4, time, time2, g2, ng2, 1)
						bezier++
						setBinaryBezier(input, timeline, bezier, frame, 5, time, time2, b2, nb2, 1)
						bezier++
					}
					time = time2
					r, g, b = nr, ng, nb
					r2, g2, b2 = nr2, ng2, nb2
				}
				timelines = append(timelines, timeline)

			case slotAlpha:
				timeline := NewAlphaTimeline(frameCount, input.readInt(true), slotIndex)
				time := input.readFloat()
				a := float32(input.read()) / 255
				for frame, bezier := 0, 0; ; frame++ {
					timeline.SetFrame(frame, time, a)
					if frame == frameLast {
						break
					}
					time2 := input.readFloat()
					a2 := float32(input.read()) / 255
					switch input.readByte() {
					case curveSteppedBin:
						timeline.SetStepped(frame)
					case curveBezierBin:
						setBinaryBezier(input, timeline, bezier, frame, 0, time, time2, a, a2, 1)
						bezier++
					}
					time = time2
					a = a2
				}
				timelines = append(timelines, timeline)
			}
		}
	}

	// Bone timelines.
	for i, n := 0, input.readInt(true); i < n; i++ {
		boneIndex := input.readInt(true)
		for ii, nn := 0, input.readInt(true); ii < nn; ii++ {
			timelineType := input.readByte()
			frameCount := input.readInt(true)
			if timelineType == boneInherit {
				timeline := NewInheritTimeline(frameCount, boneIndex)
				for frame := 0; frame < frameCount; frame++ {
					time := input.readFloat()
					timeline.SetFrame(frame, time, Inherit(input.readByte()))
				}
				timelines = append(timelines, timeline)
				continue
			}
			bezierCount := input.readInt(true)
			switch timelineType {
			case boneRotate:
				timelines = readBinaryTimeline1(input, timelines, NewRotateTimeline(frameCount, bezierCount, boneIndex), 1)
			case boneTranslate:
				timelines = readBinaryTimeline2(input, timelines, NewTranslateTimeline(frameCount, bezierCount, boneIndex), scale)
			case boneTranslateX:
				timelines = readBinaryTimeline1(input, timelines, NewTranslateXTimeline(frameCount, bezierCount, boneIndex), scale)
			case boneTranslateY:
				timelines = readBinaryTimeline1(input, timelines, NewTranslateYTimeline(frameCount, bezierCount, boneIndex), scale)
			case boneScale:
				timelines = readBinaryTimeline2(input, timelines, NewScaleTimeline(frameCount, bezierCount, boneIndex), 1)
			case boneScaleX:
				timelines = readBinaryTimeline1(input, timelines, NewScaleXTimeline(frameCount, bezierCount, boneIndex), 1)
			case boneScaleY:
				timelines = readBinaryTimeline1(input, timelines, NewScaleYTimeline(frameCount, bezierCount, boneIndex), 1)
			case boneShear:
				timelines = readBinaryTimeline2(input, timelines, NewShearTimeline(frameCount, bezierCount, boneIndex), 1)
			case boneShearX:
				timelines = readBinaryTimeline1(input, timelines, NewShearXTimeline(frameCount, bezierCount, boneIndex), 1)
			case boneShearY:
				timelines = readBinaryTimeline1(input, timelines, NewShearYTimeline(frameCount, bezierCount, boneIndex), 1)
			}
		}
	}

	// IK constraint timelines.
	for i, n := 0, input.readInt(true); i < n; i++ {
		index := input.readInt(true)
		frameCount := input.readInt(true)
		frameLast := frameCount - 1
		timeline := NewIkConstraintTimeline(frameCount, input.readInt(true), index)
		flags := input.read()
		time := input.readFloat()
		var mix float32
		if flags&1 != 0 {
			if flags&2 != 0 {
				mix = input.readFloat()
			} else {
				mix = 1
			}
		}
		var softness float32
		if flags&4 != 0 {
			softness = input.readFloat() * scale
		}
		for frame, bezier := 0, 0; ; frame++ {
			bendDirection := -1
			if flags&8 != 0 {
				bendDirection = 1
			}
			timeline.SetFrame(frame, time, mix, softness, bendDirection, flags&16 != 0, flags&32 != 0)
			if frame == frameLast {
				break
			}
			flags = input.read()
			time2 := input.readFloat()
			var mix2 float32
			if flags&1 != 0 {
				if flags&2 != 0 {
					mix2 = input.readFloat()
				} else {
					mix2 = 1
				}
			}
			var softness2 float32
			if flags&4 != 0 {
				softness2 = input.readFloat() * scale
			}
			if flags&64 != 0 {
				timeline.SetStepped(frame)
			} else if flags&128 != 0 {
				setBinaryBezier(input, timeline, bezier, frame, 0, time, time2, mix, mix2, 1)
				bezier++
				setBinaryBezier(input, timeline, bezier, frame, 1, time, time2, softness, softness2, scale)
				bezier++
			}
			time = time2
			mix = mix2
			softness = softness2
		}
		timelines = append(timelines, timeline)
	}

	// Transform constraint timelines.
	for i, n := 0, input.readInt(true); i < n; i++ {
		index := input.readInt(true)
		frameCount := input.readInt(true)
		frameLast := frameCount - 1
		timeline := NewTransformConstraintTimeline(frameCount, input.readInt(true), index)
		time := input.readFloat()
		mixRotate := input.readFloat()
		mixX := input.readFloat()
		mixY := input.readFloat()
		mixScaleX := input.readFloat()
		mixScaleY := input.readFloat()
		mixShearY := input.readFloat()
		for frame, bezier := 0, 0; ; frame++ {
			timeline.SetFrame(frame, time, mixRotate, mixX, mixY, mixScaleX, mixScaleY, mixShearY)
			if frame == frameLast {
				break
			}
			time2 := input.readFloat()
			mixRotate2 := input.readFloat()
			mixX2 := input.readFloat()
			mixY2 := input.readFloat()
			mixScaleX2 := input.readFloat()
			mixScaleY2 := input.readFloat()
			mixShearY2 := input.readFloat()
			switch input.readByte() {
			case curveSteppedBin:
				timeline.SetStepped(frame)
			case curveBezierBin:
				setBinaryBezier(input, timeline, bezier, frame, 0, time, time2, mixRotate, mixRotate2, 1)
				bezier++
				setBinaryBezier(input, timeline, bezier, frame, 1, time, time2, mixX, mixX2, 1)
				bezier++
				setBinaryBezier(input, timeline, bezier, frame, 2, time, time2, mixY, mixY2, 1)
				bezier++
				setBinaryBezier(input, timeline, bezier, frame, 3, time, time2, mixScaleX, mixScaleX2, 1)
				bezier++
				setBinaryBezier(input, timeline, bezier, frame, 4, time, time2, mixScaleY, mixScaleY2, 1)
				bezier++
				setBinaryBezier(input, timeline, bezier, frame, 5, time, time2, mixShearY, mixShearY2, 1)
				bezier++
			}
			time = time2
			mixRotate = mixRotate2
			mixX = mixX2
			mixY = mixY2
			mixScaleX = mixScaleX2
			mixScaleY = mixScaleY2
			mixShearY = mixShearY2
		}
		timelines = append(timelines, timeline)
	}

	// Path constraint timelines.
	for i, n := 0, input.readInt(true); i < n; i++ {
		index := input.readInt(true)
		data := skeletonData.PathConstraints[index]
		for ii, nn := 0, input.readInt(true); ii < nn; ii++ {
			timelineType := input.readByte()
			frameCount := input.readInt(true)
			bezierCount := input.readInt(true)
			switch timelineType {
			case pathPosition:
				s := float32(1)
				if data.PositionMode == PositionModeFixed {
					s = scale
				}
				timelines = readBinaryTimeline1(input, timelines, NewPathConstraintPositionTimeline(frameCount, bezierCount, index), s)
			case pathSpacing:
				s := float32(1)
				if data.SpacingMode == SpacingModeLength || data.SpacingMode == SpacingModeFixed {
					s = scale
				}
				timelines = readBinaryTimeline1(input, timelines, NewPathConstraintSpacingTimeline(frameCount, bezierCount, index), s)
			case pathMix:
				timeline := NewPathConstraintMixTimeline(frameCount, bezierCount, index)
				time := input.readFloat()
				mixRotate := input.readFloat()
				mixX := input.readFloat()
				mixY := input.readFloat()
				for frame, bezier, frameLast := 0, 0, timeline.FrameCount()-1; ; frame++ {
					timeline.SetFrame(frame, time, mixRotate, mixX, mixY)
					if frame == frameLast {
						break
					}
					time2 := input.readFloat()
					mixRotate2 := input.readFloat()
					mixX2 := input.readFloat()
					mixY2 := input.readFloat()
					switch input.readByte() {
					case curveSteppedBin:
						timeline.SetStepped(frame)
					case curveBezierBin:
						setBinaryBezier(input, timeline, bezier, frame, 0, time, time2, mixRotate, mixRotate2, 1)
						bezier++
						setBinaryBezier(input, timeline, bezier, frame, 1, time, time2, mixX, mixX2, 1)
						bezier++
						setBinaryBezier(input, timeline, bezier, frame, 2, time, time2, mixY, mixY2, 1)
						bezier++
					}
					time = time2
					mixRotate = mixRotate2
					mixX = mixX2
					mixY = mixY2
				}
				timelines = append(timelines, timeline)
			}
		}
	}

	// Physics constraint timelines.
	for i, n := 0, input.readInt(true); i < n; i++ {
		index := input.readInt(true) - 1
		for ii, nn := 0, input.readInt(true); ii < nn; ii++ {
			timelineType := input.readByte()
			frameCount := input.readInt(true)
			if timelineType == physicsReset {
				timeline := NewPhysicsConstraintResetTimeline(frameCount, index)
				for frame := 0; frame < frameCount; frame++ {
					timeline.SetFrame(frame, input.readFloat())
				}
				timelines = append(timelines, timeline)
				continue
			}
			bezierCount := input.readInt(true)
			switch timelineType {
			case physicsInertia:
				timelines = readBinaryTimeline1(input, timelines, NewPhysicsConstraintInertiaTimeline(frameCount, bezierCount, index), 1)
			case physicsStrength:
				timelines = readBinaryTimeline1(input, timelines, NewPhysicsConstraintStrengthTimeline(frameCount, bezierCount, index), 1)
			case physicsDamping:
				timelines = readBinaryTimeline1(input, timelines, NewPhysicsConstraintDampingTimeline(frameCount, bezierCount, index), 1)
			case physicsMass:
				timelines = readBinaryTimeline1(input, timelines, NewPhysicsConstraintMassTimeline(frameCount, bezierCount, index), 1)
			case physicsWind:
				timelines = readBinaryTimeline1(input, timelines, NewPhysicsConstraintWindTimeline(frameCount, bezierCount, index), 1)
			case physicsGravity:
				timelines = readBinaryTimeline1(input, timelines, NewPhysicsConstraintGravityTimeline(frameCount, bezierCount, index), 1)
			case physicsMix:
				timelines = readBinaryTimeline1(input, timelines, NewPhysicsConstraintMixTimeline(frameCount, bezierCount, index), 1)
			}
		}
	}

	// Attachment timelines.
	for i, n := 0, input.readInt(true); i < n; i++ {
		skin := skeletonData.Skins[input.readInt(true)]
		for ii, nn := 0, input.readInt(true); ii < nn; ii++ {
			slotIndex := input.readInt(true)
			for iii, nnn := 0, input.readInt(true); iii < nnn; iii++ {
				attachmentName := input.readStringRef()
				attachment := skin.Attachment(slotIndex, attachmentName)
				if attachment == nil {
					return nil, fmt.Errorf("spine: timeline attachment not found: %s", attachmentName)
				}

				timelineType := input.readByte()
				frameCount := input.readInt(true)
				frameLast := frameCount - 1
				switch timelineType {
				case attachmentDeformBin:
					vertexAttachment := AsVertexAttachment(attachment)
					if vertexAttachment == nil {
						return nil, fmt.Errorf("spine: deform timeline attachment is not a vertex attachment: %s", attachmentName)
					}
					weighted := vertexAttachment.Bones != nil
					vertices := vertexAttachment.Vertices
					deformLength := len(vertices)
					if weighted {
						deformLength = len(vertices) / 3 * 2
					}

					timeline := NewDeformTimeline(frameCount, input.readInt(true), slotIndex, attachment)

					time := input.readFloat()
					for frame, bezier := 0, 0; ; frame++ {
						var deform []float32
						end := input.readInt(true)
						if end == 0 {
							if weighted {
								deform = make([]float32, deformLength)
							} else {
								deform = vertices
							}
						} else {
							deform = make([]float32, deformLength)
							start := input.readInt(true)
							end += start
							if scale == 1 {
								for v := start; v < end; v++ {
									deform[v] = input.readFloat()
								}
							} else {
								for v := start; v < end; v++ {
									deform[v] = input.readFloat() * scale
								}
							}
							if !weighted {
								for v := range deform {
									deform[v] += vertices[v]
								}
							}
						}
						timeline.SetFrame(frame, time, deform)
						if frame == frameLast {
							break
						}
						time2 := input.readFloat()
						switch input.readByte() {
						case curveSteppedBin:
							timeline.SetStepped(frame)
						case curveBezierBin:
							setBinaryBezier(input, timeline, bezier, frame, 0, time, time2, 0, 1, 1)
							bezier++
						}
						time = time2
					}
					timelines = append(timelines, timeline)

				case attachmentSequenceBin:
					timeline := NewSequenceTimeline(frameCount, slotIndex, attachment)
					for frame := 0; frame < frameCount; frame++ {
						time := input.readFloat()
						modeAndIndex := int(input.readInt32())
						timeline.SetFrame(frame, time, SequenceMode(modeAndIndex&0xf), modeAndIndex>>4, input.readFloat())
					}
					timelines = append(timelines, timeline)
				}
			}
		}
	}

	// Draw order timeline.
	drawOrderCount := input.readInt(true)
	if drawOrderCount > 0 {
		timeline := NewDrawOrderTimeline(drawOrderCount)
		slotCount := len(skeletonData.Slots)
		for i := 0; i < drawOrderCount; i++ {
			time := input.readFloat()
			offsetCount := input.readInt(true)
			drawOrder := make([]int, slotCount)
			for ii := slotCount - 1; ii >= 0; ii-- {
				drawOrder[ii] = -1
			}
			unchanged := make([]int, slotCount-offsetCount)
			originalIndex, unchangedIndex := 0, 0
			for ii := 0; ii < offsetCount; ii++ {
				slotIndex := input.readInt(true)
				// Collect unchanged items.
				for originalIndex != slotIndex {
					unchanged[unchangedIndex] = originalIndex
					unchangedIndex++
					originalIndex++
				}
				// Set changed items.
				drawOrder[originalIndex+input.readInt(true)] = originalIndex
				originalIndex++
			}
			// Collect remaining unchanged items.
			for originalIndex < slotCount {
				unchanged[unchangedIndex] = originalIndex
				unchangedIndex++
				originalIndex++
			}
			// Fill in unchanged items.
			for ii := slotCount - 1; ii >= 0; ii-- {
				if drawOrder[ii] == -1 {
					unchangedIndex--
					drawOrder[ii] = unchanged[unchangedIndex]
				}
			}
			timeline.SetFrame(i, time, drawOrder)
		}
		timelines = append(timelines, timeline)
	}

	// Event timeline.
	eventCount := input.readInt(true)
	if eventCount > 0 {
		timeline := NewEventTimeline(eventCount)
		for i := 0; i < eventCount; i++ {
			time := input.readFloat()
			eventData := skeletonData.Events[input.readInt(true)]
			event := NewEvent(time, eventData)
			event.Int = input.readInt(false)
			event.Float = input.readFloat()
			if s, ok := input.readStringNullable(); ok {
				event.String = s
			} else {
				event.String = eventData.String
			}
			if eventData.AudioPath != "" {
				event.Volume = input.readFloat()
				event.Balance = input.readFloat()
			}
			timeline.SetFrame(i, event)
		}
		timelines = append(timelines, timeline)
	}

	duration := float32(0)
	for _, timeline := range timelines {
		duration = maxF(duration, timeline.Duration())
	}
	return NewAnimation(name, timelines, duration), nil
}

func readBinaryTimeline1(input *skeletonInput, timelines []Timeline, timeline curveTimeline1, scale float32) []Timeline {
	time := input.readFloat()
	value := input.readFloat() * scale
	for frame, bezier, frameLast := 0, 0, timeline.FrameCount()-1; ; frame++ {
		timeline.SetFrame(frame, time, value)
		if frame == frameLast {
			break
		}
		time2 := input.readFloat()
		value2 := input.readFloat() * scale
		switch input.readByte() {
		case curveSteppedBin:
			timeline.SetStepped(frame)
		case curveBezierBin:
			setBinaryBezier(input, timeline, bezier, frame, 0, time, time2, value, value2, scale)
			bezier++
		}
		time = time2
		value = value2
	}
	return append(timelines, timeline)
}

func readBinaryTimeline2(input *skeletonInput, timelines []Timeline, timeline curveTimeline2, scale float32) []Timeline {
	time := input.readFloat()
	value1 := input.readFloat() * scale
	value2 := input.readFloat() * scale
	for frame, bezier, frameLast := 0, 0, timeline.FrameCount()-1; ; frame++ {
		timeline.SetFrame(frame, time, value1, value2)
		if frame == frameLast {
			break
		}
		time2 := input.readFloat()
		nvalue1 := input.readFloat() * scale
		nvalue2 := input.readFloat() * scale
		switch input.readByte() {
		case curveSteppedBin:
			timeline.SetStepped(frame)
		case curveBezierBin:
			setBinaryBezier(input, timeline, bezier, frame, 0, time, time2, value1, nvalue1, scale)
			bezier++
			setBinaryBezier(input, timeline, bezier, frame, 1, time, time2, value2, nvalue2, scale)
			bezier++
		}
		time = time2
		value1 = nvalue1
		value2 = nvalue2
	}
	return append(timelines, timeline)
}

func setBinaryBezier(input *skeletonInput, timeline curveTimeline, bezier, frame, value int, time1, time2, value1, value2, scale float32) {
	cx1 := input.readFloat()
	cy1 := input.readFloat() * scale
	cx2 := input.readFloat()
	cy2 := input.readFloat() * scale
	timeline.SetBezier(bezier, frame, value, time1, value1, cx1, cy1, cx2, cy2, time2, value2)
}
