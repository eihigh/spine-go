package spine

// Triangulator triangulates a polygon using ear clipping and decomposes
// triangles into convex polygons. Ported from spine-libgdx's Triangulator.
// The returned slices are internal buffers reused across calls.
type Triangulator struct {
	convexPolygons        [][]float32
	convexPolygonsIndices [][]int

	indices   []int
	isConcave []bool
	triangles []uint16

	polygonPool        [][]float32
	polygonIndicesPool [][]int
}

// NewTriangulator creates a triangulator.
func NewTriangulator() *Triangulator {
	return &Triangulator{
		convexPolygons:        make([][]float32, 0, 16),
		convexPolygonsIndices: make([][]int, 0, 16),
	}
}

func (t *Triangulator) obtainPolygon() []float32 {
	if n := len(t.polygonPool); n > 0 {
		p := t.polygonPool[n-1]
		t.polygonPool = t.polygonPool[:n-1]
		return p
	}
	return make([]float32, 0, 16)
}

func (t *Triangulator) freePolygon(polygon []float32) {
	t.polygonPool = append(t.polygonPool, polygon)
}

func (t *Triangulator) obtainPolygonIndices() []int {
	if n := len(t.polygonIndicesPool); n > 0 {
		p := t.polygonIndicesPool[n-1]
		t.polygonIndicesPool = t.polygonIndicesPool[:n-1]
		return p
	}
	return make([]int, 0, 16)
}

func (t *Triangulator) freePolygonIndices(polygonIndices []int) {
	t.polygonIndicesPool = append(t.polygonIndicesPool, polygonIndices)
}

// Triangulate triangulates the polygon (pairs of x,y world vertices), returning
// vertex index triples. The returned slice is an internal buffer reused across
// calls.
func (t *Triangulator) Triangulate(vertices []float32) []uint16 {
	vertexCount := len(vertices) >> 1

	indices := t.indices[:0]
	for i := 0; i < vertexCount; i++ {
		indices = append(indices, i)
	}

	if cap(t.isConcave) < vertexCount {
		t.isConcave = make([]bool, vertexCount)
	}
	isConcaveArray := t.isConcave[:vertexCount]
	for i, n := 0, vertexCount; i < n; i++ {
		isConcaveArray[i] = isConcave(i, vertexCount, vertices, indices)
	}

	triangles := t.triangles[:0]

	for vertexCount > 3 {
		// Find ear tip.
		previous, i, next := vertexCount-1, 0, 1
		for {
			found := false
			if !isConcaveArray[i] {
				p1, p2, p3 := indices[previous]<<1, indices[i]<<1, indices[next]<<1
				p1x, p1y := vertices[p1], vertices[p1+1]
				p2x, p2y := vertices[p2], vertices[p2+1]
				p3x, p3y := vertices[p3], vertices[p3+1]
				found = true
				for ii := (next + 1) % vertexCount; ii != previous; ii = (ii + 1) % vertexCount {
					if !isConcaveArray[ii] {
						continue
					}
					v := indices[ii] << 1
					vx, vy := vertices[v], vertices[v+1]
					if positiveArea(p3x, p3y, p1x, p1y, vx, vy) {
						if positiveArea(p1x, p1y, p2x, p2y, vx, vy) {
							if positiveArea(p2x, p2y, p3x, p3y, vx, vy) {
								found = false
								break
							}
						}
					}
				}
				if found {
					break
				}
			}

			if next == 0 {
				for {
					if !isConcaveArray[i] {
						break
					}
					i--
					if i <= 0 {
						break
					}
				}
				break
			}

			previous = i
			i = next
			next = (next + 1) % vertexCount
		}

		// Cut ear tip.
		triangles = append(triangles, uint16(indices[(vertexCount+i-1)%vertexCount]))
		triangles = append(triangles, uint16(indices[i]))
		triangles = append(triangles, uint16(indices[(i+1)%vertexCount]))
		indices = append(indices[:i], indices[i+1:]...)
		isConcaveArray = append(isConcaveArray[:i], isConcaveArray[i+1:]...)
		vertexCount--

		previousIndex := (vertexCount + i - 1) % vertexCount
		nextIndex := i
		if i == vertexCount {
			nextIndex = 0
		}
		isConcaveArray[previousIndex] = isConcave(previousIndex, vertexCount, vertices, indices)
		isConcaveArray[nextIndex] = isConcave(nextIndex, vertexCount, vertices, indices)
	}

	if vertexCount == 3 {
		triangles = append(triangles, uint16(indices[2]))
		triangles = append(triangles, uint16(indices[0]))
		triangles = append(triangles, uint16(indices[1]))
	}

	t.indices = indices
	t.triangles = triangles
	return triangles
}

