package spine

import (
	"math"
	"strconv"
)

// Animation stores a list of timelines to animate a skeleton's pose over time.
type Animation struct {
	// Name is the animation's name, which is unique across all animations in
	// the skeleton.
	Name string

	// Duration is the duration of the animation in seconds, which is usually
	// the highest time of all frames in the timeline. The duration is used to
	// know when it has completed and when it should loop back to the start.
	Duration float32

	timelines   []Timeline
	timelineIDs map[string]struct{}
}

// NewAnimation creates an animation.
func NewAnimation(name string, timelines []Timeline, duration float32) *Animation {
	if name == "" {
		panic("spine: name cannot be empty")
	}
	a := &Animation{Name: name, Duration: duration}
	a.timelineIDs = make(map[string]struct{}, len(timelines))
	a.SetTimelines(timelines)
	return a
}

// Timelines returns the animation's timelines. If the returned slice or the
// timelines it contains are modified, SetTimelines must be called.
func (a *Animation) Timelines() []Timeline { return a.timelines }

// SetTimelines sets the animation's timelines.
func (a *Animation) SetTimelines(timelines []Timeline) {
	a.timelines = timelines

	a.timelineIDs = make(map[string]struct{}, len(timelines))
	for _, timeline := range timelines {
		for _, id := range timeline.PropertyIDs() {
			a.timelineIDs[id] = struct{}{}
		}
	}
}

// HasTimeline returns true if this animation contains a timeline with any of
// the specified property IDs.
func (a *Animation) HasTimeline(propertyIDs []string) bool {
	for _, id := range propertyIDs {
		if _, ok := a.timelineIDs[id]; ok {
			return true
		}
	}
	return false
}

// Apply applies the animation's timelines to the specified skeleton.
//
// lastTime is the last time in seconds this animation was applied. Some
// timelines trigger only at specific times rather than every frame; pass -1
// the first time an animation is applied to ensure frame 0 is triggered. If
// loop is true, the animation repeats after the duration. events may be nil to
// ignore fired events.
func (a *Animation) Apply(skeleton *Skeleton, lastTime, time float32, loop bool, events *[]*Event, alpha float32,
	blend MixBlend, direction MixDirection) {
	if skeleton == nil {
		panic("spine: skeleton cannot be nil")
	}

	if loop && a.Duration != 0 {
		time = float32(math.Mod(float64(time), float64(a.Duration)))
		if lastTime > 0 {
			lastTime = float32(math.Mod(float64(lastTime), float64(a.Duration)))
		}
	}

	for _, timeline := range a.timelines {
		timeline.Apply(skeleton, lastTime, time, events, alpha, blend, direction)
	}
}

func (a *Animation) String() string { return a.Name }

// MixBlend controls how timeline values are mixed with setup pose values or
// current pose values when a timeline is applied with alpha < 1.
type MixBlend int

const (
	// MixBlendSetup transitions from the setup value to the timeline value
	// (the current value is not used). Before the first frame, the setup value
	// is set.
	MixBlendSetup MixBlend = iota
	// MixBlendFirst transitions from the current value to the timeline value.
	// Before the first frame, transitions from the current value to the setup
	// value. Intended for the first animations applied, not for animations
	// layered on top of those.
	MixBlendFirst
	// MixBlendReplace transitions from the current value to the timeline
	// value. No change is made before the first frame. Intended for animations
	// layered on top of others.
	MixBlendReplace
	// MixBlendAdd transitions from the current value to the current value plus
	// the timeline value. No change is made before the first frame. Intended
	// for animations layered on top of others.
	MixBlendAdd
)

// MixDirection indicates whether a timeline's alpha is mixing out over time
// toward 0 (the setup or current pose value) or mixing in toward 1 (the
// timeline's value). Some timelines use this to decide how values are applied.
type MixDirection int

const (
	MixDirectionIn MixDirection = iota
	MixDirectionOut
)

type property int

const (
	propertyRotate property = iota
	propertyX
	propertyY
	propertyScaleX
	propertyScaleY
	propertyShearX
	propertyShearY
	propertyInherit
	propertyRGB
	propertyAlpha
	propertyRGB2
	propertyAttachment
	propertyDeform
	propertyEvent
	propertyDrawOrder
	propertyIkConstraint
	propertyTransformConstraint
	propertyPathConstraintPosition
	propertyPathConstraintSpacing
	propertyPathConstraintMix
	propertyPhysicsConstraintInertia
	propertyPhysicsConstraintStrength
	propertyPhysicsConstraintDamping
	propertyPhysicsConstraintMass
	propertyPhysicsConstraintWind
	propertyPhysicsConstraintGravity
	propertyPhysicsConstraintMix
	propertyPhysicsConstraintReset
	propertySequence
)

func propertyID(p property, index int) string {
	return strconv.Itoa(int(p)) + "|" + strconv.Itoa(index)
}

// Timeline is the interface for all timelines.
type Timeline interface {
	// Apply applies this timeline to the skeleton. lastTime is the last time
	// in seconds this timeline was applied; timelines such as EventTimeline
	// trigger only at specific times rather than every frame and in that case
	// trigger everything between lastTime (exclusive) and time (inclusive).
	Apply(skeleton *Skeleton, lastTime, time float32, events *[]*Event, alpha float32, blend MixBlend,
		direction MixDirection)
	// PropertyIDs uniquely encodes both the type of this timeline and the
	// skeleton properties that it affects.
	PropertyIDs() []string
	// FrameEntries returns the number of entries stored per frame.
	FrameEntries() int
	// FrameCount returns the number of frames for this timeline.
	FrameCount() int
	// Frames returns the time in seconds and any other values for each frame.
	Frames() []float32
	// Duration returns the time in seconds of the last frame.
	Duration() float32
}

// TimelineBase is the base for all timelines.
type TimelineBase struct {
	propertyIDs []string

	// FramesData is the time in seconds and any other values for each frame.
	FramesData []float32

	frameEntries int
}

func newTimelineBase(frameCount, frameEntries int, propertyIDs ...string) TimelineBase {
	if propertyIDs == nil {
		panic("spine: propertyIds cannot be nil")
	}
	return TimelineBase{
		propertyIDs:  propertyIDs,
		FramesData:   make([]float32, frameCount*frameEntries),
		frameEntries: frameEntries,
	}
}

// PropertyIDs uniquely encodes both the type of this timeline and the skeleton
// properties that it affects.
func (t *TimelineBase) PropertyIDs() []string { return t.propertyIDs }

// Frames returns the time in seconds and any other values for each frame.
func (t *TimelineBase) Frames() []float32 { return t.FramesData }

// FrameEntries returns the number of entries stored per frame.
func (t *TimelineBase) FrameEntries() int { return t.frameEntries }

// FrameCount returns the number of frames for this timeline.
func (t *TimelineBase) FrameCount() int { return len(t.FramesData) / t.frameEntries }

// Duration returns the time in seconds of the last frame.
func (t *TimelineBase) Duration() float32 {
	return t.FramesData[len(t.FramesData)-t.frameEntries]
}

// search1 is a linear search using a stride of 1. time must be >= the first
// value in frames. Returns the index of the first value <= time.
func search1(frames []float32, time float32) int {
	n := len(frames)
	for i := 1; i < n; i++ {
		if frames[i] > time {
			return i - 1
		}
	}
	return n - 1
}

// searchN is a linear search using the specified stride. time must be >= the
// first value in frames. Returns the index of the first value <= time.
func searchN(frames []float32, time float32, step int) int {
	n := len(frames)
	for i := step; i < n; i += step {
		if frames[i] > time {
			return i - step
		}
	}
	return n - step
}

// BoneTimeline is an interface for timelines which change the property of a bone.
type BoneTimeline interface {
	// GetBoneIndex returns the index of the bone in Skeleton.Bones that will
	// be changed when this timeline is applied.
	GetBoneIndex() int
}

// SlotTimeline is an interface for timelines which change the property of a slot.
type SlotTimeline interface {
	// GetSlotIndex returns the index of the slot in Skeleton.Slots that will
	// be changed when this timeline is applied.
	GetSlotIndex() int
}

// Interpolation types for CurveTimeline.
const (
	CurveLinear  = 0
	CurveStepped = 1
	CurveBezier  = 2
	BezierSize   = 18
)

// CurveTimeline is the base for timelines that interpolate between frame
// values using stepped, linear, or a Bezier curve.
type CurveTimeline struct {
	TimelineBase

	Curves []float32
}

func newCurveTimeline(frameCount, frameEntries, bezierCount int, propertyIDs ...string) CurveTimeline {
	t := CurveTimeline{TimelineBase: newTimelineBase(frameCount, frameEntries, propertyIDs...)}
	t.Curves = make([]float32, frameCount+bezierCount*BezierSize)
	t.Curves[frameCount-1] = CurveStepped
	return t
}

// SetLinear sets the specified frame to linear interpolation.
// frame is between 0 and frameCount - 1, inclusive.
func (t *CurveTimeline) SetLinear(frame int) {
	t.Curves[frame] = CurveLinear
}

// SetStepped sets the specified frame to stepped interpolation.
// frame is between 0 and frameCount - 1, inclusive.
func (t *CurveTimeline) SetStepped(frame int) {
	t.Curves[frame] = CurveStepped
}

// CurveType returns the interpolation type for the specified frame:
// CurveLinear, CurveStepped, or CurveBezier + the index of the Bezier segments.
func (t *CurveTimeline) CurveType(frame int) int {
	return int(t.Curves[frame])
}

// Shrink shrinks the storage for Bezier curves, for use when bezierCount
// (specified in the constructor) was larger than the actual number of Bezier
// curves.
func (t *CurveTimeline) Shrink(bezierCount int) {
	size := t.FrameCount() + bezierCount*BezierSize
	if len(t.Curves) > size {
		newCurves := make([]float32, size)
		copy(newCurves, t.Curves[:size])
		t.Curves = newCurves
	}
}

// SetBezier stores the segments for the specified Bezier curve. For timelines
// that modify multiple values, there may be more than one curve per frame.
// bezier is the ordinal of this Bezier curve for this timeline, between 0 and
// bezierCount - 1 (specified in the constructor), inclusive. frame is between
// 0 and frameCount - 1, inclusive. value is the index of the value for the
// frame this curve is used for.
func (t *CurveTimeline) SetBezier(bezier, frame, value int, time1, value1, cx1, cy1, cx2,
	cy2, time2, value2 float32) {
	curves := t.Curves
	i := t.FrameCount() + bezier*BezierSize
	if value == 0 {
		curves[frame] = float32(CurveBezier + i)
	}
	tmpx := (time1 - cx1*2 + cx2) * 0.03
	tmpy := (value1 - cy1*2 + cy2) * 0.03
	dddx := ((cx1-cx2)*3 - time1 + time2) * 0.006
	dddy := ((cy1-cy2)*3 - value1 + value2) * 0.006
	ddx := tmpx*2 + dddx
	ddy := tmpy*2 + dddy
	dx := (cx1-time1)*0.3 + tmpx + dddx*0.16666667
	dy := (cy1-value1)*0.3 + tmpy + dddy*0.16666667
	x := time1 + dx
	y := value1 + dy
	for n := i + BezierSize; i < n; i += 2 {
		curves[i] = x
		curves[i+1] = y
		dx += ddx
		dy += ddy
		ddx += dddx
		ddy += dddy
		x += dx
		y += dy
	}
}

// BezierValue returns the Bezier interpolated value for the specified time.
// frameIndex is the index into Frames() for the values of the frame before
// time. valueOffset is the offset from frameIndex to the value this curve is
// used for. i is the index of the Bezier segments (see CurveType).
func (t *CurveTimeline) BezierValue(time float32, frameIndex, valueOffset, i int) float32 {
	curves := t.Curves
	frames := t.FramesData
	if curves[i] > time {
		x, y := frames[frameIndex], frames[frameIndex+valueOffset]
		return y + (time-x)/(curves[i]-x)*(curves[i+1]-y)
	}
	n := i + BezierSize
	for i += 2; i < n; i += 2 {
		if curves[i] >= time {
			x, y := curves[i-2], curves[i-1]
			return y + (time-x)/(curves[i]-x)*(curves[i+1]-y)
		}
	}
	frameIndex += t.FrameEntries()
	x, y := curves[n-2], curves[n-1]
	return y + (time-x)/(frames[frameIndex]-x)*(frames[frameIndex+valueOffset]-y)
}

// CurveTimeline1 frame entries.
const (
	curve1Entries = 2
	curve1Value   = 1
)

// CurveTimeline1 is the base for a CurveTimeline that sets one property.
type CurveTimeline1 struct {
	CurveTimeline
}

func newCurveTimeline1(frameCount, bezierCount int, propertyID string) CurveTimeline1 {
	return CurveTimeline1{CurveTimeline: newCurveTimeline(frameCount, curve1Entries, bezierCount, propertyID)}
}

