package spine

import (
	"math"
	"strings"
)

// emptyAnimation is a placeholder animation with no timelines, used for mixing
// in and out. See AnimationState.SetEmptyAnimation.
var emptyAnimation = &Animation{Name: "<empty>"}

const (
	// subsequent means:
	// 1) A previously applied timeline has set this property.
	// Result: Mix from the current pose to the timeline pose.
	subsequent = 0
	// first means:
	// 1) This is the first timeline to set this property.
	// 2) The next track entry applied after this one does not have a timeline
	// to set this property.
	// Result: Mix from the setup pose to the timeline pose.
	first = 1
	// holdSubsequent means:
	// 1) A previously applied timeline has set this property.
	// 2) The next track entry to be applied does have a timeline to set this
	// property.
	// 3) The next track entry after that one does not have a timeline to set
	// this property.
	// Result: Mix from the current pose to the timeline pose, but do not mix
	// out. This avoids "dipping" when crossfading animations that key the same
	// property. A subsequent timeline will set this property using a mix.
	holdSubsequent = 2
	// holdFirst means:
	// 1) This is the first timeline to set this property.
	// 2) The next track entry to be applied does have a timeline to set this
	// property.
	// 3) The next track entry after that one does not have a timeline to set
	// this property.
	// Result: Mix from the setup pose to the timeline pose, but do not mix
	// out. This avoids "dipping" when crossfading animations that key the same
	// property. A subsequent timeline will set this property using a mix.
	holdFirst = 3
	// holdMix means:
	// 1) This is the first timeline to set this property.
	// 2) The next track entry to be applied does have a timeline to set this
	// property.
	// 3) The next track entry after that one does have a timeline to set this
	// property.
	// 4) timelineHoldMix stores the first subsequent track entry that does not
	// have a timeline to set this property.
	// Result: The same as HOLD except the mix percentage from the
	// timelineHoldMix track entry is used. This handles when more than 2 track
	// entries in a row have a timeline that sets the same property.
	// Eg, A -> B -> C -> D where A, B, and C have a timeline setting same
	// property, but D does not. When A is applied, to avoid "dipping" A is not
	// mixed out, however D (the first entry that doesn't set the property)
	// mixing in is used to mix out A (which affects B and C). Without using D
	// to mix out, A would be applied fully until mixing completes, then snap
	// to the mixed out position.
	holdMix = 4
)

const (
	setupState   = 1
	currentState = 2
)

// AnimationState applies animations over time, queues animations for later
// playback, mixes (crossfading) between animations, and applies multiple
// animations on top of each other (layering).
//
// See "Applying Animations" in the Spine Runtimes Guide:
// https://esotericsoftware.com/spine-applying-animations/
type AnimationState struct {
	// Data is the AnimationStateData to look up mix durations.
	Data *AnimationStateData

	// TimeScale is a multiplier for the delta time when the animation state is
	// updated, causing time for all animations and mixes to play slower or
	// faster. Defaults to 1.
	//
	// See TrackEntry.TimeScale for affecting a single animation.
	TimeScale float32

	tracks            []*TrackEntry
	events            []*Event
	listeners         []AnimationStateListener
	queue             eventQueue
	propertyIDs       map[string]bool
	animationsChanged bool
	unkeyedState      int
	trackEntryPool    []*TrackEntry
}

// NewAnimationState creates an animation state.
func NewAnimationState(data *AnimationStateData) *AnimationState {
	if data == nil {
		panic("spine: data cannot be nil")
	}
	s := &AnimationState{
		Data:        data,
		TimeScale:   1,
		propertyIDs: make(map[string]bool),
	}
	s.queue.state = s
	return s
}

func (s *AnimationState) obtainTrackEntry() *TrackEntry {
	n := len(s.trackEntryPool)
	if n == 0 {
		return &TrackEntry{}
	}
	entry := s.trackEntryPool[n-1]
	s.trackEntryPool[n-1] = nil
	s.trackEntryPool = s.trackEntryPool[:n-1]
	return entry
}

func (s *AnimationState) freeTrackEntry(entry *TrackEntry) {
	entry.reset()
	s.trackEntryPool = append(s.trackEntryPool, entry)
}

// Update increments each track entry's TrackTime, setting queued animations
// as current if needed.
func (s *AnimationState) Update(delta float32) {
	delta *= s.TimeScale
	tracks := s.tracks
	for i, n := 0, len(s.tracks); i < n; i++ {
		current := tracks[i]
		if current == nil {
			continue
		}

		current.AnimationLast = current.nextAnimationLast
		current.trackLast = current.nextTrackLast

		currentDelta := delta * current.TimeScale

		if current.Delay > 0 {
			current.Delay -= currentDelta
			if current.Delay > 0 {
				continue
			}
			currentDelta = -current.Delay
			current.Delay = 0
		}

		next := current.next
		if next != nil {
			// When the next entry's delay is passed, change to the next entry,
			// preserving leftover time.
			nextTime := current.trackLast - next.Delay
			if nextTime >= 0 {
				next.Delay = 0
				if current.TimeScale != 0 {
					next.TrackTime += (nextTime/current.TimeScale + delta) * next.TimeScale
				}
				current.TrackTime += currentDelta
				s.setCurrent(i, next, true)
				for next.mixingFrom != nil {
					next.MixTime += delta
					next = next.mixingFrom
				}
				continue
			}
		} else if current.trackLast >= current.TrackEnd && current.mixingFrom == nil {
			// Clear the track when there is no next entry, the track end time
			// is reached, and there is no mixingFrom.
			tracks[i] = nil
			s.queue.end(current)
			s.ClearNext(current)
			continue
		}
		if current.mixingFrom != nil && s.updateMixingFrom(current, delta) {
			// End mixing from entries once all have completed.
			from := current.mixingFrom
			current.mixingFrom = nil
			if from != nil {
				from.mixingTo = nil
			}
			for from != nil {
				s.queue.end(from)
				from = from.mixingFrom
			}
		}

		current.TrackTime += currentDelta
	}

	s.queue.drain()
}

// updateMixingFrom returns true when all mixing from entries are complete.
func (s *AnimationState) updateMixingFrom(to *TrackEntry, delta float32) bool {
	from := to.mixingFrom
	if from == nil {
		return true
	}

	finished := s.updateMixingFrom(from, delta)

	from.AnimationLast = from.nextAnimationLast
	from.trackLast = from.nextTrackLast

	// The from entry was applied at least once and the mix is complete.
	if to.nextTrackLast != -1 && to.MixTime >= to.mixDuration {
		// Mixing is complete for all entries before the from entry or the mix
		// is instantaneous.
		if from.totalAlpha == 0 || to.mixDuration == 0 {
			to.mixingFrom = from.mixingFrom
			if from.mixingFrom != nil {
				from.mixingFrom.mixingTo = to
			}
			to.interruptAlpha = from.interruptAlpha
			s.queue.end(from)
		}
		return finished
	}

	from.TrackTime += delta * from.TimeScale
	to.MixTime += delta
	return false
}

