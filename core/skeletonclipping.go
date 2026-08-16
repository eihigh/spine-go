package spine

// SkeletonClipping performs triangle clipping against the polygons of a
// ClippingAttachment. Ported from spine-libgdx's SkeletonClipping.
type SkeletonClipping struct {
	triangulator     *Triangulator
	clippingPolygon  []float32
	clipOutput       []float32
	clippedVertices  []float32
	clippedUVs       []float32
	clippedTriangles []uint16
	scratch          []float32

	clipAttachment   *ClippingAttachment
	clippingPolygons [][]float32
}

// NewSkeletonClipping creates a skeleton clipper.
func NewSkeletonClipping() *SkeletonClipping {
	return &SkeletonClipping{
		triangulator:     NewTriangulator(),
		clipOutput:       make([]float32, 0, 128),
		clippedVertices:  make([]float32, 0, 128),
		clippedUVs:       make([]float32, 0, 128),
		clippedTriangles: make([]uint16, 0, 128),
	}
}

// ClipStart sets the clipping attachment to be used until ClipEnd, computing
// its world clipping polygons. It returns the number of clipping polygons, or
// 0 if clipping was not started (Java returns void; the count matches
// spine-ts).
func (c *SkeletonClipping) ClipStart(slot *Slot, clip *ClippingAttachment) int {
	if c.clipAttachment != nil {
		return 0
	}
	n := clip.WorldVerticesLength
	if n < 6 {
		return 0
	}
	c.clipAttachment = clip

	vertices := ensureSize(&c.clippingPolygon, n)
	clip.ComputeWorldVertices(slot, 0, n, vertices, 0, 2)
	makeClockwise(c.clippingPolygon)
	triangles := c.triangulator.Triangulate(c.clippingPolygon)
	c.clippingPolygons = c.triangulator.Decompose(c.clippingPolygon, triangles)
	for i, polygon := range c.clippingPolygons {
		makeClockwise(polygon)
		polygon = append(polygon, polygon[0], polygon[1])
		c.clippingPolygons[i] = polygon
	}
	return len(c.clippingPolygons)
}

// ClipEndSlot ends clipping if the specified slot is the clipping attachment's
// end slot (Java clipEnd(Slot)).
func (c *SkeletonClipping) ClipEndSlot(slot *Slot) {
	if c.clipAttachment != nil && c.clipAttachment.EndSlot == slot.Data {
		c.ClipEnd()
	}
}

// ClipEnd ends clipping, clearing the clipping state.
func (c *SkeletonClipping) ClipEnd() {
	if c.clipAttachment == nil {
		return
	}
	c.clipAttachment = nil
	c.clippingPolygons = nil
	c.clippedVertices = c.clippedVertices[:0]
	c.clippedUVs = c.clippedUVs[:0]
	c.clippedTriangles = c.clippedTriangles[:0]
	c.clippingPolygon = c.clippingPolygon[:0]
}

// IsClipping returns true between ClipStart and ClipEnd.
func (c *SkeletonClipping) IsClipping() bool {
	return c.clipAttachment != nil
}

