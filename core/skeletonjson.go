package spine

import (
	"fmt"
	"io"
	"slices"
	"strconv"
)

// SkeletonJson loads skeleton data in the Spine JSON format.
//
// JSON is human readable but the binary format is much smaller on disk and
// faster to load. See SkeletonBinary.
//
// See "Spine JSON format" and "JSON and binary data" in the Spine Runtimes
// Guide.
type SkeletonJson struct {
	attachmentLoader AttachmentLoader

	// Scale scales bone positions, image sizes, and translations as they are
	// loaded. This allows different size images to be used at runtime than
	// were used in Spine. Must not be 0. Defaults to 1.
	//
	// See "Scaling" in the Spine Runtimes Guide.
	Scale float32

	linkedMeshes []*linkedMesh
}

// NewSkeletonJson creates a skeleton loader that loads attachments using the
// specified attachment loader.
//
// See "Loading skeleton data" in the Spine Runtimes Guide.
func NewSkeletonJson(attachmentLoader AttachmentLoader) *SkeletonJson {
	if attachmentLoader == nil {
		panic("spine: attachmentLoader cannot be nil")
	}
	return &SkeletonJson{attachmentLoader: attachmentLoader, Scale: 1}
}

// ReadSkeletonDataReader reads skeleton data from the specified reader.
func (s *SkeletonJson) ReadSkeletonDataReader(r io.Reader) (*SkeletonData, error) {
	if r == nil {
		return nil, fmt.Errorf("spine: reader cannot be nil")
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("spine: error reading skeleton JSON: %w", err)
	}
	return s.ReadSkeletonData(data)
}

// ReadSkeletonData reads skeleton data from the specified JSON bytes.
func (s *SkeletonJson) ReadSkeletonData(data []byte) (*SkeletonData, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("spine: data cannot be empty")
	}
	if s.Scale == 0 {
		return nil, fmt.Errorf("spine: scale cannot be 0")
	}
	root, err := parseJSON(data)
	if err != nil {
		return nil, err
	}
	return s.readSkeletonData(root)
}