// Apply poses the skeleton using the track entry animations. The animation
// state is not changed, so can be applied to multiple skeletons to pose them
// identically. Returns true if any animations were applied.
func (s *AnimationState) Apply(skeleton *Skeleton) bool {
	if skeleton == nil {
		panic("spine: skeleton cannot be nil")
	}
	if s.animationsChanged {
		s.handleAnimationsChanged()
	}

	events := &s.events
	applied := false
	tracks := s.tracks
	for i, n := 0, len(s.tracks); i < n; i++ {
		current := tracks[i]
		if current == nil || current.Delay > 0 {
			continue
		}
		applied = true

		// Track 0 animations aren't for layering, so do not show the
		// previously applied animations before the first key.
		blend := current.MixBlend
		if i == 0 {
			blend = MixBlendFirst
		}

		// Apply mixing from entries first.
		alpha := current.Alpha
		if current.mixingFrom != nil {
			alpha *= s.applyMixingFrom(current, skeleton, blend)
		} else if current.TrackTime >= current.TrackEnd && current.next == nil {
			alpha = 0 // Set to setup pose the last time the entry will be applied.
		}
		attachments := alpha >= current.AlphaAttachmentThreshold

		// Apply current entry.
		animationLast, animationTime := current.AnimationLast, current.AnimationTime()
		applyTime := animationTime
		applyEvents := events
		if current.Reverse {
			applyTime = current.Animation.Duration - applyTime
			applyEvents = nil
		}
		timelines := current.Animation.Timelines()
		timelineCount := len(timelines)
		if (i == 0 && alpha == 1) || blend == MixBlendAdd {
			if i == 0 {
				attachments = true
			}
			for ii := 0; ii < timelineCount; ii++ {
				timeline := timelines[ii]
				if attachmentTimeline, ok := timeline.(*AttachmentTimeline); ok {
					s.applyAttachmentTimeline(attachmentTimeline, skeleton, applyTime, blend, attachments)
				} else {
					timeline.Apply(skeleton, animationLast, applyTime, applyEvents, alpha, blend, MixDirectionIn)
				}
			}
		} else {
			timelineMode := current.timelineMode

			shortestRotation := current.ShortestRotation
			firstFrame := !shortestRotation && len(current.timelinesRotation) != timelineCount<<1
			if firstFrame {
				ensureSize(&current.timelinesRotation, timelineCount<<1)
			}
			timelinesRotation := current.timelinesRotation

			for ii := 0; ii < timelineCount; ii++ {
				timeline := timelines[ii]
				timelineBlend := MixBlendSetup
				if timelineMode[ii] == subsequent {
					timelineBlend = blend
				}
				if rotateTimeline, ok := timeline.(*RotateTimeline); !shortestRotation && ok {
					s.applyRotateTimeline(rotateTimeline, skeleton, applyTime, alpha, timelineBlend, timelinesRotation,
						ii<<1, firstFrame)
				} else if attachmentTimeline, ok := timeline.(*AttachmentTimeline); ok {
					s.applyAttachmentTimeline(attachmentTimeline, skeleton, applyTime, blend, attachments)
				} else {
					timeline.Apply(skeleton, animationLast, applyTime, applyEvents, alpha, timelineBlend, MixDirectionIn)
				}
			}
		}
		s.queueEvents(current, animationTime)
		s.events = s.events[:0]
		current.nextAnimationLast = animationTime
		current.nextTrackLast = current.TrackTime
	}

	// Set slots attachments to the setup pose, if needed. This occurs if an
	// animation that is mixing out sets attachments so subsequent timelines
	// see any deform, but the subsequent timelines don't set an attachment
	// (eg they are also mixing out or the time is before the first key).
	unkeyedSetup := s.unkeyedState + setupState
	slots := skeleton.Slots
	for i, n := 0, len(slots); i < n; i++ {
		slot := slots[i]
		if slot.attachmentState == unkeyedSetup {
			attachmentName := slot.Data.AttachmentName
			if attachmentName == "" {
				slot.SetAttachment(nil)
			} else {
				slot.SetAttachment(skeleton.AttachmentByIndex(slot.Data.Index, attachmentName))
			}
		}
	}
	s.unkeyedState += 2 // Increasing after each use avoids the need to reset attachmentState for every slot.

	s.queue.drain()
	return applied
}

func (s *AnimationState) applyMixingFrom(to *TrackEntry, skeleton *Skeleton, blend MixBlend) float32 {
	from := to.mixingFrom
	if from.mixingFrom != nil {
		s.applyMixingFrom(from, skeleton, blend)
	}

	var mix float32
	if to.mixDuration == 0 { // Single frame mix to undo mixingFrom changes.
		mix = 1
		if blend == MixBlendFirst {
			blend = MixBlendSetup // Tracks >0 are transparent and can't reset to setup pose.
		}
	} else {
		mix = to.MixTime / to.mixDuration
		if mix > 1 {
			mix = 1
		}
		if blend != MixBlendFirst {
			blend = from.MixBlend // Track 0 ignores track mix blend.
		}
	}

	attachments := mix < from.MixAttachmentThreshold
	drawOrder := mix < from.MixDrawOrderThreshold
	timelines := from.Animation.Timelines()
	timelineCount := len(timelines)
	alphaHold := from.Alpha * to.interruptAlpha
	alphaMix := alphaHold * (1 - mix)
	animationLast, animationTime := from.AnimationLast, from.AnimationTime()
	applyTime := animationTime
	var events *[]*Event
	if from.Reverse {
		applyTime = from.Animation.Duration - applyTime
	} else if mix < from.EventThreshold {
		events = &s.events
	}

	if blend == MixBlendAdd {
		for i := 0; i < timelineCount; i++ {
			timelines[i].Apply(skeleton, animationLast, applyTime, events, alphaMix, blend, MixDirectionOut)
		}
	} else {
		timelineMode := from.timelineMode
		timelineHoldMix := from.timelineHoldMix

		shortestRotation := from.ShortestRotation
		firstFrame := !shortestRotation && len(from.timelinesRotation) != timelineCount<<1
		if firstFrame {
			ensureSize(&from.timelinesRotation, timelineCount<<1)
		}
		timelinesRotation := from.timelinesRotation

		from.totalAlpha = 0
		for i := 0; i < timelineCount; i++ {
			timeline := timelines[i]
			direction := MixDirectionOut
			var timelineBlend MixBlend
			var alpha float32
			switch timelineMode[i] {
			case subsequent:
				if !drawOrder {
					if _, ok := timeline.(*DrawOrderTimeline); ok {
						continue
					}
				}
				timelineBlend = blend
				alpha = alphaMix
			case first:
				timelineBlend = MixBlendSetup
				alpha = alphaMix
			case holdSubsequent:
				timelineBlend = blend
				alpha = alphaHold
			case holdFirst:
				timelineBlend = MixBlendSetup
				alpha = alphaHold
			default: // holdMix
				timelineBlend = MixBlendSetup
				holdMixEntry := timelineHoldMix[i]
				alpha = alphaHold * maxF(0, 1-holdMixEntry.MixTime/holdMixEntry.mixDuration)
			}
			from.totalAlpha += alpha
			if rotateTimeline, ok := timeline.(*RotateTimeline); !shortestRotation && ok {
				s.applyRotateTimeline(rotateTimeline, skeleton, applyTime, alpha, timelineBlend, timelinesRotation,
					i<<1, firstFrame)
			} else if attachmentTimeline, ok := timeline.(*AttachmentTimeline); ok {
				s.applyAttachmentTimeline(attachmentTimeline, skeleton, applyTime, timelineBlend,
					attachments && alpha >= from.AlphaAttachmentThreshold)
			} else {
				if drawOrder && timelineBlend == MixBlendSetup {
					if _, ok := timeline.(*DrawOrderTimeline); ok {
						direction = MixDirectionIn
					}
				}
				timeline.Apply(skeleton, animationLast, applyTime, events, alpha, timelineBlend, direction)
			}
		}
	}

	if to.mixDuration > 0 {
		s.queueEvents(from, animationTime)
	}
	s.events = s.events[:0]
	from.nextAnimationLast = animationTime
	from.nextTrackLast = from.TrackTime

	return mix
}

