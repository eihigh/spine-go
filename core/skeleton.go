package spine

var quadTriangles = []uint16{0, 1, 2, 2, 3, 0}

// Skeleton stores the current pose for a skeleton.
type Skeleton struct {
	Data                 *SkeletonData
	Bones                []*Bone
	Slots                []*Slot
	DrawOrder            []*Slot
	IkConstraints        []*IkConstraint
	TransformConstraints []*TransformConstraint
	PathConstraints      []*PathConstraint
	PhysicsConstraints   []*PhysicsConstraint

	updateCache []Updatable

	skin *Skin

	// Color tints all the skeleton's attachments.
	Color Color

	// X and Y are added to the root bone's world position.
	// Bones that do not inherit translation are still affected.
	X, Y float32

	// ScaleX and ScaleY scale the entire skeleton.
	// Bones that do not inherit scale are still affected.
	ScaleX, ScaleY float32

	// Time is used for time-based manipulations such as PhysicsConstraint.
	Time float32

	boundsTemp []float32
}

// NewSkeleton creates a skeleton from setup pose data.
func NewSkeleton(data *SkeletonData) *Skeleton {
	if data == nil {
		panic("spine: data cannot be nil")
	}
	s := &Skeleton{
		Data:   data,
		ScaleX: 1,
		ScaleY: 1,
		Color:  Color{1, 1, 1, 1},
	}

	s.Bones = make([]*Bone, 0, len(data.Bones))
	for _, boneData := range data.Bones {
		var bone *Bone
		if boneData.Parent == nil {
			bone = NewBone(boneData, s, nil)
		} else {
			parent := s.Bones[boneData.Parent.Index]
			bone = NewBone(boneData, s, parent)
			parent.Children = append(parent.Children, bone)
		}
		s.Bones = append(s.Bones, bone)
	}

	s.Slots = make([]*Slot, 0, len(data.Slots))
	s.DrawOrder = make([]*Slot, 0, len(data.Slots))
	for _, slotData := range data.Slots {
		bone := s.Bones[slotData.BoneData.Index]
		slot := NewSlot(slotData, bone)
		s.Slots = append(s.Slots, slot)
		s.DrawOrder = append(s.DrawOrder, slot)
	}

	s.IkConstraints = make([]*IkConstraint, 0, len(data.IkConstraints))
	for _, d := range data.IkConstraints {
		s.IkConstraints = append(s.IkConstraints, NewIkConstraint(d, s))
	}

	s.TransformConstraints = make([]*TransformConstraint, 0, len(data.TransformConstraints))
	for _, d := range data.TransformConstraints {
		s.TransformConstraints = append(s.TransformConstraints, NewTransformConstraint(d, s))
	}

	s.PathConstraints = make([]*PathConstraint, 0, len(data.PathConstraints))
	for _, d := range data.PathConstraints {
		s.PathConstraints = append(s.PathConstraints, NewPathConstraint(d, s))
	}

	s.PhysicsConstraints = make([]*PhysicsConstraint, 0, len(data.PhysicsConstraints))
	for _, d := range data.PhysicsConstraints {
		s.PhysicsConstraints = append(s.PhysicsConstraints, NewPhysicsConstraint(d, s))
	}

	s.UpdateCache()
	return s
}

// UpdateCache caches information about bones and constraints. Must be called
// if the skin is modified or if bones, constraints, or weighted path
// attachments are added or removed.
func (s *Skeleton) UpdateCache() {
	s.updateCache = s.updateCache[:0]

	for _, bone := range s.Bones {
		bone.sorted = bone.Data.SkinRequired
		bone.active = !bone.sorted
	}
	if s.skin != nil {
		for _, boneData := range s.skin.Bones {
			bone := s.Bones[boneData.Index]
			for bone != nil {
				bone.sorted = false
				bone.active = true
				bone = bone.Parent
			}
		}
	}

	ikCount := len(s.IkConstraints)
	transformCount := len(s.TransformConstraints)
	pathCount := len(s.PathConstraints)
	physicsCount := len(s.PhysicsConstraints)
	constraintCount := ikCount + transformCount + pathCount + physicsCount
outer:
	for i := 0; i < constraintCount; i++ {
		for ii := 0; ii < ikCount; ii++ {
			constraint := s.IkConstraints[ii]
			if constraint.Data.Order == i {
				s.sortIkConstraint(constraint)
				continue outer
			}
		}
		for ii := 0; ii < transformCount; ii++ {
			constraint := s.TransformConstraints[ii]
			if constraint.Data.Order == i {
				s.sortTransformConstraint(constraint)
				continue outer
			}
		}
		for ii := 0; ii < pathCount; ii++ {
			constraint := s.PathConstraints[ii]
			if constraint.Data.Order == i {
				s.sortPathConstraint(constraint)
				continue outer
			}
		}
		for ii := 0; ii < physicsCount; ii++ {
			constraint := s.PhysicsConstraints[ii]
			if constraint.Data.Order == i {
				s.sortPhysicsConstraint(constraint)
				continue outer
			}
		}
	}

	for _, bone := range s.Bones {
		s.sortBone(bone)
	}
}