func (s *SkeletonJson) readSkeletonData(root *jsonValue) (_ *SkeletonData, err error) {
	defer catchJSONError(&err)

	scale := s.Scale

	// Skeleton.
	skeletonData := NewSkeletonData()
	if skeletonMap := root.Get("skeleton"); skeletonMap != nil {
		skeletonData.Hash = skeletonMap.GetString("hash", "")
		skeletonData.Version = skeletonMap.GetString("spine", "")
		skeletonData.X = skeletonMap.GetFloat("x", 0)
		skeletonData.Y = skeletonMap.GetFloat("y", 0)
		skeletonData.Width = skeletonMap.GetFloat("width", 0)
		skeletonData.Height = skeletonMap.GetFloat("height", 0)
		skeletonData.ReferenceScale = skeletonMap.GetFloat("referenceScale", 100) * scale
		skeletonData.FPS = skeletonMap.GetFloat("fps", 30)
		skeletonData.ImagesPath = skeletonMap.GetString("images", "")
		skeletonData.AudioPath = skeletonMap.GetString("audio", "")
	}

	// Bones.
	for boneMap := root.GetChild("bones"); boneMap != nil; boneMap = boneMap.Next {
		var parent *BoneData
		parentName := boneMap.GetString("parent", "")
		if parentName != "" {
			parent = skeletonData.FindBone(parentName)
			if parent == nil {
				return nil, fmt.Errorf("spine: parent bone not found: %s", parentName)
			}
		}
		data := NewBoneData(len(skeletonData.Bones), boneMap.RequireString("name"), parent)
		data.Length = boneMap.GetFloat("length", 0) * scale
		data.X = boneMap.GetFloat("x", 0) * scale
		data.Y = boneMap.GetFloat("y", 0) * scale
		data.Rotation = boneMap.GetFloat("rotation", 0)
		data.ScaleX = boneMap.GetFloat("scaleX", 1)
		data.ScaleY = boneMap.GetFloat("scaleY", 1)
		data.ShearX = boneMap.GetFloat("shearX", 0)
		data.ShearY = boneMap.GetFloat("shearY", 0)
		data.Inherit, err = inheritFromString(boneMap.GetString("inherit", "normal"))
		if err != nil {
			return nil, err
		}
		data.SkinRequired = boneMap.GetBoolean("skin", false)

		if color := boneMap.GetString("color", ""); color != "" {
			c, err := ColorFromString(color)
			if err != nil {
				return nil, err
			}
			data.Color = c
		}

		data.Icon = boneMap.GetString("icon", "")
		data.Visible = boneMap.GetBoolean("visible", true)

		skeletonData.Bones = append(skeletonData.Bones, data)
	}

	// Slots.
	for slotMap := root.GetChild("slots"); slotMap != nil; slotMap = slotMap.Next {
		slotName := slotMap.RequireString("name")
		boneName := slotMap.RequireString("bone")
		boneData := skeletonData.FindBone(boneName)
		if boneData == nil {
			return nil, fmt.Errorf("spine: slot bone not found: %s", boneName)
		}

		data := NewSlotData(len(skeletonData.Slots), slotName, boneData)

		if color := slotMap.GetString("color", ""); color != "" {
			c, err := ColorFromString(color)
			if err != nil {
				return nil, err
			}
			data.Color = c
		}

		if dark := slotMap.GetString("dark", ""); dark != "" {
			c, err := ColorFromString(dark)
			if err != nil {
				return nil, err
			}
			data.DarkColor = &c
		}

		data.AttachmentName = slotMap.GetString("attachment", "")
		data.BlendMode, err = blendModeFromString(slotMap.GetString("blend", "normal"))
		if err != nil {
			return nil, err
		}
		data.Visible = slotMap.GetBoolean("visible", true)
		skeletonData.Slots = append(skeletonData.Slots, data)
	}

	// IK constraints.
	for constraintMap := root.GetChild("ik"); constraintMap != nil; constraintMap = constraintMap.Next {
		data := NewIkConstraintData(constraintMap.RequireString("name"))
		data.Order = constraintMap.GetInt("order", 0)
		data.SkinRequired = constraintMap.GetBoolean("skin", false)

		for entry := constraintMap.GetChild("bones"); entry != nil; entry = entry.Next {
			bone := skeletonData.FindBone(entry.AsString())
			if bone == nil {
				return nil, fmt.Errorf("spine: IK bone not found: %s", entry.AsString())
			}
			data.Bones = append(data.Bones, bone)
		}

		targetName := constraintMap.RequireString("target")
		data.Target = skeletonData.FindBone(targetName)
		if data.Target == nil {
			return nil, fmt.Errorf("spine: IK target bone not found: %s", targetName)
		}

		data.Mix = constraintMap.GetFloat("mix", 1)
		data.Softness = constraintMap.GetFloat("softness", 0) * scale
		if constraintMap.GetBoolean("bendPositive", true) {
			data.BendDirection = 1
		} else {
			data.BendDirection = -1
		}
		data.Compress = constraintMap.GetBoolean("compress", false)
		data.Stretch = constraintMap.GetBoolean("stretch", false)
		data.Uniform = constraintMap.GetBoolean("uniform", false)

		skeletonData.IkConstraints = append(skeletonData.IkConstraints, data)
	}

	// Transform constraints.
	for constraintMap := root.GetChild("transform"); constraintMap != nil; constraintMap = constraintMap.Next {
		data := NewTransformConstraintData(constraintMap.RequireString("name"))
		data.Order = constraintMap.GetInt("order", 0)
		data.SkinRequired = constraintMap.GetBoolean("skin", false)

		for entry := constraintMap.GetChild("bones"); entry != nil; entry = entry.Next {
			bone := skeletonData.FindBone(entry.AsString())
			if bone == nil {
				return nil, fmt.Errorf("spine: transform constraint bone not found: %s", entry.AsString())
			}
			data.Bones = append(data.Bones, bone)
		}

		targetName := constraintMap.RequireString("target")
		data.Target = skeletonData.FindBone(targetName)
		if data.Target == nil {
			return nil, fmt.Errorf("spine: transform constraint target bone not found: %s", targetName)
		}

		data.Local = constraintMap.GetBoolean("local", false)
		data.Relative = constraintMap.GetBoolean("relative", false)

		data.OffsetRotation = constraintMap.GetFloat("rotation", 0)
		data.OffsetX = constraintMap.GetFloat("x", 0) * scale
		data.OffsetY = constraintMap.GetFloat("y", 0) * scale
		data.OffsetScaleX = constraintMap.GetFloat("scaleX", 0)
		data.OffsetScaleY = constraintMap.GetFloat("scaleY", 0)
		data.OffsetShearY = constraintMap.GetFloat("shearY", 0)

		data.MixRotate = constraintMap.GetFloat("mixRotate", 1)
		data.MixX = constraintMap.GetFloat("mixX", 1)
		data.MixY = constraintMap.GetFloat("mixY", data.MixX)
		data.MixScaleX = constraintMap.GetFloat("mixScaleX", 1)
		data.MixScaleY = constraintMap.GetFloat("mixScaleY", data.MixScaleX)
		data.MixShearY = constraintMap.GetFloat("mixShearY", 1)

		skeletonData.TransformConstraints = append(skeletonData.TransformConstraints, data)
	}

	// Path constraints.
	for constraintMap := root.GetChild("path"); constraintMap != nil; constraintMap = constraintMap.Next {
		data := NewPathConstraintData(constraintMap.RequireString("name"))
		data.Order = constraintMap.GetInt("order", 0)
		data.SkinRequired = constraintMap.GetBoolean("skin", false)

		for entry := constraintMap.GetChild("bones"); entry != nil; entry = entry.Next {
			bone := skeletonData.FindBone(entry.AsString())
			if bone == nil {
				return nil, fmt.Errorf("spine: path bone not found: %s", entry.AsString())
			}
			data.Bones = append(data.Bones, bone)
		}

		targetName := constraintMap.RequireString("target")
		data.Target = skeletonData.FindSlot(targetName)
		if data.Target == nil {
			return nil, fmt.Errorf("spine: path target slot not found: %s", targetName)
		}

		data.PositionMode, err = positionModeFromString(constraintMap.GetString("positionMode", "percent"))
		if err != nil {
			return nil, err
		}
		data.SpacingMode, err = spacingModeFromString(constraintMap.GetString("spacingMode", "length"))
		if err != nil {
			return nil, err
		}
		data.RotateMode, err = rotateModeFromString(constraintMap.GetString("rotateMode", "tangent"))
		if err != nil {
			return nil, err
		}
		data.OffsetRotation = constraintMap.GetFloat("rotation", 0)
		data.Position = constraintMap.GetFloat("position", 0)
		if data.PositionMode == PositionModeFixed {
			data.Position *= scale
		}
		data.Spacing = constraintMap.GetFloat("spacing", 0)
		if data.SpacingMode == SpacingModeLength || data.SpacingMode == SpacingModeFixed {
			data.Spacing *= scale
		}
		data.MixRotate = constraintMap.GetFloat("mixRotate", 1)
		data.MixX = constraintMap.GetFloat("mixX", 1)
		data.MixY = constraintMap.GetFloat("mixY", 1)

		skeletonData.PathConstraints = append(skeletonData.PathConstraints, data)
	}

	// Physics constraints.
	for constraintMap := root.GetChild("physics"); constraintMap != nil; constraintMap = constraintMap.Next {
		data := NewPhysicsConstraintData(constraintMap.RequireString("name"))
		data.Order = constraintMap.GetInt("order", 0)
		data.SkinRequired = constraintMap.GetBoolean("skin", false)

		boneName := constraintMap.RequireString("bone")
		data.Bone = skeletonData.FindBone(boneName)
		if data.Bone == nil {
			return nil, fmt.Errorf("spine: physics bone not found: %s", boneName)
		}

		data.X = constraintMap.GetFloat("x", 0)
		data.Y = constraintMap.GetFloat("y", 0)
		data.Rotate = constraintMap.GetFloat("rotate", 0)
		data.ScaleX = constraintMap.GetFloat("scaleX", 0)
		data.ShearX = constraintMap.GetFloat("shearX", 0)
		data.Limit = constraintMap.GetFloat("limit", 5000) * scale
		data.Step = 1 / float32(constraintMap.GetInt("fps", 60))
		data.Inertia = constraintMap.GetFloat("inertia", 1)
		data.Strength = constraintMap.GetFloat("strength", 100)
		data.Damping = constraintMap.GetFloat("damping", 1)
		data.MassInverse = 1 / constraintMap.GetFloat("mass", 1)
		data.Wind = constraintMap.GetFloat("wind", 0)
		data.Gravity = constraintMap.GetFloat("gravity", 0)
		data.Mix = constraintMap.GetFloat("mix", 1)
		data.InertiaGlobal = constraintMap.GetBoolean("inertiaGlobal", false)
		data.StrengthGlobal = constraintMap.GetBoolean("strengthGlobal", false)
		data.DampingGlobal = constraintMap.GetBoolean("dampingGlobal", false)
		data.MassGlobal = constraintMap.GetBoolean("massGlobal", false)
		data.WindGlobal = constraintMap.GetBoolean("windGlobal", false)
		data.GravityGlobal = constraintMap.GetBoolean("gravityGlobal", false)
		data.MixGlobal = constraintMap.GetBoolean("mixGlobal", false)

		skeletonData.PhysicsConstraints = append(skeletonData.PhysicsConstraints, data)
	}

	// Skins.
	for skinMap := root.GetChild("skins"); skinMap != nil; skinMap = skinMap.Next {
		skin := NewSkin(skinMap.RequireString("name"))
		for entry := skinMap.GetChild("bones"); entry != nil; entry = entry.Next {
			bone := skeletonData.FindBone(entry.AsString())
			if bone == nil {
				return nil, fmt.Errorf("spine: skin bone not found: %s", entry.AsString())
			}
			skin.Bones = append(skin.Bones, bone)
		}
		for entry := skinMap.GetChild("ik"); entry != nil; entry = entry.Next {
			constraint := skeletonData.FindIkConstraint(entry.AsString())
			if constraint == nil {
				return nil, fmt.Errorf("spine: skin IK constraint not found: %s", entry.AsString())
			}
			skin.Constraints = append(skin.Constraints, constraint)
		}
		for entry := skinMap.GetChild("transform"); entry != nil; entry = entry.Next {
			constraint := skeletonData.FindTransformConstraint(entry.AsString())
			if constraint == nil {
				return nil, fmt.Errorf("spine: skin transform constraint not found: %s", entry.AsString())
			}
			skin.Constraints = append(skin.Constraints, constraint)
		}
		for entry := skinMap.GetChild("path"); entry != nil; entry = entry.Next {
			constraint := skeletonData.FindPathConstraint(entry.AsString())
			if constraint == nil {
				return nil, fmt.Errorf("spine: skin path constraint not found: %s", entry.AsString())
			}
			skin.Constraints = append(skin.Constraints, constraint)
		}
		for entry := skinMap.GetChild("physics"); entry != nil; entry = entry.Next {
			constraint := skeletonData.FindPhysicsConstraint(entry.AsString())
			if constraint == nil {
				return nil, fmt.Errorf("spine: skin physics constraint not found: %s", entry.AsString())
			}
			skin.Constraints = append(skin.Constraints, constraint)
		}
		for slotEntry := skinMap.GetChild("attachments"); slotEntry != nil; slotEntry = slotEntry.Next {
			slot := skeletonData.FindSlot(slotEntry.Name)
			if slot == nil {
				return nil, fmt.Errorf("spine: slot not found: %s", slotEntry.Name)
			}
			for entry := slotEntry.Child; entry != nil; entry = entry.Next {
				attachment, err := s.readAttachment(entry, skin, slot.Index, entry.Name, skeletonData)
				if err != nil {
					return nil, fmt.Errorf("spine: error reading attachment: %s, skin: %s: %w", entry.Name, skin.Name, err)
				}
				if attachment != nil {
					skin.SetAttachment(slot.Index, entry.Name, attachment)
				}
			}
		}

		if color := skinMap.GetString("color", ""); color != "" {
			c, err := ColorFromString(color)
			if err != nil {
				return nil, err
			}
			skin.Color = c
		}

		skeletonData.Skins = append(skeletonData.Skins, skin)
		if skin.Name == "default" {
			skeletonData.DefaultSkin = skin
		}
	}

	// Linked meshes.
	for _, lm := range s.linkedMeshes {
		skin := skeletonData.DefaultSkin
		if lm.skin != "" {
			skin = skeletonData.FindSkin(lm.skin)
		}
		if skin == nil {
			return nil, fmt.Errorf("spine: skin not found: %s", lm.skin)
		}
		parent := skin.Attachment(lm.slotIndex, lm.parent)
		if parent == nil {
			return nil, fmt.Errorf("spine: parent mesh not found: %s", lm.parent)
		}
		parentMesh, ok := parent.(*MeshAttachment)
		if !ok {
			return nil, fmt.Errorf("spine: parent mesh must be a mesh attachment: %s", lm.parent)
		}
		if lm.inheritTimelines {
			lm.mesh.TimelineAttachment = parent
		} else {
			lm.mesh.TimelineAttachment = lm.mesh
		}
		lm.mesh.SetParentMesh(parentMesh)
		if lm.mesh.Region() != nil {
			lm.mesh.UpdateRegion()
		}
	}
	s.linkedMeshes = s.linkedMeshes[:0]

	// Events.
	for eventMap := root.GetChild("events"); eventMap != nil; eventMap = eventMap.Next {
		data := NewEventData(eventMap.Name)
		data.Int = eventMap.GetInt("int", 0)
		data.Float = eventMap.GetFloat("float", 0)
		data.String = eventMap.GetString("string", "")
		data.AudioPath = eventMap.GetString("audio", "")
		if data.AudioPath != "" {
			data.Volume = eventMap.GetFloat("volume", 1)
			data.Balance = eventMap.GetFloat("balance", 0)
		}
		skeletonData.Events = append(skeletonData.Events, data)
	}

	// Animations.
	for animationMap := root.GetChild("animations"); animationMap != nil; animationMap = animationMap.Next {
		if err := s.readAnimation(animationMap, animationMap.Name, skeletonData); err != nil {
			return nil, fmt.Errorf("spine: error reading animation: %s: %w", animationMap.Name, err)
		}
	}

	return skeletonData, nil
}

