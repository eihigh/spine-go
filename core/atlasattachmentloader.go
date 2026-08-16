package spine

import "fmt"

// AtlasAttachmentLoader is an AttachmentLoader that configures attachments
// using texture regions from a TextureAtlas.
//
// See "Loading skeleton data" in the Spine Runtimes Guide:
// https://esotericsoftware.com/spine-loading-skeleton-data
type AtlasAttachmentLoader struct {
	Atlas *TextureAtlas
}

// NewAtlasAttachmentLoader creates an attachment loader that resolves regions
// from the given atlas.
func NewAtlasAttachmentLoader(atlas *TextureAtlas) *AtlasAttachmentLoader {
	if atlas == nil {
		panic("spine: atlas cannot be nil")
	}
	return &AtlasAttachmentLoader{Atlas: atlas}
}

func (l *AtlasAttachmentLoader) loadSequence(name, basePath string, sequence *Sequence) error {
	regions := sequence.Regions
	for i := range regions {
		path := sequence.Path(basePath, i)
		region := l.Atlas.FindRegion(path)
		if region == nil {
			return fmt.Errorf("spine: region not found in atlas: %s (sequence: %s)", path, name)
		}
		regions[i] = &region.TextureRegion
	}
	return nil
}

// NewRegionAttachment implements AttachmentLoader.
func (l *AtlasAttachmentLoader) NewRegionAttachment(skin *Skin, name, path string, sequence *Sequence) (*RegionAttachment, error) {
	attachment := NewRegionAttachment(name)
	if sequence != nil {
		if err := l.loadSequence(name, path, sequence); err != nil {
			return nil, err
		}
	} else {
		region := l.Atlas.FindRegion(path)
		if region == nil {
			return nil, fmt.Errorf("spine: region not found in atlas: %s (region attachment: %s)", path, name)
		}
		attachment.SetRegion(&region.TextureRegion)
	}
	return attachment, nil
}

// NewMeshAttachment implements AttachmentLoader.
func (l *AtlasAttachmentLoader) NewMeshAttachment(skin *Skin, name, path string, sequence *Sequence) (*MeshAttachment, error) {
	attachment := NewMeshAttachment(name)
	if sequence != nil {
		if err := l.loadSequence(name, path, sequence); err != nil {
			return nil, err
		}
	} else {
		region := l.Atlas.FindRegion(path)
		if region == nil {
			return nil, fmt.Errorf("spine: region not found in atlas: %s (mesh attachment: %s)", path, name)
		}
		attachment.SetRegion(&region.TextureRegion)
	}
	return attachment, nil
}

// NewBoundingBoxAttachment implements AttachmentLoader.
func (l *AtlasAttachmentLoader) NewBoundingBoxAttachment(skin *Skin, name string) (*BoundingBoxAttachment, error) {
	return NewBoundingBoxAttachment(name), nil
}

// NewClippingAttachment implements AttachmentLoader.
func (l *AtlasAttachmentLoader) NewClippingAttachment(skin *Skin, name string) (*ClippingAttachment, error) {
	return NewClippingAttachment(name), nil
}

// NewPathAttachment implements AttachmentLoader.
func (l *AtlasAttachmentLoader) NewPathAttachment(skin *Skin, name string) (*PathAttachment, error) {
	return NewPathAttachment(name), nil
}

// NewPointAttachment implements AttachmentLoader.
func (l *AtlasAttachmentLoader) NewPointAttachment(skin *Skin, name string) (*PointAttachment, error) {
	return NewPointAttachment(name), nil
}