func (s *Skeleton) skinHasConstraint(data ConstraintData) bool {
	if s.skin == nil {
		return false
	}
	for _, c := range s.skin.Constraints {
		if c == data {
			return true
		}
	}
	return false
}

func (s *Skeleton) sortIkConstraint(constraint *IkConstraint) {
	constraint.active = constraint.Target.active &&
		(!constraint.Data.SkinRequired || s.skinHasConstraint(constraint.Data))
	if !constraint.active {
		return
	}

	target := constraint.Target
	s.sortBone(target)

	constrained := constraint.Bones
	parent := constrained[0]
	s.sortBone(parent)
	if len(constrained) == 1 {
		s.updateCache = append(s.updateCache, constraint)
		s.sortReset(parent.Children)
	} else {
		child := constrained[len(constrained)-1]
		s.sortBone(child)

		s.updateCache = append(s.updateCache, constraint)

		s.sortReset(parent.Children)
		child.sorted = true
	}
}

func (s *Skeleton) sortTransformConstraint(constraint *TransformConstraint) {
	constraint.active = constraint.Target.active &&
		(!constraint.Data.SkinRequired || s.skinHasConstraint(constraint.Data))
	if !constraint.active {
		return
	}

	s.sortBone(constraint.Target)

	constrained := constraint.Bones
	if constraint.Data.Local {
		for _, child := range constrained {
			s.sortBone(child.Parent)
			s.sortBone(child)
		}
	} else {
		for _, bone := range constrained {
			s.sortBone(bone)
		}
	}

	s.updateCache = append(s.updateCache, constraint)

	for _, bone := range constrained {
		s.sortReset(bone.Children)
	}
	for _, bone := range constrained {
		bone.sorted = true
	}
}

func (s *Skeleton) sortPathConstraint(constraint *PathConstraint) {
	constraint.active = constraint.Target.Bone.active &&
		(!constraint.Data.SkinRequired || s.skinHasConstraint(constraint.Data))
	if !constraint.active {
		return
	}

	slot := constraint.Target
	slotIndex := slot.Data.Index
	slotBone := slot.Bone
	if s.skin != nil {
		s.sortPathConstraintAttachmentsInSkin(s.skin, slotIndex, slotBone)
	}
	if s.Data.DefaultSkin != nil && s.Data.DefaultSkin != s.skin {
		s.sortPathConstraintAttachmentsInSkin(s.Data.DefaultSkin, slotIndex, slotBone)
	}

	if attachment, ok := slot.attachment.(*PathAttachment); ok {
		s.sortPathConstraintAttachment(attachment, slotBone)
	}

	constrained := constraint.Bones
	for _, bone := range constrained {
		s.sortBone(bone)
	}

	s.updateCache = append(s.updateCache, constraint)

	for _, bone := range constrained {
		s.sortReset(bone.Children)
	}
	for _, bone := range constrained {
		bone.sorted = true
	}
}

func (s *Skeleton) sortPathConstraintAttachmentsInSkin(skin *Skin, slotIndex int, slotBone *Bone) {
	for _, entry := range skin.entries {
		if entry.SlotIndex == slotIndex {
			s.sortPathConstraintAttachment(entry.Attachment, slotBone)
		}
	}
}

func (s *Skeleton) sortPathConstraintAttachment(attachment Attachment, slotBone *Bone) {
	pathAttachment, ok := attachment.(*PathAttachment)
	if !ok {
		return
	}
	pathBones := pathAttachment.Bones
	if pathBones == nil {
		s.sortBone(slotBone)
	} else {
		for i, n := 0, len(pathBones); i < n; {
			nn := int(pathBones[i])
			i++
			nn += i
			for i < nn {
				s.sortBone(s.Bones[pathBones[i]])
				i++
			}
		}
	}
}

func (s *Skeleton) sortPhysicsConstraint(constraint *PhysicsConstraint) {
	bone := constraint.Bone
	constraint.active = bone.active &&
		(!constraint.Data.SkinRequired || s.skinHasConstraint(constraint.Data))
	if !constraint.active {
		return
	}

	s.sortBone(bone)

	s.updateCache = append(s.updateCache, constraint)

	s.sortReset(bone.Children)
	bone.sorted = true
}