func (s *SkeletonJson) readAttachment(m *jsonValue, skin *Skin, slotIndex int, name string, skeletonData *SkeletonData) (_ Attachment, err error) {
	defer catchJSONError(&err)
	scale := s.Scale
	name = m.GetString("name", name)

	switch typeName := m.GetString("type", "region"); typeName {
	case "region":
		path := m.GetString("path", name)
		sequence := readSequence(m.Get("sequence"))
		region, err := s.attachmentLoader.NewRegionAttachment(skin, name, path, sequence)
		if err != nil {
			return nil, err
		}
		if region == nil {
			return nil, nil
		}
		region.Path = path
		region.X = m.GetFloat("x", 0) * scale
		region.Y = m.GetFloat("y", 0) * scale
		region.ScaleX = m.GetFloat("scaleX", 1)
		region.ScaleY = m.GetFloat("scaleY", 1)
		region.Rotation = m.GetFloat("rotation", 0)
		region.Width = m.RequireFloat("width") * scale
		region.Height = m.RequireFloat("height") * scale
		region.Sequence = sequence

		if color := m.GetString("color", ""); color != "" {
			c, err := ColorFromString(color)
			if err != nil {
				return nil, err
			}
			region.Color = c
		}

		if region.Region() != nil {
			region.UpdateRegion()
		}
		return region, nil

	case "boundingbox":
		box, err := s.attachmentLoader.NewBoundingBoxAttachment(skin, name)
		if err != nil {
			return nil, err
		}
		if box == nil {
			return nil, nil
		}
		s.readVertices(m, &box.VertexAttachment, m.RequireInt("vertexCount")<<1)

		if color := m.GetString("color", ""); color != "" {
			c, err := ColorFromString(color)
			if err != nil {
				return nil, err
			}
			box.Color = c
		}
		return box, nil

	case "mesh", "linkedmesh":
		path := m.GetString("path", name)
		sequence := readSequence(m.Get("sequence"))
		mesh, err := s.attachmentLoader.NewMeshAttachment(skin, name, path, sequence)
		if err != nil {
			return nil, err
		}
		if mesh == nil {
			return nil, nil
		}
		mesh.Path = path

		if color := m.GetString("color", ""); color != "" {
			c, err := ColorFromString(color)
			if err != nil {
				return nil, err
			}
			mesh.Color = c
		}

		mesh.Width = m.GetFloat("width", 0) * scale
		mesh.Height = m.GetFloat("height", 0) * scale
		mesh.Sequence = sequence

		if parent := m.GetString("parent", ""); parent != "" {
			s.linkedMeshes = append(s.linkedMeshes, &linkedMesh{
				mesh:             mesh,
				skin:             m.GetString("skin", ""),
				slotIndex:        slotIndex,
				parent:           parent,
				inheritTimelines: m.GetBoolean("timelines", true),
			})
			return mesh, nil
		}

		uvs := m.Require("uvs").AsFloatArray()
		s.readVertices(m, &mesh.VertexAttachment, len(uvs))
		mesh.Triangles = m.Require("triangles").AsShortArray()
		mesh.RegionUVs = uvs
		if mesh.Region() != nil {
			mesh.UpdateRegion()
		}

		if m.Has("hull") {
			mesh.HullLength = m.Require("hull").AsInt() << 1
		}
		if m.Has("edges") {
			mesh.Edges = m.Require("edges").AsShortArray()
		}
		return mesh, nil

	case "path":
		path, err := s.attachmentLoader.NewPathAttachment(skin, name)
		if err != nil {
			return nil, err
		}
		if path == nil {
			return nil, nil
		}
		path.Closed = m.GetBoolean("closed", false)
		path.ConstantSpeed = m.GetBoolean("constantSpeed", true)

		vertexCount := m.RequireInt("vertexCount")
		s.readVertices(m, &path.VertexAttachment, vertexCount<<1)

		lengths := make([]float32, vertexCount/3)
		i := 0
		for curves := m.Require("lengths").Child; curves != nil; curves = curves.Next {
			lengths[i] = curves.AsFloat() * scale
			i++
		}
		path.Lengths = lengths

		if color := m.GetString("color", ""); color != "" {
			c, err := ColorFromString(color)
			if err != nil {
				return nil, err
			}
			path.Color = c
		}
		return path, nil

	case "point":
		point, err := s.attachmentLoader.NewPointAttachment(skin, name)
		if err != nil {
			return nil, err
		}
		if point == nil {
			return nil, nil
		}
		point.X = m.GetFloat("x", 0) * scale
		point.Y = m.GetFloat("y", 0) * scale
		point.Rotation = m.GetFloat("rotation", 0)

		if color := m.GetString("color", ""); color != "" {
			c, err := ColorFromString(color)
			if err != nil {
				return nil, err
			}
			point.Color = c
		}
		return point, nil

	case "clipping":
		clip, err := s.attachmentLoader.NewClippingAttachment(skin, name)
		if err != nil {
			return nil, err
		}
		if clip == nil {
			return nil, nil
		}

		if end := m.GetString("end", ""); end != "" {
			slot := skeletonData.FindSlot(end)
			if slot == nil {
				return nil, fmt.Errorf("spine: clipping end slot not found: %s", end)
			}
			clip.EndSlot = slot
		}

		s.readVertices(m, &clip.VertexAttachment, m.RequireInt("vertexCount")<<1)

		if color := m.GetString("color", ""); color != "" {
			c, err := ColorFromString(color)
			if err != nil {
				return nil, err
			}
			clip.Color = c
		}
		return clip, nil

	default:
		return nil, fmt.Errorf("spine: unknown attachment type: %s", typeName)
	}
}