// SetFrame sets the time and value for the specified frame.
// frame is between 0 and frameCount, inclusive.
func (t *CurveTimeline1) SetFrame(frame int, time, value float32) {
	frame <<= 1
	t.FramesData[frame] = time
	t.FramesData[frame+curve1Value] = value
}

// GetCurveValue returns the interpolated value for the specified time.
func (t *CurveTimeline1) GetCurveValue(time float32) float32 {
	frames := t.FramesData
	i := len(frames) - 2
	for ii := 2; ii <= i; ii += 2 {
		if frames[ii] > time {
			i = ii - 2
			break
		}
	}

	curveType := int(t.Curves[i>>1])
	switch curveType {
	case CurveLinear:
		before, value := frames[i], frames[i+curve1Value]
		return value + (time-before)/(frames[i+curve1Entries]-before)*(frames[i+curve1Entries+curve1Value]-value)
	case CurveStepped:
		return frames[i+curve1Value]
	}
	return t.BezierValue(time, i, curve1Value, curveType-CurveBezier)
}

// GetRelativeValue returns the value blended relative to the setup value.
func (t *CurveTimeline1) GetRelativeValue(time, alpha float32, blend MixBlend, current, setup float32) float32 {
	if time < t.FramesData[0] {
		switch blend {
		case MixBlendSetup:
			return setup
		case MixBlendFirst:
			return current + (setup-current)*alpha
		}
		return current
	}
	value := t.GetCurveValue(time)
	switch blend {
	case MixBlendSetup:
		return setup + value*alpha
	case MixBlendFirst, MixBlendReplace:
		value += setup - current
	}
	return current + value*alpha
}

// GetAbsoluteValue returns the absolute value blended with the current or
// setup value.
func (t *CurveTimeline1) GetAbsoluteValue(time, alpha float32, blend MixBlend, current, setup float32) float32 {
	if time < t.FramesData[0] {
		switch blend {
		case MixBlendSetup:
			return setup
		case MixBlendFirst:
			return current + (setup-current)*alpha
		}
		return current
	}
	value := t.GetCurveValue(time)
	if blend == MixBlendSetup {
		return setup + (value-setup)*alpha
	}
	return current + (value-current)*alpha
}

// GetAbsoluteValueWith is GetAbsoluteValue using the specified timeline value.
func (t *CurveTimeline1) GetAbsoluteValueWith(time, alpha float32, blend MixBlend, current, setup, value float32) float32 {
	if time < t.FramesData[0] {
		switch blend {
		case MixBlendSetup:
			return setup
		case MixBlendFirst:
			return current + (setup-current)*alpha
		}
		return current
	}
	if blend == MixBlendSetup {
		return setup + (value-setup)*alpha
	}
	return current + (value-current)*alpha
}

// GetScaleValue returns the scale value blended with the current or setup value.
func (t *CurveTimeline1) GetScaleValue(time, alpha float32, blend MixBlend, direction MixDirection, current,
	setup float32) float32 {
	frames := t.FramesData
	if time < frames[0] {
		switch blend {
		case MixBlendSetup:
			return setup
		case MixBlendFirst:
			return current + (setup-current)*alpha
		}
		return current
	}
	value := t.GetCurveValue(time) * setup
	if alpha == 1 {
		if blend == MixBlendAdd {
			return current + value - setup
		}
		return value
	}
	// Mixing out uses sign of setup or current pose, else use sign of key.
	if direction == MixDirectionOut {
		switch blend {
		case MixBlendSetup:
			return setup + (abs(value)*signum(setup)-setup)*alpha
		case MixBlendFirst, MixBlendReplace:
			return current + (abs(value)*signum(current)-current)*alpha
		}
	} else {
		var s float32
		switch blend {
		case MixBlendSetup:
			s = abs(setup) * signum(value)
			return s + (value-s)*alpha
		case MixBlendFirst, MixBlendReplace:
			s = abs(current) * signum(value)
			return s + (value-s)*alpha
		}
	}
	return current + (value-setup)*alpha
}

// CurveTimeline2 frame entries.
const (
	curve2Entries = 3
	curve2Value1  = 1
	curve2Value2  = 2
)

// CurveTimeline2 is the base for a CurveTimeline which sets two properties.
type CurveTimeline2 struct {
	CurveTimeline
}

func newCurveTimeline2(frameCount, bezierCount int, propertyID1, propertyID2 string) CurveTimeline2 {
	return CurveTimeline2{CurveTimeline: newCurveTimeline(frameCount, curve2Entries, bezierCount, propertyID1, propertyID2)}
}

// SetFrame sets the time and values for the specified frame.
// frame is between 0 and frameCount, inclusive.
func (t *CurveTimeline2) SetFrame(frame int, time, value1, value2 float32) {
	frame *= curve2Entries
	t.FramesData[frame] = time
	t.FramesData[frame+curve2Value1] = value1
	t.FramesData[frame+curve2Value2] = value2
}

// RotateTimeline changes a bone's local rotation.
type RotateTimeline struct {
	CurveTimeline1
	BoneIndex int
}

// NewRotateTimeline creates a rotate timeline.
func NewRotateTimeline(frameCount, bezierCount, boneIndex int) *RotateTimeline {
	return &RotateTimeline{
		CurveTimeline1: newCurveTimeline1(frameCount, bezierCount, propertyID(propertyRotate, boneIndex)),
		BoneIndex:      boneIndex,
	}
}

// GetBoneIndex implements BoneTimeline.
func (t *RotateTimeline) GetBoneIndex() int { return t.BoneIndex }

// Apply implements Timeline.
func (t *RotateTimeline) Apply(skeleton *Skeleton, lastTime, time float32, events *[]*Event, alpha float32,
	blend MixBlend, direction MixDirection) {

	bone := skeleton.Bones[t.BoneIndex]
	if bone.active {
		bone.Rotation = t.GetRelativeValue(time, alpha, blend, bone.Rotation, bone.Data.Rotation)
	}
}

// TranslateTimeline changes a bone's local x and y.
type TranslateTimeline struct {
	CurveTimeline2
	BoneIndex int
}

// NewTranslateTimeline creates a translate timeline.
func NewTranslateTimeline(frameCount, bezierCount, boneIndex int) *TranslateTimeline {
	return &TranslateTimeline{
		CurveTimeline2: newCurveTimeline2(frameCount, bezierCount,
			propertyID(propertyX, boneIndex),
			propertyID(propertyY, boneIndex)),
		BoneIndex: boneIndex,
	}
}

// GetBoneIndex implements BoneTimeline.
func (t *TranslateTimeline) GetBoneIndex() int { return t.BoneIndex }

// Apply implements Timeline.
func (t *TranslateTimeline) Apply(skeleton *Skeleton, lastTime, time float32, events *[]*Event, alpha float32,
	blend MixBlend, direction MixDirection) {

	bone := skeleton.Bones[t.BoneIndex]
	if !bone.active {
		return
	}

	frames := t.FramesData
	if time < frames[0] {
		switch blend {
		case MixBlendSetup:
			bone.X = bone.Data.X
			bone.Y = bone.Data.Y
			return
		case MixBlendFirst:
			bone.X += (bone.Data.X - bone.X) * alpha
			bone.Y += (bone.Data.Y - bone.Y) * alpha
		}
		return
	}

	var x, y float32
	i := searchN(frames, time, curve2Entries)
	curveType := int(t.Curves[i/curve2Entries])
	switch curveType {
	case CurveLinear:
		before := frames[i]
		x = frames[i+curve2Value1]
		y = frames[i+curve2Value2]
		tt := (time - before) / (frames[i+curve2Entries] - before)
		x += (frames[i+curve2Entries+curve2Value1] - x) * tt
		y += (frames[i+curve2Entries+curve2Value2] - y) * tt
	case CurveStepped:
		x = frames[i+curve2Value1]
		y = frames[i+curve2Value2]
	default:
		x = t.BezierValue(time, i, curve2Value1, curveType-CurveBezier)
		y = t.BezierValue(time, i, curve2Value2, curveType+BezierSize-CurveBezier)
	}

	switch blend {
	case MixBlendSetup:
		bone.X = bone.Data.X + x*alpha
		bone.Y = bone.Data.Y + y*alpha
	case MixBlendFirst, MixBlendReplace:
		bone.X += (bone.Data.X + x - bone.X) * alpha
		bone.Y += (bone.Data.Y + y - bone.Y) * alpha
	case MixBlendAdd:
		bone.X += x * alpha
		bone.Y += y * alpha
	}
}

// TranslateXTimeline changes a bone's local x.
type TranslateXTimeline struct {
	CurveTimeline1
	BoneIndex int
}

// NewTranslateXTimeline creates a translate X timeline.
func NewTranslateXTimeline(frameCount, bezierCount, boneIndex int) *TranslateXTimeline {
	return &TranslateXTimeline{
		CurveTimeline1: newCurveTimeline1(frameCount, bezierCount, propertyID(propertyX, boneIndex)),
		BoneIndex:      boneIndex,
	}
}

// GetBoneIndex implements BoneTimeline.
func (t *TranslateXTimeline) GetBoneIndex() int { return t.BoneIndex }

// Apply implements Timeline.
func (t *TranslateXTimeline) Apply(skeleton *Skeleton, lastTime, time float32, events *[]*Event, alpha float32,
	blend MixBlend, direction MixDirection) {

	bone := skeleton.Bones[t.BoneIndex]
	if bone.active {
		bone.X = t.GetRelativeValue(time, alpha, blend, bone.X, bone.Data.X)
	}
}

// TranslateYTimeline changes a bone's local y.
type TranslateYTimeline struct {
	CurveTimeline1
	BoneIndex int
}

// NewTranslateYTimeline creates a translate Y timeline.
func NewTranslateYTimeline(frameCount, bezierCount, boneIndex int) *TranslateYTimeline {
	return &TranslateYTimeline{
		CurveTimeline1: newCurveTimeline1(frameCount, bezierCount, propertyID(propertyY, boneIndex)),
		BoneIndex:      boneIndex,
	}
}

// GetBoneIndex implements BoneTimeline.
func (t *TranslateYTimeline) GetBoneIndex() int { return t.BoneIndex }

// Apply implements Timeline.
func (t *TranslateYTimeline) Apply(skeleton *Skeleton, lastTime, time float32, events *[]*Event, alpha float32,
	blend MixBlend, direction MixDirection) {

	bone := skeleton.Bones[t.BoneIndex]
	if bone.active {
		bone.Y = t.GetRelativeValue(time, alpha, blend, bone.Y, bone.Data.Y)
	}
}

// ScaleTimeline changes a bone's local scaleX and scaleY.
type ScaleTimeline struct {
	CurveTimeline2
	BoneIndex int
}

// NewScaleTimeline creates a scale timeline.
func NewScaleTimeline(frameCount, bezierCount, boneIndex int) *ScaleTimeline {
	return &ScaleTimeline{
		CurveTimeline2: newCurveTimeline2(frameCount, bezierCount,
			propertyID(propertyScaleX, boneIndex),
			propertyID(propertyScaleY, boneIndex)),
		BoneIndex: boneIndex,
	}
}

// GetBoneIndex implements BoneTimeline.
func (t *ScaleTimeline) GetBoneIndex() int { return t.BoneIndex }

// Apply implements Timeline.
func (t *ScaleTimeline) Apply(skeleton *Skeleton, lastTime, time float32, events *[]*Event, alpha float32,
	blend MixBlend, direction MixDirection) {

	bone := skeleton.Bones[t.BoneIndex]
	if !bone.active {
		return
	}

	frames := t.FramesData
	if time < frames[0] {
		switch blend {
		case MixBlendSetup:
			bone.ScaleX = bone.Data.ScaleX
			bone.ScaleY = bone.Data.ScaleY
			return
		case MixBlendFirst:
			bone.ScaleX += (bone.Data.ScaleX - bone.ScaleX) * alpha
			bone.ScaleY += (bone.Data.ScaleY - bone.ScaleY) * alpha
		}
		return
	}

	var x, y float32
	i := searchN(frames, time, curve2Entries)
	curveType := int(t.Curves[i/curve2Entries])
	switch curveType {
	case CurveLinear:
		before := frames[i]
		x = frames[i+curve2Value1]
		y = frames[i+curve2Value2]
		tt := (time - before) / (frames[i+curve2Entries] - before)
		x += (frames[i+curve2Entries+curve2Value1] - x) * tt
		y += (frames[i+curve2Entries+curve2Value2] - y) * tt
	case CurveStepped:
		x = frames[i+curve2Value1]
		y = frames[i+curve2Value2]
	default:
		x = t.BezierValue(time, i, curve2Value1, curveType-CurveBezier)
		y = t.BezierValue(time, i, curve2Value2, curveType+BezierSize-CurveBezier)
	}
	x *= bone.Data.ScaleX
	y *= bone.Data.ScaleY

	if alpha == 1 {
		if blend == MixBlendAdd {
			bone.ScaleX += x - bone.Data.ScaleX
			bone.ScaleY += y - bone.Data.ScaleY
		} else {
			bone.ScaleX = x
			bone.ScaleY = y
		}
	} else {
		// Mixing out uses sign of setup or current pose, else use sign of key.
		var bx, by float32
		if direction == MixDirectionOut {
			switch blend {
			case MixBlendSetup:
				bx = bone.Data.ScaleX
				by = bone.Data.ScaleY
				bone.ScaleX = bx + (abs(x)*signum(bx)-bx)*alpha
				bone.ScaleY = by + (abs(y)*signum(by)-by)*alpha
			case MixBlendFirst, MixBlendReplace:
				bx = bone.ScaleX
				by = bone.ScaleY
				bone.ScaleX = bx + (abs(x)*signum(bx)-bx)*alpha
				bone.ScaleY = by + (abs(y)*signum(by)-by)*alpha
			case MixBlendAdd:
				bone.ScaleX += (x - bone.Data.ScaleX) * alpha
				bone.ScaleY += (y - bone.Data.ScaleY) * alpha
			}
		} else {
			switch blend {
			case MixBlendSetup:
				bx = abs(bone.Data.ScaleX) * signum(x)
				by = abs(bone.Data.ScaleY) * signum(y)
				bone.ScaleX = bx + (x-bx)*alpha
				bone.ScaleY = by + (y-by)*alpha
			case MixBlendFirst, MixBlendReplace:
				bx = abs(bone.ScaleX) * signum(x)
				by = abs(bone.ScaleY) * signum(y)
				bone.ScaleX = bx + (x-bx)*alpha
				bone.ScaleY = by + (y-by)*alpha
			case MixBlendAdd:
				bone.ScaleX += (x - bone.Data.ScaleX) * alpha
				bone.ScaleY += (y - bone.Data.ScaleY) * alpha
			}
		}
	}
}

