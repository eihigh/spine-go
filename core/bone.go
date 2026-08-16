package spine

// Bone stores a bone's current pose.
//
// A bone has a local transform which is used to compute its world transform.
// A bone also has an applied transform, which is a local transform that can be
// applied to compute the world transform. The local transform and applied
// transform may differ if a constraint or application code modifies the world
// transform after it was computed from the local transform.
type Bone struct {
	Data     *BoneData
	Skeleton *Skeleton
	Parent   *Bone
	Children []*Bone

	// Local transform.
	X, Y, Rotation, ScaleX, ScaleY, ShearX, ShearY float32

	// Applied transform.
	AX, AY, ARotation, AScaleX, AScaleY, AShearX, AShearY float32

	// World transform matrix: A B WorldX / C D WorldY.
	A, B, WorldX float32
	C, D, WorldY float32

	Inherit Inherit

	sorted bool
	active bool
}

// NewBone creates a bone. parent may be nil for the root bone.
func NewBone(data *BoneData, skeleton *Skeleton, parent *Bone) *Bone {
	if data == nil {
		panic("spine: data cannot be nil")
	}
	if skeleton == nil {
		panic("spine: skeleton cannot be nil")
	}
	b := &Bone{Data: data, Skeleton: skeleton, Parent: parent}
	b.SetToSetupPose()
	return b
}

// Update computes the world transform using the parent bone and this bone's
// local applied transform.
func (b *Bone) Update(physics Physics) {
	b.UpdateWorldTransformWith(b.AX, b.AY, b.ARotation, b.AScaleX, b.AScaleY, b.AShearX, b.AShearY)
}

// IsActive returns false when the bone won't be updated by
// Skeleton.UpdateWorldTransform because a skin is required and the active skin
// does not contain this bone.
func (b *Bone) IsActive() bool { return b.active }

// UpdateWorldTransform computes the world transform using the parent bone and
// this bone's local transform.
func (b *Bone) UpdateWorldTransform() {
	b.UpdateWorldTransformWith(b.X, b.Y, b.Rotation, b.ScaleX, b.ScaleY, b.ShearX, b.ShearY)
}

// UpdateWorldTransformWith computes the world transform using the parent bone
// and the specified local transform. The applied transform is set to the
// specified local transform. Child bones are not updated.
func (b *Bone) UpdateWorldTransformWith(x, y, rotation, scaleX, scaleY, shearX, shearY float32) {
	b.AX = x
	b.AY = y
	b.ARotation = rotation
	b.AScaleX = scaleX
	b.AScaleY = scaleY
	b.AShearX = shearX
	b.AShearY = shearY

	parent := b.Parent
	if parent == nil { // Root bone.
		skeleton := b.Skeleton
		sx, sy := skeleton.ScaleX, skeleton.ScaleY
		rx := (rotation + shearX) * degRad
		ry := (rotation + 90 + shearY) * degRad
		b.A = cos(rx) * scaleX * sx
		b.B = cos(ry) * scaleY * sx
		b.C = sin(rx) * scaleX * sy
		b.D = sin(ry) * scaleY * sy
		b.WorldX = x*sx + skeleton.X
		b.WorldY = y*sy + skeleton.Y
		return
	}

	pa, pb, pc, pd := parent.A, parent.B, parent.C, parent.D
	b.WorldX = pa*x + pb*y + parent.WorldX
	b.WorldY = pc*x + pd*y + parent.WorldY

	switch b.Inherit {
	case InheritNormal:
		rx := (rotation + shearX) * degRad
		ry := (rotation + 90 + shearY) * degRad
		la := cos(rx) * scaleX
		lb := cos(ry) * scaleY
		lc := sin(rx) * scaleX
		ld := sin(ry) * scaleY
		b.A = pa*la + pb*lc
		b.B = pa*lb + pb*ld
		b.C = pc*la + pd*lc
		b.D = pc*lb + pd*ld
		return
	case InheritOnlyTranslation:
		rx := (rotation + shearX) * degRad
		ry := (rotation + 90 + shearY) * degRad
		b.A = cos(rx) * scaleX
		b.B = cos(ry) * scaleY
		b.C = sin(rx) * scaleX
		b.D = sin(ry) * scaleY
	case InheritNoRotationOrReflection:
		sx, sy := 1/b.Skeleton.ScaleX, 1/b.Skeleton.ScaleY
		pa *= sx
		pc *= sy
		s := pa*pa + pc*pc
		var prx float32
		if s > 0.0001 {
			s = abs(pa*pd*sy-pb*sx*pc) / s
			pb = pc * s
			pd = pa * s
			prx = atan2Deg(pc, pa)
		} else {
			pa = 0
			pc = 0
			prx = 90 - atan2Deg(pd, pb)
		}
		rx := (rotation + shearX - prx) * degRad
		ry := (rotation + shearY - prx + 90) * degRad
		la := cos(rx) * scaleX
		lb := cos(ry) * scaleY
		lc := sin(rx) * scaleX
		ld := sin(ry) * scaleY
		b.A = pa*la - pb*lc
		b.B = pa*lb - pb*ld
		b.C = pc*la + pd*lc
		b.D = pc*lb + pd*ld
	case InheritNoScale, InheritNoScaleOrReflection:
		rotation *= degRad
		cosr, sinr := cos(rotation), sin(rotation)
		za := (pa*cosr + pb*sinr) / b.Skeleton.ScaleX
		zc := (pc*cosr + pd*sinr) / b.Skeleton.ScaleY
		s := sqrt(za*za + zc*zc)
		if s > 0.00001 {
			s = 1 / s
		}
		za *= s
		zc *= s
		s = sqrt(za*za + zc*zc)
		if b.Inherit == InheritNoScale &&
			(pa*pd-pb*pc < 0) != ((b.Skeleton.ScaleX < 0) != (b.Skeleton.ScaleY < 0)) {
			s = -s
		}
		rotation = piF/2 + atan2(zc, za)
		zb := cos(rotation) * s
		zd := sin(rotation) * s
		shearX *= degRad
		shearY = (90 + shearY) * degRad
		la := cos(shearX) * scaleX
		lb := cos(shearY) * scaleY
		lc := sin(shearX) * scaleX
		ld := sin(shearY) * scaleY
		b.A = za*la + zb*lc
		b.B = za*lb + zb*ld
		b.C = zc*la + zd*lc
		b.D = zc*lb + zd*ld
	}
	b.A *= b.Skeleton.ScaleX
	b.B *= b.Skeleton.ScaleX
	b.C *= b.Skeleton.ScaleY
	b.D *= b.Skeleton.ScaleY
}