// ClipTriangles clips the triangles of the x,y vertices against the clipping
// polygons, storing the results in ClippedVertices and ClippedTriangles.
func (c *SkeletonClipping) ClipTriangles(vertices []float32, triangles []uint16, trianglesLength int) {
	polygons := c.clippingPolygons
	polygonsCount := len(polygons)

	var index uint16
	clippedVertices := c.clippedVertices[:0]
	c.clippedUVs = c.clippedUVs[:0]
	clippedTriangles := c.clippedTriangles[:0]
	for i := 0; i < trianglesLength; i += 3 {
		vertexOffset := int(triangles[i]) << 1
		x1, y1 := vertices[vertexOffset], vertices[vertexOffset+1]

		vertexOffset = int(triangles[i+1]) << 1
		x2, y2 := vertices[vertexOffset], vertices[vertexOffset+1]

		vertexOffset = int(triangles[i+2]) << 1
		x3, y3 := vertices[vertexOffset], vertices[vertexOffset+1]

		for p := 0; p < polygonsCount; p++ {
			if c.clip(x1, y1, x2, y2, x3, y3, polygons[p]) {
				clipOutput := c.clipOutput
				clipOutputLength := len(clipOutput)
				if clipOutputLength == 0 {
					continue
				}

				clipOutputCount := clipOutputLength >> 1
				for ii := 0; ii < clipOutputLength; ii += 2 {
					clippedVertices = append(clippedVertices, clipOutput[ii], clipOutput[ii+1])
				}

				clipOutputCount--
				for ii := 1; ii < clipOutputCount; ii++ {
					clippedTriangles = append(clippedTriangles, index, index+uint16(ii), index+uint16(ii)+1)
				}
				index += uint16(clipOutputCount) + 1

			} else {
				clippedVertices = append(clippedVertices, x1, y1, x2, y2, x3, y3)
				clippedTriangles = append(clippedTriangles, index, index+1, index+2)
				index += 3
				break
			}
		}
	}
	c.clippedVertices = clippedVertices
	c.clippedTriangles = clippedTriangles
}

// ClipTrianglesPacked clips the triangles, storing packed renderer vertices
// (x, y, light, [dark,] u, v) in ClippedVertices (Java
// clipTriangles(vertices, triangles, trianglesLength, uvs, light, dark,
// twoColor)).
func (c *SkeletonClipping) ClipTrianglesPacked(vertices []float32, triangles []uint16, trianglesLength int,
	uvs []float32, light, dark float32, twoColor bool) {

	polygons := c.clippingPolygons
	polygonsCount := len(polygons)

	var index uint16
	clippedVertices := c.clippedVertices[:0]
	c.clippedUVs = c.clippedUVs[:0]
	clippedTriangles := c.clippedTriangles[:0]
	for i := 0; i < trianglesLength; i += 3 {
		vertexOffset := int(triangles[i]) << 1
		x1, y1 := vertices[vertexOffset], vertices[vertexOffset+1]
		u1, v1 := uvs[vertexOffset], uvs[vertexOffset+1]

		vertexOffset = int(triangles[i+1]) << 1
		x2, y2 := vertices[vertexOffset], vertices[vertexOffset+1]
		u2, v2 := uvs[vertexOffset], uvs[vertexOffset+1]

		vertexOffset = int(triangles[i+2]) << 1
		x3, y3 := vertices[vertexOffset], vertices[vertexOffset+1]
		u3, v3 := uvs[vertexOffset], uvs[vertexOffset+1]

		for p := 0; p < polygonsCount; p++ {
			if c.clip(x1, y1, x2, y2, x3, y3, polygons[p]) {
				clipOutput := c.clipOutput
				clipOutputLength := len(clipOutput)
				if clipOutputLength == 0 {
					continue
				}
				d0, d1, d2, d4 := y2-y3, x3-x2, x1-x3, y3-y1
				d := 1 / (d0*d2 + d1*(y1-y3))

				clipOutputCount := clipOutputLength >> 1
				for ii := 0; ii < clipOutputLength; ii += 2 {
					x, y := clipOutput[ii], clipOutput[ii+1]
					clippedVertices = append(clippedVertices, x, y, light)
					if twoColor {
						clippedVertices = append(clippedVertices, dark)
					}
					c0, c1 := x-x3, y-y3
					a := (d0*c0 + d1*c1) * d
					b := (d4*c0 + d2*c1) * d
					cc := 1 - a - b
					clippedVertices = append(clippedVertices, u1*a+u2*b+u3*cc, v1*a+v2*b+v3*cc)
				}

				clipOutputCount--
				for ii := 1; ii < clipOutputCount; ii++ {
					clippedTriangles = append(clippedTriangles, index, index+uint16(ii), index+uint16(ii)+1)
				}
				index += uint16(clipOutputCount) + 1

			} else {
				if !twoColor {
					clippedVertices = append(clippedVertices,
						x1, y1, light, u1, v1,
						x2, y2, light, u2, v2,
						x3, y3, light, u3, v3)
				} else {
					clippedVertices = append(clippedVertices,
						x1, y1, light, dark, u1, v1,
						x2, y2, light, dark, u2, v2,
						x3, y3, light, dark, u3, v3)
				}
				clippedTriangles = append(clippedTriangles, index, index+1, index+2)
				index += 3
				break
			}
		}
	}
	c.clippedVertices = clippedVertices
	c.clippedTriangles = clippedTriangles
}

