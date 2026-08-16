package spine

// TransformConstraint stores the current pose for a transform constraint. A
// transform constraint adjusts the world transform of the constrained bones to
// match that of the target bone.
type TransformConstraint struct {
	Data *TransformConstraintData

	// Bones that will be modified by this transform constraint.
	Bones []*Bone

	// Target is the bone whose world transform will be copied to the
	// constrained bones.
	Target *Bone

	// Mixes are percentages (0-1) that control the mix between the constrained
	// and unconstrained values.
	MixRotate, MixX, MixY, MixScaleX, MixScaleY, MixShearY float32

	active bool
}

// NewTransformConstraint creates a transform constraint.
func NewTransformConstraint(data *TransformConstraintData, skeleton *Skeleton) *TransformConstraint {
	if data == nil {
		panic("spine: data cannot be nil")
	}
	if skeleton == nil {
		panic("spine: skeleton cannot be nil")
	}
	c := &TransformConstraint{Data: data}
	c.Bones = make([]*Bone, 0, len(data.Bones))
	for _, boneData := range data.Bones {
		c.Bones = append(c.Bones, skeleton.Bones[boneData.Index])
	}
	c.Target = skeleton.Bones[data.Target.Index]
	c.MixRotate = data.MixRotate
	c.MixX = data.MixX
	c.MixY = data.MixY
	c.MixScaleX = data.MixScaleX
	c.MixScaleY = data.MixScaleY
	c.MixShearY = data.MixShearY
	return c
}

// SetToSetupPose sets the constraint to its setup pose values.
func (c *TransformConstraint) SetToSetupPose() {
	data := c.Data
	c.MixRotate = data.MixRotate
	c.MixX = data.MixX
	c.MixY = data.MixY
	c.MixScaleX = data.MixScaleX
	c.MixScaleY = data.MixScaleY
	c.MixShearY = data.MixShearY
}

// Update applies the constraint to the constrained bones.
func (c *TransformConstraint) Update(physics Physics) {
	if c.MixRotate == 0 && c.MixX == 0 && c.MixY == 0 && c.MixScaleX == 0 && c.MixScaleY == 0 && c.MixShearY == 0 {
		return
	}
	if c.Data.Local {
		if c.Data.Relative {
			c.applyRelativeLocal()
		} else {
			c.applyAbsoluteLocal()
		}
	} else {
		if c.Data.Relative {
			c.applyRelativeWorld()
		} else {
			c.applyAbsoluteWorld()
		}
	}
}

// IsActive implements Updatable.
func (c *TransformConstraint) IsActive() bool { return c.active }

func (c *TransformConstraint) String() string { return c.Data.Name }

func (c *TransformConstraint) applyAbsoluteWorld() {
	mixRotate, mixX, mixY := c.MixRotate, c.MixX, c.MixY
	mixScaleX, mixScaleY, mixShearY := c.MixScaleX, c.MixScaleY, c.MixShearY
	translate := mixX != 0 || mixY != 0

	target := c.Target
	ta, tb, tc, td := target.A, target.B, target.C, target.D
	degRadReflect := degRad
	if ta*td-tb*tc <= 0 {
		degRadReflect = -degRad
	}
	offsetRotation := c.Data.OffsetRotation * degRadReflect
	offsetShearY := c.Data.OffsetShearY * degRadReflect

	for _, bone := range c.Bones {
		if mixRotate != 0 {
			a, b, cc, d := bone.A, bone.B, bone.C, bone.D
			r := atan2(tc, ta) - atan2(cc, a) + offsetRotation
			if r > piF {
				r -= pi2
			} else if r < -piF {
				r += pi2
			}
			r *= mixRotate
			cosr, sinr := cos(r), sin(r)
			bone.A = cosr*a - sinr*cc
			bone.B = cosr*b - sinr*d
			bone.C = sinr*a + cosr*cc
			bone.D = sinr*b + cosr*d
		}

		if translate {
			tx, ty := target.LocalToWorld(c.Data.OffsetX, c.Data.OffsetY)
			bone.WorldX += (tx - bone.WorldX) * mixX
			bone.WorldY += (ty - bone.WorldY) * mixY
		}

		if mixScaleX != 0 {
			s := sqrt(bone.A*bone.A + bone.C*bone.C)
			if s != 0 {
				s = (s + (sqrt(ta*ta+tc*tc)-s+c.Data.OffsetScaleX)*mixScaleX) / s
			}
			bone.A *= s
			bone.C *= s
		}
		if mixScaleY != 0 {
			s := sqrt(bone.B*bone.B + bone.D*bone.D)
			if s != 0 {
				s = (s + (sqrt(tb*tb+td*td)-s+c.Data.OffsetScaleY)*mixScaleY) / s
			}
			bone.B *= s
			bone.D *= s
		}

		if mixShearY > 0 {
			b, d := bone.B, bone.D
			by := atan2(d, b)
			r := atan2(td, tb) - atan2(tc, ta) - (by - atan2(bone.C, bone.A))
			if r > piF {
				r -= pi2
			} else if r < -piF {
				r += pi2
			}
			r = by + (r+offsetShearY)*mixShearY
			s := sqrt(b*b + d*d)
			bone.B = cos(r) * s
			bone.D = sin(r) * s
		}

		bone.UpdateAppliedTransform()
	}
}