func readSequence(m *jsonValue) *Sequence {
	if m == nil {
		return nil
	}
	sequence := NewSequence(m.RequireInt("count"))
	sequence.Start = m.GetInt("start", 1)
	sequence.Digits = m.GetInt("digits", 0)
	sequence.SetupIndex = m.GetInt("setup", 0)
	return sequence
}

func (s *SkeletonJson) readVertices(m *jsonValue, attachment *VertexAttachment, verticesLength int) {
	scale := s.Scale
	attachment.WorldVerticesLength = verticesLength
	vertices := m.Require("vertices").AsFloatArray()
	if verticesLength == len(vertices) {
		if scale != 1 {
			for i, n := 0, len(vertices); i < n; i++ {
				vertices[i] *= scale
			}
		}
		attachment.Vertices = vertices
		return
	}
	weights := make([]float32, 0, verticesLength*3*3)
	bones := make([]int32, 0, verticesLength*3)
	for i, n := 0, len(vertices); i < n; {
		boneCount := int(vertices[i])
		i++
		bones = append(bones, int32(boneCount))
		for nn := i + (boneCount << 2); i < nn; i += 4 {
			bones = append(bones, int32(vertices[i]))
			weights = append(weights, vertices[i+1]*scale, vertices[i+2]*scale, vertices[i+3])
		}
	}
	attachment.Bones = bones
	attachment.Vertices = weights
}

