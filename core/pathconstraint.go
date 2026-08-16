package spine

const (
	pathNone   = -1
	pathBefore = -2
	pathAfter  = -3
)

// PathConstraint stores the current pose for a path constraint. A path
// constraint adjusts the rotation, translation, and scale of the constrained
// bones so they follow a PathAttachment.
type PathConstraint struct {
	Data *PathConstraintData

	// Bones that will be modified by this path constraint.
	Bones []*Bone

	// Target is the slot whose path attachment will be used to constrain the bones.
	Target *Slot

	// Position is the position along the path.
	Position float32

	// Spacing is the spacing between bones.
	Spacing float32

	// MixRotate, MixX, MixY are percentages (0-1) that control the mix between
	// the constrained and unconstrained values.
	MixRotate, MixX, MixY float32

	active bool

	spaces, positions []float32
	world, curves     []float32
	lengths           []float32
	segments          [10]float32
}

// NewPathConstraint creates a path constraint.
func NewPathConstraint(data *PathConstraintData, skeleton *Skeleton) *PathConstraint {
	if data == nil {
		panic("spine: data cannot be nil")
	}
	if skeleton == nil {
		panic("spine: skeleton cannot be nil")
	}
	c := &PathConstraint{Data: data}
	c.Bones = make([]*Bone, 0, len(data.Bones))
	for _, boneData := range data.Bones {
		c.Bones = append(c.Bones, skeleton.Bones[boneData.Index])
	}
	c.Target = skeleton.Slots[data.Target.Index]
	c.Position = data.Position
	c.Spacing = data.Spacing
	c.MixRotate = data.MixRotate
	c.MixX = data.MixX
	c.MixY = data.MixY
	return c
}

// SetToSetupPose sets the constraint to its setup pose values.
func (c *PathConstraint) SetToSetupPose() {
	data := c.Data
	c.Position = data.Position
	c.Spacing = data.Spacing
	c.MixRotate = data.MixRotate
	c.MixX = data.MixX
	c.MixY = data.MixY
}

// IsActive implements Updatable.
func (c *PathConstraint) IsActive() bool { return c.active }

func (c *PathConstraint) String() string { return c.Data.Name }