// applyAttachmentTimeline applies the attachment timeline and sets
// Slot.attachmentState.
//
// attachments is false when: 1) the attachment timeline is mixing out,
// 2) mix < attachmentThreshold, and 3) the timeline is not the last timeline
// to set the slot's attachment. In that case the timeline is applied only so
// subsequent timelines see any deform.
func (s *AnimationState) applyAttachmentTimeline(timeline *AttachmentTimeline, skeleton *Skeleton, time float32,
	blend MixBlend, attachments bool) {

	slot := skeleton.Slots[timeline.GetSlotIndex()]
	if !slot.Bone.IsActive() {
		return
	}

	frames := timeline.Frames()
	if time < frames[0] { // Time is before first frame.
		if blend == MixBlendSetup || blend == MixBlendFirst {
			s.setAttachment(skeleton, slot, slot.Data.AttachmentName, attachments)
		}
	} else {
		s.setAttachment(skeleton, slot, timeline.AttachmentNames[search1(frames, time)], attachments)
	}

	// If an attachment wasn't set (ie before the first frame or attachments
	// is false), set the setup attachment later.
	if slot.attachmentState <= s.unkeyedState {
		slot.attachmentState = s.unkeyedState + setupState
	}
}

func (s *AnimationState) setAttachment(skeleton *Skeleton, slot *Slot, attachmentName string, attachments bool) {
	if attachmentName == "" {
		slot.SetAttachment(nil)
	} else {
		slot.SetAttachment(skeleton.AttachmentByIndex(slot.Data.Index, attachmentName))
	}
	if attachments {
		slot.attachmentState = s.unkeyedState + currentState
	}
}

// applyRotateTimeline applies the rotate timeline, mixing with the current
// pose while keeping the same rotation direction chosen as the shortest the
// first time the mixing was applied.
func (s *AnimationState) applyRotateTimeline(timeline *RotateTimeline, skeleton *Skeleton, time, alpha float32,
	blend MixBlend, timelinesRotation []float32, i int, firstFrame bool) {

	if firstFrame {
		timelinesRotation[i] = 0
	}

	if alpha == 1 {
		timeline.Apply(skeleton, 0, time, nil, 1, blend, MixDirectionIn)
		return
	}

	bone := skeleton.Bones[timeline.GetBoneIndex()]
	if !bone.IsActive() {
		return
	}
	frames := timeline.Frames()
	var r1, r2 float32
	if time < frames[0] { // Time is before first frame.
		switch blend {
		case MixBlendSetup:
			bone.Rotation = bone.Data.Rotation
			return
		case MixBlendFirst:
			r1 = bone.Rotation
			r2 = bone.Data.Rotation
		default:
			return
		}
	} else {
		if blend == MixBlendSetup {
			r1 = bone.Data.Rotation
		} else {
			r1 = bone.Rotation
		}
		r2 = bone.Data.Rotation + timeline.GetCurveValue(time)
	}

	// Mix between rotations using the direction of the shortest route on the
	// first frame.
	var total float32
	diff := r2 - r1
	diff -= float32(math.Ceil(float64(diff/360-0.5))) * 360
	if diff == 0 {
		total = timelinesRotation[i]
	} else {
		var lastTotal, lastDiff float32
		if firstFrame {
			lastTotal = 0
			lastDiff = diff
		} else {
			lastTotal = timelinesRotation[i]
			lastDiff = timelinesRotation[i+1]
		}
		loops := lastTotal - float32(math.Mod(float64(lastTotal), 360))
		total = diff + loops
		current, dir := diff >= 0, lastTotal >= 0
		if abs(lastDiff) <= 90 && signum(lastDiff) != signum(diff) {
			if abs(lastTotal-loops) > 180 {
				total += 360 * signum(lastTotal)
				dir = current
			} else if loops != 0 {
				total -= 360 * signum(lastTotal)
			} else {
				dir = current
			}
		}
		if dir != current {
			total += 360 * signum(lastTotal)
		}
		timelinesRotation[i] = total
	}
	timelinesRotation[i+1] = diff
	bone.Rotation = r1 + total*alpha
}

func (s *AnimationState) queueEvents(entry *TrackEntry, animationTime float32) {
	animationStart, animationEnd := entry.AnimationStart, entry.AnimationEnd
	duration := animationEnd - animationStart
	trackLastWrapped := float32(math.Mod(float64(entry.trackLast), float64(duration)))

	// Queue events before complete.
	events := s.events
	i, n := 0, len(s.events)
	for ; i < n; i++ {
		event := events[i]
		if event.Time < trackLastWrapped {
			break
		}
		if event.Time > animationEnd {
			continue // Discard events outside animation start/end.
		}
		s.queue.event(entry, event)
	}

	// Queue complete if completed a loop iteration or the animation.
	var complete bool
	if entry.Loop {
		if duration == 0 {
			complete = true
		} else {
			cycles := int(entry.TrackTime / duration)
			complete = cycles > 0 && cycles > int(entry.trackLast/duration)
		}
	} else {
		complete = animationTime >= animationEnd && entry.AnimationLast < animationEnd
	}
	if complete {
		s.queue.complete(entry)
	}

	// Queue events after complete.
	for ; i < n; i++ {
		event := events[i]
		if event.Time < animationStart {
			continue // Discard events outside animation start/end.
		}
		s.queue.event(entry, event)
	}
}

// ClearTracks removes all animations from all tracks, leaving skeletons in
// their current pose.
//
// It may be desired to use SetEmptyAnimations to mix the skeletons back to
// the setup pose, rather than leaving them in their current pose.
func (s *AnimationState) ClearTracks() {
	oldDrainDisabled := s.queue.drainDisabled
	s.queue.drainDisabled = true
	for i, n := 0, len(s.tracks); i < n; i++ {
		s.ClearTrack(i)
	}
	s.tracks = s.tracks[:0]
	s.queue.drainDisabled = oldDrainDisabled
	s.queue.drain()
}