func (s *SkeletonJson) readAnimation(m *jsonValue, name string, skeletonData *SkeletonData) (err error) {
	defer catchJSONError(&err)
	scale := s.Scale
	var timelines []Timeline

	// Slot timelines.
	for slotMap := m.GetChild("slots"); slotMap != nil; slotMap = slotMap.Next {
		slot := skeletonData.FindSlot(slotMap.Name)
		if slot == nil {
			return fmt.Errorf("spine: slot not found: %s", slotMap.Name)
		}
		for timelineMap := slotMap.Child; timelineMap != nil; timelineMap = timelineMap.Next {
			keyMap := timelineMap.Child
			if keyMap == nil {
				continue
			}

			frames := timelineMap.Size
			switch timelineName := timelineMap.Name; timelineName {
			case "attachment":
				timeline := NewAttachmentTimeline(frames, slot.Index)
				for frame := 0; keyMap != nil; keyMap, frame = keyMap.Next, frame+1 {
					timeline.SetFrame(frame, keyMap.GetFloat("time", 0), keyMap.GetString("name", ""))
				}
				timelines = append(timelines, timeline)

			case "rgba":
				timeline := NewRGBATimeline(frames, frames<<2, slot.Index)
				time := keyMap.GetFloat("time", 0)
				color := keyMap.RequireString("color")
				r := hexComponent(color, 0)
				g := hexComponent(color, 2)
				b := hexComponent(color, 4)
				a := hexComponent(color, 6)
				for frame, bezier := 0, 0; ; frame++ {
					timeline.SetFrame(frame, time, r, g, b, a)
					nextMap := keyMap.Next
					if nextMap == nil {
						timeline.Shrink(bezier)
						break
					}
					time2 := nextMap.GetFloat("time", 0)
					color = nextMap.RequireString("color")
					nr := hexComponent(color, 0)
					ng := hexComponent(color, 2)
					nb := hexComponent(color, 4)
					na := hexComponent(color, 6)
					if curve := keyMap.Get("curve"); curve != nil {
						bezier = readCurve(curve, timeline, bezier, frame, 0, time, time2, r, nr, 1)
						bezier = readCurve(curve, timeline, bezier, frame, 1, time, time2, g, ng, 1)
						bezier = readCurve(curve, timeline, bezier, frame, 2, time, time2, b, nb, 1)
						bezier = readCurve(curve, timeline, bezier, frame, 3, time, time2, a, na, 1)
					}
					time = time2
					r = nr
					g = ng
					b = nb
					a = na
					keyMap = nextMap
				}
				timelines = append(timelines, timeline)

			case "rgb":
				timeline := NewRGBTimeline(frames, frames*3, slot.Index)
				time := keyMap.GetFloat("time", 0)
				color := keyMap.RequireString("color")
				r := hexComponent(color, 0)
				g := hexComponent(color, 2)
				b := hexComponent(color, 4)
				for frame, bezier := 0, 0; ; frame++ {
					timeline.SetFrame(frame, time, r, g, b)
					nextMap := keyMap.Next
					if nextMap == nil {
						timeline.Shrink(bezier)
						break
					}
					time2 := nextMap.GetFloat("time", 0)
					color = nextMap.RequireString("color")
					nr := hexComponent(color, 0)
					ng := hexComponent(color, 2)
					nb := hexComponent(color, 4)
					if curve := keyMap.Get("curve"); curve != nil {
						bezier = readCurve(curve, timeline, bezier, frame, 0, time, time2, r, nr, 1)
						bezier = readCurve(curve, timeline, bezier, frame, 1, time, time2, g, ng, 1)
						bezier = readCurve(curve, timeline, bezier, frame, 2, time, time2, b, nb, 1)
					}
					time = time2
					r = nr
					g = ng
					b = nb
					keyMap = nextMap
				}
				timelines = append(timelines, timeline)

			case "alpha":
				timelines = append(timelines, readTimeline1(keyMap, NewAlphaTimeline(frames, frames, slot.Index), 0, 1))

			case "rgba2":
				timeline := NewRGBA2Timeline(frames, frames*7, slot.Index)
				time := keyMap.GetFloat("time", 0)
				color := keyMap.RequireString("light")
				r := hexComponent(color, 0)
				g := hexComponent(color, 2)
				b := hexComponent(color, 4)
				a := hexComponent(color, 6)
				color = keyMap.RequireString("dark")
				r2 := hexComponent(color, 0)
				g2 := hexComponent(color, 2)
				b2 := hexComponent(color, 4)
				for frame, bezier := 0, 0; ; frame++ {
					timeline.SetFrame(frame, time, r, g, b, a, r2, g2, b2)
					nextMap := keyMap.Next
					if nextMap == nil {
						timeline.Shrink(bezier)
						break
					}
					time2 := nextMap.GetFloat("time", 0)
					color = nextMap.RequireString("light")
					nr := hexComponent(color, 0)
					ng := hexComponent(color, 2)
					nb := hexComponent(color, 4)
					na := hexComponent(color, 6)
					color = nextMap.RequireString("dark")
					nr2 := hexComponent(color, 0)
					ng2 := hexComponent(color, 2)
					nb2 := hexComponent(color, 4)
					if curve := keyMap.Get("curve"); curve != nil {
						bezier = readCurve(curve, timeline, bezier, frame, 0, time, time2, r, nr, 1)
						bezier = readCurve(curve, timeline, bezier, frame, 1, time, time2, g, ng, 1)
						bezier = readCurve(curve, timeline, bezier, frame, 2, time, time2, b, nb, 1)
						bezier = readCurve(curve, timeline, bezier, frame, 3, time, time2, a, na, 1)
						bezier = readCurve(curve, timeline, bezier, frame, 4, time, time2, r2, nr2, 1)
						bezier = readCurve(curve, timeline, bezier, frame, 5, time, time2, g2, ng2, 1)
						bezier = readCurve(curve, timeline, bezier, frame, 6, time, time2, b2, nb2, 1)
					}
					time = time2
					r = nr
					g = ng
					b = nb
					a = na
					r2 = nr2
					g2 = ng2
					b2 = nb2
					keyMap = nextMap
				}
				timelines = append(timelines, timeline)

			case "rgb2":
				timeline := NewRGB2Timeline(frames, frames*6, slot.Index)
				time := keyMap.GetFloat("time", 0)
				color := keyMap.RequireString("light")
				r := hexComponent(color, 0)
				g := hexComponent(color, 2)
				b := hexComponent(color, 4)
				color = keyMap.RequireString("dark")
				r2 := hexComponent(color, 0)
				g2 := hexComponent(color, 2)
				b2 := hexComponent(color, 4)
				for frame, bezier := 0, 0; ; frame++ {
					timeline.SetFrame(frame, time, r, g, b, r2, g2, b2)
					nextMap := keyMap.Next
					if nextMap == nil {
						timeline.Shrink(bezier)
						break
					}
					time2 := nextMap.GetFloat("time", 0)
					color = nextMap.RequireString("light")
					nr := hexComponent(color, 0)
					ng := hexComponent(color, 2)
					nb := hexComponent(color, 4)
					color = nextMap.RequireString("dark")
					nr2 := hexComponent(color, 0)
					ng2 := hexComponent(color, 2)
					nb2 := hexComponent(color, 4)
					if curve := keyMap.Get("curve"); curve != nil {
						bezier = readCurve(curve, timeline, bezier, frame, 0, time, time2, r, nr, 1)
						bezier = readCurve(curve, timeline, bezier, frame, 1, time, time2, g, ng, 1)
						bezier = readCurve(curve, timeline, bezier, frame, 2, time, time2, b, nb, 1)
						bezier = readCurve(curve, timeline, bezier, frame, 3, time, time2, r2, nr2, 1)
						bezier = readCurve(curve, timeline, bezier, frame, 4, time, time2, g2, ng2, 1)
						bezier = readCurve(curve, timeline, bezier, frame, 5, time, time2, b2, nb2, 1)
					}
					time = time2
					r = nr
					g = ng
					b = nb
					r2 = nr2
					g2 = ng2
					b2 = nb2
					keyMap = nextMap
				}
				timelines = append(timelines, timeline)

			default:
				return fmt.Errorf("spine: invalid timeline type for a slot: %s (%s)", timelineName, slotMap.Name)
			}
		}
	}

	// Bone timelines.
	for boneMap := m.GetChild("bones"); boneMap != nil; boneMap = boneMap.Next {
		bone := skeletonData.FindBone(boneMap.Name)
		if bone == nil {
			return fmt.Errorf("spine: bone not found: %s", boneMap.Name)
		}
		for timelineMap := boneMap.Child; timelineMap != nil; timelineMap = timelineMap.Next {
			keyMap := timelineMap.Child
			if keyMap == nil {
				continue
			}

			frames := timelineMap.Size
			switch timelineName := timelineMap.Name; timelineName {
			case "rotate":
				timelines = append(timelines, readTimeline1(keyMap, NewRotateTimeline(frames, frames, bone.Index), 0, 1))
			case "translate":
				timeline := NewTranslateTimeline(frames, frames<<1, bone.Index)
				timelines = append(timelines, readTimeline2(keyMap, timeline, "x", "y", 0, scale))
			case "translatex":
				timelines = append(timelines, readTimeline1(keyMap, NewTranslateXTimeline(frames, frames, bone.Index), 0, scale))
			case "translatey":
				timelines = append(timelines, readTimeline1(keyMap, NewTranslateYTimeline(frames, frames, bone.Index), 0, scale))
			case "scale":
				timeline := NewScaleTimeline(frames, frames<<1, bone.Index)
				timelines = append(timelines, readTimeline2(keyMap, timeline, "x", "y", 1, 1))
			case "scalex":
				timelines = append(timelines, readTimeline1(keyMap, NewScaleXTimeline(frames, frames, bone.Index), 1, 1))
			case "scaley":
				timelines = append(timelines, readTimeline1(keyMap, NewScaleYTimeline(frames, frames, bone.Index), 1, 1))
			case "shear":
				timeline := NewShearTimeline(frames, frames<<1, bone.Index)
				timelines = append(timelines, readTimeline2(keyMap, timeline, "x", "y", 0, 1))
			case "shearx":
				timelines = append(timelines, readTimeline1(keyMap, NewShearXTimeline(frames, frames, bone.Index), 0, 1))
			case "sheary":
				timelines = append(timelines, readTimeline1(keyMap, NewShearYTimeline(frames, frames, bone.Index), 0, 1))
			case "inherit":
				timeline := NewInheritTimeline(frames, bone.Index)
				for frame := 0; keyMap != nil; keyMap, frame = keyMap.Next, frame+1 {
					time := keyMap.GetFloat("time", 0)
					inherit, err := inheritFromString(keyMap.GetString("inherit", "normal"))
					if err != nil {
						return err
					}
					timeline.SetFrame(frame, time, inherit)
				}
				timelines = append(timelines, timeline)
			default:
				return fmt.Errorf("spine: invalid timeline type for a bone: %s (%s)", timelineName, boneMap.Name)
			}
		}
	}

	// IK constraint timelines.
	for timelineMap := m.GetChild("ik"); timelineMap != nil; timelineMap = timelineMap.Next {
		keyMap := timelineMap.Child
		if keyMap == nil {
			continue
		}
		constraint := skeletonData.FindIkConstraint(timelineMap.Name)
		timeline := NewIkConstraintTimeline(timelineMap.Size, timelineMap.Size<<1,
			slices.Index(skeletonData.IkConstraints, constraint))
		time := keyMap.GetFloat("time", 0)
		mix, softness := keyMap.GetFloat("mix", 1), keyMap.GetFloat("softness", 0)*scale
		for frame, bezier := 0, 0; ; frame++ {
			bendDirection := 1
			if !keyMap.GetBoolean("bendPositive", true) {
				bendDirection = -1
			}
			timeline.SetFrame(frame, time, mix, softness, bendDirection,
				keyMap.GetBoolean("compress", false), keyMap.GetBoolean("stretch", false))
			nextMap := keyMap.Next
			if nextMap == nil {
				timeline.Shrink(bezier)
				break
			}
			time2 := nextMap.GetFloat("time", 0)
			mix2, softness2 := nextMap.GetFloat("mix", 1), nextMap.GetFloat("softness", 0)*scale
			if curve := keyMap.Get("curve"); curve != nil {
				bezier = readCurve(curve, timeline, bezier, frame, 0, time, time2, mix, mix2, 1)
				bezier = readCurve(curve, timeline, bezier, frame, 1, time, time2, softness, softness2, scale)
			}
			time = time2
			mix = mix2
			softness = softness2
			keyMap = nextMap
		}
		timelines = append(timelines, timeline)
	}

	// Transform constraint timelines.
	for timelineMap := m.GetChild("transform"); timelineMap != nil; timelineMap = timelineMap.Next {
		keyMap := timelineMap.Child
		if keyMap == nil {
			continue
		}
		constraint := skeletonData.FindTransformConstraint(timelineMap.Name)
		timeline := NewTransformConstraintTimeline(timelineMap.Size, timelineMap.Size*6,
			slices.Index(skeletonData.TransformConstraints, constraint))
		time := keyMap.GetFloat("time", 0)
		mixRotate := keyMap.GetFloat("mixRotate", 1)
		mixX := keyMap.GetFloat("mixX", 1)
		mixY := keyMap.GetFloat("mixY", mixX)
		mixScaleX := keyMap.GetFloat("mixScaleX", 1)
		mixScaleY := keyMap.GetFloat("mixScaleY", mixScaleX)
		mixShearY := keyMap.GetFloat("mixShearY", 1)
		for frame, bezier := 0, 0; ; frame++ {
			timeline.SetFrame(frame, time, mixRotate, mixX, mixY, mixScaleX, mixScaleY, mixShearY)
			nextMap := keyMap.Next
			if nextMap == nil {
				timeline.Shrink(bezier)
				break
			}
			time2 := nextMap.GetFloat("time", 0)
			mixRotate2 := nextMap.GetFloat("mixRotate", 1)
			mixX2 := nextMap.GetFloat("mixX", 1)
			mixY2 := nextMap.GetFloat("mixY", mixX2)
			mixScaleX2 := nextMap.GetFloat("mixScaleX", 1)
			mixScaleY2 := nextMap.GetFloat("mixScaleY", mixScaleX2)
			mixShearY2 := nextMap.GetFloat("mixShearY", 1)
			if curve := keyMap.Get("curve"); curve != nil {
				bezier = readCurve(curve, timeline, bezier, frame, 0, time, time2, mixRotate, mixRotate2, 1)
				bezier = readCurve(curve, timeline, bezier, frame, 1, time, time2, mixX, mixX2, 1)
				bezier = readCurve(curve, timeline, bezier, frame, 2, time, time2, mixY, mixY2, 1)
				bezier = readCurve(curve, timeline, bezier, frame, 3, time, time2, mixScaleX, mixScaleX2, 1)
				bezier = readCurve(curve, timeline, bezier, frame, 4, time, time2, mixScaleY, mixScaleY2, 1)
				bezier = readCurve(curve, timeline, bezier, frame, 5, time, time2, mixShearY, mixShearY2, 1)
			}
			time = time2
			mixRotate = mixRotate2
			mixX = mixX2
			mixY = mixY2
			mixScaleX = mixScaleX2
			mixScaleY = mixScaleY2
			mixShearY = mixShearY2
			keyMap = nextMap
		}
		timelines = append(timelines, timeline)
	}

	// Path constraint timelines.
	for constraintMap := m.GetChild("path"); constraintMap != nil; constraintMap = constraintMap.Next {
		constraint := skeletonData.FindPathConstraint(constraintMap.Name)
		if constraint == nil {
			return fmt.Errorf("spine: path constraint not found: %s", constraintMap.Name)
		}
		index := slices.Index(skeletonData.PathConstraints, constraint)
		for timelineMap := constraintMap.Child; timelineMap != nil; timelineMap = timelineMap.Next {
			keyMap := timelineMap.Child
			if keyMap == nil {
				continue
			}

			frames := timelineMap.Size
			switch timelineName := timelineMap.Name; timelineName {
			case "position":
				timeline := NewPathConstraintPositionTimeline(frames, frames, index)
				timelineScale := float32(1)
				if constraint.PositionMode == PositionModeFixed {
					timelineScale = scale
				}
				timelines = append(timelines, readTimeline1(keyMap, timeline, 0, timelineScale))
			case "spacing":
				timeline := NewPathConstraintSpacingTimeline(frames, frames, index)
				timelineScale := float32(1)
				if constraint.SpacingMode == SpacingModeLength || constraint.SpacingMode == SpacingModeFixed {
					timelineScale = scale
				}
				timelines = append(timelines, readTimeline1(keyMap, timeline, 0, timelineScale))
			case "mix":
				timeline := NewPathConstraintMixTimeline(frames, frames*3, index)
				time := keyMap.GetFloat("time", 0)
				mixRotate := keyMap.GetFloat("mixRotate", 1)
				mixX := keyMap.GetFloat("mixX", 1)
				mixY := keyMap.GetFloat("mixY", mixX)
				for frame, bezier := 0, 0; ; frame++ {
					timeline.SetFrame(frame, time, mixRotate, mixX, mixY)
					nextMap := keyMap.Next
					if nextMap == nil {
						timeline.Shrink(bezier)
						break
					}
					time2 := nextMap.GetFloat("time", 0)
					mixRotate2 := nextMap.GetFloat("mixRotate", 1)
					mixX2 := nextMap.GetFloat("mixX", 1)
					mixY2 := nextMap.GetFloat("mixY", mixX2)
					if curve := keyMap.Get("curve"); curve != nil {
						bezier = readCurve(curve, timeline, bezier, frame, 0, time, time2, mixRotate, mixRotate2, 1)
						bezier = readCurve(curve, timeline, bezier, frame, 1, time, time2, mixX, mixX2, 1)
						bezier = readCurve(curve, timeline, bezier, frame, 2, time, time2, mixY, mixY2, 1)
					}
					time = time2
					mixRotate = mixRotate2
					mixX = mixX2
					mixY = mixY2
					keyMap = nextMap
				}
				timelines = append(timelines, timeline)
			}
		}
	}

	// Physics constraint timelines.
	for constraintMap := m.GetChild("physics"); constraintMap != nil; constraintMap = constraintMap.Next {
		index := -1
		if constraintMap.Name != "" {
			constraint := skeletonData.FindPhysicsConstraint(constraintMap.Name)
			if constraint == nil {
				return fmt.Errorf("spine: physics constraint not found: %s", constraintMap.Name)
			}
			index = slices.Index(skeletonData.PhysicsConstraints, constraint)
		}
		for timelineMap := constraintMap.Child; timelineMap != nil; timelineMap = timelineMap.Next {
			keyMap := timelineMap.Child
			if keyMap == nil {
				continue
			}

			frames := timelineMap.Size
			timelineName := timelineMap.Name
			if timelineName == "reset" {
				timeline := NewPhysicsConstraintResetTimeline(frames, index)
				for frame := 0; keyMap != nil; keyMap, frame = keyMap.Next, frame+1 {
					timeline.SetFrame(frame, keyMap.GetFloat("time", 0))
				}
				timelines = append(timelines, timeline)
				continue
			}

			var timeline curveTimeline1
			switch timelineName {
			case "inertia":
				timeline = NewPhysicsConstraintInertiaTimeline(frames, frames, index)
			case "strength":
				timeline = NewPhysicsConstraintStrengthTimeline(frames, frames, index)
			case "damping":
				timeline = NewPhysicsConstraintDampingTimeline(frames, frames, index)
			case "mass":
				timeline = NewPhysicsConstraintMassTimeline(frames, frames, index)
			case "wind":
				timeline = NewPhysicsConstraintWindTimeline(frames, frames, index)
			case "gravity":
				timeline = NewPhysicsConstraintGravityTimeline(frames, frames, index)
			case "mix":
				timeline = NewPhysicsConstraintMixTimeline(frames, frames, index)
			default:
				continue
			}
			timelines = append(timelines, readTimeline1(keyMap, timeline, 0, 1))
		}
	}

	// Attachment timelines.
	for attachmentsMap := m.GetChild("attachments"); attachmentsMap != nil; attachmentsMap = attachmentsMap.Next {
		skin := skeletonData.FindSkin(attachmentsMap.Name)
		if skin == nil {
			return fmt.Errorf("spine: skin not found: %s", attachmentsMap.Name)
		}
		for slotMap := attachmentsMap.Child; slotMap != nil; slotMap = slotMap.Next {
			slot := skeletonData.FindSlot(slotMap.Name)
			if slot == nil {
				return fmt.Errorf("spine: slot not found: %s", slotMap.Name)
			}
			for attachmentMap := slotMap.Child; attachmentMap != nil; attachmentMap = attachmentMap.Next {
				attachment := skin.Attachment(slot.Index, attachmentMap.Name)
				if attachment == nil {
					return fmt.Errorf("spine: timeline attachment not found: %s", attachmentMap.Name)
				}
				for timelineMap := attachmentMap.Child; timelineMap != nil; timelineMap = timelineMap.Next {
					keyMap := timelineMap.Child
					frames := timelineMap.Size
					switch timelineName := timelineMap.Name; timelineName {
					case "deform":
						vertexAttachment := AsVertexAttachment(attachment)
						if vertexAttachment == nil {
							return fmt.Errorf("spine: deform attachment is not a vertex attachment: %s", attachmentMap.Name)
						}
						weighted := vertexAttachment.Bones != nil
						vertices := vertexAttachment.Vertices
						deformLength := len(vertices)
						if weighted {
							deformLength = (len(vertices) / 3) << 1
						}

						timeline := NewDeformTimeline(frames, frames, slot.Index, attachment)
						time := keyMap.GetFloat("time", 0)
						for frame, bezier := 0, 0; ; frame++ {
							var deform []float32
							verticesValue := keyMap.Get("vertices")
							if verticesValue == nil {
								if weighted {
									deform = make([]float32, deformLength)
								} else {
									deform = vertices
								}
							} else {
								deform = make([]float32, deformLength)
								start := keyMap.GetInt("offset", 0)
								copy(deform[start:], verticesValue.AsFloatArray())
								if scale != 1 {
									for i, n := start, start+verticesValue.Size; i < n; i++ {
										deform[i] *= scale
									}
								}
								if !weighted {
									for i := 0; i < deformLength; i++ {
										deform[i] += vertices[i]
									}
								}
							}

							timeline.SetFrame(frame, time, deform)
							nextMap := keyMap.Next
							if nextMap == nil {
								timeline.Shrink(bezier)
								break
							}
							time2 := nextMap.GetFloat("time", 0)
							if curve := keyMap.Get("curve"); curve != nil {
								bezier = readCurve(curve, timeline, bezier, frame, 0, time, time2, 0, 1, 1)
							}
							time = time2
							keyMap = nextMap
						}
						timelines = append(timelines, timeline)

					case "sequence":
						timeline := NewSequenceTimeline(frames, slot.Index, attachment)
						lastDelay := float32(0)
						for frame := 0; keyMap != nil; keyMap, frame = keyMap.Next, frame+1 {
							delay := keyMap.GetFloat("delay", lastDelay)
							mode, err := sequenceModeFromString(keyMap.GetString("mode", "hold"))
							if err != nil {
								return err
							}
							timeline.SetFrame(frame, keyMap.GetFloat("time", 0), mode, keyMap.GetInt("index", 0), delay)
							lastDelay = delay
						}
						timelines = append(timelines, timeline)
					}
				}
			}
		}
	}

	// Draw order timeline.
	if drawOrderMap := m.Get("drawOrder"); drawOrderMap != nil {
		timeline := NewDrawOrderTimeline(drawOrderMap.Size)
		slotCount := len(skeletonData.Slots)
		frame := 0
		for keyMap := drawOrderMap.Child; keyMap != nil; keyMap, frame = keyMap.Next, frame+1 {
			var drawOrder []int
			if offsets := keyMap.Get("offsets"); offsets != nil {
				drawOrder = make([]int, slotCount)
				for i := slotCount - 1; i >= 0; i-- {
					drawOrder[i] = -1
				}
				unchanged := make([]int, slotCount-offsets.Size)
				originalIndex, unchangedIndex := 0, 0
				for offsetMap := offsets.Child; offsetMap != nil; offsetMap = offsetMap.Next {
					slotName := offsetMap.RequireString("slot")
					slot := skeletonData.FindSlot(slotName)
					if slot == nil {
						return fmt.Errorf("spine: slot not found: %s", slotName)
					}
					// Collect unchanged items.
					for originalIndex != slot.Index {
						unchanged[unchangedIndex] = originalIndex
						unchangedIndex++
						originalIndex++
					}
					// Set changed items.
					drawOrder[originalIndex+offsetMap.RequireInt("offset")] = originalIndex
					originalIndex++
				}
				// Collect remaining unchanged items.
				for originalIndex < slotCount {
					unchanged[unchangedIndex] = originalIndex
					unchangedIndex++
					originalIndex++
				}
				// Fill in unchanged items.
				for i := slotCount - 1; i >= 0; i-- {
					if drawOrder[i] == -1 {
						unchangedIndex--
						drawOrder[i] = unchanged[unchangedIndex]
					}
				}
			}
			timeline.SetFrame(frame, keyMap.GetFloat("time", 0), drawOrder)
		}
		timelines = append(timelines, timeline)
	}

	// Event timeline.
	if eventsMap := m.Get("events"); eventsMap != nil {
		timeline := NewEventTimeline(eventsMap.Size)
		frame := 0
		for keyMap := eventsMap.Child; keyMap != nil; keyMap, frame = keyMap.Next, frame+1 {
			eventName := keyMap.RequireString("name")
			eventData := skeletonData.FindEvent(eventName)
			if eventData == nil {
				return fmt.Errorf("spine: event not found: %s", eventName)
			}
			event := NewEvent(keyMap.GetFloat("time", 0), eventData)
			event.Int = keyMap.GetInt("int", eventData.Int)
			event.Float = keyMap.GetFloat("float", eventData.Float)
			event.String = keyMap.GetString("string", eventData.String)
			if event.Data.AudioPath != "" {
				event.Volume = keyMap.GetFloat("volume", eventData.Volume)
				event.Balance = keyMap.GetFloat("balance", eventData.Balance)
			}
			timeline.SetFrame(frame, event)
		}
		timelines = append(timelines, timeline)
	}

	var duration float32
	for _, timeline := range timelines {
		duration = maxF(duration, timeline.Duration())
	}
	skeletonData.Animations = append(skeletonData.Animations, NewAnimation(name, timelines, duration))
	return nil
}

