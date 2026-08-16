package spine

import "math"

// IkConstraint stores the current pose for an IK constraint. An IK constraint
// adjusts the rotation of 1 or 2 constrained bones so the tip of the last bone
// is as close to the target bone as possible.
type IkConstraint struct {
	Data *IkConstraintData

	// Bones that will be modified by this IK constraint.
	Bones []*Bone

	// Target is the bone that is the IK target.
	Target *Bone

	// BendDirection controls, for two bone IK, the bend direction, 1 or -1.
	BendDirection int

	// Compress: for one bone IK, when true and the target is too close, the
	// bone is scaled to reach it.
	Compress bool

	// Stretch: when true and the target is out of range, the parent bone is
	// scaled to reach it.
	Stretch bool

	// Mix is a percentage (0-1) that controls the mix between the constrained
	// and unconstrained rotation.
	Mix float32

	// Softness: for two bone IK, the target bone's distance from the maximum
	// reach of the bones where rotation begins to slow.
	Softness float32

	active bool
}

// NewIkConstraint creates an IK constraint.
func NewIkConstraint(data *IkConstraintData, skeleton *Skeleton) *IkConstraint {
	if data == nil {
		panic("spine: data cannot be nil")
	}
	if skeleton == nil {
		panic("spine: skeleton cannot be nil")
	}
	c := &IkConstraint{Data: data}
	c.Bones = make([]*Bone, 0, len(data.Bones))
	for _, boneData := range data.Bones {
		c.Bones = append(c.Bones, skeleton.Bones[boneData.Index])
	}
	c.Target = skeleton.Bones[data.Target.Index]
	c.Mix = data.Mix
	c.Softness = data.Softness
	c.BendDirection = data.BendDirection
	c.Compress = data.Compress
	c.Stretch = data.Stretch
	return c
}

// SetToSetupPose sets the constraint to its setup pose values.
func (c *IkConstraint) SetToSetupPose() {
	data := c.Data
	c.Mix = data.Mix
	c.Softness = data.Softness
	c.BendDirection = data.BendDirection
	c.Compress = data.Compress
	c.Stretch = data.Stretch
}

// Update applies the constraint to the constrained bones.
func (c *IkConstraint) Update(physics Physics) {
	if c.Mix == 0 {
		return
	}
	target := c.Target
	switch len(c.Bones) {
	case 1:
		ApplyIk1(c.Bones[0], target.WorldX, target.WorldY, c.Compress, c.Stretch, c.Data.Uniform, c.Mix)
	case 2:
		ApplyIk2(c.Bones[0], c.Bones[1], target.WorldX, target.WorldY, c.BendDirection, c.Stretch, c.Data.Uniform, c.Softness, c.Mix)
	}
}

// IsActive implements Updatable.
func (c *IkConstraint) IsActive() bool { return c.active }

func (c *IkConstraint) String() string { return c.Data.Name }