// ClearTrack removes all animations from the track, leaving skeletons in
// their current pose.
//
// It may be desired to use SetEmptyAnimation to mix the skeletons back to the
// setup pose, rather than leaving them in their current pose.
func (s *AnimationState) ClearTrack(trackIndex int) {
	if trackIndex < 0 {
		panic("spine: trackIndex must be >= 0")
	}
	if trackIndex >= len(s.tracks) {
		return
	}
	current := s.tracks[trackIndex]
	if current == nil {
		return
	}

	s.queue.end(current)

	s.ClearNext(current)

	entry := current
	for {
		from := entry.mixingFrom
		if from == nil {
			break
		}
		s.queue.end(from)
		entry.mixingFrom = nil
		entry.mixingTo = nil
		entry = from
	}

	s.tracks[current.trackIndex] = nil

	s.queue.drain()
}

func (s *AnimationState) setCurrent(index int, current *TrackEntry, interrupt bool) {
	from := s.expandToIndex(index)
	s.tracks[index] = current
	current.previous = nil

	if from != nil {
		if interrupt {
			s.queue.interrupt(from)
		}
		current.mixingFrom = from
		from.mixingTo = current
		current.MixTime = 0

		// Store the interrupted mix percentage.
		if from.mixingFrom != nil && from.mixDuration > 0 {
			current.interruptAlpha *= minF(1, from.MixTime/from.mixDuration)
		}

		from.timelinesRotation = from.timelinesRotation[:0] // Reset rotation for mixing out, in case entry was mixed in.
	}

	s.queue.start(current)
}

// SetAnimationByName sets an animation by name.
//
// See SetAnimation.
func (s *AnimationState) SetAnimationByName(trackIndex int, animationName string, loop bool) *TrackEntry {
	animation := s.Data.SkeletonData.FindAnimation(animationName)
	if animation == nil {
		panic("spine: animation not found: " + animationName)
	}
	return s.SetAnimation(trackIndex, animation, loop)
}

// SetAnimation sets the current animation for a track, discarding any queued
// animations. If the formerly current track entry was never applied to a
// skeleton, it is replaced (not mixed from).
//
// If loop is true, the animation will repeat. If false it will not, instead
// its last frame is applied if played beyond its duration. In either case
// TrackEntry.TrackEnd determines when the track is cleared.
//
// Returns a track entry to allow further customization of animation playback.
// References to the track entry must not be kept after the
// AnimationStateListener.Dispose event occurs.
func (s *AnimationState) SetAnimation(trackIndex int, animation *Animation, loop bool) *TrackEntry {
	if trackIndex < 0 {
		panic("spine: trackIndex must be >= 0")
	}
	if animation == nil {
		panic("spine: animation cannot be nil")
	}
	interrupt := true
	current := s.expandToIndex(trackIndex)
	if current != nil {
		if current.nextTrackLast == -1 {
			// Don't mix from an entry that was never applied.
			s.tracks[trackIndex] = current.mixingFrom
			s.queue.interrupt(current)
			s.queue.end(current)
			s.ClearNext(current)
			current = current.mixingFrom
			interrupt = false // mixingFrom is current again, but don't interrupt it twice.
		} else {
			s.ClearNext(current)
		}
	}
	entry := s.trackEntry(trackIndex, animation, loop, current)
	s.setCurrent(trackIndex, entry, interrupt)
	s.queue.drain()
	return entry
}

// AddAnimationByName queues an animation by name.
//
// See AddAnimation.
func (s *AnimationState) AddAnimationByName(trackIndex int, animationName string, loop bool, delay float32) *TrackEntry {
	animation := s.Data.SkeletonData.FindAnimation(animationName)
	if animation == nil {
		panic("spine: animation not found: " + animationName)
	}
	return s.AddAnimation(trackIndex, animation, loop, delay)
}

// AddAnimation adds an animation to be played after the current or last
// queued animation for a track. If the track is empty, it is equivalent to
// calling SetAnimation.
//
// If delay > 0, sets TrackEntry.Delay. If delay <= 0, the delay set is the
// duration of the previous track entry minus any mix duration (from the
// AnimationStateData) plus the specified delay (ie the mix ends at
// (delay = 0) or before (delay < 0) the previous track entry duration). If
// the previous entry is looping, its next loop completion is used instead of
// its duration.
//
// Returns a track entry to allow further customization of animation playback.
// References to the track entry must not be kept after the
// AnimationStateListener.Dispose event occurs.
func (s *AnimationState) AddAnimation(trackIndex int, animation *Animation, loop bool, delay float32) *TrackEntry {
	if trackIndex < 0 {
		panic("spine: trackIndex must be >= 0")
	}
	if animation == nil {
		panic("spine: animation cannot be nil")
	}

	last := s.expandToIndex(trackIndex)
	if last != nil {
		for last.next != nil {
			last = last.next
		}
	}

	entry := s.trackEntry(trackIndex, animation, loop, last)

	if last == nil {
		s.setCurrent(trackIndex, entry, true)
		s.queue.drain()
		if delay < 0 {
			delay = 0
		}
	} else {
		last.next = entry
		entry.previous = last
		if delay <= 0 {
			delay = maxF(delay+last.TrackComplete()-entry.mixDuration, 0)
		}
	}

	entry.Delay = delay
	return entry
}

// SetEmptyAnimation sets an empty animation for a track, discarding any
// queued animations, and sets the track entry's MixDuration. An empty
// animation has no timelines and serves as a placeholder for mixing in or
// out.
//
// Mixing out is done by setting an empty animation with a mix duration using
// either SetEmptyAnimation, SetEmptyAnimations, or AddEmptyAnimation. Mixing
// to an empty animation causes the previous animation to be applied less and
// less over the mix duration. Properties keyed in the previous animation
// transition to the value from lower tracks or to the setup pose value if no
// lower tracks key the property. A mix duration of 0 still mixes out over one
// frame.
//
// Mixing in is done by first setting an empty animation, then adding an
// animation using AddAnimation with the desired delay (an empty animation has
// a duration of 0) and on the returned track entry, set the mix duration with
// SetMixDuration. Mixing from an empty animation causes the new animation to
// be applied more and more over the mix duration. Properties keyed in the new
// animation transition from the value from lower tracks or from the setup
// pose value if no lower tracks key the property to the value keyed in the
// new animation.
func (s *AnimationState) SetEmptyAnimation(trackIndex int, mixDuration float32) *TrackEntry {
	entry := s.SetAnimation(trackIndex, emptyAnimation, false)
	entry.mixDuration = mixDuration
	entry.TrackEnd = mixDuration
	return entry
}