// ClipTrianglesUV clips the triangles along with their UVs, storing x,y pairs
// in ClippedVertices and u,v pairs in ClippedUVs (Java clipTrianglesUnpacked
// with vertexStart 0).
func (c *SkeletonClipping) ClipTrianglesUV(vertices []float32, triangles []uint16, trianglesLength int, uvs []float32) {
	polygons := c.clippingPolygons
	polygonsCount := len(polygons)

	var index uint16
	clippedVertices := c.clippedVertices[:0]
	clippedUVs := c.clippedUVs[:0]
	clippedTriangles := c.clippedTriangles[:0]
	for i := 0; i < trianglesLength; i += 3 {
		vertexOffset := int(triangles[i]) << 1
		x1, y1 := vertices[vertexOffset], vertices[vertexOffset+1]
		u1, v1 := uvs[vertexOffset], uvs[vertexOffset+1]

		vertexOffset = int(triangles[i+1]) << 1
		x2, y2 := vertices[vertexOffset], vertices[vertexOffset+1]
		u2, v2 := uvs[vertexOffset], uvs[vertexOffset+1]

		vertexOffset = int(triangles[i+2]) << 1
		x3, y3 := vertices[vertexOffset], vertices[vertexOffset+1]
		u3, v3 := uvs[vertexOffset], uvs[vertexOffset+1]

		for p := 0; p < polygonsCount; p++ {
			if c.clip(x1, y1, x2, y2, x3, y3, polygons[p]) {
				clipOutput := c.clipOutput
				clipOutputLength := len(clipOutput)
				if clipOutputLength == 0 {
					continue
				}
				d0, d1, d2, d4 := y2-y3, x3-x2, x1-x3, y3-y1
				d := 1 / (d0*d2 + d1*(y1-y3))

				clipOutputCount := clipOutputLength >> 1
				for ii := 0; ii < clipOutputLength; ii += 2 {
					x, y := clipOutput[ii], clipOutput[ii+1]
					clippedVertices = append(clippedVertices, x, y)

					c0, c1 := x-x3, y-y3
					a := (d0*c0 + d1*c1) * d
					b := (d4*c0 + d2*c1) * d
					cc := 1 - a - b
					clippedUVs = append(clippedUVs, u1*a+u2*b+u3*cc, v1*a+v2*b+v3*cc)
				}

				clipOutputCount--
				for ii := 1; ii < clipOutputCount; ii++ {
					clippedTriangles = append(clippedTriangles, index, index+uint16(ii), index+uint16(ii)+1)
				}
				index += uint16(clipOutputCount) + 1
			} else {
				clippedVertices = append(clippedVertices, x1, y1, x2, y2, x3, y3)
				clippedUVs = append(clippedUVs, u1, v1, u2, v2, u3, v3)
				clippedTriangles = append(clippedTriangles, index, index+1, index+2)
				index += 3
				break
			}
		}
	}
	c.clippedVertices = clippedVertices
	c.clippedUVs = clippedUVs
	c.clippedTriangles = clippedTriangles
}

