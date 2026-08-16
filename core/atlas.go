package spine

import (
	"fmt"
	"strconv"
	"strings"
)

// TextureFormat is the pixel format of an atlas page.
type TextureFormat int

const (
	TextureFormatAlpha TextureFormat = iota
	TextureFormatIntensity
	TextureFormatLuminanceAlpha
	TextureFormatRGB565
	TextureFormatRGBA4444
	TextureFormatRGB888
	TextureFormatRGBA8888
)

// TextureFilter is the texture filtering used for an atlas page.
type TextureFilter int

const (
	TextureFilterNearest TextureFilter = iota
	TextureFilterLinear
	TextureFilterMipMap
	TextureFilterMipMapNearestNearest
	TextureFilterMipMapLinearNearest
	TextureFilterMipMapNearestLinear
	TextureFilterMipMapLinearLinear
)

// TextureWrap is the texture wrapping used for an atlas page.
type TextureWrap int

const (
	TextureWrapMirroredRepeat TextureWrap = iota
	TextureWrapClampToEdge
	TextureWrapRepeat
)

// AtlasPage describes one texture page of a texture atlas.
type AtlasPage struct {
	// Name is the image file name of the page.
	Name string

	Format        TextureFormat
	MinFilter     TextureFilter
	MagFilter     TextureFilter
	UWrap, VWrap  TextureWrap
	Width, Height int

	// PMA is true if the page image has premultiplied alpha.
	PMA bool

	// Texture is a renderer-specific handle for the page image, set by the
	// rendering layer. The core runtime never touches it.
	Texture any
}

// AtlasRegion describes a packed image region within an AtlasPage.
type AtlasRegion struct {
	TextureRegion

	// Page is the page this region is packed on.
	Page *AtlasPage

	// Name of the region, unique within the atlas.
	Name string

	// Index is the number at the end of the original image file name, or 0 if
	// none.
	Index int

	// X, Y is the top left corner of the region on the page, in pixels.
	X, Y int

	// Splits are ninepatch splits, or nil (legacy libgdx atlas format).
	Splits []int

	// Pads are ninepatch pads, or nil (legacy libgdx atlas format).
	Pads []int

	// Names and Values hold any custom name:value entries for the region.
	Names  []string
	Values [][]int
}

// TextureAtlas holds the pages and regions parsed from a texture atlas file.
type TextureAtlas struct {
	Pages   []*AtlasPage
	Regions []*AtlasRegion
}

// atlasReader iterates the lines of an atlas file, distinguishing empty lines
// from end of input like C#/Java readLine (which returns null at EOF).
type atlasReader struct {
	lines []string
	pos   int
}

func (r *atlasReader) readLine() (string, bool) {
	if r.pos >= len(r.lines) {
		return "", false
	}
	line := strings.TrimSuffix(r.lines[r.pos], "\r")
	r.pos++
	return line, true
}

// readAtlasEntry parses a "name: value1, value2, ..." line into entry,
// returning the number of values, or 0 if the line is not an entry.
func readAtlasEntry(entry *[5]string, line string, ok bool) int {
	if !ok {
		return 0
	}
	line = strings.TrimSpace(line)
	if len(line) == 0 {
		return 0
	}
	colon := strings.IndexByte(line, ':')
	if colon == -1 {
		return 0
	}
	entry[0] = strings.TrimSpace(line[:colon])
	for i, lastMatch := 1, colon+1; ; i++ {
		comma := strings.IndexByte(line[lastMatch:], ',')
		if comma == -1 {
			entry[i] = strings.TrimSpace(line[lastMatch:])
			return i
		}
		comma += lastMatch
		entry[i] = strings.TrimSpace(line[lastMatch:comma])
		lastMatch = comma + 1
		if i == 4 {
			return 4
		}
	}
}

func parseTextureFormat(value string) (TextureFormat, error) {
	switch value {
	case "Alpha":
		return TextureFormatAlpha, nil
	case "Intensity":
		return TextureFormatIntensity, nil
	case "LuminanceAlpha":
		return TextureFormatLuminanceAlpha, nil
	case "RGB565":
		return TextureFormatRGB565, nil
	case "RGBA4444":
		return TextureFormatRGBA4444, nil
	case "RGB888":
		return TextureFormatRGB888, nil
	case "RGBA8888":
		return TextureFormatRGBA8888, nil
	}
	return 0, fmt.Errorf("spine: unknown texture format: %s", value)
}

func parseTextureFilter(value string) (TextureFilter, error) {
	switch value {
	case "Nearest":
		return TextureFilterNearest, nil
	case "Linear":
		return TextureFilterLinear, nil
	case "MipMap":
		return TextureFilterMipMap, nil
	case "MipMapNearestNearest":
		return TextureFilterMipMapNearestNearest, nil
	case "MipMapLinearNearest":
		return TextureFilterMipMapLinearNearest, nil
	case "MipMapNearestLinear":
		return TextureFilterMipMapNearestLinear, nil
	case "MipMapLinearLinear":
		return TextureFilterMipMapLinearLinear, nil
	}
	return 0, fmt.Errorf("spine: unknown texture filter: %s", value)
}