// ScaleXTimeline changes a bone's local scaleX.
type ScaleXTimeline struct {
	CurveTimeline1
	BoneIndex int
}

// NewScaleXTimeline creates a scale X timeline.
func NewScaleXTimeline(frameCount, bezierCount, boneIndex int) *ScaleXTimeline {
	return &ScaleXTimeline{
		CurveTimeline1: newCurveTimeline1(frameCount, bezierCount, propertyID(propertyScaleX, boneIndex)),
		BoneIndex:      boneIndex,
	}
}

// GetBoneIndex implements BoneTimeline.
func (t *ScaleXTimeline) GetBoneIndex() int { return t.BoneIndex }

// Apply implements Timeline.
func (t *ScaleXTimeline) Apply(skeleton *Skeleton, lastTime, time float32, events *[]*Event, alpha float32,
	blend MixBlend, direction MixDirection) {

	bone := skeleton.Bones[t.BoneIndex]
	if bone.active {
		bone.ScaleX = t.GetScaleValue(time, alpha, blend, direction, bone.ScaleX, bone.Data.ScaleX)
	}
}

// ScaleYTimeline changes a bone's local scaleY.
type ScaleYTimeline struct {
	CurveTimeline1
	BoneIndex int
}

// NewScaleYTimeline creates a scale Y timeline.
func NewScaleYTimeline(frameCount, bezierCount, boneIndex int) *ScaleYTimeline {
	return &ScaleYTimeline{
		CurveTimeline1: newCurveTimeline1(frameCount, bezierCount, propertyID(propertyScaleY, boneIndex)),
		BoneIndex:      boneIndex,
	}
}

// GetBoneIndex implements BoneTimeline.
func (t *ScaleYTimeline) GetBoneIndex() int { return t.BoneIndex }

// Apply implements Timeline.
func (t *ScaleYTimeline) Apply(skeleton *Skeleton, lastTime, time float32, events *[]*Event, alpha float32,
	blend MixBlend, direction MixDirection) {

	bone := skeleton.Bones[t.BoneIndex]
	if bone.active {
		bone.ScaleY = t.GetScaleValue(time, alpha, blend, direction, bone.ScaleY, bone.Data.ScaleY)
	}
}

// ShearTimeline changes a bone's local shearX and shearY.
type ShearTimeline struct {
	CurveTimeline2
	BoneIndex int
}

// NewShearTimeline creates a shear timeline.
func NewShearTimeline(frameCount, bezierCount, boneIndex int) *ShearTimeline {
	return &ShearTimeline{
		CurveTimeline2: newCurveTimeline2(frameCount, bezierCount,
			propertyID(propertyShearX, boneIndex),
			propertyID(propertyShearY, boneIndex)),
		BoneIndex: boneIndex,
	}
}

// GetBoneIndex implements BoneTimeline.
func (t *ShearTimeline) GetBoneIndex() int { return t.BoneIndex }

// Apply implements Timeline.
func (t *ShearTimeline) Apply(skeleton *Skeleton, lastTime, time float32, events *[]*Event, alpha float32,
	blend MixBlend, direction MixDirection) {

	bone := skeleton.Bones[t.BoneIndex]
	if !bone.active {
		return
	}

	frames := t.FramesData
	if time < frames[0] {
		switch blend {
		case MixBlendSetup:
			bone.ShearX = bone.Data.ShearX
			bone.ShearY = bone.Data.ShearY
			return
		case MixBlendFirst:
			bone.ShearX += (bone.Data.ShearX - bone.ShearX) * alpha
			bone.ShearY += (bone.Data.ShearY - bone.ShearY) * alpha
		}
		return
	}

	var x, y float32
	i := searchN(frames, time, curve2Entries)
	curveType := int(t.Curves[i/curve2Entries])
	switch curveType {
	case CurveLinear:
		before := frames[i]
		x = frames[i+curve2Value1]
		y = frames[i+curve2Value2]
		tt := (time - before) / (frames[i+curve2Entries] - before)
		x += (frames[i+curve2Entries+curve2Value1] - x) * tt
		y += (frames[i+curve2Entries+curve2Value2] - y) * tt
	case CurveStepped:
		x = frames[i+curve2Value1]
		y = frames[i+curve2Value2]
	default:
		x = t.BezierValue(time, i, curve2Value1, curveType-CurveBezier)
		y = t.BezierValue(time, i, curve2Value2, curveType+BezierSize-CurveBezier)
	}

	switch blend {
	case MixBlendSetup:
		bone.ShearX = bone.Data.ShearX + x*alpha
		bone.ShearY = bone.Data.ShearY + y*alpha
	case MixBlendFirst, MixBlendReplace:
		bone.ShearX += (bone.Data.ShearX + x - bone.ShearX) * alpha
		bone.ShearY += (bone.Data.ShearY + y - bone.ShearY) * alpha
	case MixBlendAdd:
		bone.ShearX += x * alpha
		bone.ShearY += y * alpha
	}
}

// ShearXTimeline changes a bone's local shearX.
type ShearXTimeline struct {
	CurveTimeline1
	BoneIndex int
}

// NewShearXTimeline creates a shear X timeline.
func NewShearXTimeline(frameCount, bezierCount, boneIndex int) *ShearXTimeline {
	return &ShearXTimeline{
		CurveTimeline1: newCurveTimeline1(frameCount, bezierCount, propertyID(propertyShearX, boneIndex)),
		BoneIndex:      boneIndex,
	}
}

// GetBoneIndex implements BoneTimeline.
func (t *ShearXTimeline) GetBoneIndex() int { return t.BoneIndex }

// Apply implements Timeline.
func (t *ShearXTimeline) Apply(skeleton *Skeleton, lastTime, time float32, events *[]*Event, alpha float32,
	blend MixBlend, direction MixDirection) {

	bone := skeleton.Bones[t.BoneIndex]
	if bone.active {
		bone.ShearX = t.GetRelativeValue(time, alpha, blend, bone.ShearX, bone.Data.ShearX)
	}
}

// ShearYTimeline changes a bone's local shearY.
type ShearYTimeline struct {
	CurveTimeline1
	BoneIndex int
}

// NewShearYTimeline creates a shear Y timeline.
func NewShearYTimeline(frameCount, bezierCount, boneIndex int) *ShearYTimeline {
	return &ShearYTimeline{
		CurveTimeline1: newCurveTimeline1(frameCount, bezierCount, propertyID(propertyShearY, boneIndex)),
		BoneIndex:      boneIndex,
	}
}

// GetBoneIndex implements BoneTimeline.
func (t *ShearYTimeline) GetBoneIndex() int { return t.BoneIndex }

// Apply implements Timeline.
func (t *ShearYTimeline) Apply(skeleton *Skeleton, lastTime, time float32, events *[]*Event, alpha float32,
	blend MixBlend, direction MixDirection) {

	bone := skeleton.Bones[t.BoneIndex]
	if bone.active {
		bone.ShearY = t.GetRelativeValue(time, alpha, blend, bone.ShearY, bone.Data.ShearY)
	}
}

// InheritTimeline frame entries.
const (
	inheritEntries = 2
	inheritOffset  = 1
)

// InheritTimeline changes a bone's inherit.
type InheritTimeline struct {
	TimelineBase
	BoneIndex int
}

// NewInheritTimeline creates an inherit timeline.
func NewInheritTimeline(frameCount, boneIndex int) *InheritTimeline {
	return &InheritTimeline{
		TimelineBase: newTimelineBase(frameCount, inheritEntries, propertyID(propertyInherit, boneIndex)),
		BoneIndex:    boneIndex,
	}
}

// GetBoneIndex implements BoneTimeline.
func (t *InheritTimeline) GetBoneIndex() int { return t.BoneIndex }

// SetFrame sets the transform mode for the specified frame.
// frame is between 0 and frameCount, inclusive.
func (t *InheritTimeline) SetFrame(frame int, time float32, inherit Inherit) {
	frame *= inheritEntries
	t.FramesData[frame] = time
	t.FramesData[frame+inheritOffset] = float32(inherit)
}

// Apply implements Timeline.
func (t *InheritTimeline) Apply(skeleton *Skeleton, lastTime, time float32, events *[]*Event, alpha float32,
	blend MixBlend, direction MixDirection) {

	bone := skeleton.Bones[t.BoneIndex]
	if !bone.active {
		return
	}

	if direction == MixDirectionOut {
		if blend == MixBlendSetup {
			bone.Inherit = bone.Data.Inherit
		}
		return
	}

	frames := t.FramesData
	if time < frames[0] {
		if blend == MixBlendSetup || blend == MixBlendFirst {
			bone.Inherit = bone.Data.Inherit
		}
		return
	}
	bone.Inherit = Inherit(int(frames[searchN(frames, time, inheritEntries)+inheritOffset]))
}

// RGBATimeline frame entries.
const (
	rgbaEntries = 5
	rgbaR       = 1
	rgbaG       = 2
	rgbaB       = 3
	rgbaA       = 4
)

// RGBATimeline changes a slot's color.
type RGBATimeline struct {
	CurveTimeline
	SlotIndex int
}

// NewRGBATimeline creates an RGBA timeline.
func NewRGBATimeline(frameCount, bezierCount, slotIndex int) *RGBATimeline {
	return &RGBATimeline{
		CurveTimeline: newCurveTimeline(frameCount, rgbaEntries, bezierCount,
			propertyID(propertyRGB, slotIndex),
			propertyID(propertyAlpha, slotIndex)),
		SlotIndex: slotIndex,
	}
}

// GetSlotIndex implements SlotTimeline.
func (t *RGBATimeline) GetSlotIndex() int { return t.SlotIndex }

// SetFrame sets the time and color for the specified frame.
// frame is between 0 and frameCount, inclusive.
func (t *RGBATimeline) SetFrame(frame int, time, r, g, b, a float32) {
	frame *= rgbaEntries
	t.FramesData[frame] = time
	t.FramesData[frame+rgbaR] = r
	t.FramesData[frame+rgbaG] = g
	t.FramesData[frame+rgbaB] = b
	t.FramesData[frame+rgbaA] = a
}

// Apply implements Timeline.
func (t *RGBATimeline) Apply(skeleton *Skeleton, lastTime, time float32, events *[]*Event, alpha float32,
	blend MixBlend, direction MixDirection) {

	slot := skeleton.Slots[t.SlotIndex]
	if !slot.Bone.active {
		return
	}

	frames := t.FramesData
	color := &slot.Color
	if time < frames[0] {
		setup := &slot.Data.Color
		switch blend {
		case MixBlendSetup:
			color.SetColor(*setup)
			return
		case MixBlendFirst:
			color.Add((setup.R-color.R)*alpha, (setup.G-color.G)*alpha, (setup.B-color.B)*alpha,
				(setup.A-color.A)*alpha)
		}
		return
	}

	var r, g, b, a float32
	i := searchN(frames, time, rgbaEntries)
	curveType := int(t.Curves[i/rgbaEntries])
	switch curveType {
	case CurveLinear:
		before := frames[i]
		r = frames[i+rgbaR]
		g = frames[i+rgbaG]
		b = frames[i+rgbaB]
		a = frames[i+rgbaA]
		tt := (time - before) / (frames[i+rgbaEntries] - before)
		r += (frames[i+rgbaEntries+rgbaR] - r) * tt
		g += (frames[i+rgbaEntries+rgbaG] - g) * tt
		b += (frames[i+rgbaEntries+rgbaB] - b) * tt
		a += (frames[i+rgbaEntries+rgbaA] - a) * tt
	case CurveStepped:
		r = frames[i+rgbaR]
		g = frames[i+rgbaG]
		b = frames[i+rgbaB]
		a = frames[i+rgbaA]
	default:
		r = t.BezierValue(time, i, rgbaR, curveType-CurveBezier)
		g = t.BezierValue(time, i, rgbaG, curveType+BezierSize-CurveBezier)
		b = t.BezierValue(time, i, rgbaB, curveType+BezierSize*2-CurveBezier)
		a = t.BezierValue(time, i, rgbaA, curveType+BezierSize*3-CurveBezier)
	}

	if alpha == 1 {
		color.Set(r, g, b, a)
	} else {
		if blend == MixBlendSetup {
			color.SetColor(slot.Data.Color)
		}
		color.Add((r-color.R)*alpha, (g-color.G)*alpha, (b-color.B)*alpha, (a-color.A)*alpha)
	}
}

