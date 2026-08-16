package spine

// SkeletonData stores the setup pose and all of the stateless data for a skeleton.
type SkeletonData struct {
	Name                 string
	Bones                []*BoneData // Ordered parents first.
	Slots                []*SlotData // Setup pose draw order.
	Skins                []*Skin
	DefaultSkin          *Skin
	Events               []*EventData
	Animations           []*Animation
	IkConstraints        []*IkConstraintData
	TransformConstraints []*TransformConstraintData
	PathConstraints      []*PathConstraintData
	PhysicsConstraints   []*PhysicsConstraintData

	X, Y, Width, Height float32
	ReferenceScale      float32
	Version, Hash       string

	// Nonessential.
	FPS        float32
	ImagesPath string
	AudioPath  string
}

// NewSkeletonData creates empty skeleton data.
func NewSkeletonData() *SkeletonData {
	return &SkeletonData{ReferenceScale: 100, FPS: 30}
}

// FindBone finds a bone by comparing each bone's name. It is more efficient to
// cache the results of this method than to call it multiple times.
func (d *SkeletonData) FindBone(boneName string) *BoneData {
	for _, bone := range d.Bones {
		if bone.Name == boneName {
			return bone
		}
	}
	return nil
}

// FindSlot finds a slot by comparing each slot's name.
func (d *SkeletonData) FindSlot(slotName string) *SlotData {
	for _, slot := range d.Slots {
		if slot.Name == slotName {
			return slot
		}
	}
	return nil
}

// FindSkin finds a skin by comparing each skin's name.
func (d *SkeletonData) FindSkin(skinName string) *Skin {
	for _, skin := range d.Skins {
		if skin.Name == skinName {
			return skin
		}
	}
	return nil
}

// FindEvent finds an event by comparing each event's name.
func (d *SkeletonData) FindEvent(eventDataName string) *EventData {
	for _, event := range d.Events {
		if event.Name == eventDataName {
			return event
		}
	}
	return nil
}

// FindAnimation finds an animation by comparing each animation's name.
func (d *SkeletonData) FindAnimation(animationName string) *Animation {
	for _, animation := range d.Animations {
		if animation.Name == animationName {
			return animation
		}
	}
	return nil
}

// FindIkConstraint finds an IK constraint by comparing each IK constraint's name.
func (d *SkeletonData) FindIkConstraint(constraintName string) *IkConstraintData {
	for _, c := range d.IkConstraints {
		if c.Name == constraintName {
			return c
		}
	}
	return nil
}

// FindTransformConstraint finds a transform constraint by comparing each
// transform constraint's name.
func (d *SkeletonData) FindTransformConstraint(constraintName string) *TransformConstraintData {
	for _, c := range d.TransformConstraints {
		if c.Name == constraintName {
			return c
		}
	}
	return nil
}

// FindPathConstraint finds a path constraint by comparing each path constraint's name.
func (d *SkeletonData) FindPathConstraint(constraintName string) *PathConstraintData {
	for _, c := range d.PathConstraints {
		if c.Name == constraintName {
			return c
		}
	}
	return nil
}

// FindPhysicsConstraint finds a physics constraint by comparing each physics
// constraint's name.
func (d *SkeletonData) FindPhysicsConstraint(constraintName string) *PhysicsConstraintData {
	for _, c := range d.PhysicsConstraints {
		if c.Name == constraintName {
			return c
		}
	}
	return nil
}

func (d *SkeletonData) String() string { return d.Name }