// AddEmptyAnimation adds an empty animation to be played after the current or
// last queued animation for a track, and sets the track entry's MixDuration.
// If the track is empty, it is equivalent to calling SetEmptyAnimation.
//
// See SetEmptyAnimation.
//
// If delay > 0, sets TrackEntry.Delay. If delay <= 0, the delay set is the
// duration of the previous track entry minus any mix duration plus the
// specified delay (ie the mix ends at (delay = 0) or before (delay < 0) the
// previous track entry duration). If the previous entry is looping, its next
// loop completion is used instead of its duration.
//
// Returns a track entry to allow further customization of animation playback.
// References to the track entry must not be kept after the
// AnimationStateListener.Dispose event occurs.
func (s *AnimationState) AddEmptyAnimation(trackIndex int, mixDuration, delay float32) *TrackEntry {
	entry := s.AddAnimation(trackIndex, emptyAnimation, false, delay)
	if delay <= 0 {
		entry.Delay = maxF(entry.Delay+entry.mixDuration-mixDuration, 0)
	}
	entry.mixDuration = mixDuration
	entry.TrackEnd = mixDuration
	return entry
}

// SetEmptyAnimations sets an empty animation for every track, discarding any
// queued animations, and mixes to it over the specified mix duration.
func (s *AnimationState) SetEmptyAnimations(mixDuration float32) {
	oldDrainDisabled := s.queue.drainDisabled
	s.queue.drainDisabled = true
	tracks := s.tracks
	for i, n := 0, len(s.tracks); i < n; i++ {
		current := tracks[i]
		if current != nil {
			s.SetEmptyAnimation(current.trackIndex, mixDuration)
		}
	}
	s.queue.drainDisabled = oldDrainDisabled
	s.queue.drain()
}

func (s *AnimationState) expandToIndex(index int) *TrackEntry {
	if index < len(s.tracks) {
		return s.tracks[index]
	}
	for len(s.tracks) <= index {
		s.tracks = append(s.tracks, nil)
	}
	return nil
}

func (s *AnimationState) trackEntry(trackIndex int, animation *Animation, loop bool, last *TrackEntry) *TrackEntry {
	entry := s.obtainTrackEntry()
	entry.trackIndex = trackIndex
	entry.Animation = animation
	entry.Loop = loop
	entry.HoldPrevious = false

	entry.Reverse = false
	entry.ShortestRotation = false

	entry.EventThreshold = 0
	entry.AlphaAttachmentThreshold = 0
	entry.MixAttachmentThreshold = 0
	entry.MixDrawOrderThreshold = 0

	entry.AnimationStart = 0
	entry.AnimationEnd = animation.Duration
	entry.AnimationLast = -1
	entry.nextAnimationLast = -1

	entry.Delay = 0
	entry.TrackTime = 0
	entry.trackLast = -1
	entry.nextTrackLast = -1
	entry.TrackEnd = math.MaxFloat32
	entry.TimeScale = 1

	entry.Alpha = 1
	entry.MixTime = 0
	if last == nil {
		entry.mixDuration = 0
	} else {
		entry.mixDuration = s.Data.GetMix(last.Animation, animation)
	}
	entry.interruptAlpha = 1
	entry.totalAlpha = 0
	entry.MixBlend = MixBlendReplace
	return entry
}

// ClearNext removes the TrackEntry.Next entry and all entries after it for
// the specified entry.
func (s *AnimationState) ClearNext(entry *TrackEntry) {
	next := entry.next
	for next != nil {
		s.queue.dispose(next)
		next = next.next
	}
	entry.next = nil
}

func (s *AnimationState) handleAnimationsChanged() {
	s.animationsChanged = false

	// Process in the order that animations are applied.
	clear(s.propertyIDs)
	n := len(s.tracks)
	tracks := s.tracks
	for i := 0; i < n; i++ {
		entry := tracks[i]
		if entry == nil {
			continue
		}
		for entry.mixingFrom != nil { // Move to last entry, then iterate in reverse.
			entry = entry.mixingFrom
		}
		for {
			if entry.mixingTo == nil || entry.MixBlend != MixBlendAdd {
				s.computeHold(entry)
			}
			entry = entry.mixingTo
			if entry == nil {
				break
			}
		}
	}
}

func (s *AnimationState) computeHold(entry *TrackEntry) {
	to := entry.mixingTo
	timelines := entry.Animation.Timelines()
	timelinesCount := len(timelines)
	if cap(entry.timelineMode) < timelinesCount {
		entry.timelineMode = make([]int, timelinesCount)
	} else {
		entry.timelineMode = entry.timelineMode[:timelinesCount]
	}
	timelineMode := entry.timelineMode
	for i := range entry.timelineHoldMix {
		entry.timelineHoldMix[i] = nil
	}
	entry.timelineHoldMix = entry.timelineHoldMix[:0]
	for i := 0; i < timelinesCount; i++ {
		entry.timelineHoldMix = append(entry.timelineHoldMix, nil)
	}
	timelineHoldMix := entry.timelineHoldMix
	propertyIDs := s.propertyIDs

	if to != nil && to.HoldPrevious {
		for i := 0; i < timelinesCount; i++ {
			if addAllPropertyIDs(propertyIDs, timelines[i].PropertyIDs()) {
				timelineMode[i] = holdFirst
			} else {
				timelineMode[i] = holdSubsequent
			}
		}
		return
	}

outer:
	for i := 0; i < timelinesCount; i++ {
		timeline := timelines[i]
		ids := timeline.PropertyIDs()
		if !addAllPropertyIDs(propertyIDs, ids) {
			timelineMode[i] = subsequent
		} else {
			_, isAttachment := timeline.(*AttachmentTimeline)
			_, isDrawOrder := timeline.(*DrawOrderTimeline)
			_, isEvent := timeline.(*EventTimeline)
			if to == nil || isAttachment || isDrawOrder || isEvent || !to.Animation.HasTimeline(ids) {
				timelineMode[i] = first
			} else {
				for next := to.mixingTo; next != nil; next = next.mixingTo {
					if next.Animation.HasTimeline(ids) {
						continue
					}
					if next.mixDuration > 0 {
						timelineMode[i] = holdMix
						timelineHoldMix[i] = next
						continue outer
					}
					break
				}
				timelineMode[i] = holdFirst
			}
		}
	}
}

// addAllPropertyIDs adds all ids to the set, returning true if the set was
// modified (Java ObjectSet.addAll semantics).
func addAllPropertyIDs(set map[string]bool, ids []string) bool {
	modified := false
	for _, id := range ids {
		if !set[id] {
			set[id] = true
			modified = true
		}
	}
	return modified
}

// GetCurrent returns the track entry for the animation currently playing on
// the track, or nil if no animation is currently playing.
func (s *AnimationState) GetCurrent(trackIndex int) *TrackEntry {
	if trackIndex < 0 {
		panic("spine: trackIndex must be >= 0")
	}
	if trackIndex >= len(s.tracks) {
		return nil
	}
	return s.tracks[trackIndex]
}

// AddListener adds a listener to receive events for all track entries.
func (s *AnimationState) AddListener(listener AnimationStateListener) {
	if listener == nil {
		panic("spine: listener cannot be nil")
	}
	s.listeners = append(s.listeners, listener)
}

// RemoveListener removes the listener added with AddListener.
func (s *AnimationState) RemoveListener(listener AnimationStateListener) {
	for i, l := range s.listeners {
		if l == listener {
			// Copy on write so any snapshot taken while draining is unaffected.
			newListeners := make([]AnimationStateListener, 0, len(s.listeners)-1)
			newListeners = append(newListeners, s.listeners[:i]...)
			newListeners = append(newListeners, s.listeners[i+1:]...)
			s.listeners = newListeners
			return
		}
	}
}

