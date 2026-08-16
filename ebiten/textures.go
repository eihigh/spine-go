// Package spineebiten renders spine-go skeletons with Ebitengine.
package spineebiten

import (
	"fmt"
	"image"
	"image/draw"
	_ "image/png"
	"os"
	"path/filepath"

	"github.com/hajimehoshi/ebiten/v2"

	spine "github.com/eihigh/spine-go/core"
)

// LoadTextures loads the page images of the atlas from dir and assigns an
// *ebiten.Image to every page and region (TextureRegion.Texture).
//
// Pages exported with premultiplied alpha ("pma: true" in the atlas) are
// uploaded without further premultiplication; straight-alpha pages are
// premultiplied by Ebitengine on upload. Either way the resulting GPU
// textures are premultiplied, which is what the Renderer's blend factors
// assume.
func LoadTextures(atlas *spine.TextureAtlas, dir string) error {
	for _, page := range atlas.Pages {
		img, err := loadImage(filepath.Join(dir, page.Name), page.PMA)
		if err != nil {
			return err
		}
		page.Texture = img
	}
	for _, region := range atlas.Regions {
		region.TextureRegion.Texture = region.Page.Texture
	}
	return nil
}

// SetPageTexture assigns a texture to an atlas page and all its regions. Use
// this instead of LoadTextures when page images come from somewhere other
// than the file system (embedded assets, downloads, ...). The image must
// contain premultiplied-alpha pixel data as all *ebiten.Image do; if the
// source PNG was exported with premultiplied alpha, decode it with
// NewImageFromPMA to avoid double premultiplication.
func SetPageTexture(atlas *spine.TextureAtlas, pageName string, texture *ebiten.Image) error {
	for _, page := range atlas.Pages {
		if page.Name != pageName {
			continue
		}
		page.Texture = texture
		for _, region := range atlas.Regions {
			if region.Page == page {
				region.TextureRegion.Texture = texture
			}
		}
		return nil
	}
	return fmt.Errorf("spineebiten: page not found in atlas: %s", pageName)
}

// NewImageFromPMA creates an *ebiten.Image from an image whose pixel values
// are already premultiplied but whose Go type reports straight alpha (as
// happens when decoding a PNG exported by Spine with "premultiply alpha").
// The pixels are copied verbatim so they are not premultiplied a second time.
func NewImageFromPMA(src image.Image) *ebiten.Image {
	bounds := src.Bounds()
	rgba := image.NewRGBA(bounds)
	switch s := src.(type) {
	case *image.NRGBA:
		// Same byte layout; reinterpret the (already premultiplied) values.
		copy(rgba.Pix, s.Pix)
	default:
		draw.Draw(rgba, bounds, src, bounds.Min, draw.Src)
	}
	return ebiten.NewImageFromImage(rgba)
}

func loadImage(path string, pma bool) (*ebiten.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("spineebiten: %w", err)
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("spineebiten: decoding %s: %w", path, err)
	}
	if pma {
		return NewImageFromPMA(img), nil
	}
	return ebiten.NewImageFromImage(img), nil
}
