package spine

// Physics determines how physics and other non-deterministic updates are
// applied by Skeleton.UpdateWorldTransform.
type Physics int

const (
	// PhysicsNone means physics are not updated or applied.
	PhysicsNone Physics = iota
	// PhysicsReset means physics are reset to the current pose.
	PhysicsReset
	// PhysicsUpdate means physics are updated and the pose from physics is applied.
	PhysicsUpdate
	// PhysicsPose means physics are not updated but the pose from physics is applied.
	PhysicsPose
)

// Updatable is the interface for items updated by Skeleton.UpdateWorldTransform.
type Updatable interface {
	// Update updates the item. physics determines how physics and other
	// non-deterministic updates are applied.
	Update(physics Physics)

	// IsActive returns false when this item won't be updated by
	// Skeleton.UpdateWorldTransform because a skin is required and the active
	// skin does not contain this item.
	IsActive() bool
}