// Decompose decomposes the triangulation into convex polygons (each a slice of
// x,y vertex pairs). The returned slice and its polygons are internal buffers
// reused across calls.
func (t *Triangulator) Decompose(vertices []float32, triangles []uint16) [][]float32 {
	convexPolygons := t.convexPolygons
	for _, p := range convexPolygons {
		t.freePolygon(p)
	}
	convexPolygons = convexPolygons[:0]

	convexPolygonsIndices := t.convexPolygonsIndices
	for _, p := range convexPolygonsIndices {
		t.freePolygonIndices(p)
	}
	convexPolygonsIndices = convexPolygonsIndices[:0]

	polygonIndices := t.obtainPolygonIndices()[:0]
	polygon := t.obtainPolygon()[:0]

	// Merge subsequent triangles if they form a triangle fan.
	fanBaseIndex, lastWinding := -1, 0
	for i, n := 0, len(triangles); i < n; i += 3 {
		t1, t2, t3 := int(triangles[i])<<1, int(triangles[i+1])<<1, int(triangles[i+2])<<1
		x1, y1 := vertices[t1], vertices[t1+1]
		x2, y2 := vertices[t2], vertices[t2+1]
		x3, y3 := vertices[t3], vertices[t3+1]

		// If the base of the last triangle is the same as this triangle, check if they form a convex polygon (triangle fan).
		merged := false
		if fanBaseIndex == t1 {
			o := len(polygon) - 4
			winding1 := winding(polygon[o], polygon[o+1], polygon[o+2], polygon[o+3], x3, y3)
			winding2 := winding(x3, y3, polygon[0], polygon[1], polygon[2], polygon[3])
			if winding1 == lastWinding && winding2 == lastWinding {
				polygon = append(polygon, x3, y3)
				polygonIndices = append(polygonIndices, t3)
				merged = true
			}
		}

		// Otherwise make this triangle the new base.
		if !merged {
			if len(polygon) > 0 {
				convexPolygons = append(convexPolygons, polygon)
				convexPolygonsIndices = append(convexPolygonsIndices, polygonIndices)
				polygon = t.obtainPolygon()
				polygonIndices = t.obtainPolygonIndices()
			}
			polygon = append(polygon[:0], x1, y1, x2, y2, x3, y3)
			polygonIndices = append(polygonIndices[:0], t1, t2, t3)
			lastWinding = winding(x1, y1, x2, y2, x3, y3)
			fanBaseIndex = t1
		}
	}

	if len(polygon) > 0 {
		convexPolygons = append(convexPolygons, polygon)
		convexPolygonsIndices = append(convexPolygonsIndices, polygonIndices)
	}

	// Go through the list of polygons and try to merge the remaining triangles with the found triangle fans.
	for i, n := 0, len(convexPolygons); i < n; i++ {
		polygonIndices = convexPolygonsIndices[i]
		if len(polygonIndices) == 0 {
			continue
		}
		firstIndex := polygonIndices[0]
		lastIndex := polygonIndices[len(polygonIndices)-1]

		polygon = convexPolygons[i]
		o := len(polygon) - 4
		prevPrevX, prevPrevY := polygon[o], polygon[o+1]
		prevX, prevY := polygon[o+2], polygon[o+3]
		firstX, firstY := polygon[0], polygon[1]
		secondX, secondY := polygon[2], polygon[3]
		currWinding := winding(prevPrevX, prevPrevY, prevX, prevY, firstX, firstY)

		for ii := 0; ii < n; ii++ {
			if ii == i {
				continue
			}
			otherIndices := convexPolygonsIndices[ii]
			if len(otherIndices) != 3 {
				continue
			}
			otherFirstIndex := otherIndices[0]
			otherSecondIndex := otherIndices[1]
			otherLastIndex := otherIndices[2]

			otherPoly := convexPolygons[ii]
			x3, y3 := otherPoly[len(otherPoly)-2], otherPoly[len(otherPoly)-1]

			if otherFirstIndex != firstIndex || otherSecondIndex != lastIndex {
				continue
			}
			winding1 := winding(prevPrevX, prevPrevY, prevX, prevY, x3, y3)
			winding2 := winding(x3, y3, firstX, firstY, secondX, secondY)
			if winding1 == currWinding && winding2 == currWinding {
				convexPolygons[ii] = otherPoly[:0]
				convexPolygonsIndices[ii] = otherIndices[:0]
				polygon = append(polygon, x3, y3)
				polygonIndices = append(polygonIndices, otherLastIndex)
				convexPolygons[i] = polygon
				convexPolygonsIndices[i] = polygonIndices
				prevPrevX = prevX
				prevPrevY = prevY
				prevX = x3
				prevY = y3
				ii = 0
			}
		}
	}

	// Remove empty polygons that resulted from the merge step above.
	for i := len(convexPolygons) - 1; i >= 0; i-- {
		polygon = convexPolygons[i]
		if len(polygon) == 0 {
			convexPolygons = append(convexPolygons[:i], convexPolygons[i+1:]...)
			t.freePolygon(polygon)
			polygonIndices = convexPolygonsIndices[i]
			convexPolygonsIndices = append(convexPolygonsIndices[:i], convexPolygonsIndices[i+1:]...)
			t.freePolygonIndices(polygonIndices)
		}
	}

	t.convexPolygons = convexPolygons
	t.convexPolygonsIndices = convexPolygonsIndices
	return convexPolygons
}

func isConcave(index, vertexCount int, vertices []float32, indices []int) bool {
	previous := indices[(vertexCount+index-1)%vertexCount] << 1
	current := indices[index] << 1
	next := indices[(index+1)%vertexCount] << 1
	return !positiveArea(vertices[previous], vertices[previous+1], vertices[current], vertices[current+1],
		vertices[next], vertices[next+1])
}

func positiveArea(p1x, p1y, p2x, p2y, p3x, p3y float32) bool {
	return p1x*(p3y-p2y)+p2x*(p1y-p3y)+p3x*(p2y-p1y) >= 0
}

func winding(p1x, p1y, p2x, p2y, p3x, p3y float32) int {
	px, py := p2x-p1x, p2y-p1y
	if p3x*py-p3y*px+px*p1y-p1x*py >= 0 {
		return 1
	}
	return -1
}