// RGBTimeline frame entries.
const (
	rgbEntries = 4
	rgbR       = 1
	rgbG       = 2
	rgbB       = 3
)

// RGBTimeline changes the RGB for a slot's color.
type RGBTimeline struct {
	CurveTimeline
	SlotIndex int
}

// NewRGBTimeline creates an RGB timeline.
func NewRGBTimeline(frameCount, bezierCount, slotIndex int) *RGBTimeline {
	return &RGBTimeline{
		CurveTimeline: newCurveTimeline(frameCount, rgbEntries, bezierCount, propertyID(propertyRGB, slotIndex)),
		SlotIndex:     slotIndex,
	}
}

// GetSlotIndex implements SlotTimeline.
func (t *RGBTimeline) GetSlotIndex() int { return t.SlotIndex }

// SetFrame sets the time and color for the specified frame.
// frame is between 0 and frameCount, inclusive.
func (t *RGBTimeline) SetFrame(frame int, time, r, g, b float32) {
	frame <<= 2
	t.FramesData[frame] = time
	t.FramesData[frame+rgbR] = r
	t.FramesData[frame+rgbG] = g
	t.FramesData[frame+rgbB] = b
}

// Apply implements Timeline.
func (t *RGBTimeline) Apply(skeleton *Skeleton, lastTime, time float32, events *[]*Event, alpha float32,
	blend MixBlend, direction MixDirection) {

	slot := skeleton.Slots[t.SlotIndex]
	if !slot.Bone.active {
		return
	}

	frames := t.FramesData
	color := &slot.Color
	if time < frames[0] {
		setup := &slot.Data.Color
		switch blend {
		case MixBlendSetup:
			color.R = setup.R
			color.G = setup.G
			color.B = setup.B
			return
		case MixBlendFirst:
			color.R += (setup.R - color.R) * alpha
			color.G += (setup.G - color.G) * alpha
			color.B += (setup.B - color.B) * alpha
		}
		return
	}

	var r, g, b float32
	i := searchN(frames, time, rgbEntries)
	curveType := int(t.Curves[i>>2])
	switch curveType {
	case CurveLinear:
		before := frames[i]
		r = frames[i+rgbR]
		g = frames[i+rgbG]
		b = frames[i+rgbB]
		tt := (time - before) / (frames[i+rgbEntries] - before)
		r += (frames[i+rgbEntries+rgbR] - r) * tt
		g += (frames[i+rgbEntries+rgbG] - g) * tt
		b += (frames[i+rgbEntries+rgbB] - b) * tt
	case CurveStepped:
		r = frames[i+rgbR]
		g = frames[i+rgbG]
		b = frames[i+rgbB]
	default:
		r = t.BezierValue(time, i, rgbR, curveType-CurveBezier)
		g = t.BezierValue(time, i, rgbG, curveType+BezierSize-CurveBezier)
		b = t.BezierValue(time, i, rgbB, curveType+BezierSize*2-CurveBezier)
	}

	if alpha == 1 {
		color.R = r
		color.G = g
		color.B = b
	} else {
		if blend == MixBlendSetup {
			setup := &slot.Data.Color
			color.R = setup.R
			color.G = setup.G
			color.B = setup.B
		}
		color.R += (r - color.R) * alpha
		color.G += (g - color.G) * alpha
		color.B += (b - color.B) * alpha
	}
}

// AlphaTimeline changes the alpha for a slot's color.
type AlphaTimeline struct {
	CurveTimeline1
	SlotIndex int
}

// NewAlphaTimeline creates an alpha timeline.
func NewAlphaTimeline(frameCount, bezierCount, slotIndex int) *AlphaTimeline {
	return &AlphaTimeline{
		CurveTimeline1: newCurveTimeline1(frameCount, bezierCount, propertyID(propertyAlpha, slotIndex)),
		SlotIndex:      slotIndex,
	}
}

// GetSlotIndex implements SlotTimeline.
func (t *AlphaTimeline) GetSlotIndex() int { return t.SlotIndex }

// Apply implements Timeline.
func (t *AlphaTimeline) Apply(skeleton *Skeleton, lastTime, time float32, events *[]*Event, alpha float32,
	blend MixBlend, direction MixDirection) {

	slot := skeleton.Slots[t.SlotIndex]
	if !slot.Bone.active {
		return
	}

	frames := t.FramesData
	color := &slot.Color
	if time < frames[0] {
		setup := &slot.Data.Color
		switch blend {
		case MixBlendSetup:
			color.A = setup.A
			return
		case MixBlendFirst:
			color.A += (setup.A - color.A) * alpha
		}
		return
	}

	a := t.GetCurveValue(time)
	if alpha == 1 {
		color.A = a
	} else {
		if blend == MixBlendSetup {
			color.A = slot.Data.Color.A
		}
		color.A += (a - color.A) * alpha
	}
}

// RGBA2Timeline frame entries.
const (
	rgba2Entries = 8
	rgba2R       = 1
	rgba2G       = 2
	rgba2B       = 3
	rgba2A       = 4
	rgba2R2      = 5
	rgba2G2      = 6
	rgba2B2      = 7
)

// RGBA2Timeline changes a slot's color and dark color for two color tinting.
type RGBA2Timeline struct {
	CurveTimeline
	SlotIndex int
}

// NewRGBA2Timeline creates an RGBA2 timeline.
func NewRGBA2Timeline(frameCount, bezierCount, slotIndex int) *RGBA2Timeline {
	return &RGBA2Timeline{
		CurveTimeline: newCurveTimeline(frameCount, rgba2Entries, bezierCount,
			propertyID(propertyRGB, slotIndex),
			propertyID(propertyAlpha, slotIndex),
			propertyID(propertyRGB2, slotIndex)),
		SlotIndex: slotIndex,
	}
}

// GetSlotIndex implements SlotTimeline. The slot's DarkColor must not be nil.
func (t *RGBA2Timeline) GetSlotIndex() int { return t.SlotIndex }

// SetFrame sets the time, light color, and dark color for the specified frame.
// frame is between 0 and frameCount, inclusive.
func (t *RGBA2Timeline) SetFrame(frame int, time, r, g, b, a, r2, g2, b2 float32) {
	frame <<= 3
	t.FramesData[frame] = time
	t.FramesData[frame+rgba2R] = r
	t.FramesData[frame+rgba2G] = g
	t.FramesData[frame+rgba2B] = b
	t.FramesData[frame+rgba2A] = a
	t.FramesData[frame+rgba2R2] = r2
	t.FramesData[frame+rgba2G2] = g2
	t.FramesData[frame+rgba2B2] = b2
}

// Apply implements Timeline.
func (t *RGBA2Timeline) Apply(skeleton *Skeleton, lastTime, time float32, events *[]*Event, alpha float32,
	blend MixBlend, direction MixDirection) {

	slot := skeleton.Slots[t.SlotIndex]
	if !slot.Bone.active {
		return
	}

	frames := t.FramesData
	light, dark := &slot.Color, slot.DarkColor
	if time < frames[0] {
		setupLight, setupDark := &slot.Data.Color, slot.Data.DarkColor
		switch blend {
		case MixBlendSetup:
			light.SetColor(*setupLight)
			dark.R = setupDark.R
			dark.G = setupDark.G
			dark.B = setupDark.B
			return
		case MixBlendFirst:
			light.Add((setupLight.R-light.R)*alpha, (setupLight.G-light.G)*alpha, (setupLight.B-light.B)*alpha,
				(setupLight.A-light.A)*alpha)
			dark.R += (setupDark.R - dark.R) * alpha
			dark.G += (setupDark.G - dark.G) * alpha
			dark.B += (setupDark.B - dark.B) * alpha
		}
		return
	}

	var r, g, b, a, r2, g2, b2 float32
	i := searchN(frames, time, rgba2Entries)
	curveType := int(t.Curves[i>>3])
	switch curveType {
	case CurveLinear:
		before := frames[i]
		r = frames[i+rgba2R]
		g = frames[i+rgba2G]
		b = frames[i+rgba2B]
		a = frames[i+rgba2A]
		r2 = frames[i+rgba2R2]
		g2 = frames[i+rgba2G2]
		b2 = frames[i+rgba2B2]
		tt := (time - before) / (frames[i+rgba2Entries] - before)
		r += (frames[i+rgba2Entries+rgba2R] - r) * tt
		g += (frames[i+rgba2Entries+rgba2G] - g) * tt
		b += (frames[i+rgba2Entries+rgba2B] - b) * tt
		a += (frames[i+rgba2Entries+rgba2A] - a) * tt
		r2 += (frames[i+rgba2Entries+rgba2R2] - r2) * tt
		g2 += (frames[i+rgba2Entries+rgba2G2] - g2) * tt
		b2 += (frames[i+rgba2Entries+rgba2B2] - b2) * tt
	case CurveStepped:
		r = frames[i+rgba2R]
		g = frames[i+rgba2G]
		b = frames[i+rgba2B]
		a = frames[i+rgba2A]
		r2 = frames[i+rgba2R2]
		g2 = frames[i+rgba2G2]
		b2 = frames[i+rgba2B2]
	default:
		r = t.BezierValue(time, i, rgba2R, curveType-CurveBezier)
		g = t.BezierValue(time, i, rgba2G, curveType+BezierSize-CurveBezier)
		b = t.BezierValue(time, i, rgba2B, curveType+BezierSize*2-CurveBezier)
		a = t.BezierValue(time, i, rgba2A, curveType+BezierSize*3-CurveBezier)
		r2 = t.BezierValue(time, i, rgba2R2, curveType+BezierSize*4-CurveBezier)
		g2 = t.BezierValue(time, i, rgba2G2, curveType+BezierSize*5-CurveBezier)
		b2 = t.BezierValue(time, i, rgba2B2, curveType+BezierSize*6-CurveBezier)
	}

	if alpha == 1 {
		light.Set(r, g, b, a)
		dark.R = r2
		dark.G = g2
		dark.B = b2
	} else {
		if blend == MixBlendSetup {
			light.SetColor(slot.Data.Color)
			setupDark := slot.Data.DarkColor
			dark.R = setupDark.R
			dark.G = setupDark.G
			dark.B = setupDark.B
		}
		light.Add((r-light.R)*alpha, (g-light.G)*alpha, (b-light.B)*alpha, (a-light.A)*alpha)
		dark.R += (r2 - dark.R) * alpha
		dark.G += (g2 - dark.G) * alpha
		dark.B += (b2 - dark.B) * alpha
	}
}

// RGB2Timeline frame entries.
const (
	rgb2Entries = 7
	rgb2R       = 1
	rgb2G       = 2
	rgb2B       = 3
	rgb2R2      = 4
	rgb2G2      = 5
	rgb2B2      = 6
)

// RGB2Timeline changes the RGB for a slot's color and dark color for two color
// tinting.
type RGB2Timeline struct {
	CurveTimeline
	SlotIndex int
}

// NewRGB2Timeline creates an RGB2 timeline.
func NewRGB2Timeline(frameCount, bezierCount, slotIndex int) *RGB2Timeline {
	return &RGB2Timeline{
		CurveTimeline: newCurveTimeline(frameCount, rgb2Entries, bezierCount,
			propertyID(propertyRGB, slotIndex),
			propertyID(propertyRGB2, slotIndex)),
		SlotIndex: slotIndex,
	}
}

// GetSlotIndex implements SlotTimeline. The slot's DarkColor must not be nil.
func (t *RGB2Timeline) GetSlotIndex() int { return t.SlotIndex }

// SetFrame sets the time, light color, and dark color for the specified frame.
// frame is between 0 and frameCount, inclusive.
func (t *RGB2Timeline) SetFrame(frame int, time, r, g, b, r2, g2, b2 float32) {
	frame *= rgb2Entries
	t.FramesData[frame] = time
	t.FramesData[frame+rgb2R] = r
	t.FramesData[frame+rgb2G] = g
	t.FramesData[frame+rgb2B] = b
	t.FramesData[frame+rgb2R2] = r2
	t.FramesData[frame+rgb2G2] = g2
	t.FramesData[frame+rgb2B2] = b2
}