func (s *Skeleton) sortBone(bone *Bone) {
	if bone.sorted {
		return
	}
	if bone.Parent != nil {
		s.sortBone(bone.Parent)
	}
	bone.sorted = true
	s.updateCache = append(s.updateCache, bone)
}

func (s *Skeleton) sortReset(bones []*Bone) {
	for _, bone := range bones {
		if !bone.active {
			continue
		}
		if bone.sorted {
			s.sortReset(bone.Children)
		}
		bone.sorted = false
	}
}

// UpdateCacheList returns the list of bones and constraints, sorted in the
// order they should be updated, as computed by UpdateCache.
func (s *Skeleton) UpdateCacheList() []Updatable { return s.updateCache }

// UpdateWorldTransform updates the world transform for each bone and applies
// all constraints.
func (s *Skeleton) UpdateWorldTransform(physics Physics) {
	for _, bone := range s.Bones {
		bone.AX = bone.X
		bone.AY = bone.Y
		bone.ARotation = bone.Rotation
		bone.AScaleX = bone.ScaleX
		bone.AScaleY = bone.ScaleY
		bone.AShearX = bone.ShearX
		bone.AShearY = bone.ShearY
	}

	for _, updatable := range s.updateCache {
		updatable.Update(physics)
	}
}

// UpdateWorldTransformWith temporarily sets the root bone as a child of the
// specified bone, then updates the world transform for each bone and applies
// all constraints.
func (s *Skeleton) UpdateWorldTransformWith(physics Physics, parent *Bone) {
	if parent == nil {
		panic("spine: parent cannot be nil")
	}

	for i := 1; i < len(s.Bones); i++ { // Skip root bone.
		bone := s.Bones[i]
		bone.AX = bone.X
		bone.AY = bone.Y
		bone.ARotation = bone.Rotation
		bone.AScaleX = bone.ScaleX
		bone.AScaleY = bone.ScaleY
		bone.AShearX = bone.ShearX
		bone.AShearY = bone.ShearY
	}

	// Apply the parent bone transform to the root bone. The root bone always
	// inherits scale, rotation and reflection.
	rootBone := s.RootBone()
	pa, pb, pc, pd := parent.A, parent.B, parent.C, parent.D
	rootBone.WorldX = pa*s.X + pb*s.Y + parent.WorldX
	rootBone.WorldY = pc*s.X + pd*s.Y + parent.WorldY

	rx := (rootBone.Rotation + rootBone.ShearX) * degRad
	ry := (rootBone.Rotation + 90 + rootBone.ShearY) * degRad
	la := cos(rx) * rootBone.ScaleX
	lb := cos(ry) * rootBone.ScaleY
	lc := sin(rx) * rootBone.ScaleX
	ld := sin(ry) * rootBone.ScaleY
	rootBone.A = (pa*la + pb*lc) * s.ScaleX
	rootBone.B = (pa*lb + pb*ld) * s.ScaleX
	rootBone.C = (pc*la + pd*lc) * s.ScaleY
	rootBone.D = (pc*lb + pd*ld) * s.ScaleY

	// Update everything except root bone.
	for _, updatable := range s.updateCache {
		if updatable != Updatable(rootBone) {
			updatable.Update(physics)
		}
	}
}

// SetToSetupPose sets the bones, constraints, slots, and draw order to their
// setup pose values.
func (s *Skeleton) SetToSetupPose() {
	s.SetBonesToSetupPose()
	s.SetSlotsToSetupPose()
}

// SetBonesToSetupPose sets the bones and constraints to their setup pose values.
func (s *Skeleton) SetBonesToSetupPose() {
	for _, bone := range s.Bones {
		bone.SetToSetupPose()
	}
	for _, c := range s.IkConstraints {
		c.SetToSetupPose()
	}
	for _, c := range s.TransformConstraints {
		c.SetToSetupPose()
	}
	for _, c := range s.PathConstraints {
		c.SetToSetupPose()
	}
	for _, c := range s.PhysicsConstraints {
		c.SetToSetupPose()
	}
}

// SetSlotsToSetupPose sets the slots and draw order to their setup pose values.
func (s *Skeleton) SetSlotsToSetupPose() {
	copy(s.DrawOrder, s.Slots)
	for _, slot := range s.Slots {
		slot.SetToSetupPose()
	}
}