// SetToSetupPose sets this bone's local transform to the setup pose.
func (b *Bone) SetToSetupPose() {
	data := b.Data
	b.X = data.X
	b.Y = data.Y
	b.Rotation = data.Rotation
	b.ScaleX = data.ScaleX
	b.ScaleY = data.ScaleY
	b.ShearX = data.ShearX
	b.ShearY = data.ShearY
	b.Inherit = data.Inherit
}

// SetPosition sets the local x and y translation.
func (b *Bone) SetPosition(x, y float32) {
	b.X = x
	b.Y = y
}

// SetScale sets the local scaleX and scaleY.
func (b *Bone) SetScale(scaleX, scaleY float32) {
	b.ScaleX = scaleX
	b.ScaleY = scaleY
}

// UpdateAppliedTransform computes the applied transform values from the world
// transform.
//
// If the world transform is modified (by a constraint, RotateWorld, etc) then
// this method should be called so the applied transform matches the world
// transform. Some information is ambiguous in the world transform, such as
// -1,-1 scale versus 180 rotation. The applied transform after calling this
// method is equivalent to the local transform used to compute the world
// transform, but may not be identical.
func (b *Bone) UpdateAppliedTransform() {
	parent := b.Parent
	if parent == nil {
		b.AX = b.WorldX - b.Skeleton.X
		b.AY = b.WorldY - b.Skeleton.Y
		a, bb, c, d := b.A, b.B, b.C, b.D
		b.ARotation = atan2Deg(c, a)
		b.AScaleX = sqrt(a*a + c*c)
		b.AScaleY = sqrt(bb*bb + d*d)
		b.AShearX = 0
		b.AShearY = atan2Deg(a*bb+c*d, a*d-bb*c)
		return
	}

	pa, pb, pc, pd := parent.A, parent.B, parent.C, parent.D
	pid := 1 / (pa*pd - pb*pc)
	ia, ib, ic, id := pd*pid, pb*pid, pc*pid, pa*pid
	dx, dy := b.WorldX-parent.WorldX, b.WorldY-parent.WorldY
	b.AX = dx*ia - dy*ib
	b.AY = dy*id - dx*ic

	var ra, rb, rc, rd float32
	if b.Inherit == InheritOnlyTranslation {
		ra, rb, rc, rd = b.A, b.B, b.C, b.D
	} else {
		switch b.Inherit {
		case InheritNoRotationOrReflection:
			s := abs(pa*pd-pb*pc) / (pa*pa + pc*pc)
			pb = -pc * b.Skeleton.ScaleX * s / b.Skeleton.ScaleY
			pd = pa * b.Skeleton.ScaleY * s / b.Skeleton.ScaleX
			pid = 1 / (pa*pd - pb*pc)
			ia = pd * pid
			ib = pb * pid
		case InheritNoScale, InheritNoScaleOrReflection:
			r := b.Rotation * degRad
			cosr, sinr := cos(r), sin(r)
			pa = (pa*cosr + pb*sinr) / b.Skeleton.ScaleX
			pc = (pc*cosr + pd*sinr) / b.Skeleton.ScaleY
			s := sqrt(pa*pa + pc*pc)
			if s > 0.00001 {
				s = 1 / s
			}
			pa *= s
			pc *= s
			s = sqrt(pa*pa + pc*pc)
			if b.Inherit == InheritNoScale &&
				(pid < 0) != ((b.Skeleton.ScaleX < 0) != (b.Skeleton.ScaleY < 0)) {
				s = -s
			}
			r = piF/2 + atan2(pc, pa)
			pb = cos(r) * s
			pd = sin(r) * s
			pid = 1 / (pa*pd - pb*pc)
			ia = pd * pid
			ib = pb * pid
			ic = pc * pid
			id = pa * pid
		}
		ra = ia*b.A - ib*b.C
		rb = ia*b.B - ib*b.D
		rc = id*b.C - ic*b.A
		rd = id*b.D - ic*b.B
	}

	b.AShearX = 0
	b.AScaleX = sqrt(ra*ra + rc*rc)
	if b.AScaleX > 0.0001 {
		det := ra*rd - rb*rc
		b.AScaleY = det / b.AScaleX
		b.AShearY = -atan2Deg(ra*rb+rc*rd, det)
		b.ARotation = atan2Deg(rc, ra)
	} else {
		b.AScaleX = 0
		b.AScaleY = sqrt(rb*rb + rd*rd)
		b.AShearY = 0
		b.ARotation = 90 - atan2Deg(rd, rb)
	}
}

