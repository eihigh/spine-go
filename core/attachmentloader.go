package spine

// AttachmentLoader creates attachments as skeleton data is loaded. Loader
// methods may return a nil attachment (with a nil error) to omit the
// attachment from the loaded skeleton data.
type AttachmentLoader interface {
	NewRegionAttachment(skin *Skin, name, path string, sequence *Sequence) (*RegionAttachment, error)
	NewMeshAttachment(skin *Skin, name, path string, sequence *Sequence) (*MeshAttachment, error)
	NewBoundingBoxAttachment(skin *Skin, name string) (*BoundingBoxAttachment, error)
	NewClippingAttachment(skin *Skin, name string) (*ClippingAttachment, error)
	NewPathAttachment(skin *Skin, name string) (*PathAttachment, error)
	NewPointAttachment(skin *Skin, name string) (*PointAttachment, error)
}