// ClearListeners removes all listeners added with AddListener.
func (s *AnimationState) ClearListeners() {
	s.listeners = nil
}

// ClearListenerNotifications discards all listener notifications that have
// not yet been delivered. This can be useful to call from an
// AnimationStateListener when it is known that further notifications that may
// have been already queued for delivery are not wanted because new animations
// are being set.
func (s *AnimationState) ClearListenerNotifications() {
	s.queue.clear()
}

// Tracks returns the list of tracks that have had animations, which may
// contain nil entries for tracks that currently have no animation.
func (s *AnimationState) Tracks() []*TrackEntry {
	return s.tracks
}

func (s *AnimationState) String() string {
	var buffer strings.Builder
	tracks := s.tracks
	for i, n := 0, len(s.tracks); i < n; i++ {
		entry := tracks[i]
		if entry == nil {
			continue
		}
		if buffer.Len() > 0 {
			buffer.WriteString(", ")
		}
		buffer.WriteString(entry.String())
	}
	if buffer.Len() == 0 {
		return "<none>"
	}
	return buffer.String()
}

// TrackEntry stores settings and other state for the playback of an animation
// on an AnimationState track.
//
// References to a track entry must not be kept after the
// AnimationStateListener.Dispose event occurs.
type TrackEntry struct {
	// Animation is the animation to apply for this track entry.
	Animation *Animation

	previous, next, mixingFrom, mixingTo *TrackEntry

	// Listener is the listener for events generated by this track entry, or
	// nil.
	//
	// A track entry returned from AnimationState.SetAnimation is already the
	// current animation for the track, so the track entry listener Start will
	// not be called.
	Listener AnimationStateListener

	trackIndex int

	// Loop: if true, the animation will repeat. If false it will not, instead
	// its last frame is applied if played beyond its duration.
	Loop bool

	// HoldPrevious: if true, when mixing from the previous animation to this
	// animation, the previous animation is applied as normal instead of being
	// mixed out.
	//
	// When mixing between animations that key the same property, if a lower
	// track also keys that property then the value will briefly dip toward
	// the lower track value during the mix. This happens because the first
	// animation mixes from 100% to 0% while the second animation mixes from
	// 0% to 100%. Setting HoldPrevious to true applies the first animation at
	// 100% during the mix so the lower track value is overwritten. Such
	// dipping does not occur on the lowest track which keys the property,
	// only when a higher track also keys the property.
	//
	// Snapping will occur if HoldPrevious is true and this animation does not
	// key all the same properties as the previous animation.
	HoldPrevious bool

	// Reverse: if true, the animation will be applied in reverse. Events are
	// not fired when an animation is applied in reverse.
	Reverse bool

	// ShortestRotation: if true, mixing rotation between tracks always uses
	// the shortest rotation direction. If the rotation is animated, the
	// shortest rotation direction may change during the mix.
	//
	// If false, the shortest rotation direction is remembered when the mix
	// starts and the same direction is used for the rest of the mix. Defaults
	// to false.
	ShortestRotation bool

	// EventThreshold: when the mix percentage (MixTime / MixDuration) is less
	// than the EventThreshold, event timelines are applied while this
	// animation is being mixed out. Defaults to 0, so event timelines are not
	// applied while this animation is being mixed out.
	EventThreshold float32

	// MixAttachmentThreshold: when the mix percentage (MixTime / MixDuration)
	// is less than the MixAttachmentThreshold, attachment timelines are
	// applied while this animation is being mixed out. Defaults to 0, so
	// attachment timelines are not applied while this animation is being
	// mixed out.
	MixAttachmentThreshold float32

	// AlphaAttachmentThreshold: when Alpha is greater than
	// AlphaAttachmentThreshold, attachment timelines are applied. Defaults to
	// 0, so attachment timelines are always applied.
	AlphaAttachmentThreshold float32

	// MixDrawOrderThreshold: when the mix percentage (MixTime / MixDuration)
	// is less than the MixDrawOrderThreshold, draw order timelines are
	// applied while this animation is being mixed out. Defaults to 0, so draw
	// order timelines are not applied while this animation is being mixed
	// out.
	MixDrawOrderThreshold float32

	// AnimationStart: seconds when this animation starts, both initially and
	// after looping. Defaults to 0.
	//
	// When changing the AnimationStart time, it often makes sense to set
	// AnimationLast to the same value to prevent timeline keys before the
	// start time from triggering.
	AnimationStart float32

	// AnimationEnd: seconds for the last frame of this animation. Non-looping
	// animations won't play past this time. Looping animations will loop back
	// to AnimationStart at this time. Defaults to the animation Duration.
	AnimationEnd float32

	// AnimationLast is the time in seconds this animation was last applied.
	// Some timelines use this for one-time triggers. Eg, when this animation
	// is applied, event timelines will fire all events between the
	// AnimationLast time (exclusive) and animation time (inclusive). Defaults
	// to -1 to ensure triggers on frame 0 happen the first time this
	// animation is applied.
	//
	// Use SetAnimationLast to also set nextAnimationLast, as Java's
	// setAnimationLast does.
	AnimationLast float32

	nextAnimationLast float32

	// Delay: seconds to postpone playing the animation. Must be >= 0. When
	// this track entry is the current track entry, Delay postpones
	// incrementing the TrackTime. When this track entry is queued, Delay is
	// the time from the start of the previous animation to when this track
	// entry will become the current track entry (ie when the previous track
	// entry TrackTime >= this track entry's Delay).
	//
	// TimeScale affects the delay.
	//
	// When passing delay <= 0 to AnimationState.AddAnimation this Delay is
	// set using a mix duration from AnimationStateData. To change the
	// MixDuration afterward, use SetMixDurationWithDelay so this Delay is
	// adjusted.
	Delay float32

	// TrackTime is the current time in seconds this track entry has been the
	// current track entry. The track time determines AnimationTime. The track
	// time can be set to start the animation at a time other than 0, without
	// affecting looping.
	TrackTime float32

	trackLast, nextTrackLast float32

	// TrackEnd is the track time in seconds when this animation will be
	// removed from the track. Defaults to the highest possible float value,
	// meaning the animation will be applied until a new animation is set or
	// the track is cleared. If the track end time is reached, no other
	// animations are queued for playback, and mixing from any previous
	// animations is complete, then the properties keyed by the animation are
	// set to the setup pose and the track is cleared.
	//
	// It may be desired to use AnimationState.AddEmptyAnimation rather than
	// have the animation abruptly cease being applied.
	TrackEnd float32

	// TimeScale is a multiplier for the delta time when this track entry is
	// updated, causing time for this animation to pass slower or faster.
	// Defaults to 1.
	//
	// Values < 0 are not supported. To play an animation in reverse, use
	// Reverse.
	//
	// MixTime is not affected by track entry time scale, so MixDuration may
	// need to be adjusted to match the animation speed.
	//
	// When using AnimationState.AddAnimation with a delay <= 0, the Delay is
	// set using the mix duration from the AnimationStateData, assuming time
	// scale to be 1. If the time scale is not 1, the delay may need to be
	// adjusted.
	//
	// See AnimationState.TimeScale for affecting all animations.
	TimeScale float32

	// Alpha: values < 1 mix this animation with the skeleton's current pose
	// (usually the pose resulting from lower tracks). Defaults to 1, which
	// overwrites the skeleton's current pose with this animation.
	//
	// Typically track 0 is used to completely pose the skeleton, then alpha
	// is used on higher tracks. It doesn't make sense to use alpha on track 0
	// if the skeleton pose is from the last frame render.
	Alpha float32

	// MixTime: seconds from 0 to the MixDuration when mixing from the
	// previous animation to this animation. May be slightly more than
	// MixDuration when the mix is complete.
	MixTime float32

	mixDuration, interruptAlpha, totalAlpha float32

	// MixBlend controls how properties keyed in the animation are mixed with
	// lower tracks. Defaults to MixBlendReplace.
	//
	// Track entries on track 0 ignore this setting and always use
	// MixBlendFirst.
	//
	// The MixBlend can be set for a new track entry only before
	// AnimationState.Apply is first called.
	MixBlend MixBlend

	timelineMode      []int
	timelineHoldMix   []*TrackEntry
	timelinesRotation []float32
}