// RootBone returns the root bone, or nil if the skeleton has no bones.
func (s *Skeleton) RootBone() *Bone {
	if len(s.Bones) == 0 {
		return nil
	}
	return s.Bones[0]
}

// FindBone finds a bone by comparing each bone's name. It is more efficient to
// cache the results of this method than to call it repeatedly.
func (s *Skeleton) FindBone(boneName string) *Bone {
	for _, bone := range s.Bones {
		if bone.Data.Name == boneName {
			return bone
		}
	}
	return nil
}

// FindSlot finds a slot by comparing each slot's name. It is more efficient to
// cache the results of this method than to call it repeatedly.
func (s *Skeleton) FindSlot(slotName string) *Slot {
	for _, slot := range s.Slots {
		if slot.Data.Name == slotName {
			return slot
		}
	}
	return nil
}

// Skin returns the skeleton's current skin, or nil.
func (s *Skeleton) Skin() *Skin { return s.skin }

// SetSkinByName sets a skin by name. Panics if the skin is not found.
func (s *Skeleton) SetSkinByName(skinName string) {
	skin := s.Data.FindSkin(skinName)
	if skin == nil {
		panic("spine: skin not found: " + skinName)
	}
	s.SetSkin(skin)
}

// SetSkin sets the skin used to look up attachments before looking in the
// default skin. If the skin is changed, UpdateCache is called.
//
// Attachments from the new skin are attached if the corresponding attachment
// from the old skin was attached. If there was no old skin, each slot's setup
// mode attachment is attached from the new skin.
//
// After changing the skin, the visible attachments can be reset to those
// attached in the setup pose by calling SetSlotsToSetupPose. Also, often
// AnimationState.Apply is called before the next time the skeleton is rendered
// to allow any attachment keys in the current animation(s) to hide or show
// attachments from the new skin.
func (s *Skeleton) SetSkin(newSkin *Skin) {
	if newSkin == s.skin {
		return
	}
	if newSkin != nil {
		if s.skin != nil {
			newSkin.AttachAll(s, s.skin)
		} else {
			for i, slot := range s.Slots {
				name := slot.Data.AttachmentName
				if name != "" {
					attachment := newSkin.Attachment(i, name)
					if attachment != nil {
						slot.SetAttachment(attachment)
					}
				}
			}
		}
	}
	s.skin = newSkin
	s.UpdateCache()
}

// Attachment finds an attachment by looking in the skin and default skin using
// the slot name and attachment name.
func (s *Skeleton) Attachment(slotName, attachmentName string) Attachment {
	slot := s.Data.FindSlot(slotName)
	if slot == nil {
		panic("spine: slot not found: " + slotName)
	}
	return s.AttachmentByIndex(slot.Index, attachmentName)
}

// AttachmentByIndex finds an attachment by looking in the skin and default
// skin using the slot index and attachment name. First the skin is checked and
// if the attachment was not found, the default skin is checked.
func (s *Skeleton) AttachmentByIndex(slotIndex int, attachmentName string) Attachment {
	if attachmentName == "" {
		panic("spine: attachmentName cannot be empty")
	}
	if s.skin != nil {
		attachment := s.skin.Attachment(slotIndex, attachmentName)
		if attachment != nil {
			return attachment
		}
	}
	if s.Data.DefaultSkin != nil {
		return s.Data.DefaultSkin.Attachment(slotIndex, attachmentName)
	}
	return nil
}

// SetAttachment is a convenience method to set an attachment: it finds the
// slot with FindSlot, finds the attachment with AttachmentByIndex, then sets
// the slot's attachment. attachmentName may be empty to clear the slot's
// attachment.
func (s *Skeleton) SetAttachment(slotName, attachmentName string) {
	if slotName == "" {
		panic("spine: slotName cannot be empty")
	}
	slot := s.FindSlot(slotName)
	if slot == nil {
		panic("spine: slot not found: " + slotName)
	}
	var attachment Attachment
	if attachmentName != "" {
		attachment = s.AttachmentByIndex(slot.Data.Index, attachmentName)
		if attachment == nil {
			panic("spine: attachment not found: " + attachmentName + ", for slot: " + slotName)
		}
	}
	slot.SetAttachment(attachment)
}

// FindIkConstraint finds an IK constraint by comparing each IK constraint's
// name. It is more efficient to cache the results of this method than to call
// it repeatedly.
func (s *Skeleton) FindIkConstraint(constraintName string) *IkConstraint {
	for _, c := range s.IkConstraints {
		if c.Data.Name == constraintName {
			return c
		}
	}
	return nil
}