// curveTimeline mirrors the Animation.CurveTimeline methods used while loading.
type curveTimeline interface {
	Timeline
	SetBezier(bezier, frame, value int, time1, value1, cx1, cy1, cx2, cy2, time2, value2 float32)
	SetStepped(frame int)
	Shrink(bezierCount int)
}

// curveTimeline1 mirrors Animation.CurveTimeline1 (a curve timeline with one
// value per frame).
type curveTimeline1 interface {
	curveTimeline
	SetFrame(frame int, time, value float32)
}

// curveTimeline2 mirrors Animation.CurveTimeline2 (a curve timeline with two
// values per frame).
type curveTimeline2 interface {
	curveTimeline
	SetFrame(frame int, time, value1, value2 float32)
}

func readTimeline1(keyMap *jsonValue, timeline curveTimeline1, defaultValue, scale float32) Timeline {
	time := keyMap.GetFloat("time", 0)
	value := keyMap.GetFloat("value", defaultValue) * scale
	for frame, bezier := 0, 0; ; frame++ {
		timeline.SetFrame(frame, time, value)
		nextMap := keyMap.Next
		if nextMap == nil {
			timeline.Shrink(bezier)
			return timeline
		}
		time2 := nextMap.GetFloat("time", 0)
		value2 := nextMap.GetFloat("value", defaultValue) * scale
		if curve := keyMap.Get("curve"); curve != nil {
			bezier = readCurve(curve, timeline, bezier, frame, 0, time, time2, value, value2, scale)
		}
		time = time2
		value = value2
		keyMap = nextMap
	}
}