// Apply implements Timeline.
func (t *RGB2Timeline) Apply(skeleton *Skeleton, lastTime, time float32, events *[]*Event, alpha float32,
	blend MixBlend, direction MixDirection) {

	slot := skeleton.Slots[t.SlotIndex]
	if !slot.Bone.active {
		return
	}

	frames := t.FramesData
	light, dark := &slot.Color, slot.DarkColor
	if time < frames[0] {
		setupLight, setupDark := &slot.Data.Color, slot.Data.DarkColor
		switch blend {
		case MixBlendSetup:
			light.R = setupLight.R
			light.G = setupLight.G
			light.B = setupLight.B
			dark.R = setupDark.R
			dark.G = setupDark.G
			dark.B = setupDark.B
			return
		case MixBlendFirst:
			light.R += (setupLight.R - light.R) * alpha
			light.G += (setupLight.G - light.G) * alpha
			light.B += (setupLight.B - light.B) * alpha
			dark.R += (setupDark.R - dark.R) * alpha
			dark.G += (setupDark.G - dark.G) * alpha
			dark.B += (setupDark.B - dark.B) * alpha
		}
		return
	}

	var r, g, b, r2, g2, b2 float32
	i := searchN(frames, time, rgb2Entries)
	curveType := int(t.Curves[i/rgb2Entries])
	switch curveType {
	case CurveLinear:
		before := frames[i]
		r = frames[i+rgb2R]
		g = frames[i+rgb2G]
		b = frames[i+rgb2B]
		r2 = frames[i+rgb2R2]
		g2 = frames[i+rgb2G2]
		b2 = frames[i+rgb2B2]
		tt := (time - before) / (frames[i+rgb2Entries] - before)
		r += (frames[i+rgb2Entries+rgb2R] - r) * tt
		g += (frames[i+rgb2Entries+rgb2G] - g) * tt
		b += (frames[i+rgb2Entries+rgb2B] - b) * tt
		r2 += (frames[i+rgb2Entries+rgb2R2] - r2) * tt
		g2 += (frames[i+rgb2Entries+rgb2G2] - g2) * tt
		b2 += (frames[i+rgb2Entries+rgb2B2] - b2) * tt
	case CurveStepped:
		r = frames[i+rgb2R]
		g = frames[i+rgb2G]
		b = frames[i+rgb2B]
		r2 = frames[i+rgb2R2]
		g2 = frames[i+rgb2G2]
		b2 = frames[i+rgb2B2]
	default:
		r = t.BezierValue(time, i, rgb2R, curveType-CurveBezier)
		g = t.BezierValue(time, i, rgb2G, curveType+BezierSize-CurveBezier)
		b = t.BezierValue(time, i, rgb2B, curveType+BezierSize*2-CurveBezier)
		r2 = t.BezierValue(time, i, rgb2R2, curveType+BezierSize*3-CurveBezier)
		g2 = t.BezierValue(time, i, rgb2G2, curveType+BezierSize*4-CurveBezier)
		b2 = t.BezierValue(time, i, rgb2B2, curveType+BezierSize*5-CurveBezier)
	}

	if alpha == 1 {
		light.R = r
		light.G = g
		light.B = b
		dark.R = r2
		dark.G = g2
		dark.B = b2
	} else {
		if blend == MixBlendSetup {
			setupLight, setupDark := &slot.Data.Color, slot.Data.DarkColor
			light.R = setupLight.R
			light.G = setupLight.G
			light.B = setupLight.B
			dark.R = setupDark.R
			dark.G = setupDark.G
			dark.B = setupDark.B
		}
		light.R += (r - light.R) * alpha
		light.G += (g - light.G) * alpha
		light.B += (b - light.B) * alpha
		dark.R += (r2 - dark.R) * alpha
		dark.G += (g2 - dark.G) * alpha
		dark.B += (b2 - dark.B) * alpha
	}
}

// AttachmentTimeline changes a slot's attachment.
type AttachmentTimeline struct {
	TimelineBase
	SlotIndex int

	// AttachmentNames holds the attachment name for each frame. May contain
	// empty strings to clear the attachment.
	AttachmentNames []string
}

// NewAttachmentTimeline creates an attachment timeline.
func NewAttachmentTimeline(frameCount, slotIndex int) *AttachmentTimeline {
	return &AttachmentTimeline{
		TimelineBase:    newTimelineBase(frameCount, 1, propertyID(propertyAttachment, slotIndex)),
		SlotIndex:       slotIndex,
		AttachmentNames: make([]string, frameCount),
	}
}

// GetSlotIndex implements SlotTimeline.
func (t *AttachmentTimeline) GetSlotIndex() int { return t.SlotIndex }

// SetFrame sets the time and attachment name for the specified frame.
// frame is between 0 and frameCount, inclusive.
func (t *AttachmentTimeline) SetFrame(frame int, time float32, attachmentName string) {
	t.FramesData[frame] = time
	t.AttachmentNames[frame] = attachmentName
}

// Apply implements Timeline.
func (t *AttachmentTimeline) Apply(skeleton *Skeleton, lastTime, time float32, events *[]*Event, alpha float32,
	blend MixBlend, direction MixDirection) {

	slot := skeleton.Slots[t.SlotIndex]
	if !slot.Bone.active {
		return
	}

	if direction == MixDirectionOut {
		if blend == MixBlendSetup {
			t.setAttachment(skeleton, slot, slot.Data.AttachmentName)
		}
		return
	}

	if time < t.FramesData[0] {
		if blend == MixBlendSetup || blend == MixBlendFirst {
			t.setAttachment(skeleton, slot, slot.Data.AttachmentName)
		}
		return
	}

	t.setAttachment(skeleton, slot, t.AttachmentNames[search1(t.FramesData, time)])
}

func (t *AttachmentTimeline) setAttachment(skeleton *Skeleton, slot *Slot, attachmentName string) {
	if attachmentName == "" {
		slot.SetAttachment(nil)
	} else {
		slot.SetAttachment(skeleton.AttachmentByIndex(t.SlotIndex, attachmentName))
	}
}

// DeformTimeline changes a slot's deform to deform a VertexAttachment.
type DeformTimeline struct {
	CurveTimeline
	SlotIndex int

	// Attachment is the attachment that will be deformed. Timelines are only
	// applied when the slot attachment's TimelineAttachment is this attachment.
	Attachment Attachment

	// Vertices holds the vertices for each frame.
	Vertices [][]float32
}

// NewDeformTimeline creates a deform timeline. attachment must be a vertex
// attachment.
func NewDeformTimeline(frameCount, bezierCount, slotIndex int, attachment Attachment) *DeformTimeline {
	va := AsVertexAttachment(attachment)
	if va == nil {
		panic("spine: attachment must be a vertex attachment")
	}
	return &DeformTimeline{
		CurveTimeline: newCurveTimeline(frameCount, 1, bezierCount,
			propertyID(propertyDeform, slotIndex)+"|"+strconv.Itoa(va.ID())),
		SlotIndex:  slotIndex,
		Attachment: attachment,
		Vertices:   make([][]float32, frameCount),
	}
}

// GetSlotIndex implements SlotTimeline.
func (t *DeformTimeline) GetSlotIndex() int { return t.SlotIndex }

// SetFrame sets the time and vertices for the specified frame. frame is
// between 0 and frameCount, inclusive. vertices are vertex positions for an
// unweighted VertexAttachment, or deform offsets if it has weights.
func (t *DeformTimeline) SetFrame(frame int, time float32, vertices []float32) {
	t.FramesData[frame] = time
	t.Vertices[frame] = vertices
}

// SetBezier stores the segments for the specified Bezier curve. value1 is
// ignored (0 is used for a deform timeline). value2 is ignored (1 is used for
// a deform timeline).
func (t *DeformTimeline) SetBezier(bezier, frame, value int, time1, value1, cx1, cy1, cx2,
	cy2, time2, value2 float32) {
	curves := t.Curves
	i := t.FrameCount() + bezier*BezierSize
	if value == 0 {
		curves[frame] = float32(CurveBezier + i)
	}
	tmpx := (time1 - cx1*2 + cx2) * 0.03
	tmpy := cy2*0.03 - cy1*0.06
	dddx := ((cx1-cx2)*3 - time1 + time2) * 0.006
	dddy := (cy1 - cy2 + 0.33333333) * 0.018
	ddx := tmpx*2 + dddx
	ddy := tmpy*2 + dddy
	dx := (cx1-time1)*0.3 + tmpx + dddx*0.16666667
	dy := cy1*0.3 + tmpy + dddy*0.16666667
	x := time1 + dx
	y := dy
	for n := i + BezierSize; i < n; i += 2 {
		curves[i] = x
		curves[i+1] = y
		dx += ddx
		dy += ddy
		ddx += dddx
		ddy += dddy
		x += dx
		y += dy
	}
}

// getCurvePercent returns the interpolated percentage for the specified time.
// frame is the frame before time.
func (t *DeformTimeline) getCurvePercent(time float32, frame int) float32 {
	curves := t.Curves
	i := int(curves[frame])
	switch i {
	case CurveLinear:
		x := t.FramesData[frame]
		return (time - x) / (t.FramesData[frame+t.FrameEntries()] - x)
	case CurveStepped:
		return 0
	}
	i -= CurveBezier
	if curves[i] > time {
		x := t.FramesData[frame]
		return curves[i+1] * (time - x) / (curves[i] - x)
	}
	n := i + BezierSize
	for i += 2; i < n; i += 2 {
		if curves[i] >= time {
			x, y := curves[i-2], curves[i-1]
			return y + (time-x)/(curves[i]-x)*(curves[i+1]-y)
		}
	}
	x, y := curves[n-2], curves[n-1]
	return y + (1-y)*(time-x)/(t.FramesData[frame+t.FrameEntries()]-x)
}

// Apply implements Timeline.
func (t *DeformTimeline) Apply(skeleton *Skeleton, lastTime, time float32, events *[]*Event, alpha float32,
	blend MixBlend, direction MixDirection) {

	slot := skeleton.Slots[t.SlotIndex]
	if !slot.Bone.active {
		return
	}
	slotAttachment := slot.Attachment()
	vertexAttachment := AsVertexAttachment(slotAttachment)
	if vertexAttachment == nil || vertexAttachment.TimelineAttachment != t.Attachment {
		return
	}

	if len(slot.Deform) == 0 {
		blend = MixBlendSetup
	}

	vertices := t.Vertices
	vertexCount := len(vertices[0])

	frames := t.FramesData
	if time < frames[0] {
		switch blend {
		case MixBlendSetup:
			slot.Deform = slot.Deform[:0]
			return
		case MixBlendFirst:
			if alpha == 1 {
				slot.Deform = slot.Deform[:0]
				return
			}
			deform := ensureSize(&slot.Deform, vertexCount)
			if vertexAttachment.Bones == nil {
				// Unweighted vertex positions.
				setupVertices := vertexAttachment.Vertices
				for i := 0; i < vertexCount; i++ {
					deform[i] += (setupVertices[i] - deform[i]) * alpha
				}
			} else {
				// Weighted deform offsets.
				alpha = 1 - alpha
				for i := 0; i < vertexCount; i++ {
					deform[i] *= alpha
				}
			}
		}
		return
	}

	deform := ensureSize(&slot.Deform, vertexCount)

	if time >= frames[len(frames)-1] { // Time is after last frame.
		lastVertices := vertices[len(frames)-1]
		if alpha == 1 {
			if blend == MixBlendAdd {
				if vertexAttachment.Bones == nil {
					// Unweighted vertex positions, no alpha.
					setupVertices := vertexAttachment.Vertices
					for i := 0; i < vertexCount; i++ {
						deform[i] += lastVertices[i] - setupVertices[i]
					}
				} else {
					// Weighted deform offsets, no alpha.
					for i := 0; i < vertexCount; i++ {
						deform[i] += lastVertices[i]
					}
				}
			} else {
				// Vertex positions or deform offsets, no alpha.
				copy(deform[:vertexCount], lastVertices)
			}
		} else {
			switch blend {
			case MixBlendSetup:
				if vertexAttachment.Bones == nil {
					// Unweighted vertex positions, with alpha.
					setupVertices := vertexAttachment.Vertices
					for i := 0; i < vertexCount; i++ {
						setup := setupVertices[i]
						deform[i] = setup + (lastVertices[i]-setup)*alpha
					}
				} else {
					// Weighted deform offsets, with alpha.
					for i := 0; i < vertexCount; i++ {
						deform[i] = lastVertices[i] * alpha
					}
				}
			case MixBlendFirst, MixBlendReplace:
				// Vertex positions or deform offsets, with alpha.
				for i := 0; i < vertexCount; i++ {
					deform[i] += (lastVertices[i] - deform[i]) * alpha
				}
			case MixBlendAdd:
				if vertexAttachment.Bones == nil {
					// Unweighted vertex positions, no alpha.
					setupVertices := vertexAttachment.Vertices
					for i := 0; i < vertexCount; i++ {
						deform[i] += (lastVertices[i] - setupVertices[i]) * alpha
					}
				} else {
					// Weighted deform offsets, alpha.
					for i := 0; i < vertexCount; i++ {
						deform[i] += lastVertices[i] * alpha
					}
				}
			}
		}
		return
	}

	frame := search1(frames, time)
	percent := t.getCurvePercent(time, frame)
	prevVertices := vertices[frame]
	nextVertices := vertices[frame+1]

	if alpha == 1 {
		if blend == MixBlendAdd {
			if vertexAttachment.Bones == nil {
				// Unweighted vertex positions, no alpha.
				setupVertices := vertexAttachment.Vertices
				for i := 0; i < vertexCount; i++ {
					prev := prevVertices[i]
					deform[i] += prev + (nextVertices[i]-prev)*percent - setupVertices[i]
				}
			} else {
				// Weighted deform offsets, no alpha.
				for i := 0; i < vertexCount; i++ {
					prev := prevVertices[i]
					deform[i] += prev + (nextVertices[i]-prev)*percent
				}
			}
		} else {
			// Vertex positions or deform offsets, no alpha.
			for i := 0; i < vertexCount; i++ {
				prev := prevVertices[i]
				deform[i] = prev + (nextVertices[i]-prev)*percent
			}
		}
	} else {
		switch blend {
		case MixBlendSetup:
			if vertexAttachment.Bones == nil {
				// Unweighted vertex positions, with alpha.
				setupVertices := vertexAttachment.Vertices
				for i := 0; i < vertexCount; i++ {
					prev, setup := prevVertices[i], setupVertices[i]
					deform[i] = setup + (prev+(nextVertices[i]-prev)*percent-setup)*alpha
				}
			} else {
				// Weighted deform offsets, with alpha.
				for i := 0; i < vertexCount; i++ {
					prev := prevVertices[i]
					deform[i] = (prev + (nextVertices[i]-prev)*percent) * alpha
				}
			}
		case MixBlendFirst, MixBlendReplace:
			// Vertex positions or deform offsets, with alpha.
			for i := 0; i < vertexCount; i++ {
				prev := prevVertices[i]
				deform[i] += (prev + (nextVertices[i]-prev)*percent - deform[i]) * alpha
			}
		case MixBlendAdd:
			if vertexAttachment.Bones == nil {
				// Unweighted vertex positions, with alpha.
				setupVertices := vertexAttachment.Vertices
				for i := 0; i < vertexCount; i++ {
					prev := prevVertices[i]
					deform[i] += (prev + (nextVertices[i]-prev)*percent - setupVertices[i]) * alpha
				}
			} else {
				// Weighted deform offsets, with alpha.
				for i := 0; i < vertexCount; i++ {
					prev := prevVertices[i]
					deform[i] += (prev + (nextVertices[i]-prev)*percent) * alpha
				}
			}
		}
	}
}