// Update applies the constraint to the constrained bones.
func (c *PathConstraint) Update(physics Physics) {
	path, ok := c.Target.attachment.(*PathAttachment)
	if !ok {
		return
	}

	mixRotate, mixX, mixY := c.MixRotate, c.MixX, c.MixY
	if mixRotate == 0 && mixX == 0 && mixY == 0 {
		return
	}

	data := c.Data
	tangents := data.RotateMode == RotateModeTangent
	scale := data.RotateMode == RotateModeChainScale
	boneCount := len(c.Bones)
	spacesCount := boneCount + 1
	if tangents {
		spacesCount = boneCount
	}
	bones := c.Bones
	spaces := ensureSize(&c.spaces, spacesCount)
	var lengths []float32
	if scale {
		lengths = ensureSize(&c.lengths, boneCount)
	}
	spacing := c.Spacing

	switch data.SpacingMode {
	case SpacingModePercent:
		if scale {
			for i, n := 0, spacesCount-1; i < n; i++ {
				bone := bones[i]
				setupLength := bone.Data.Length
				x, y := setupLength*bone.A, setupLength*bone.C
				lengths[i] = sqrt(x*x + y*y)
			}
		}
		for i := 1; i < spacesCount; i++ {
			spaces[i] = spacing
		}
	case SpacingModeProportional:
		sum := float32(0)
		for i, n := 0, spacesCount-1; i < n; {
			bone := bones[i]
			setupLength := bone.Data.Length
			if setupLength < epsilon {
				if scale {
					lengths[i] = 0
				}
				i++
				spaces[i] = spacing
			} else {
				x, y := setupLength*bone.A, setupLength*bone.C
				length := sqrt(x*x + y*y)
				if scale {
					lengths[i] = length
				}
				i++
				spaces[i] = length
				sum += length
			}
		}
		if sum > 0 {
			sum = float32(spacesCount) / sum * spacing
			for i := 1; i < spacesCount; i++ {
				spaces[i] *= sum
			}
		}
	default:
		lengthSpacing := data.SpacingMode == SpacingModeLength
		for i, n := 0, spacesCount-1; i < n; {
			bone := bones[i]
			setupLength := bone.Data.Length
			if setupLength < epsilon {
				if scale {
					lengths[i] = 0
				}
				i++
				spaces[i] = spacing
			} else {
				x, y := setupLength*bone.A, setupLength*bone.C
				length := sqrt(x*x + y*y)
				if scale {
					lengths[i] = length
				}
				i++
				if lengthSpacing {
					spaces[i] = (setupLength + spacing) * length / setupLength
				} else {
					spaces[i] = spacing * length / setupLength
				}
			}
		}
	}

	positions := c.computeWorldPositions(path, spacesCount, tangents)
	boneX, boneY := positions[0], positions[1]
	offsetRotation := data.OffsetRotation
	var tip bool
	if offsetRotation == 0 {
		tip = data.RotateMode == RotateModeChain
	} else {
		tip = false
		p := c.Target.Bone
		if p.A*p.D-p.B*p.C > 0 {
			offsetRotation *= degRad
		} else {
			offsetRotation *= -degRad
		}
	}
	for i, p := 0, 3; i < boneCount; i, p = i+1, p+3 {
		bone := bones[i]
		bone.WorldX += (boneX - bone.WorldX) * mixX
		bone.WorldY += (boneY - bone.WorldY) * mixY
		x, y := positions[p], positions[p+1]
		dx, dy := x-boneX, y-boneY
		if scale {
			length := lengths[i]
			if length >= epsilon {
				s := (sqrt(dx*dx+dy*dy)/length-1)*mixRotate + 1
				bone.A *= s
				bone.C *= s
			}
		}
		boneX = x
		boneY = y
		if mixRotate > 0 {
			a, b, cc, d := bone.A, bone.B, bone.C, bone.D
			var r, cosr, sinr float32
			if tangents {
				r = positions[p-1]
			} else if spaces[i+1] < epsilon {
				r = positions[p+2]
			} else {
				r = atan2(dy, dx)
			}
			r -= atan2(cc, a)
			if tip {
				cosr = cos(r)
				sinr = sin(r)
				length := bone.Data.Length
				boneX += (length*(cosr*a-sinr*cc) - dx) * mixRotate
				boneY += (length*(sinr*a+cosr*cc) - dy) * mixRotate
			} else {
				r += offsetRotation
			}
			if r > piF {
				r -= pi2
			} else if r < -piF {
				r += pi2
			}
			r *= mixRotate
			cosr = cos(r)
			sinr = sin(r)
			bone.A = cosr*a - sinr*cc
			bone.B = cosr*b - sinr*d
			bone.C = sinr*a + cosr*cc
			bone.D = sinr*b + cosr*d
		}
		bone.UpdateAppliedTransform()
	}
}