func (e *TrackEntry) reset() {
	e.previous = nil
	e.next = nil
	e.mixingFrom = nil
	e.mixingTo = nil
	e.Animation = nil
	e.Listener = nil
	e.timelineMode = e.timelineMode[:0]
	for i := range e.timelineHoldMix {
		e.timelineHoldMix[i] = nil
	}
	e.timelineHoldMix = e.timelineHoldMix[:0]
	e.timelinesRotation = e.timelinesRotation[:0]
}

// TrackIndex is the index of the track where this track entry is either
// current or queued.
//
// See AnimationState.GetCurrent.
func (e *TrackEntry) TrackIndex() int { return e.trackIndex }

// TrackComplete returns, if this track entry is non-looping, the track time
// in seconds when AnimationEnd is reached, or the current TrackTime if it has
// already been reached. If this track entry is looping, the track time when
// this animation will reach its next AnimationEnd (the next loop completion).
func (e *TrackEntry) TrackComplete() float32 {
	duration := e.AnimationEnd - e.AnimationStart
	if duration != 0 {
		if e.Loop {
			return duration * float32(1+int(e.TrackTime/duration)) // Completion of next loop.
		}
		if e.TrackTime < duration {
			return duration // Before duration.
		}
	}
	return e.TrackTime // Next update.
}

// SetAnimationLast sets AnimationLast and the internal next animation last
// time, matching Java's setAnimationLast side effect.
func (e *TrackEntry) SetAnimationLast(animationLast float32) {
	e.AnimationLast = animationLast
	e.nextAnimationLast = animationLast
}

// AnimationTime uses TrackTime to compute the animation time. When the
// TrackTime is 0, the animation time is equal to the AnimationStart time.
//
// The animation time is between AnimationStart and AnimationEnd, except if
// this track entry is non-looping and AnimationEnd is >= to the animation
// Duration, then the animation time continues to increase past AnimationEnd.
func (e *TrackEntry) AnimationTime() float32 {
	if e.Loop {
		duration := e.AnimationEnd - e.AnimationStart
		if duration == 0 {
			return e.AnimationStart
		}
		return float32(math.Mod(float64(e.TrackTime), float64(duration))) + e.AnimationStart
	}
	animationTime := e.TrackTime + e.AnimationStart
	if e.AnimationEnd >= e.Animation.Duration {
		return animationTime
	}
	return minF(animationTime, e.AnimationEnd)
}

// Next returns the animation queued to start after this animation, or nil if
// there is none. next makes up a doubly linked list.
//
// See AnimationState.ClearNext to truncate the list.
func (e *TrackEntry) Next() *TrackEntry { return e.next }

// Previous returns the animation queued to play before this animation, or
// nil. previous makes up a doubly linked list.
func (e *TrackEntry) Previous() *TrackEntry { return e.previous }

// WasApplied returns true if this track entry has been applied at least once.
//
// See AnimationState.Apply.
func (e *TrackEntry) WasApplied() bool { return e.nextTrackLast != -1 }

// IsNextReady returns true if there is a Next track entry and it will become
// the current track entry during the next AnimationState.Update.
func (e *TrackEntry) IsNextReady() bool {
	return e.next != nil && e.nextTrackLast-e.next.Delay >= 0
}

// IsComplete returns true if at least one loop has been completed.
//
// See AnimationStateListener.Complete.
func (e *TrackEntry) IsComplete() bool {
	return e.TrackTime >= e.AnimationEnd-e.AnimationStart
}

// MixDuration returns the seconds for mixing from the previous animation to
// this animation. Defaults to the value provided by AnimationStateData.GetMix
// based on the animation before this animation (if any).
//
// A mix duration of 0 still mixes out over one frame to provide the track
// entry being mixed out a chance to revert the properties it was animating. A
// mix duration of 0 can be set at any time to end the mix on the next
// AnimationState.Update.
//
// The mix duration can be set manually rather than use the value from
// AnimationStateData.GetMix. In that case, the mix duration can be set for a
// new track entry only before AnimationState.Update is first called.
//
// When using AnimationState.AddAnimation with a delay <= 0, the Delay is set
// using the mix duration from the AnimationStateData. If the mix duration is
// set afterward, the delay may need to be adjusted. For example:
// entry.Delay = entry.Previous().TrackComplete() - entry.MixDuration().
// Alternatively, SetMixDurationWithDelay can be used to recompute the delay.
func (e *TrackEntry) MixDuration() float32 { return e.mixDuration }

// SetMixDuration sets the mix duration. See MixDuration.
func (e *TrackEntry) SetMixDuration(mixDuration float32) { e.mixDuration = mixDuration }

// SetMixDurationWithDelay sets both the mix duration and Delay.
//
// If delay > 0, sets Delay. If delay <= 0, the delay set is the duration of
// the previous track entry minus the specified mix duration plus the
// specified delay (ie the mix ends at (delay = 0) or before (delay < 0) the
// previous track entry duration). If the previous entry is looping, its next
// loop completion is used instead of its duration.
func (e *TrackEntry) SetMixDurationWithDelay(mixDuration, delay float32) {
	e.mixDuration = mixDuration
	if delay <= 0 {
		if e.previous != nil {
			delay = maxF(delay+e.previous.TrackComplete()-mixDuration, 0)
		} else {
			delay = 0
		}
	}
	e.Delay = delay
}

// MixingFrom returns the track entry for the previous animation when mixing
// from the previous animation to this animation, or nil if no mixing is
// currently occurring. When mixing from multiple animations, mixingFrom makes
// up a linked list.
func (e *TrackEntry) MixingFrom() *TrackEntry { return e.mixingFrom }