// EventTimeline fires an Event when specific animation times are reached.
type EventTimeline struct {
	TimelineBase

	// Events holds the event for each frame.
	Events []*Event
}

// NewEventTimeline creates an event timeline.
func NewEventTimeline(frameCount int) *EventTimeline {
	return &EventTimeline{
		TimelineBase: newTimelineBase(frameCount, 1, strconv.Itoa(int(propertyEvent))),
		Events:       make([]*Event, frameCount),
	}
}

// SetFrame sets the time and event for the specified frame.
// frame is between 0 and frameCount, inclusive.
func (t *EventTimeline) SetFrame(frame int, event *Event) {
	t.FramesData[frame] = event.Time
	t.Events[frame] = event
}

// Apply fires events for frames > lastTime and <= time.
func (t *EventTimeline) Apply(skeleton *Skeleton, lastTime, time float32, firedEvents *[]*Event, alpha float32,
	blend MixBlend, direction MixDirection) {

	if firedEvents == nil {
		return
	}

	frames := t.FramesData
	frameCount := len(frames)

	if lastTime > time { // Apply after lastTime for looped animations.
		t.Apply(skeleton, lastTime, float32(math.MaxInt32), firedEvents, alpha, blend, direction)
		lastTime = -1
	} else if lastTime >= frames[frameCount-1] { // Last time is after last frame.
		return
	}
	if time < frames[0] {
		return
	}

	var i int
	if lastTime < frames[0] {
		i = 0
	} else {
		i = search1(frames, lastTime) + 1
		frameTime := frames[i]
		for i > 0 { // Fire multiple events with the same frame.
			if frames[i-1] != frameTime {
				break
			}
			i--
		}
	}
	for ; i < frameCount && time >= frames[i]; i++ {
		*firedEvents = append(*firedEvents, t.Events[i])
	}
}

// DrawOrderTimeline changes a skeleton's draw order.
type DrawOrderTimeline struct {
	TimelineBase

	// DrawOrders holds the draw order for each frame. See SetFrame.
	DrawOrders [][]int
}

// NewDrawOrderTimeline creates a draw order timeline.
func NewDrawOrderTimeline(frameCount int) *DrawOrderTimeline {
	return &DrawOrderTimeline{
		TimelineBase: newTimelineBase(frameCount, 1, strconv.Itoa(int(propertyDrawOrder))),
		DrawOrders:   make([][]int, frameCount),
	}
}

// SetFrame sets the time and draw order for the specified frame. frame is
// between 0 and frameCount, inclusive. drawOrder holds, for each slot in
// Skeleton.Slots, the index of the slot in the new draw order. May be nil to
// use setup pose draw order.
func (t *DrawOrderTimeline) SetFrame(frame int, time float32, drawOrder []int) {
	t.FramesData[frame] = time
	t.DrawOrders[frame] = drawOrder
}

// Apply implements Timeline.
func (t *DrawOrderTimeline) Apply(skeleton *Skeleton, lastTime, time float32, events *[]*Event, alpha float32,
	blend MixBlend, direction MixDirection) {

	if direction == MixDirectionOut {
		if blend == MixBlendSetup {
			copy(skeleton.DrawOrder, skeleton.Slots)
		}
		return
	}

	if time < t.FramesData[0] {
		if blend == MixBlendSetup || blend == MixBlendFirst {
			copy(skeleton.DrawOrder, skeleton.Slots)
		}
		return
	}

	drawOrderToSetupIndex := t.DrawOrders[search1(t.FramesData, time)]
	if drawOrderToSetupIndex == nil {
		copy(skeleton.DrawOrder, skeleton.Slots)
	} else {
		slots := skeleton.Slots
		drawOrder := skeleton.DrawOrder
		for i, n := 0, len(drawOrderToSetupIndex); i < n; i++ {
			drawOrder[i] = slots[drawOrderToSetupIndex[i]]
		}
	}
}

// IkConstraintTimeline frame entries.
const (
	ikEntries       = 6
	ikMix           = 1
	ikSoftness      = 2
	ikBendDirection = 3
	ikCompress      = 4
	ikStretch       = 5
)

// IkConstraintTimeline changes an IK constraint's mix, softness, bend
// direction, stretch, and compress.
type IkConstraintTimeline struct {
	CurveTimeline

	// ConstraintIndex is the index of the IK constraint in
	// Skeleton.IkConstraints that will be changed when this timeline is applied.
	ConstraintIndex int
}

// NewIkConstraintTimeline creates an IK constraint timeline.
func NewIkConstraintTimeline(frameCount, bezierCount, ikConstraintIndex int) *IkConstraintTimeline {
	return &IkConstraintTimeline{
		CurveTimeline:   newCurveTimeline(frameCount, ikEntries, bezierCount, propertyID(propertyIkConstraint, ikConstraintIndex)),
		ConstraintIndex: ikConstraintIndex,
	}
}

// SetFrame sets the time, mix, softness, bend direction, compress, and stretch
// for the specified frame. frame is between 0 and frameCount, inclusive.
// bendDirection is 1 or -1.
func (t *IkConstraintTimeline) SetFrame(frame int, time, mix, softness float32, bendDirection int, compress,
	stretch bool) {
	frame *= ikEntries
	t.FramesData[frame] = time
	t.FramesData[frame+ikMix] = mix
	t.FramesData[frame+ikSoftness] = softness
	t.FramesData[frame+ikBendDirection] = float32(bendDirection)
	if compress {
		t.FramesData[frame+ikCompress] = 1
	} else {
		t.FramesData[frame+ikCompress] = 0
	}
	if stretch {
		t.FramesData[frame+ikStretch] = 1
	} else {
		t.FramesData[frame+ikStretch] = 0
	}
}

// Apply implements Timeline.
func (t *IkConstraintTimeline) Apply(skeleton *Skeleton, lastTime, time float32, events *[]*Event, alpha float32,
	blend MixBlend, direction MixDirection) {

	constraint := skeleton.IkConstraints[t.ConstraintIndex]
	if !constraint.active {
		return
	}

	frames := t.FramesData
	if time < frames[0] {
		switch blend {
		case MixBlendSetup:
			constraint.Mix = constraint.Data.Mix
			constraint.Softness = constraint.Data.Softness
			constraint.BendDirection = constraint.Data.BendDirection
			constraint.Compress = constraint.Data.Compress
			constraint.Stretch = constraint.Data.Stretch
			return
		case MixBlendFirst:
			constraint.Mix += (constraint.Data.Mix - constraint.Mix) * alpha
			constraint.Softness += (constraint.Data.Softness - constraint.Softness) * alpha
			constraint.BendDirection = constraint.Data.BendDirection
			constraint.Compress = constraint.Data.Compress
			constraint.Stretch = constraint.Data.Stretch
		}
		return
	}

	var mix, softness float32
	i := searchN(frames, time, ikEntries)
	curveType := int(t.Curves[i/ikEntries])
	switch curveType {
	case CurveLinear:
		before := frames[i]
		mix = frames[i+ikMix]
		softness = frames[i+ikSoftness]
		tt := (time - before) / (frames[i+ikEntries] - before)
		mix += (frames[i+ikEntries+ikMix] - mix) * tt
		softness += (frames[i+ikEntries+ikSoftness] - softness) * tt
	case CurveStepped:
		mix = frames[i+ikMix]
		softness = frames[i+ikSoftness]
	default:
		mix = t.BezierValue(time, i, ikMix, curveType-CurveBezier)
		softness = t.BezierValue(time, i, ikSoftness, curveType+BezierSize-CurveBezier)
	}

	if blend == MixBlendSetup {
		constraint.Mix = constraint.Data.Mix + (mix-constraint.Data.Mix)*alpha
		constraint.Softness = constraint.Data.Softness + (softness-constraint.Data.Softness)*alpha
		if direction == MixDirectionOut {
			constraint.BendDirection = constraint.Data.BendDirection
			constraint.Compress = constraint.Data.Compress
			constraint.Stretch = constraint.Data.Stretch
		} else {
			constraint.BendDirection = int(frames[i+ikBendDirection])
			constraint.Compress = frames[i+ikCompress] != 0
			constraint.Stretch = frames[i+ikStretch] != 0
		}
	} else {
		constraint.Mix += (mix - constraint.Mix) * alpha
		constraint.Softness += (softness - constraint.Softness) * alpha
		if direction == MixDirectionIn {
			constraint.BendDirection = int(frames[i+ikBendDirection])
			constraint.Compress = frames[i+ikCompress] != 0
			constraint.Stretch = frames[i+ikStretch] != 0
		}
	}
}

// TransformConstraintTimeline frame entries.
const (
	tcEntries = 7
	tcRotate  = 1
	tcX       = 2
	tcY       = 3
	tcScaleX  = 4
	tcScaleY  = 5
	tcShearY  = 6
)

// TransformConstraintTimeline changes a transform constraint's mixes.
type TransformConstraintTimeline struct {
	CurveTimeline

	// ConstraintIndex is the index of the transform constraint in
	// Skeleton.TransformConstraints that will be changed when this timeline is
	// applied.
	ConstraintIndex int
}

// NewTransformConstraintTimeline creates a transform constraint timeline.
func NewTransformConstraintTimeline(frameCount, bezierCount, transformConstraintIndex int) *TransformConstraintTimeline {
	return &TransformConstraintTimeline{
		CurveTimeline: newCurveTimeline(frameCount, tcEntries, bezierCount,
			propertyID(propertyTransformConstraint, transformConstraintIndex)),
		ConstraintIndex: transformConstraintIndex,
	}
}

// SetFrame sets the time, rotate mix, translate mix, scale mix, and shear mix
// for the specified frame. frame is between 0 and frameCount, inclusive.
func (t *TransformConstraintTimeline) SetFrame(frame int, time, mixRotate, mixX, mixY, mixScaleX, mixScaleY,
	mixShearY float32) {
	frame *= tcEntries
	t.FramesData[frame] = time
	t.FramesData[frame+tcRotate] = mixRotate
	t.FramesData[frame+tcX] = mixX
	t.FramesData[frame+tcY] = mixY
	t.FramesData[frame+tcScaleX] = mixScaleX
	t.FramesData[frame+tcScaleY] = mixScaleY
	t.FramesData[frame+tcShearY] = mixShearY
}

