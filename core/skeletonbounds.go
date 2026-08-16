package spine

import "math"

// SkeletonBounds collects each visible BoundingBoxAttachment and computes the
// world vertices for its polygon. The polygon vertices are provided along with
// convenience methods for doing hit detection.
type SkeletonBounds struct {
	// MinX is the left edge of the axis aligned bounding box.
	MinX float32

	// MinY is the bottom edge of the axis aligned bounding box.
	MinY float32

	// MaxX is the right edge of the axis aligned bounding box.
	MaxX float32

	// MaxY is the top edge of the axis aligned bounding box.
	MaxY float32

	// BoundingBoxes are the visible bounding boxes.
	BoundingBoxes []*BoundingBoxAttachment

	// Polygons are the world vertices for the bounding box polygons.
	Polygons [][]float32

	polygonPool [][]float32
}

// NewSkeletonBounds creates a skeleton bounds.
func NewSkeletonBounds() *SkeletonBounds {
	return &SkeletonBounds{}
}

// Update clears any previous polygons, finds all visible bounding box
// attachments, and computes the world vertices for each bounding box's
// polygon. If updateAabb is true, the axis aligned bounding box containing all
// the polygons is computed. If false, the SkeletonBounds AABB methods will
// always return true.
func (sb *SkeletonBounds) Update(skeleton *Skeleton, updateAabb bool) {
	if skeleton == nil {
		panic("spine: skeleton cannot be nil")
	}

	sb.BoundingBoxes = sb.BoundingBoxes[:0]
	sb.polygonPool = append(sb.polygonPool, sb.Polygons...)
	sb.Polygons = sb.Polygons[:0]

	for _, slot := range skeleton.Slots {
		if !slot.Bone.active {
			continue
		}
		if boundingBox, ok := slot.attachment.(*BoundingBoxAttachment); ok {
			sb.BoundingBoxes = append(sb.BoundingBoxes, boundingBox)

			polygon := sb.obtainPolygon()
			polygon = ensureSize(&polygon, boundingBox.WorldVerticesLength)
			sb.Polygons = append(sb.Polygons, polygon)
			boundingBox.ComputeWorldVertices(slot, 0, boundingBox.WorldVerticesLength, polygon, 0, 2)
		}
	}

	if updateAabb {
		sb.aabbCompute()
	} else {
		sb.MinX = float32(math.MinInt32)
		sb.MinY = float32(math.MinInt32)
		sb.MaxX = float32(math.MaxInt32)
		sb.MaxY = float32(math.MaxInt32)
	}
}

func (sb *SkeletonBounds) obtainPolygon() []float32 {
	if n := len(sb.polygonPool); n > 0 {
		p := sb.polygonPool[n-1]
		sb.polygonPool = sb.polygonPool[:n-1]
		return p
	}
	return nil
}

func (sb *SkeletonBounds) aabbCompute() {
	minX, minY := float32(math.MaxInt32), float32(math.MaxInt32)
	maxX, maxY := float32(math.MinInt32), float32(math.MinInt32)
	for _, polygon := range sb.Polygons {
		for ii, nn := 0, len(polygon); ii < nn; ii += 2 {
			x := polygon[ii]
			y := polygon[ii+1]
			minX = minF(minX, x)
			minY = minF(minY, y)
			maxX = maxF(maxX, x)
			maxY = maxF(maxY, y)
		}
	}
	sb.MinX = minX
	sb.MinY = minY
	sb.MaxX = maxX
	sb.MaxY = maxY
}

// AabbContainsPoint returns true if the axis aligned bounding box contains the
// point.
func (sb *SkeletonBounds) AabbContainsPoint(x, y float32) bool {
	return x >= sb.MinX && x <= sb.MaxX && y >= sb.MinY && y <= sb.MaxY
}

// AabbIntersectsSegment returns true if the axis aligned bounding box
// intersects the line segment.
func (sb *SkeletonBounds) AabbIntersectsSegment(x1, y1, x2, y2 float32) bool {
	minX := sb.MinX
	minY := sb.MinY
	maxX := sb.MaxX
	maxY := sb.MaxY
	if (x1 <= minX && x2 <= minX) || (y1 <= minY && y2 <= minY) || (x1 >= maxX && x2 >= maxX) || (y1 >= maxY && y2 >= maxY) {
		return false
	}
	m := (y2 - y1) / (x2 - x1)
	y := m*(minX-x1) + y1
	if y > minY && y < maxY {
		return true
	}
	y = m*(maxX-x1) + y1
	if y > minY && y < maxY {
		return true
	}
	x := (minY-y1)/m + x1
	if x > minX && x < maxX {
		return true
	}
	x = (maxY-y1)/m + x1
	if x > minX && x < maxX {
		return true
	}
	return false
}