func (c *TransformConstraint) applyRelativeWorld() {
	mixRotate, mixX, mixY := c.MixRotate, c.MixX, c.MixY
	mixScaleX, mixScaleY, mixShearY := c.MixScaleX, c.MixScaleY, c.MixShearY
	translate := mixX != 0 || mixY != 0

	target := c.Target
	ta, tb, tc, td := target.A, target.B, target.C, target.D
	degRadReflect := degRad
	if ta*td-tb*tc <= 0 {
		degRadReflect = -degRad
	}
	offsetRotation := c.Data.OffsetRotation * degRadReflect
	offsetShearY := c.Data.OffsetShearY * degRadReflect

	for _, bone := range c.Bones {
		if mixRotate != 0 {
			a, b, cc, d := bone.A, bone.B, bone.C, bone.D
			r := atan2(tc, ta) + offsetRotation
			if r > piF {
				r -= pi2
			} else if r < -piF {
				r += pi2
			}
			r *= mixRotate
			cosr, sinr := cos(r), sin(r)
			bone.A = cosr*a - sinr*cc
			bone.B = cosr*b - sinr*d
			bone.C = sinr*a + cosr*cc
			bone.D = sinr*b + cosr*d
		}

		if translate {
			tx, ty := target.LocalToWorld(c.Data.OffsetX, c.Data.OffsetY)
			bone.WorldX += tx * mixX
			bone.WorldY += ty * mixY
		}

		if mixScaleX != 0 {
			s := (sqrt(ta*ta+tc*tc)-1+c.Data.OffsetScaleX)*mixScaleX + 1
			bone.A *= s
			bone.C *= s
		}
		if mixScaleY != 0 {
			s := (sqrt(tb*tb+td*td)-1+c.Data.OffsetScaleY)*mixScaleY + 1
			bone.B *= s
			bone.D *= s
		}

		if mixShearY > 0 {
			r := atan2(td, tb) - atan2(tc, ta)
			if r > piF {
				r -= pi2
			} else if r < -piF {
				r += pi2
			}
			b, d := bone.B, bone.D
			r = atan2(d, b) + (r-piF/2+offsetShearY)*mixShearY
			s := sqrt(b*b + d*d)
			bone.B = cos(r) * s
			bone.D = sin(r) * s
		}

		bone.UpdateAppliedTransform()
	}
}

func (c *TransformConstraint) applyAbsoluteLocal() {
	mixRotate, mixX, mixY := c.MixRotate, c.MixX, c.MixY
	mixScaleX, mixScaleY, mixShearY := c.MixScaleX, c.MixScaleY, c.MixShearY

	target := c.Target

	for _, bone := range c.Bones {
		rotation := bone.ARotation
		if mixRotate != 0 {
			rotation += (target.ARotation - rotation + c.Data.OffsetRotation) * mixRotate
		}

		x, y := bone.AX, bone.AY
		x += (target.AX - x + c.Data.OffsetX) * mixX
		y += (target.AY - y + c.Data.OffsetY) * mixY

		scaleX, scaleY := bone.AScaleX, bone.AScaleY
		if mixScaleX != 0 && scaleX != 0 {
			scaleX = (scaleX + (target.AScaleX-scaleX+c.Data.OffsetScaleX)*mixScaleX) / scaleX
		}
		if mixScaleY != 0 && scaleY != 0 {
			scaleY = (scaleY + (target.AScaleY-scaleY+c.Data.OffsetScaleY)*mixScaleY) / scaleY
		}

		shearY := bone.AShearY
		if mixShearY != 0 {
			shearY += (target.AShearY - shearY + c.Data.OffsetShearY) * mixShearY
		}

		bone.UpdateWorldTransformWith(x, y, rotation, scaleX, scaleY, bone.AShearX, shearY)
	}
}

func (c *TransformConstraint) applyRelativeLocal() {
	mixRotate, mixX, mixY := c.MixRotate, c.MixX, c.MixY
	mixScaleX, mixScaleY, mixShearY := c.MixScaleX, c.MixScaleY, c.MixShearY

	target := c.Target

	for _, bone := range c.Bones {
		rotation := bone.ARotation + (target.ARotation+c.Data.OffsetRotation)*mixRotate
		x := bone.AX + (target.AX+c.Data.OffsetX)*mixX
		y := bone.AY + (target.AY+c.Data.OffsetY)*mixY
		scaleX := bone.AScaleX * ((target.AScaleX-1+c.Data.OffsetScaleX)*mixScaleX + 1)
		scaleY := bone.AScaleY * ((target.AScaleY-1+c.Data.OffsetScaleY)*mixScaleY + 1)
		shearY := bone.AShearY + (target.AShearY+c.Data.OffsetShearY)*mixShearY

		bone.UpdateWorldTransformWith(x, y, rotation, scaleX, scaleY, bone.AShearX, shearY)
	}
}