// Apply implements Timeline.
func (t *TransformConstraintTimeline) Apply(skeleton *Skeleton, lastTime, time float32, events *[]*Event, alpha float32,
	blend MixBlend, direction MixDirection) {

	constraint := skeleton.TransformConstraints[t.ConstraintIndex]
	if !constraint.active {
		return
	}

	frames := t.FramesData
	if time < frames[0] {
		data := constraint.Data
		switch blend {
		case MixBlendSetup:
			constraint.MixRotate = data.MixRotate
			constraint.MixX = data.MixX
			constraint.MixY = data.MixY
			constraint.MixScaleX = data.MixScaleX
			constraint.MixScaleY = data.MixScaleY
			constraint.MixShearY = data.MixShearY
			return
		case MixBlendFirst:
			constraint.MixRotate += (data.MixRotate - constraint.MixRotate) * alpha
			constraint.MixX += (data.MixX - constraint.MixX) * alpha
			constraint.MixY += (data.MixY - constraint.MixY) * alpha
			constraint.MixScaleX += (data.MixScaleX - constraint.MixScaleX) * alpha
			constraint.MixScaleY += (data.MixScaleY - constraint.MixScaleY) * alpha
			constraint.MixShearY += (data.MixShearY - constraint.MixShearY) * alpha
		}
		return
	}

	var rotate, x, y, scaleX, scaleY, shearY float32
	i := searchN(frames, time, tcEntries)
	curveType := int(t.Curves[i/tcEntries])
	switch curveType {
	case CurveLinear:
		before := frames[i]
		rotate = frames[i+tcRotate]
		x = frames[i+tcX]
		y = frames[i+tcY]
		scaleX = frames[i+tcScaleX]
		scaleY = frames[i+tcScaleY]
		shearY = frames[i+tcShearY]
		tt := (time - before) / (frames[i+tcEntries] - before)
		rotate += (frames[i+tcEntries+tcRotate] - rotate) * tt
		x += (frames[i+tcEntries+tcX] - x) * tt
		y += (frames[i+tcEntries+tcY] - y) * tt
		scaleX += (frames[i+tcEntries+tcScaleX] - scaleX) * tt
		scaleY += (frames[i+tcEntries+tcScaleY] - scaleY) * tt
		shearY += (frames[i+tcEntries+tcShearY] - shearY) * tt
	case CurveStepped:
		rotate = frames[i+tcRotate]
		x = frames[i+tcX]
		y = frames[i+tcY]
		scaleX = frames[i+tcScaleX]
		scaleY = frames[i+tcScaleY]
		shearY = frames[i+tcShearY]
	default:
		rotate = t.BezierValue(time, i, tcRotate, curveType-CurveBezier)
		x = t.BezierValue(time, i, tcX, curveType+BezierSize-CurveBezier)
		y = t.BezierValue(time, i, tcY, curveType+BezierSize*2-CurveBezier)
		scaleX = t.BezierValue(time, i, tcScaleX, curveType+BezierSize*3-CurveBezier)
		scaleY = t.BezierValue(time, i, tcScaleY, curveType+BezierSize*4-CurveBezier)
		shearY = t.BezierValue(time, i, tcShearY, curveType+BezierSize*5-CurveBezier)
	}

	if blend == MixBlendSetup {
		data := constraint.Data
		constraint.MixRotate = data.MixRotate + (rotate-data.MixRotate)*alpha
		constraint.MixX = data.MixX + (x-data.MixX)*alpha
		constraint.MixY = data.MixY + (y-data.MixY)*alpha
		constraint.MixScaleX = data.MixScaleX + (scaleX-data.MixScaleX)*alpha
		constraint.MixScaleY = data.MixScaleY + (scaleY-data.MixScaleY)*alpha
		constraint.MixShearY = data.MixShearY + (shearY-data.MixShearY)*alpha
	} else {
		constraint.MixRotate += (rotate - constraint.MixRotate) * alpha
		constraint.MixX += (x - constraint.MixX) * alpha
		constraint.MixY += (y - constraint.MixY) * alpha
		constraint.MixScaleX += (scaleX - constraint.MixScaleX) * alpha
		constraint.MixScaleY += (scaleY - constraint.MixScaleY) * alpha
		constraint.MixShearY += (shearY - constraint.MixShearY) * alpha
	}
}

// PathConstraintPositionTimeline changes a path constraint's position.
type PathConstraintPositionTimeline struct {
	CurveTimeline1

	// ConstraintIndex is the index of the path constraint in
	// Skeleton.PathConstraints that will be changed when this timeline is
	// applied.
	ConstraintIndex int
}

// NewPathConstraintPositionTimeline creates a path constraint position timeline.
func NewPathConstraintPositionTimeline(frameCount, bezierCount, pathConstraintIndex int) *PathConstraintPositionTimeline {
	return &PathConstraintPositionTimeline{
		CurveTimeline1: newCurveTimeline1(frameCount, bezierCount,
			propertyID(propertyPathConstraintPosition, pathConstraintIndex)),
		ConstraintIndex: pathConstraintIndex,
	}
}

// Apply implements Timeline.
func (t *PathConstraintPositionTimeline) Apply(skeleton *Skeleton, lastTime, time float32, events *[]*Event,
	alpha float32, blend MixBlend, direction MixDirection) {

	constraint := skeleton.PathConstraints[t.ConstraintIndex]
	if constraint.active {
		constraint.Position = t.GetAbsoluteValue(time, alpha, blend, constraint.Position, constraint.Data.Position)
	}
}

// PathConstraintSpacingTimeline changes a path constraint's spacing.
type PathConstraintSpacingTimeline struct {
	CurveTimeline1

	// ConstraintIndex is the index of the path constraint in
	// Skeleton.PathConstraints that will be changed when this timeline is
	// applied.
	ConstraintIndex int
}

// NewPathConstraintSpacingTimeline creates a path constraint spacing timeline.
func NewPathConstraintSpacingTimeline(frameCount, bezierCount, pathConstraintIndex int) *PathConstraintSpacingTimeline {
	return &PathConstraintSpacingTimeline{
		CurveTimeline1: newCurveTimeline1(frameCount, bezierCount,
			propertyID(propertyPathConstraintSpacing, pathConstraintIndex)),
		ConstraintIndex: pathConstraintIndex,
	}
}

// Apply implements Timeline.
func (t *PathConstraintSpacingTimeline) Apply(skeleton *Skeleton, lastTime, time float32, events *[]*Event,
	alpha float32, blend MixBlend, direction MixDirection) {

	constraint := skeleton.PathConstraints[t.ConstraintIndex]
	if constraint.active {
		constraint.Spacing = t.GetAbsoluteValue(time, alpha, blend, constraint.Spacing, constraint.Data.Spacing)
	}
}

// PathConstraintMixTimeline frame entries.
const (
	pcmEntries = 4
	pcmRotate  = 1
	pcmX       = 2
	pcmY       = 3
)

// PathConstraintMixTimeline changes a path constraint's mixRotate, mixX, and
// mixY.
type PathConstraintMixTimeline struct {
	CurveTimeline

	// ConstraintIndex is the index of the path constraint in
	// Skeleton.PathConstraints that will be changed when this timeline is
	// applied.
	ConstraintIndex int
}

// NewPathConstraintMixTimeline creates a path constraint mix timeline.
func NewPathConstraintMixTimeline(frameCount, bezierCount, pathConstraintIndex int) *PathConstraintMixTimeline {
	return &PathConstraintMixTimeline{
		CurveTimeline: newCurveTimeline(frameCount, pcmEntries, bezierCount,
			propertyID(propertyPathConstraintMix, pathConstraintIndex)),
		ConstraintIndex: pathConstraintIndex,
	}
}

// SetFrame sets the time and mixes for the specified frame.
// frame is between 0 and frameCount, inclusive.
func (t *PathConstraintMixTimeline) SetFrame(frame int, time, mixRotate, mixX, mixY float32) {
	frame <<= 2
	t.FramesData[frame] = time
	t.FramesData[frame+pcmRotate] = mixRotate
	t.FramesData[frame+pcmX] = mixX
	t.FramesData[frame+pcmY] = mixY
}

// Apply implements Timeline.
func (t *PathConstraintMixTimeline) Apply(skeleton *Skeleton, lastTime, time float32, events *[]*Event, alpha float32,
	blend MixBlend, direction MixDirection) {

	constraint := skeleton.PathConstraints[t.ConstraintIndex]
	if !constraint.active {
		return
	}

	frames := t.FramesData
	if time < frames[0] {
		data := constraint.Data
		switch blend {
		case MixBlendSetup:
			constraint.MixRotate = data.MixRotate
			constraint.MixX = data.MixX
			constraint.MixY = data.MixY
			return
		case MixBlendFirst:
			constraint.MixRotate += (data.MixRotate - constraint.MixRotate) * alpha
			constraint.MixX += (data.MixX - constraint.MixX) * alpha
			constraint.MixY += (data.MixY - constraint.MixY) * alpha
		}
		return
	}

	var rotate, x, y float32
	i := searchN(frames, time, pcmEntries)
	curveType := int(t.Curves[i>>2])
	switch curveType {
	case CurveLinear:
		before := frames[i]
		rotate = frames[i+pcmRotate]
		x = frames[i+pcmX]
		y = frames[i+pcmY]
		tt := (time - before) / (frames[i+pcmEntries] - before)
		rotate += (frames[i+pcmEntries+pcmRotate] - rotate) * tt
		x += (frames[i+pcmEntries+pcmX] - x) * tt
		y += (frames[i+pcmEntries+pcmY] - y) * tt
	case CurveStepped:
		rotate = frames[i+pcmRotate]
		x = frames[i+pcmX]
		y = frames[i+pcmY]
	default:
		rotate = t.BezierValue(time, i, pcmRotate, curveType-CurveBezier)
		x = t.BezierValue(time, i, pcmX, curveType+BezierSize-CurveBezier)
		y = t.BezierValue(time, i, pcmY, curveType+BezierSize*2-CurveBezier)
	}

	if blend == MixBlendSetup {
		data := constraint.Data
		constraint.MixRotate = data.MixRotate + (rotate-data.MixRotate)*alpha
		constraint.MixX = data.MixX + (x-data.MixX)*alpha
		constraint.MixY = data.MixY + (y-data.MixY)*alpha
	} else {
		constraint.MixRotate += (rotate - constraint.MixRotate) * alpha
		constraint.MixX += (x - constraint.MixX) * alpha
		constraint.MixY += (y - constraint.MixY) * alpha
	}
}

// PhysicsConstraintTimeline is the base for most PhysicsConstraint timelines.
type PhysicsConstraintTimeline struct {
	CurveTimeline1

	// ConstraintIndex is the index of the physics constraint in
	// Skeleton.PhysicsConstraints that will be changed when this timeline is
	// applied, or -1 if all physics constraints in the skeleton will be changed.
	ConstraintIndex int

	setup  func(constraint *PhysicsConstraint) float32
	get    func(constraint *PhysicsConstraint) float32
	set    func(constraint *PhysicsConstraint, value float32)
	global func(data *PhysicsConstraintData) bool
}

// physicsConstraintIndex may be -1 for all physics constraints in the skeleton.
func newPhysicsConstraintTimeline(frameCount, bezierCount, physicsConstraintIndex int,
	prop property) PhysicsConstraintTimeline {
	return PhysicsConstraintTimeline{
		CurveTimeline1:  newCurveTimeline1(frameCount, bezierCount, propertyID(prop, physicsConstraintIndex)),
		ConstraintIndex: physicsConstraintIndex,
	}
}

// Apply implements Timeline.
func (t *PhysicsConstraintTimeline) Apply(skeleton *Skeleton, lastTime, time float32, events *[]*Event, alpha float32,
	blend MixBlend, direction MixDirection) {

	if t.ConstraintIndex == -1 {
		var value float32
		if time >= t.FramesData[0] {
			value = t.GetCurveValue(time)
		}

		for _, constraint := range skeleton.PhysicsConstraints {
			if constraint.active && t.global(constraint.Data) {
				t.set(constraint, t.GetAbsoluteValueWith(time, alpha, blend, t.get(constraint), t.setup(constraint), value))
			}
		}
	} else {
		constraint := skeleton.PhysicsConstraints[t.ConstraintIndex]
		if constraint.active {
			t.set(constraint, t.GetAbsoluteValue(time, alpha, blend, t.get(constraint), t.setup(constraint)))
		}
	}
}

// PhysicsConstraintInertiaTimeline changes a physics constraint's inertia.
type PhysicsConstraintInertiaTimeline struct {
	PhysicsConstraintTimeline
}