func (c *PathConstraint) computeWorldPositions(path *PathAttachment, spacesCount int, tangents bool) []float32 {
	target := c.Target
	position := c.Position
	spaces := c.spaces
	out := ensureSize(&c.positions, spacesCount*3+2)
	var world []float32
	closed := path.Closed
	verticesLength := path.WorldVerticesLength
	curveCount := verticesLength / 6
	prevCurve := pathNone

	if !path.ConstantSpeed {
		lengths := path.Lengths
		if closed {
			curveCount -= 1
		} else {
			curveCount -= 2
		}
		pathLength := lengths[curveCount]

		if c.Data.PositionMode == PositionModePercent {
			position *= pathLength
		}

		var multiplier float32
		switch c.Data.SpacingMode {
		case SpacingModePercent:
			multiplier = pathLength
		case SpacingModeProportional:
			multiplier = pathLength / float32(spacesCount)
		default:
			multiplier = 1
		}

		world = ensureSize(&c.world, 8)
		for i, o, curve := 0, 0, 0; i < spacesCount; i, o = i+1, o+3 {
			space := spaces[i] * multiplier
			position += space
			p := position

			if closed {
				p = mod32(p, pathLength)
				if p < 0 {
					p += pathLength
				}
				curve = 0
			} else if p < 0 {
				if prevCurve != pathBefore {
					prevCurve = pathBefore
					path.ComputeWorldVertices(target, 2, 4, world, 0, 2)
				}
				addBeforePosition(p, world, 0, out, o)
				continue
			} else if p > pathLength {
				if prevCurve != pathAfter {
					prevCurve = pathAfter
					path.ComputeWorldVertices(target, verticesLength-6, 4, world, 0, 2)
				}
				addAfterPosition(p-pathLength, world, 0, out, o)
				continue
			}

			// Determine curve containing position.
			for ; ; curve++ {
				length := lengths[curve]
				if p > length {
					continue
				}
				if curve == 0 {
					p /= length
				} else {
					prev := lengths[curve-1]
					p = (p - prev) / (length - prev)
				}
				break
			}
			if curve != prevCurve {
				prevCurve = curve
				if closed && curve == curveCount {
					path.ComputeWorldVertices(target, verticesLength-4, 4, world, 0, 2)
					path.ComputeWorldVertices(target, 0, 4, world, 4, 2)
				} else {
					path.ComputeWorldVertices(target, curve*6+2, 8, world, 0, 2)
				}
			}
			addCurvePosition(p, world[0], world[1], world[2], world[3], world[4], world[5], world[6], world[7], out, o,
				tangents || (i > 0 && space < epsilon))
		}
		return out
	}

	// World vertices.
	if closed {
		verticesLength += 2
		world = ensureSize(&c.world, verticesLength)
		path.ComputeWorldVertices(target, 2, verticesLength-4, world, 0, 2)
		path.ComputeWorldVertices(target, 0, 2, world, verticesLength-4, 2)
		world[verticesLength-2] = world[0]
		world[verticesLength-1] = world[1]
	} else {
		curveCount--
		verticesLength -= 4
		world = ensureSize(&c.world, verticesLength)
		path.ComputeWorldVertices(target, 2, verticesLength, world, 0, 2)
	}

	// Curve lengths.
	curves := ensureSize(&c.curves, curveCount)
	pathLength := float32(0)
	x1, y1 := world[0], world[1]
	var cx1, cy1, cx2, cy2, x2, y2 float32
	var tmpx, tmpy, dddfx, dddfy, ddfx, ddfy, dfx, dfy float32
	for i, w := 0, 2; i < curveCount; i, w = i+1, w+6 {
		cx1 = world[w]
		cy1 = world[w+1]
		cx2 = world[w+2]
		cy2 = world[w+3]
		x2 = world[w+4]
		y2 = world[w+5]
		tmpx = (x1 - cx1*2 + cx2) * 0.1875
		tmpy = (y1 - cy1*2 + cy2) * 0.1875
		dddfx = ((cx1-cx2)*3 - x1 + x2) * 0.09375
		dddfy = ((cy1-cy2)*3 - y1 + y2) * 0.09375
		ddfx = tmpx*2 + dddfx
		ddfy = tmpy*2 + dddfy
		dfx = (cx1-x1)*0.75 + tmpx + dddfx*0.16666667
		dfy = (cy1-y1)*0.75 + tmpy + dddfy*0.16666667
		pathLength += sqrt(dfx*dfx + dfy*dfy)
		dfx += ddfx
		dfy += ddfy
		ddfx += dddfx
		ddfy += dddfy
		pathLength += sqrt(dfx*dfx + dfy*dfy)
		dfx += ddfx
		dfy += ddfy
		pathLength += sqrt(dfx*dfx + dfy*dfy)
		dfx += ddfx + dddfx
		dfy += ddfy + dddfy
		pathLength += sqrt(dfx*dfx + dfy*dfy)
		curves[i] = pathLength
		x1 = x2
		y1 = y2
	}

	if c.Data.PositionMode == PositionModePercent {
		position *= pathLength
	}

	var multiplier float32
	switch c.Data.SpacingMode {
	case SpacingModePercent:
		multiplier = pathLength
	case SpacingModeProportional:
		multiplier = pathLength / float32(spacesCount)
	default:
		multiplier = 1
	}

	segments := &c.segments
	curveLength := float32(0)
	for i, o, curve, segment := 0, 0, 0, 0; i < spacesCount; i, o = i+1, o+3 {
		space := spaces[i] * multiplier
		position += space
		p := position

		if closed {
			p = mod32(p, pathLength)
			if p < 0 {
				p += pathLength
			}
			curve = 0
		} else if p < 0 {
			addBeforePosition(p, world, 0, out, o)
			continue
		} else if p > pathLength {
			addAfterPosition(p-pathLength, world, verticesLength-4, out, o)
			continue
		}

		// Determine curve containing position.
		for ; ; curve++ {
			length := curves[curve]
			if p > length {
				continue
			}
			if curve == 0 {
				p /= length
			} else {
				prev := curves[curve-1]
				p = (p - prev) / (length - prev)
			}
			break
		}

		// Curve segment lengths.
		if curve != prevCurve {
			prevCurve = curve
			ii := curve * 6
			x1 = world[ii]
			y1 = world[ii+1]
			cx1 = world[ii+2]
			cy1 = world[ii+3]
			cx2 = world[ii+4]
			cy2 = world[ii+5]
			x2 = world[ii+6]
			y2 = world[ii+7]
			tmpx = (x1 - cx1*2 + cx2) * 0.03
			tmpy = (y1 - cy1*2 + cy2) * 0.03
			dddfx = ((cx1-cx2)*3 - x1 + x2) * 0.006
			dddfy = ((cy1-cy2)*3 - y1 + y2) * 0.006
			ddfx = tmpx*2 + dddfx
			ddfy = tmpy*2 + dddfy
			dfx = (cx1-x1)*0.3 + tmpx + dddfx*0.16666667
			dfy = (cy1-y1)*0.3 + tmpy + dddfy*0.16666667
			curveLength = sqrt(dfx*dfx + dfy*dfy)
			segments[0] = curveLength
			for ii = 1; ii < 8; ii++ {
				dfx += ddfx
				dfy += ddfy
				ddfx += dddfx
				ddfy += dddfy
				curveLength += sqrt(dfx*dfx + dfy*dfy)
				segments[ii] = curveLength
			}
			dfx += ddfx
			dfy += ddfy
			curveLength += sqrt(dfx*dfx + dfy*dfy)
			segments[8] = curveLength
			dfx += ddfx + dddfx
			dfy += ddfy + dddfy
			curveLength += sqrt(dfx*dfx + dfy*dfy)
			segments[9] = curveLength
			segment = 0
		}

		// Weight by segment length.
		p *= curveLength
		for ; ; segment++ {
			length := segments[segment]
			if p > length {
				continue
			}
			if segment == 0 {
				p /= length
			} else {
				prev := segments[segment-1]
				p = float32(segment) + (p-prev)/(length-prev)
			}
			break
		}
		addCurvePosition(p*0.1, x1, y1, cx1, cy1, cx2, cy2, x2, y2, out, o, tangents || (i > 0 && space < epsilon))
	}
	return out
}