// ApplyIk1 applies 1 bone IK. The target is specified in the world coordinate system.
func ApplyIk1(bone *Bone, targetX, targetY float32, compress, stretch, uniform bool, alpha float32) {
	if bone == nil {
		panic("spine: bone cannot be nil")
	}
	p := bone.Parent
	pa, pb, pc, pd := p.A, p.B, p.C, p.D
	rotationIK := -bone.AShearX - bone.ARotation
	var tx, ty float32
	switch bone.Inherit {
	case InheritOnlyTranslation:
		tx = (targetX - bone.WorldX) * signum(bone.Skeleton.ScaleX)
		ty = (targetY - bone.WorldY) * signum(bone.Skeleton.ScaleY)
	case InheritNoRotationOrReflection:
		s := abs(pa*pd-pb*pc) / maxF(0.0001, pa*pa+pc*pc)
		sa := pa / bone.Skeleton.ScaleX
		sc := pc / bone.Skeleton.ScaleY
		pb = -sc * s * bone.Skeleton.ScaleX
		pd = sa * s * bone.Skeleton.ScaleY
		rotationIK += atan2Deg(sc, sa)
		fallthrough
	default:
		x, y := targetX-p.WorldX, targetY-p.WorldY
		d := pa*pd - pb*pc
		if abs(d) <= 0.0001 {
			tx = 0
			ty = 0
		} else {
			tx = (x*pd-y*pb)/d - bone.AX
			ty = (y*pa-x*pc)/d - bone.AY
		}
	}
	rotationIK += atan2Deg(ty, tx)
	if bone.AScaleX < 0 {
		rotationIK += 180
	}
	if rotationIK > 180 {
		rotationIK -= 360
	} else if rotationIK < -180 {
		rotationIK += 360
	}
	sx, sy := bone.AScaleX, bone.AScaleY
	if compress || stretch {
		switch bone.Inherit {
		case InheritNoScale, InheritNoScaleOrReflection:
			tx = targetX - bone.WorldX
			ty = targetY - bone.WorldY
		}
		b := bone.Data.Length * sx
		if b > 0.0001 {
			dd := tx*tx + ty*ty
			if (compress && dd < b*b) || (stretch && dd > b*b) {
				s := (sqrt(dd)/b-1)*alpha + 1
				sx *= s
				if uniform {
					sy *= s
				}
			}
		}
	}
	bone.UpdateWorldTransformWith(bone.AX, bone.AY, bone.ARotation+rotationIK*alpha, sx, sy, bone.AShearX, bone.AShearY)
}