func readTimeline2(keyMap *jsonValue, timeline curveTimeline2, name1, name2 string, defaultValue, scale float32) Timeline {
	time := keyMap.GetFloat("time", 0)
	value1 := keyMap.GetFloat(name1, defaultValue) * scale
	value2 := keyMap.GetFloat(name2, defaultValue) * scale
	for frame, bezier := 0, 0; ; frame++ {
		timeline.SetFrame(frame, time, value1, value2)
		nextMap := keyMap.Next
		if nextMap == nil {
			timeline.Shrink(bezier)
			return timeline
		}
		time2 := nextMap.GetFloat("time", 0)
		nvalue1 := nextMap.GetFloat(name1, defaultValue) * scale
		nvalue2 := nextMap.GetFloat(name2, defaultValue) * scale
		if curve := keyMap.Get("curve"); curve != nil {
			bezier = readCurve(curve, timeline, bezier, frame, 0, time, time2, value1, nvalue1, scale)
			bezier = readCurve(curve, timeline, bezier, frame, 1, time, time2, value2, nvalue2, scale)
		}
		time = time2
		value1 = nvalue1
		value2 = nvalue2
		keyMap = nextMap
	}
}

func readCurve(curve *jsonValue, timeline curveTimeline, bezier, frame, value int, time1, time2, value1, value2, scale float32) int {
	if curve.IsString() {
		if curve.AsString() == "stepped" {
			timeline.SetStepped(frame)
		}
		return bezier
	}
	curve = curve.GetIndex(value << 2)
	if curve == nil {
		jsonErrorf("spine: invalid curve values")
	}
	cx1 := curve.AsFloat()
	curve = curve.Next
	if curve == nil {
		jsonErrorf("spine: invalid curve values")
	}
	cy1 := curve.AsFloat() * scale
	curve = curve.Next
	if curve == nil {
		jsonErrorf("spine: invalid curve values")
	}
	cx2 := curve.AsFloat()
	curve = curve.Next
	if curve == nil {
		jsonErrorf("spine: invalid curve values")
	}
	cy2 := curve.AsFloat() * scale
	setTimelineBezier(timeline, frame, value, bezier, time1, value1, cx1, cy1, cx2, cy2, time2, value2)
	return bezier + 1
}