func addBeforePosition(p float32, temp []float32, i int, out []float32, o int) {
	x1, y1 := temp[i], temp[i+1]
	dx, dy := temp[i+2]-x1, temp[i+3]-y1
	r := atan2(dy, dx)
	out[o] = x1 + p*cos(r)
	out[o+1] = y1 + p*sin(r)
	out[o+2] = r
}

func addAfterPosition(p float32, temp []float32, i int, out []float32, o int) {
	x1, y1 := temp[i+2], temp[i+3]
	dx, dy := x1-temp[i], y1-temp[i+1]
	r := atan2(dy, dx)
	out[o] = x1 + p*cos(r)
	out[o+1] = y1 + p*sin(r)
	out[o+2] = r
}

func addCurvePosition(p, x1, y1, cx1, cy1, cx2, cy2, x2, y2 float32, out []float32, o int, tangents bool) {
	if p < epsilon || isNaN32(p) {
		out[o] = x1
		out[o+1] = y1
		out[o+2] = atan2(cy1-y1, cx1-x1)
		return
	}
	tt := p * p
	ttt := tt * p
	u := 1 - p
	uu := u * u
	uuu := uu * u
	ut := u * p
	ut3 := ut * 3
	uut3 := u * ut3
	utt3 := ut3 * p
	x := x1*uuu + cx1*uut3 + cx2*utt3 + x2*ttt
	y := y1*uuu + cy1*uut3 + cy2*utt3 + y2*ttt
	out[o] = x
	out[o+1] = y
	if tangents {
		if p < 0.001 {
			out[o+2] = atan2(cy1-y1, cx1-x1)
		} else {
			out[o+2] = atan2(y-(y1*uu+cy1*ut*2+cy2*tt), x-(x1*uu+cx1*ut*2+cx2*tt))
		}
	}
}