// ApplyIk2 applies 2 bone IK. The target is specified in the world coordinate
// system. child must be a direct descendant of the parent bone.
func ApplyIk2(parent, child *Bone, targetX, targetY float32, bendDir int, stretch, uniform bool, softness, alpha float32) {
	if parent == nil {
		panic("spine: parent cannot be nil")
	}
	if child == nil {
		panic("spine: child cannot be nil")
	}
	if parent.Inherit != InheritNormal || child.Inherit != InheritNormal {
		return
	}
	px, py := parent.AX, parent.AY
	psx, psy := parent.AScaleX, parent.AScaleY
	sx, sy := psx, psy
	csx := child.AScaleX
	var os1, os2, s2 int
	if psx < 0 {
		psx = -psx
		os1 = 180
		s2 = -1
	} else {
		os1 = 0
		s2 = 1
	}
	if psy < 0 {
		psy = -psy
		s2 = -s2
	}
	if csx < 0 {
		csx = -csx
		os2 = 180
	} else {
		os2 = 0
	}
	cx := child.AX
	var cy, cwx, cwy float32
	a, b, c, d := parent.A, parent.B, parent.C, parent.D
	u := abs(psx-psy) <= 0.0001
	if !u || stretch {
		cy = 0
		cwx = a*cx + parent.WorldX
		cwy = c*cx + parent.WorldY
	} else {
		cy = child.AY
		cwx = a*cx + b*cy + parent.WorldX
		cwy = c*cx + d*cy + parent.WorldY
	}
	pp := parent.Parent
	a = pp.A
	b = pp.B
	c = pp.C
	d = pp.D
	id := a*d - b*c
	x, y := cwx-pp.WorldX, cwy-pp.WorldY
	if abs(id) <= 0.0001 {
		id = 0
	} else {
		id = 1 / id
	}
	dx, dy := (x*d-y*b)*id-px, (y*a-x*c)*id-py
	l1 := sqrt(dx*dx + dy*dy)
	l2 := child.Data.Length * csx
	var a1, a2 float32
	if l1 < 0.0001 {
		ApplyIk1(parent, targetX, targetY, false, stretch, false, alpha)
		child.UpdateWorldTransformWith(cx, cy, 0, child.AScaleX, child.AScaleY, child.AShearX, child.AShearY)
		return
	}
	x = targetX - pp.WorldX
	y = targetY - pp.WorldY
	tx, ty := (x*d-y*b)*id-px, (y*a-x*c)*id-py
	dd := tx*tx + ty*ty
	if softness != 0 {
		softness *= psx * (csx + 1) * 0.5
		td := sqrt(dd)
		sd := td - l1 - l2*psx + softness
		if sd > 0 {
			p := minF(1, sd/(softness*2)) - 1
			p = (sd - softness*(1-p*p)) / td
			tx -= p * tx
			ty -= p * ty
			dd = tx*tx + ty*ty
		}
	}
	breakOuter := false
	if u {
		l2 *= psx
		cosv := (dd - l1*l1 - l2*l2) / (2 * l1 * l2)
		if cosv < -1 {
			cosv = -1
			a2 = piF * float32(bendDir)
		} else if cosv > 1 {
			cosv = 1
			a2 = 0
			if stretch {
				a = (sqrt(dd)/(l1+l2)-1)*alpha + 1
				sx *= a
				if uniform {
					sy *= a
				}
			}
		} else {
			a2 = float32(math.Acos(float64(cosv))) * float32(bendDir)
		}
		a = l1 + l2*cosv
		b = l2 * sin(a2)
		a1 = atan2(ty*a-tx*b, tx*a+ty*b)
	} else {
		a = psx * l2
		b = psy * l2
		aa, bb := a*a, b*b
		ta := atan2(ty, tx)
		c = bb*l1*l1 + aa*dd - aa*bb
		c1, c2 := -2*bb*l1, bb-aa
		d = c1*c1 - 4*c2*c
		if d >= 0 {
			q := sqrt(d)
			if c1 < 0 {
				q = -q
			}
			q = -(c1 + q) * 0.5
			r0, r1 := q/c2, c/q
			var r float32
			if abs(r0) < abs(r1) {
				r = r0
			} else {
				r = r1
			}
			r0 = dd - r*r
			if r0 >= 0 {
				y = sqrt(r0) * float32(bendDir)
				a1 = ta - atan2(y, r)
				a2 = atan2(y/psy, (r-l1)/psx)
				breakOuter = true
			}
		}
		if !breakOuter {
			minAngle, minX, minY := piF, l1-a, float32(0)
			minDist := minX * minX
			maxAngle, maxX, maxY := float32(0), l1+a, float32(0)
			maxDist := maxX * maxX
			c = -a * l1 / (aa - bb)
			if c >= -1 && c <= 1 {
				c = float32(math.Acos(float64(c)))
				x = a*cos(c) + l1
				y = b * sin(c)
				d = x*x + y*y
				if d < minDist {
					minAngle = c
					minDist = d
					minX = x
					minY = y
				}
				if d > maxDist {
					maxAngle = c
					maxDist = d
					maxX = x
					maxY = y
				}
			}
			if dd <= (minDist+maxDist)*0.5 {
				a1 = ta - atan2(minY*float32(bendDir), minX)
				a2 = minAngle * float32(bendDir)
			} else {
				a1 = ta - atan2(maxY*float32(bendDir), maxX)
				a2 = maxAngle * float32(bendDir)
			}
		}
	}
	os := atan2(cy, cx) * float32(s2)
	rotation := parent.ARotation
	a1 = (a1-os)*radDeg + float32(os1) - rotation
	if a1 > 180 {
		a1 -= 360
	} else if a1 < -180 {
		a1 += 360
	}
	parent.UpdateWorldTransformWith(px, py, rotation+a1*alpha, sx, sy, 0, 0)
	rotation = child.ARotation
	a2 = ((a2+os)*radDeg-child.AShearX)*float32(s2) + float32(os2) - rotation
	if a2 > 180 {
		a2 -= 360
	} else if a2 < -180 {
		a2 += 360
	}
	child.UpdateWorldTransformWith(cx, cy, rotation+a2*alpha, child.AScaleX, child.AScaleY, child.AShearX, child.AShearY)
}