// FindTransformConstraint finds a transform constraint by comparing each
// transform constraint's name.
func (s *Skeleton) FindTransformConstraint(constraintName string) *TransformConstraint {
	for _, c := range s.TransformConstraints {
		if c.Data.Name == constraintName {
			return c
		}
	}
	return nil
}

// FindPathConstraint finds a path constraint by comparing each path
// constraint's name.
func (s *Skeleton) FindPathConstraint(constraintName string) *PathConstraint {
	for _, c := range s.PathConstraints {
		if c.Data.Name == constraintName {
			return c
		}
	}
	return nil
}

// FindPhysicsConstraint finds a physics constraint by comparing each physics
// constraint's name.
func (s *Skeleton) FindPhysicsConstraint(constraintName string) *PhysicsConstraint {
	for _, c := range s.PhysicsConstraints {
		if c.Data.Name == constraintName {
			return c
		}
	}
	return nil
}

// Bounds returns the axis aligned bounding box (AABB) of the region and mesh
// attachments for the current pose. If clipper is non-nil, clipping is applied.
// It returns the distance from the skeleton origin to the bottom left corner
// of the AABB (x, y) and the AABB width and height.
func (s *Skeleton) Bounds(clipper *SkeletonClipping) (x, y, width, height float32) {
	minX, minY := maxF32, maxF32
	maxX, maxY := -maxF32, -maxF32
	for _, slot := range s.DrawOrder {
		if !slot.Bone.active {
			continue
		}
		verticesLength := 0
		var vertices []float32
		var triangles []uint16
		attachment := slot.attachment
		switch a := attachment.(type) {
		case *RegionAttachment:
			verticesLength = 8
			vertices = ensureSize(&s.boundsTemp, 8)
			a.ComputeWorldVertices(slot, vertices, 0, 2)
			triangles = quadTriangles
		case *MeshAttachment:
			verticesLength = a.WorldVerticesLength
			vertices = ensureSize(&s.boundsTemp, verticesLength)
			a.ComputeWorldVertices(slot, 0, verticesLength, vertices, 0, 2)
			triangles = a.Triangles
		case *ClippingAttachment:
			if clipper != nil {
				clipper.ClipStart(slot, a)
				continue
			}
		}
		if vertices != nil {
			if clipper != nil && clipper.IsClipping() {
				clipper.ClipTriangles(vertices, triangles, len(triangles))
				vertices = clipper.ClippedVertices()
				verticesLength = len(vertices)
			}
			for ii := 0; ii < verticesLength; ii += 2 {
				vx, vy := vertices[ii], vertices[ii+1]
				minX = minF(minX, vx)
				minY = minF(minY, vy)
				maxX = maxF(maxX, vx)
				maxY = maxF(maxY, vy)
			}
		}
		if clipper != nil {
			clipper.ClipEndSlot(slot)
		}
	}
	if clipper != nil {
		clipper.ClipEnd()
	}
	return minX, minY, maxX - minX, maxY - minY
}

// SetColor is a convenience method for setting the skeleton color.
func (s *Skeleton) SetColor(r, g, b, a float32) { s.Color.Set(r, g, b, a) }

// SetScale scales the entire skeleton on the X and Y axes.
// Bones that do not inherit scale are still affected by this property.
func (s *Skeleton) SetScale(scaleX, scaleY float32) {
	s.ScaleX = scaleX
	s.ScaleY = scaleY
}

// SetPosition sets the skeleton X and Y position, which is added to the root
// bone worldX and worldY position. Bones that do not inherit translation are
// still affected by this property.
func (s *Skeleton) SetPosition(x, y float32) {
	s.X = x
	s.Y = y
}

// PhysicsTranslate calls PhysicsConstraint.Translate for each physics constraint.
func (s *Skeleton) PhysicsTranslate(x, y float32) {
	for _, c := range s.PhysicsConstraints {
		c.Translate(x, y)
	}
}

// PhysicsRotate calls PhysicsConstraint.Rotate for each physics constraint.
func (s *Skeleton) PhysicsRotate(x, y, degrees float32) {
	for _, c := range s.PhysicsConstraints {
		c.Rotate(x, y, degrees)
	}
}

// Update increments the skeleton's Time.
func (s *Skeleton) Update(delta float32) { s.Time += delta }

func (s *Skeleton) String() string { return s.Data.Name }

// ensureSize grows *buf to at least n elements and returns (*buf)[:n].
func ensureSize(buf *[]float32, n int) []float32 {
	if cap(*buf) < n {
		*buf = make([]float32, n)
	}
	*buf = (*buf)[:n]
	return *buf
}