// MixingTo returns the track entry for the next animation when mixing from
// this animation to the next animation, or nil if no mixing is currently
// occurring. When mixing to multiple animations, mixingTo makes up a linked
// list.
func (e *TrackEntry) MixingTo() *TrackEntry { return e.mixingTo }

// ResetRotationDirections resets the rotation directions for mixing this
// entry's rotate timelines. This can be useful to avoid bones rotating the
// long way around when using Alpha and starting animations on other tracks.
//
// Mixing with MixBlendReplace involves finding a rotation between two others,
// which has two possible solutions: the short way or the long way around. The
// two rotations likely change over time, so which direction is the short or
// long way also changes. If the short way was always chosen, bones would flip
// to the other side when that direction became the long way. TrackEntry
// chooses the short way the first time it is applied and remembers that
// direction.
func (e *TrackEntry) ResetRotationDirections() {
	e.timelinesRotation = e.timelinesRotation[:0]
}

// IsEmptyAnimation returns true if this entry is for the empty animation. See
// AnimationState.SetEmptyAnimation, AnimationState.AddEmptyAnimation, and
// AnimationState.SetEmptyAnimations.
func (e *TrackEntry) IsEmptyAnimation() bool { return e.Animation == emptyAnimation }

func (e *TrackEntry) String() string {
	if e.Animation == nil {
		return "<none>"
	}
	return e.Animation.Name
}

type eventType int

const (
	eventTypeStart eventType = iota
	eventTypeInterrupt
	eventTypeEnd
	eventTypeDispose
	eventTypeComplete
	eventTypeEvent
)

type eventQueue struct {
	state         *AnimationState
	objects       []any
	drainDisabled bool
}

func (q *eventQueue) start(entry *TrackEntry) {
	q.objects = append(q.objects, eventTypeStart, entry)
	q.state.animationsChanged = true
}

func (q *eventQueue) interrupt(entry *TrackEntry) {
	q.objects = append(q.objects, eventTypeInterrupt, entry)
}

func (q *eventQueue) end(entry *TrackEntry) {
	q.objects = append(q.objects, eventTypeEnd, entry)
	q.state.animationsChanged = true
}

func (q *eventQueue) dispose(entry *TrackEntry) {
	q.objects = append(q.objects, eventTypeDispose, entry)
}

func (q *eventQueue) complete(entry *TrackEntry) {
	q.objects = append(q.objects, eventTypeComplete, entry)
}

func (q *eventQueue) event(entry *TrackEntry, event *Event) {
	q.objects = append(q.objects, eventTypeEvent, entry, event)
}

func (q *eventQueue) drain() {
	if q.drainDisabled {
		return // Not reentrant.
	}
	q.drainDisabled = true

	state := q.state
	for i := 0; i < len(q.objects); i += 2 {
		objectType := q.objects[i].(eventType)
		entry := q.objects[i+1].(*TrackEntry)
		listeners := state.listeners
		listenersCount := len(listeners)
		switch objectType {
		case eventTypeStart:
			if entry.Listener != nil {
				entry.Listener.Start(entry)
			}
			for ii := 0; ii < listenersCount; ii++ {
				listeners[ii].Start(entry)
			}
		case eventTypeInterrupt:
			if entry.Listener != nil {
				entry.Listener.Interrupt(entry)
			}
			for ii := 0; ii < listenersCount; ii++ {
				listeners[ii].Interrupt(entry)
			}
		case eventTypeEnd, eventTypeDispose:
			if objectType == eventTypeEnd {
				if entry.Listener != nil {
					entry.Listener.End(entry)
				}
				for ii := 0; ii < listenersCount; ii++ {
					listeners[ii].End(entry)
				}
			}
			// Fall through from end to dispose.
			if entry.Listener != nil {
				entry.Listener.Dispose(entry)
			}
			for ii := 0; ii < listenersCount; ii++ {
				listeners[ii].Dispose(entry)
			}
			state.freeTrackEntry(entry)
		case eventTypeComplete:
			if entry.Listener != nil {
				entry.Listener.Complete(entry)
			}
			for ii := 0; ii < listenersCount; ii++ {
				listeners[ii].Complete(entry)
			}
		case eventTypeEvent:
			event := q.objects[i+2].(*Event)
			i++
			if entry.Listener != nil {
				entry.Listener.Event(entry, event)
			}
			for ii := 0; ii < listenersCount; ii++ {
				listeners[ii].Event(entry, event)
			}
		}
	}
	q.clear()

	q.drainDisabled = false
}

func (q *eventQueue) clear() {
	for i := range q.objects {
		q.objects[i] = nil
	}
	q.objects = q.objects[:0]
}

// AnimationStateListener is the interface to implement for receiving
// TrackEntry events. It is always safe to call AnimationState methods when
// receiving events.
//
// TrackEntry events are collected during AnimationState.Update and
// AnimationState.Apply and fired only after those methods are finished.
//
// See TrackEntry.Listener and AnimationState.AddListener.
type AnimationStateListener interface {
	// Start is invoked when this entry has been set as the current entry.
	// End will occur when this entry will no longer be applied.
	Start(entry *TrackEntry)

	// Interrupt is invoked when another entry has replaced this entry as the
	// current entry. This entry may continue being applied for mixing.
	Interrupt(entry *TrackEntry)

	// End is invoked when this entry will never be applied again. This only
	// occurs if this entry has previously been set as the current entry
	// (Start was invoked).
	End(entry *TrackEntry)

	// Dispose is invoked when this entry will be disposed. This may occur
	// without the entry ever being set as the current entry.
	//
	// References to the entry should not be kept after Dispose is called, as
	// it may be destroyed or reused.
	Dispose(entry *TrackEntry)

	// Complete is invoked every time this entry's animation completes a loop.
	// This may occur during mixing (after Interrupt).
	//
	// If this entry's MixingTo is not nil, this entry is mixing out (it is
	// not the current entry).
	//
	// Because this event is triggered at the end of AnimationState.Apply, any
	// animations set in response to the event won't be applied until the next
	// time the AnimationState is applied.
	Complete(entry *TrackEntry)

	// Event is invoked when this entry's animation triggers an event. This
	// may occur during mixing (after Interrupt), see
	// TrackEntry.EventThreshold.
	//
	// Because this event is triggered at the end of AnimationState.Apply, any
	// animations set in response to the event won't be applied until the next
	// time the AnimationState is applied.
	Event(entry *TrackEntry, event *Event)
}

// AnimationStateAdapter provides no-op implementations of
// AnimationStateListener for embedding.
type AnimationStateAdapter struct{}

func (AnimationStateAdapter) Start(entry *TrackEntry)               {}
func (AnimationStateAdapter) Interrupt(entry *TrackEntry)           {}
func (AnimationStateAdapter) End(entry *TrackEntry)                 {}
func (AnimationStateAdapter) Dispose(entry *TrackEntry)             {}
func (AnimationStateAdapter) Complete(entry *TrackEntry)            {}
func (AnimationStateAdapter) Event(entry *TrackEntry, event *Event) {}