// AabbIntersectsSkeleton returns true if the axis aligned bounding box
// intersects the axis aligned bounding box of the specified bounds.
func (sb *SkeletonBounds) AabbIntersectsSkeleton(bounds *SkeletonBounds) bool {
	if bounds == nil {
		panic("spine: bounds cannot be nil")
	}
	return sb.MinX < bounds.MaxX && sb.MaxX > bounds.MinX && sb.MinY < bounds.MaxY && sb.MaxY > bounds.MinY
}

// ContainsPoint returns the first bounding box attachment that contains the
// point, or nil. When doing many checks, it is usually more efficient to only
// call this method if AabbContainsPoint returns true.
func (sb *SkeletonBounds) ContainsPoint(x, y float32) *BoundingBoxAttachment {
	for i, polygon := range sb.Polygons {
		if sb.ContainsPointPolygon(polygon, x, y) {
			return sb.BoundingBoxes[i]
		}
	}
	return nil
}

// ContainsPointPolygon returns true if the polygon contains the point.
func (sb *SkeletonBounds) ContainsPointPolygon(polygon []float32, x, y float32) bool {
	if polygon == nil {
		panic("spine: polygon cannot be nil")
	}
	vertices := polygon
	nn := len(polygon)

	prevIndex := nn - 2
	inside := false
	for ii := 0; ii < nn; ii += 2 {
		vertexY := vertices[ii+1]
		prevY := vertices[prevIndex+1]
		if (vertexY < y && prevY >= y) || (prevY < y && vertexY >= y) {
			vertexX := vertices[ii]
			if vertexX+(y-vertexY)/(prevY-vertexY)*(vertices[prevIndex]-vertexX) < x {
				inside = !inside
			}
		}
		prevIndex = ii
	}
	return inside
}

// IntersectsSegment returns the first bounding box attachment that contains
// any part of the line segment, or nil. When doing many checks, it is usually
// more efficient to only call this method if AabbIntersectsSegment returns
// true.
func (sb *SkeletonBounds) IntersectsSegment(x1, y1, x2, y2 float32) *BoundingBoxAttachment {
	for i, polygon := range sb.Polygons {
		if sb.IntersectsSegmentPolygon(polygon, x1, y1, x2, y2) {
			return sb.BoundingBoxes[i]
		}
	}
	return nil
}

// IntersectsSegmentPolygon returns true if the polygon contains any part of
// the line segment.
func (sb *SkeletonBounds) IntersectsSegmentPolygon(polygon []float32, x1, y1, x2, y2 float32) bool {
	if polygon == nil {
		panic("spine: polygon cannot be nil")
	}
	vertices := polygon
	nn := len(polygon)

	width12, height12 := x1-x2, y1-y2
	det1 := x1*y2 - y1*x2
	x3, y3 := vertices[nn-2], vertices[nn-1]
	for ii := 0; ii < nn; ii += 2 {
		x4, y4 := vertices[ii], vertices[ii+1]
		det2 := x3*y4 - y3*x4
		width34, height34 := x3-x4, y3-y4
		det3 := width12*height34 - height12*width34
		x := (det1*width34 - width12*det2) / det3
		if ((x >= x3 && x <= x4) || (x >= x4 && x <= x3)) && ((x >= x1 && x <= x2) || (x >= x2 && x <= x1)) {
			y := (det1*height34 - height12*det2) / det3
			if ((y >= y3 && y <= y4) || (y >= y4 && y <= y3)) && ((y >= y1 && y <= y2) || (y >= y2 && y <= y1)) {
				return true
			}
		}
		x3 = x4
		y3 = y4
	}
	return false
}

// Width returns the width of the axis aligned bounding box.
func (sb *SkeletonBounds) Width() float32 {
	return sb.MaxX - sb.MinX
}

// Height returns the height of the axis aligned bounding box.
func (sb *SkeletonBounds) Height() float32 {
	return sb.MaxY - sb.MinY
}

// Polygon returns the polygon for the specified bounding box, or nil.
func (sb *SkeletonBounds) Polygon(boundingBox *BoundingBoxAttachment) []float32 {
	if boundingBox == nil {
		panic("spine: boundingBox cannot be nil")
	}
	for i, b := range sb.BoundingBoxes {
		if b == boundingBox {
			return sb.Polygons[i]
		}
	}
	return nil
}
