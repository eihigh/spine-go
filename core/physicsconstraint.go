package spine

// PhysicsConstraint stores the current pose for a physics constraint. A
// physics constraint applies physics to bones.
type PhysicsConstraint struct {
	Data *PhysicsConstraintData

	// Bone constrained by this physics constraint.
	Bone *Bone

	Inertia, Strength, Damping, MassInverse float32
	Wind, Gravity, Mix                      float32

	reset          bool
	ux, uy, cx, cy float32
	tx, ty         float32

	xOffset, xVelocity           float32
	yOffset, yVelocity           float32
	rotateOffset, rotateVelocity float32
	scaleOffset, scaleVelocity   float32

	active bool

	skeleton            *Skeleton
	remaining, lastTime float32
}

// NewPhysicsConstraint creates a physics constraint.
func NewPhysicsConstraint(data *PhysicsConstraintData, skeleton *Skeleton) *PhysicsConstraint {
	if data == nil {
		panic("spine: data cannot be nil")
	}
	if skeleton == nil {
		panic("spine: skeleton cannot be nil")
	}
	c := &PhysicsConstraint{Data: data, skeleton: skeleton, reset: true}
	c.Bone = skeleton.Bones[data.Bone.Index]
	c.Inertia = data.Inertia
	c.Strength = data.Strength
	c.Damping = data.Damping
	c.MassInverse = data.MassInverse
	c.Wind = data.Wind
	c.Gravity = data.Gravity
	c.Mix = data.Mix
	return c
}

// Reset resets the physics constraint's state.
func (c *PhysicsConstraint) Reset() {
	c.remaining = 0
	c.lastTime = c.skeleton.Time
	c.reset = true
	c.xOffset = 0
	c.xVelocity = 0
	c.yOffset = 0
	c.yVelocity = 0
	c.rotateOffset = 0
	c.rotateVelocity = 0
	c.scaleOffset = 0
	c.scaleVelocity = 0
}

// SetToSetupPose sets the constraint to its setup pose values.
func (c *PhysicsConstraint) SetToSetupPose() {
	data := c.Data
	c.Inertia = data.Inertia
	c.Strength = data.Strength
	c.Damping = data.Damping
	c.MassInverse = data.MassInverse
	c.Wind = data.Wind
	c.Gravity = data.Gravity
	c.Mix = data.Mix
}

// Translate translates the physics constraint so the next Update forces are
// applied as if the bone moved an additional amount in world space.
func (c *PhysicsConstraint) Translate(x, y float32) {
	c.ux -= x
	c.uy -= y
	c.cx -= x
	c.cy -= y
}

// Rotate rotates the physics constraint so the next Update forces are applied
// as if the bone rotated around the specified point in world space.
func (c *PhysicsConstraint) Rotate(x, y, degrees float32) {
	r := degrees * degRad
	cosr, sinr := cos(r), sin(r)
	dx, dy := c.cx-x, c.cy-y
	c.Translate(dx*cosr-dy*sinr-dx, dx*sinr+dy*cosr-dy)
}

// IsActive implements Updatable.
func (c *PhysicsConstraint) IsActive() bool { return c.active }

func (c *PhysicsConstraint) String() string { return c.Data.Name }