// WorldRotationX returns the world rotation for the X axis, calculated using A and C.
func (b *Bone) WorldRotationX() float32 { return atan2Deg(b.C, b.A) }

// WorldRotationY returns the world rotation for the Y axis, calculated using B and D.
func (b *Bone) WorldRotationY() float32 { return atan2Deg(b.D, b.B) }

// WorldScaleX returns the magnitude (always positive) of the world scale X.
func (b *Bone) WorldScaleX() float32 { return sqrt(b.A*b.A + b.C*b.C) }

// WorldScaleY returns the magnitude (always positive) of the world scale Y.
func (b *Bone) WorldScaleY() float32 { return sqrt(b.B*b.B + b.D*b.D) }

// WorldToLocal transforms a point from world coordinates to the bone's local coordinates.
func (b *Bone) WorldToLocal(worldX, worldY float32) (x, y float32) {
	det := b.A*b.D - b.B*b.C
	dx, dy := worldX-b.WorldX, worldY-b.WorldY
	return (dx*b.D - dy*b.B) / det, (dy*b.A - dx*b.C) / det
}

// LocalToWorld transforms a point from the bone's local coordinates to world coordinates.
func (b *Bone) LocalToWorld(localX, localY float32) (x, y float32) {
	return localX*b.A + localY*b.B + b.WorldX, localX*b.C + localY*b.D + b.WorldY
}

// WorldToParent transforms a point from world coordinates to the parent bone's
// local coordinates.
func (b *Bone) WorldToParent(worldX, worldY float32) (x, y float32) {
	if b.Parent == nil {
		return worldX, worldY
	}
	return b.Parent.WorldToLocal(worldX, worldY)
}

// ParentToWorld transforms a point from the parent bone's coordinates to world
// coordinates.
func (b *Bone) ParentToWorld(localX, localY float32) (x, y float32) {
	if b.Parent == nil {
		return localX, localY
	}
	return b.Parent.LocalToWorld(localX, localY)
}

// WorldToLocalRotation transforms a world rotation to a local rotation.
func (b *Bone) WorldToLocalRotation(worldRotation float32) float32 {
	worldRotation *= degRad
	s, c := sin(worldRotation), cos(worldRotation)
	return atan2Deg(b.A*s-b.C*c, b.D*c-b.B*s) + b.Rotation - b.ShearX
}

// LocalToWorldRotation transforms a local rotation to a world rotation.
func (b *Bone) LocalToWorldRotation(localRotation float32) float32 {
	localRotation = (localRotation - b.Rotation - b.ShearX) * degRad
	s, c := sin(localRotation), cos(localRotation)
	return atan2Deg(c*b.C+s*b.D, c*b.A+s*b.B)
}

// RotateWorld rotates the world transform the specified amount.
//
// After changes are made to the world transform, UpdateAppliedTransform should
// be called and Update will need to be called on any child bones, recursively.
func (b *Bone) RotateWorld(degrees float32) {
	degrees *= degRad
	s, c := sin(degrees), cos(degrees)
	ra, rb := b.A, b.B
	b.A = c*ra - s*b.C
	b.B = c*rb - s*b.D
	b.C = s*ra + c*b.C
	b.D = s*rb + c*b.D
}

func (b *Bone) String() string { return b.Data.Name }