// NewTextureAtlas parses the text of an atlas file. Both the Spine 4.x atlas
// format and the legacy libgdx atlas format are supported. The pages' Texture
// handles are left nil for the rendering layer to fill in.
func NewTextureAtlas(data []byte) (*TextureAtlas, error) {
	atlas := &TextureAtlas{}
	reader := &atlasReader{lines: strings.Split(string(data), "\n")}

	var entry [5]string
	var page *AtlasPage
	var region *AtlasRegion

	var firstErr error
	atoi := func(value string) int {
		v, err := strconv.Atoi(value)
		if err != nil && firstErr == nil {
			firstErr = fmt.Errorf("spine: invalid atlas value: %q", value)
		}
		return v
	}

	line, ok := reader.readLine()
	// Ignore empty lines before first entry.
	for ok && strings.TrimSpace(line) == "" {
		line, ok = reader.readLine()
	}
	// Header entries.
	for {
		if !ok || strings.TrimSpace(line) == "" {
			break
		}
		if readAtlasEntry(&entry, line, ok) == 0 {
			break // Silently ignore all header fields.
		}
		line, ok = reader.readLine()
	}
	// Page and region entries.
	var names []string
	var values [][]int
	for {
		if !ok {
			break
		}
		if strings.TrimSpace(line) == "" {
			page = nil
			line, ok = reader.readLine()
		} else if page == nil {
			page = &AtlasPage{
				Name:      strings.TrimSpace(line),
				Format:    TextureFormatRGBA8888,
				MinFilter: TextureFilterNearest,
				MagFilter: TextureFilterNearest,
				UWrap:     TextureWrapClampToEdge,
				VWrap:     TextureWrapClampToEdge,
			}
			for {
				line, ok = reader.readLine()
				if readAtlasEntry(&entry, line, ok) == 0 {
					break
				}
				switch entry[0] { // Silently ignore unknown page fields.
				case "size":
					page.Width = atoi(entry[1])
					page.Height = atoi(entry[2])
				case "format":
					format, err := parseTextureFormat(entry[1])
					if err != nil && firstErr == nil {
						firstErr = err
					}
					page.Format = format
				case "filter":
					minFilter, err := parseTextureFilter(entry[1])
					if err != nil && firstErr == nil {
						firstErr = err
					}
					magFilter, err := parseTextureFilter(entry[2])
					if err != nil && firstErr == nil {
						firstErr = err
					}
					page.MinFilter = minFilter
					page.MagFilter = magFilter
				case "repeat":
					if strings.ContainsRune(entry[1], 'x') {
						page.UWrap = TextureWrapRepeat
					}
					if strings.ContainsRune(entry[1], 'y') {
						page.VWrap = TextureWrapRepeat
					}
				case "pma":
					page.PMA = entry[1] == "true"
				}
			}
			atlas.Pages = append(atlas.Pages, page)
		} else {
			region = &AtlasRegion{Page: page, Name: line}
			for {
				var count int
				line, ok = reader.readLine()
				if count = readAtlasEntry(&entry, line, ok); count == 0 {
					break
				}
				switch entry[0] {
				case "xy": // Deprecated, use bounds.
					region.X = atoi(entry[1])
					region.Y = atoi(entry[2])
				case "size": // Deprecated, use bounds.
					region.Width = float32(atoi(entry[1]))
					region.Height = float32(atoi(entry[2]))
				case "bounds":
					region.X = atoi(entry[1])
					region.Y = atoi(entry[2])
					region.Width = float32(atoi(entry[3]))
					region.Height = float32(atoi(entry[4]))
				case "offset": // Deprecated, use offsets.
					region.OffsetX = float32(atoi(entry[1]))
					region.OffsetY = float32(atoi(entry[2]))
				case "orig": // Deprecated, use offsets.
					region.OriginalWidth = float32(atoi(entry[1]))
					region.OriginalHeight = float32(atoi(entry[2]))
				case "offsets":
					region.OffsetX = float32(atoi(entry[1]))
					region.OffsetY = float32(atoi(entry[2]))
					region.OriginalWidth = float32(atoi(entry[3]))
					region.OriginalHeight = float32(atoi(entry[4]))
				case "rotate":
					value := entry[1]
					if value == "true" {
						region.Degrees = 90
					} else if value != "false" {
						region.Degrees = atoi(value)
					}
				case "index":
					region.Index = atoi(entry[1])
				case "split": // Legacy libgdx ninepatch splits.
					region.Splits = []int{atoi(entry[1]), atoi(entry[2]), atoi(entry[3]), atoi(entry[4])}
				case "pad": // Legacy libgdx ninepatch pads.
					region.Pads = []int{atoi(entry[1]), atoi(entry[2]), atoi(entry[3]), atoi(entry[4])}
				default:
					names = append(names, entry[0])
					entryValues := make([]int, count)
					for i := 0; i < count; i++ {
						// Silently ignore non-integer values.
						entryValues[i], _ = strconv.Atoi(entry[i+1])
					}
					values = append(values, entryValues)
				}
			}
			if region.OriginalWidth == 0 && region.OriginalHeight == 0 {
				region.OriginalWidth = region.Width
				region.OriginalHeight = region.Height
			}
			if len(names) > 0 {
				region.Names = names
				region.Values = values
				names = nil
				values = nil
			}
			region.PageWidth = float32(page.Width)
			region.PageHeight = float32(page.Height)
			region.U = float32(region.X) / region.PageWidth
			region.V = float32(region.Y) / region.PageHeight
			if region.Degrees == 90 {
				region.U2 = (float32(region.X) + region.Height) / region.PageWidth
				region.V2 = (float32(region.Y) + region.Width) / region.PageHeight
				region.Width, region.Height = region.Height, region.Width
			} else {
				region.U2 = (float32(region.X) + region.Width) / region.PageWidth
				region.V2 = (float32(region.Y) + region.Height) / region.PageHeight
			}
			atlas.Regions = append(atlas.Regions, region)
		}
	}
	if firstErr != nil {
		return nil, firstErr
	}
	return atlas, nil
}

// FindRegion returns the first region found with the specified name, or nil.
// This method uses string comparison to find the region, so the result should
// be cached rather than calling this method multiple times.
func (a *TextureAtlas) FindRegion(name string) *AtlasRegion {
	for _, region := range a.Regions {
		if region.Name == name {
			return region
		}
	}
	return nil
}