// Update applies the constraint to the constrained bones.
func (c *PhysicsConstraint) Update(physics Physics) {
	mix := c.Mix
	if mix == 0 {
		return
	}

	x := c.Data.X > 0
	y := c.Data.Y > 0
	rotateOrShearX := c.Data.Rotate > 0 || c.Data.ShearX > 0
	scaleX := c.Data.ScaleX > 0
	bone := c.Bone
	l := bone.Data.Length

	switch physics {
	case PhysicsNone:
		return
	case PhysicsReset, PhysicsUpdate:
		if physics == PhysicsReset {
			c.Reset()
		}
		skeleton := c.skeleton
		delta := maxF(skeleton.Time-c.lastTime, 0)
		c.remaining += delta
		c.lastTime = skeleton.Time

		bx, by := bone.WorldX, bone.WorldY
		if c.reset {
			c.reset = false
			c.ux = bx
			c.uy = by
		} else {
			a := c.remaining
			i := c.Inertia
			t := c.Data.Step
			f := skeleton.Data.ReferenceScale
			d := float32(-1)
			qx := c.Data.Limit * delta
			qy := qx * abs(skeleton.ScaleY)
			qx *= abs(skeleton.ScaleX)
			if x || y {
				if x {
					u := (c.ux - bx) * i
					if u > qx {
						u = qx
					} else if u < -qx {
						u = -qx
					}
					c.xOffset += u
					c.ux = bx
				}
				if y {
					u := (c.uy - by) * i
					if u > qy {
						u = qy
					} else if u < -qy {
						u = -qy
					}
					c.yOffset += u
					c.uy = by
				}
				if a >= t {
					d = pow(c.Damping, 60*t)
					m := c.MassInverse * t
					e := c.Strength
					w := c.Wind * f * skeleton.ScaleX
					g := c.Gravity * f * skeleton.ScaleY
					for {
						if x {
							c.xVelocity += (w - c.xOffset*e) * m
							c.xOffset += c.xVelocity * t
							c.xVelocity *= d
						}
						if y {
							c.yVelocity -= (g + c.yOffset*e) * m
							c.yOffset += c.yVelocity * t
							c.yVelocity *= d
						}
						a -= t
						if a < t {
							break
						}
					}
				}
				if x {
					bone.WorldX += c.xOffset * mix * c.Data.X
				}
				if y {
					bone.WorldY += c.yOffset * mix * c.Data.Y
				}
			}
			if rotateOrShearX || scaleX {
				ca := atan2(bone.C, bone.A)
				var cosv, sinv float32
				mr := float32(0)
				dx, dy := c.cx-bone.WorldX, c.cy-bone.WorldY
				if dx > qx {
					dx = qx
				} else if dx < -qx {
					dx = -qx
				}
				if dy > qy {
					dy = qy
				} else if dy < -qy {
					dy = -qy
				}
				if rotateOrShearX {
					mr = (c.Data.Rotate + c.Data.ShearX) * mix
					r := atan2(dy+c.ty, dx+c.tx) - ca - c.rotateOffset*mr
					c.rotateOffset += (r - float32(ceilInt(r*invPI2-0.5))*pi2) * i
					r = c.rotateOffset*mr + ca
					cosv = cos(r)
					sinv = sin(r)
					if scaleX {
						r = l * bone.WorldScaleX()
						if r > 0 {
							c.scaleOffset += (dx*cosv + dy*sinv) * i / r
						}
					}
				} else {
					cosv = cos(ca)
					sinv = sin(ca)
					r := l * bone.WorldScaleX()
					if r > 0 {
						c.scaleOffset += (dx*cosv + dy*sinv) * i / r
					}
				}
				a = c.remaining
				if a >= t {
					if d == -1 {
						d = pow(c.Damping, 60*t)
					}
					m := c.MassInverse * t
					e := c.Strength
					w := c.Wind
					g := c.Gravity
					h := l / f
					for {
						a -= t
						if scaleX {
							c.scaleVelocity += (w*cosv - g*sinv - c.scaleOffset*e) * m
							c.scaleOffset += c.scaleVelocity * t
							c.scaleVelocity *= d
						}
						if rotateOrShearX {
							c.rotateVelocity -= ((w*sinv+g*cosv)*h + c.rotateOffset*e) * m
							c.rotateOffset += c.rotateVelocity * t
							c.rotateVelocity *= d
							if a < t {
								break
							}
							r := c.rotateOffset*mr + ca
							cosv = cos(r)
							sinv = sin(r)
						} else if a < t {
							break
						}
					}
				}
			}
			c.remaining = a
		}
		c.cx = bone.WorldX
		c.cy = bone.WorldY
	case PhysicsPose:
		if x {
			bone.WorldX += c.xOffset * mix * c.Data.X
		}
		if y {
			bone.WorldY += c.yOffset * mix * c.Data.Y
		}
	}

	if rotateOrShearX {
		o := c.rotateOffset * mix
		var sinv, cosv, a float32
		if c.Data.ShearX > 0 {
			r := float32(0)
			if c.Data.Rotate > 0 {
				r = o * c.Data.Rotate
				sinv = sin(r)
				cosv = cos(r)
				a = bone.B
				bone.B = cosv*a - sinv*bone.D
				bone.D = sinv*a + cosv*bone.D
			}
			r += o * c.Data.ShearX
			sinv = sin(r)
			cosv = cos(r)
			a = bone.A
			bone.A = cosv*a - sinv*bone.C
			bone.C = sinv*a + cosv*bone.C
		} else {
			o *= c.Data.Rotate
			sinv = sin(o)
			cosv = cos(o)
			a = bone.A
			bone.A = cosv*a - sinv*bone.C
			bone.C = sinv*a + cosv*bone.C
			a = bone.B
			bone.B = cosv*a - sinv*bone.D
			bone.D = sinv*a + cosv*bone.D
		}
	}
	if scaleX {
		s := 1 + c.scaleOffset*mix*c.Data.ScaleX
		bone.A *= s
		bone.C *= s
	}
	if physics != PhysicsPose {
		c.tx = l * bone.A
		c.ty = l * bone.C
	}
	bone.UpdateAppliedTransform()
}