func setTimelineBezier(timeline curveTimeline, frame, value, bezier int, time1, value1, cx1, cy1, cx2, cy2, time2, value2 float32) {
	timeline.SetBezier(bezier, frame, value, time1, value1, cx1, cy1, cx2, cy2, time2, value2)
}

// hexComponent parses a two digit hex color component starting at index i,
// returning it as a float in [0,1]. Raises a jsonError for invalid input.
func hexComponent(s string, i int) float32 {
	if i+2 > len(s) {
		jsonErrorf("spine: invalid color: %q", s)
	}
	v, err := strconv.ParseUint(s[i:i+2], 16, 32)
	if err != nil {
		jsonErrorf("spine: invalid color: %q", s)
	}
	return float32(v) / 255
}

// linkedMesh stores a linked mesh until all skins have been read and the
// parent mesh can be looked up.
type linkedMesh struct {
	parent, skin     string
	slotIndex        int
	mesh             *MeshAttachment
	inheritTimelines bool
}

func inheritFromString(s string) (Inherit, error) {
	switch s {
	case "normal":
		return InheritNormal, nil
	case "onlyTranslation":
		return InheritOnlyTranslation, nil
	case "noRotationOrReflection":
		return InheritNoRotationOrReflection, nil
	case "noScale":
		return InheritNoScale, nil
	case "noScaleOrReflection":
		return InheritNoScaleOrReflection, nil
	}
	return 0, fmt.Errorf("spine: unknown inherit: %q", s)
}

func blendModeFromString(s string) (BlendMode, error) {
	switch s {
	case "normal":
		return BlendModeNormal, nil
	case "additive":
		return BlendModeAdditive, nil
	case "multiply":
		return BlendModeMultiply, nil
	case "screen":
		return BlendModeScreen, nil
	}
	return 0, fmt.Errorf("spine: unknown blend mode: %q", s)
}

func positionModeFromString(s string) (PositionMode, error) {
	switch s {
	case "fixed":
		return PositionModeFixed, nil
	case "percent":
		return PositionModePercent, nil
	}
	return 0, fmt.Errorf("spine: unknown position mode: %q", s)
}

func spacingModeFromString(s string) (SpacingMode, error) {
	switch s {
	case "length":
		return SpacingModeLength, nil
	case "fixed":
		return SpacingModeFixed, nil
	case "percent":
		return SpacingModePercent, nil
	case "proportional":
		return SpacingModeProportional, nil
	}
	return 0, fmt.Errorf("spine: unknown spacing mode: %q", s)
}

func rotateModeFromString(s string) (RotateMode, error) {
	switch s {
	case "tangent":
		return RotateModeTangent, nil
	case "chain":
		return RotateModeChain, nil
	case "chainScale":
		return RotateModeChainScale, nil
	}
	return 0, fmt.Errorf("spine: unknown rotate mode: %q", s)
}

func sequenceModeFromString(s string) (SequenceMode, error) {
	switch s {
	case "hold":
		return SequenceModeHold, nil
	case "once":
		return SequenceModeOnce, nil
	case "loop":
		return SequenceModeLoop, nil
	case "pingpong":
		return SequenceModePingpong, nil
	case "onceReverse":
		return SequenceModeOnceReverse, nil
	case "loopReverse":
		return SequenceModeLoopReverse, nil
	case "pingpongReverse":
		return SequenceModePingpongReverse, nil
	}
	return 0, fmt.Errorf("spine: unknown sequence mode: %q", s)
}