// NewPhysicsConstraintInertiaTimeline creates a physics constraint inertia timeline.
func NewPhysicsConstraintInertiaTimeline(frameCount, bezierCount, physicsConstraintIndex int) *PhysicsConstraintInertiaTimeline {
	t := &PhysicsConstraintInertiaTimeline{
		PhysicsConstraintTimeline: newPhysicsConstraintTimeline(frameCount, bezierCount, physicsConstraintIndex,
			propertyPhysicsConstraintInertia),
	}
	t.setup = func(constraint *PhysicsConstraint) float32 { return constraint.Data.Inertia }
	t.get = func(constraint *PhysicsConstraint) float32 { return constraint.Inertia }
	t.set = func(constraint *PhysicsConstraint, value float32) { constraint.Inertia = value }
	t.global = func(data *PhysicsConstraintData) bool { return data.InertiaGlobal }
	return t
}

// PhysicsConstraintStrengthTimeline changes a physics constraint's strength.
type PhysicsConstraintStrengthTimeline struct {
	PhysicsConstraintTimeline
}

// NewPhysicsConstraintStrengthTimeline creates a physics constraint strength timeline.
func NewPhysicsConstraintStrengthTimeline(frameCount, bezierCount, physicsConstraintIndex int) *PhysicsConstraintStrengthTimeline {
	t := &PhysicsConstraintStrengthTimeline{
		PhysicsConstraintTimeline: newPhysicsConstraintTimeline(frameCount, bezierCount, physicsConstraintIndex,
			propertyPhysicsConstraintStrength),
	}
	t.setup = func(constraint *PhysicsConstraint) float32 { return constraint.Data.Strength }
	t.get = func(constraint *PhysicsConstraint) float32 { return constraint.Strength }
	t.set = func(constraint *PhysicsConstraint, value float32) { constraint.Strength = value }
	t.global = func(data *PhysicsConstraintData) bool { return data.StrengthGlobal }
	return t
}

// PhysicsConstraintDampingTimeline changes a physics constraint's damping.
type PhysicsConstraintDampingTimeline struct {
	PhysicsConstraintTimeline
}

// NewPhysicsConstraintDampingTimeline creates a physics constraint damping timeline.
func NewPhysicsConstraintDampingTimeline(frameCount, bezierCount, physicsConstraintIndex int) *PhysicsConstraintDampingTimeline {
	t := &PhysicsConstraintDampingTimeline{
		PhysicsConstraintTimeline: newPhysicsConstraintTimeline(frameCount, bezierCount, physicsConstraintIndex,
			propertyPhysicsConstraintDamping),
	}
	t.setup = func(constraint *PhysicsConstraint) float32 { return constraint.Data.Damping }
	t.get = func(constraint *PhysicsConstraint) float32 { return constraint.Damping }
	t.set = func(constraint *PhysicsConstraint, value float32) { constraint.Damping = value }
	t.global = func(data *PhysicsConstraintData) bool { return data.DampingGlobal }
	return t
}

// PhysicsConstraintMassTimeline changes a physics constraint's mass inverse.
// The timeline values are not inverted.
type PhysicsConstraintMassTimeline struct {
	PhysicsConstraintTimeline
}

// NewPhysicsConstraintMassTimeline creates a physics constraint mass timeline.
func NewPhysicsConstraintMassTimeline(frameCount, bezierCount, physicsConstraintIndex int) *PhysicsConstraintMassTimeline {
	t := &PhysicsConstraintMassTimeline{
		PhysicsConstraintTimeline: newPhysicsConstraintTimeline(frameCount, bezierCount, physicsConstraintIndex,
			propertyPhysicsConstraintMass),
	}
	t.setup = func(constraint *PhysicsConstraint) float32 { return 1 / constraint.Data.MassInverse }
	t.get = func(constraint *PhysicsConstraint) float32 { return 1 / constraint.MassInverse }
	t.set = func(constraint *PhysicsConstraint, value float32) { constraint.MassInverse = 1 / value }
	t.global = func(data *PhysicsConstraintData) bool { return data.MassGlobal }
	return t
}

// PhysicsConstraintWindTimeline changes a physics constraint's wind.
type PhysicsConstraintWindTimeline struct {
	PhysicsConstraintTimeline
}

// NewPhysicsConstraintWindTimeline creates a physics constraint wind timeline.
func NewPhysicsConstraintWindTimeline(frameCount, bezierCount, physicsConstraintIndex int) *PhysicsConstraintWindTimeline {
	t := &PhysicsConstraintWindTimeline{
		PhysicsConstraintTimeline: newPhysicsConstraintTimeline(frameCount, bezierCount, physicsConstraintIndex,
			propertyPhysicsConstraintWind),
	}
	t.setup = func(constraint *PhysicsConstraint) float32 { return constraint.Data.Wind }
	t.get = func(constraint *PhysicsConstraint) float32 { return constraint.Wind }
	t.set = func(constraint *PhysicsConstraint, value float32) { constraint.Wind = value }
	t.global = func(data *PhysicsConstraintData) bool { return data.WindGlobal }
	return t
}

// PhysicsConstraintGravityTimeline changes a physics constraint's gravity.
type PhysicsConstraintGravityTimeline struct {
	PhysicsConstraintTimeline
}

// NewPhysicsConstraintGravityTimeline creates a physics constraint gravity timeline.
func NewPhysicsConstraintGravityTimeline(frameCount, bezierCount, physicsConstraintIndex int) *PhysicsConstraintGravityTimeline {
	t := &PhysicsConstraintGravityTimeline{
		PhysicsConstraintTimeline: newPhysicsConstraintTimeline(frameCount, bezierCount, physicsConstraintIndex,
			propertyPhysicsConstraintGravity),
	}
	t.setup = func(constraint *PhysicsConstraint) float32 { return constraint.Data.Gravity }
	t.get = func(constraint *PhysicsConstraint) float32 { return constraint.Gravity }
	t.set = func(constraint *PhysicsConstraint, value float32) { constraint.Gravity = value }
	t.global = func(data *PhysicsConstraintData) bool { return data.GravityGlobal }
	return t
}

// PhysicsConstraintMixTimeline changes a physics constraint's mix.
type PhysicsConstraintMixTimeline struct {
	PhysicsConstraintTimeline
}

// NewPhysicsConstraintMixTimeline creates a physics constraint mix timeline.
func NewPhysicsConstraintMixTimeline(frameCount, bezierCount, physicsConstraintIndex int) *PhysicsConstraintMixTimeline {
	t := &PhysicsConstraintMixTimeline{
		PhysicsConstraintTimeline: newPhysicsConstraintTimeline(frameCount, bezierCount, physicsConstraintIndex,
			propertyPhysicsConstraintMix),
	}
	t.setup = func(constraint *PhysicsConstraint) float32 { return constraint.Data.Mix }
	t.get = func(constraint *PhysicsConstraint) float32 { return constraint.Mix }
	t.set = func(constraint *PhysicsConstraint, value float32) { constraint.Mix = value }
	t.global = func(data *PhysicsConstraintData) bool { return data.MixGlobal }
	return t
}

// PhysicsConstraintResetTimeline resets a physics constraint when specific
// animation times are reached.
type PhysicsConstraintResetTimeline struct {
	TimelineBase

	// ConstraintIndex is the index of the physics constraint in
	// Skeleton.PhysicsConstraints that will be reset when this timeline is
	// applied, or -1 if all physics constraints in the skeleton will be reset.
	ConstraintIndex int
}

// NewPhysicsConstraintResetTimeline creates a physics constraint reset
// timeline. physicsConstraintIndex may be -1 for all physics constraints in
// the skeleton.
func NewPhysicsConstraintResetTimeline(frameCount, physicsConstraintIndex int) *PhysicsConstraintResetTimeline {
	return &PhysicsConstraintResetTimeline{
		TimelineBase:    newTimelineBase(frameCount, 1, strconv.Itoa(int(propertyPhysicsConstraintReset))),
		ConstraintIndex: physicsConstraintIndex,
	}
}

// SetFrame sets the time for the specified frame.
// frame is between 0 and frameCount, inclusive.
func (t *PhysicsConstraintResetTimeline) SetFrame(frame int, time float32) {
	t.FramesData[frame] = time
}

// Apply resets the physics constraint when frames > lastTime and <= time.
func (t *PhysicsConstraintResetTimeline) Apply(skeleton *Skeleton, lastTime, time float32, firedEvents *[]*Event,
	alpha float32, blend MixBlend, direction MixDirection) {

	var constraint *PhysicsConstraint
	if t.ConstraintIndex != -1 {
		constraint = skeleton.PhysicsConstraints[t.ConstraintIndex]
		if !constraint.active {
			return
		}
	}

	frames := t.FramesData

	if lastTime > time { // Apply after lastTime for looped animations.
		t.Apply(skeleton, lastTime, float32(math.MaxInt32), nil, alpha, blend, direction)
		lastTime = -1
	} else if lastTime >= frames[len(frames)-1] { // Last time is after last frame.
		return
	}
	if time < frames[0] {
		return
	}

	if lastTime < frames[0] || time >= frames[search1(frames, lastTime)+1] {
		if constraint != nil {
			constraint.Reset()
		} else {
			for _, constraint := range skeleton.PhysicsConstraints {
				if constraint.active {
					constraint.Reset()
				}
			}
		}
	}
}

// SequenceTimeline frame entries.
const (
	seqEntries = 3
	seqMode    = 1
	seqDelay   = 2
)

// attachmentSequence returns the Sequence of an attachment that has texture
// regions, or nil.
func attachmentSequence(attachment Attachment) *Sequence {
	switch a := attachment.(type) {
	case *RegionAttachment:
		return a.Sequence
	case *MeshAttachment:
		return a.Sequence
	}
	return nil
}

// SequenceTimeline changes a slot's SequenceIndex for an attachment's Sequence.
type SequenceTimeline struct {
	TimelineBase
	SlotIndex int

	// Attachment is the attachment with the sequence.
	Attachment Attachment
}

// NewSequenceTimeline creates a sequence timeline. attachment must have a
// Sequence.
func NewSequenceTimeline(frameCount, slotIndex int, attachment Attachment) *SequenceTimeline {
	sequence := attachmentSequence(attachment)
	if sequence == nil {
		panic("spine: attachment must have a sequence")
	}
	return &SequenceTimeline{
		TimelineBase: newTimelineBase(frameCount, seqEntries,
			propertyID(propertySequence, slotIndex)+"|"+strconv.Itoa(sequence.ID())),
		SlotIndex:  slotIndex,
		Attachment: attachment,
	}
}

// GetSlotIndex implements SlotTimeline.
func (t *SequenceTimeline) GetSlotIndex() int { return t.SlotIndex }

// SetFrame sets the time, mode, index, and frame time for the specified frame.
// frame is between 0 and frameCount, inclusive. delay is seconds between frames.
func (t *SequenceTimeline) SetFrame(frame int, time float32, mode SequenceMode, index int, delay float32) {
	frame *= seqEntries
	t.FramesData[frame] = time
	t.FramesData[frame+seqMode] = float32(int(mode) | (index << 4))
	t.FramesData[frame+seqDelay] = delay
}

// Apply implements Timeline.
func (t *SequenceTimeline) Apply(skeleton *Skeleton, lastTime, time float32, events *[]*Event, alpha float32,
	blend MixBlend, direction MixDirection) {

	slot := skeleton.Slots[t.SlotIndex]
	if !slot.Bone.active {
		return
	}
	slotAttachment := slot.Attachment()
	if slotAttachment != t.Attachment {
		va := AsVertexAttachment(slotAttachment)
		if va == nil || va.TimelineAttachment != t.Attachment {
			return
		}
	}
	sequence := attachmentSequence(slotAttachment)
	if sequence == nil {
		return
	}

	if direction == MixDirectionOut {
		if blend == MixBlendSetup {
			slot.SequenceIndex = -1
		}
		return
	}

	frames := t.FramesData
	if time < frames[0] {
		if blend == MixBlendSetup || blend == MixBlendFirst {
			slot.SequenceIndex = -1
		}
		return
	}

	i := searchN(frames, time, seqEntries)
	before := frames[i]
	modeAndIndex := int(frames[i+seqMode])
	delay := frames[i+seqDelay]

	index := modeAndIndex >> 4
	count := len(sequence.Regions)
	mode := SequenceMode(modeAndIndex & 0xf)
	if mode != SequenceModeHold {
		index = int(float32(index) + (time-before)/delay + 0.0001)
		switch mode {
		case SequenceModeOnce:
			index = min(count-1, index)
		case SequenceModeLoop:
			index %= count
		case SequenceModePingpong:
			n := (count << 1) - 2
			if n == 0 {
				index = 0
			} else {
				index %= n
			}
			if index >= count {
				index = n - index
			}
		case SequenceModeOnceReverse:
			index = max(count-1-index, 0)
		case SequenceModeLoopReverse:
			index = count - 1 - (index % count)
		case SequenceModePingpongReverse:
			n := (count << 1) - 2
			if n == 0 {
				index = 0
			} else {
				index = (index + count - 1) % n
			}
			if index >= count {
				index = n - index
			}
		}
	}
	slot.SequenceIndex = index
}