// clip clips the input triangle against the convex, clockwise clipping area.
// If the triangle lies entirely within the clipping area, false is returned.
// The clipping area must duplicate the first vertex at the end of the vertices
// list. The result is left in c.clipOutput.
func (c *SkeletonClipping) clip(x1, y1, x2, y2, x3, y3 float32, clippingArea []float32) bool {
	clipped := false

	// Avoid copy at the end.
	var input, output []float32
	outputIsOriginal := false
	if len(clippingArea)%4 >= 2 {
		input = c.clipOutput
		output = c.scratch
	} else {
		input = c.scratch
		output = c.clipOutput
		outputIsOriginal = true
	}

	input = append(input[:0], x1, y1, x2, y2, x3, y3, x1, y1)
	output = output[:0]

	clippingVerticesLast := len(clippingArea) - 4
	for i := 0; ; i += 2 {
		edgeX, edgeY := clippingArea[i], clippingArea[i+1]
		ex, ey := edgeX-clippingArea[i+2], edgeY-clippingArea[i+3]

		outputStart := len(output)
		for ii, nn := 0, len(input)-2; ii < nn; {
			inputX, inputY := input[ii], input[ii+1]
			ii += 2
			inputX2, inputY2 := input[ii], input[ii+1]
			s2 := ey*(edgeX-inputX2) > ex*(edgeY-inputY2)
			s1 := ey*(edgeX-inputX) - ex*(edgeY-inputY)
			if s1 > 0 {
				if s2 { // v1 inside, v2 inside
					output = append(output, inputX2, inputY2)
					continue
				}
				// v1 inside, v2 outside
				ix, iy := inputX2-inputX, inputY2-inputY
				t := s1 / (ix*ey - iy*ex)
				if t >= 0 && t <= 1 {
					output = append(output, inputX+ix*t, inputY+iy*t)
				} else {
					output = append(output, inputX2, inputY2)
					continue
				}
			} else if s2 { // v1 outside, v2 inside
				ix, iy := inputX2-inputX, inputY2-inputY
				t := s1 / (ix*ey - iy*ex)
				if t >= 0 && t <= 1 {
					output = append(output, inputX+ix*t, inputY+iy*t)
					output = append(output, inputX2, inputY2)
				} else {
					output = append(output, inputX2, inputY2)
					continue
				}
			}
			clipped = true
		}

		if outputStart == len(output) { // All edges outside.
			if outputIsOriginal {
				c.clipOutput = output[:0]
				c.scratch = input
			} else {
				c.clipOutput = input[:0]
				c.scratch = output
			}
			return true
		}

		output = append(output, output[0], output[1])

		if i == clippingVerticesLast {
			break
		}
		input, output = output, input[:0]
		outputIsOriginal = !outputIsOriginal
	}

	if !outputIsOriginal {
		original := append(input[:0], output[:len(output)-2]...)
		c.clipOutput = original
		c.scratch = output
	} else {
		c.clipOutput = output[:len(output)-2]
		c.scratch = input
	}

	return clipped
}

// ClippedVertices returns the internal clipped vertices buffer from the last
// ClipTriangles call.
func (c *SkeletonClipping) ClippedVertices() []float32 {
	return c.clippedVertices
}

// ClippedUVs returns the internal clipped UVs buffer. Only non-empty if
// ClipTrianglesUV was used.
func (c *SkeletonClipping) ClippedUVs() []float32 {
	return c.clippedUVs
}

// ClippedTriangles returns the internal clipped triangles buffer from the last
// ClipTriangles call.
func (c *SkeletonClipping) ClippedTriangles() []uint16 {
	return c.clippedTriangles
}

// makeClockwise reverses the polygon's winding if it is counterclockwise.
func makeClockwise(polygon []float32) {
	verticesLength := len(polygon)

	area := polygon[verticesLength-2]*polygon[1] - polygon[0]*polygon[verticesLength-1]
	var p1x, p1y, p2x, p2y float32
	for i, n := 0, verticesLength-3; i < n; i += 2 {
		p1x = polygon[i]
		p1y = polygon[i+1]
		p2x = polygon[i+2]
		p2y = polygon[i+3]
		area += p1x*p2y - p2x*p1y
	}
	if area < 0 {
		return
	}

	for i, lastX, n := 0, verticesLength-2, verticesLength>>1; i < n; i += 2 {
		x, y := polygon[i], polygon[i+1]
		other := lastX - i
		polygon[i] = polygon[other]
		polygon[i+1] = polygon[other+1]
		polygon[other] = x
		polygon[other+1] = y
	}
}
