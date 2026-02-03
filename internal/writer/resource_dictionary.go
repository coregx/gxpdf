// Package writer provides PDF writing infrastructure for generating PDF files.
package writer

import (
	"bytes"
	"fmt"
	"sort"
)

// ResourceDictionary manages PDF page resources (fonts, images, graphics states, etc.).
//
// Resources are referenced in content streams by name (e.g., /F1 for fonts, /Im1 for images).
// This struct tracks resource names and their corresponding PDF object numbers.
//
// PDF Dictionary Format:
//
//	/Resources <<
//	  /Font << /F1 5 0 R /F2 6 0 R >>
//	  /XObject << /Im1 7 0 R >>
//	  /ExtGState << /GS1 8 0 R >>
//	  /ProcSet [/PDF /Text /ImageB /ImageC /ImageI]
//	>>
//
// Thread Safety: Not thread-safe. Caller must synchronize if needed.
type ResourceDictionary struct {
	fonts           map[string]int     // Font resource name -> object number (e.g., "F1" -> 5)
	fontIDs         map[string]string  // Font ID -> resource name (e.g., "custom:font_1" -> "F1")
	xobjects        map[string]int     // XObject resource name -> object number (e.g., "Im1" -> 10)
	extgstates      map[string]int     // ExtGState resource name -> object number (e.g., "GS1" -> 15)
	extgstateCache  map[float64]string // Opacity -> ExtGState name (for caching, e.g., 0.5 -> "GS1")
	extgstateObjMap map[string]int     // ExtGState name -> object number (for later setting)
}

// NewResourceDictionary creates a new empty resource dictionary.
func NewResourceDictionary() *ResourceDictionary {
	return &ResourceDictionary{
		fonts:           make(map[string]int),
		fontIDs:         make(map[string]string),
		xobjects:        make(map[string]int),
		extgstates:      make(map[string]int),
		extgstateCache:  make(map[float64]string),
		extgstateObjMap: make(map[string]int),
	}
}

// AddFont adds a font resource and returns its resource name.
//
// Fonts are named sequentially: F1, F2, F3, etc.
//
// Parameters:
//   - objNum: PDF object number of the font dictionary
//
// Returns:
//   - Resource name (e.g., "F1")
//
// Example:
//
//	rd := NewResourceDictionary()
//	name := rd.AddFont(5)  // Returns "F1"
//	// In content stream: /F1 12 Tf (set font F1 at 12pt)
func (rd *ResourceDictionary) AddFont(objNum int) string {
	name := fmt.Sprintf("F%d", len(rd.fonts)+1)
	rd.fonts[name] = objNum
	return name
}

// AddFontWithID adds a font resource with an associated ID and returns its resource name.
//
// The fontID is used to later set the correct object number via SetFontObjNumByID.
// This enables correct font object assignment when fonts are created after content streams.
//
// If a font with the same fontID already exists, returns the existing resource name
// instead of creating a duplicate entry.
//
// Parameters:
//   - objNum: PDF object number (can be 0 as placeholder)
//   - fontID: Unique identifier for this font (e.g., "custom:font_1" or "std:Helvetica")
//
// Returns:
//   - Resource name (e.g., "F1")
func (rd *ResourceDictionary) AddFontWithID(objNum int, fontID string) string {
	// If font with this ID already exists, return existing resource name.
	if existingName, exists := rd.fontIDs[fontID]; exists {
		return existingName
	}

	// Create new resource name.
	name := fmt.Sprintf("F%d", len(rd.fonts)+1)
	rd.fonts[name] = objNum
	rd.fontIDs[fontID] = name
	return name
}

// SetFontObjNumByID sets the object number for a font identified by its ID.
//
// This is used after font objects are created to update the placeholder object numbers.
//
// Returns true if the font was found and updated, false otherwise.
func (rd *ResourceDictionary) SetFontObjNumByID(fontID string, objNum int) bool {
	resName, ok := rd.fontIDs[fontID]
	if !ok {
		return false
	}
	rd.fonts[resName] = objNum
	return true
}

// GetFontIDMapping returns a copy of the font ID to resource name mapping.
//
// This is useful for debugging and testing.
func (rd *ResourceDictionary) GetFontIDMapping() map[string]string {
	result := make(map[string]string, len(rd.fontIDs))
	for k, v := range rd.fontIDs {
		result[k] = v
	}
	return result
}

// GetFontResourceName returns the resource name for a font ID.
//
// Returns empty string if the font ID is not found.
func (rd *ResourceDictionary) GetFontResourceName(fontID string) string {
	return rd.fontIDs[fontID]
}

// AddImage adds an image XObject resource and returns its resource name.
//
// Images are named sequentially: Im1, Im2, Im3, etc.
//
// Parameters:
//   - objNum: PDF object number of the image XObject
//
// Returns:
//   - Resource name (e.g., "Im1")
//
// Example:
//
//	rd := NewResourceDictionary()
//	name := rd.AddImage(10)  // Returns "Im1"
//	// In content stream: /Im1 Do (draw image Im1)
func (rd *ResourceDictionary) AddImage(objNum int) string {
	name := fmt.Sprintf("Im%d", len(rd.xobjects)+1)
	rd.xobjects[name] = objNum
	return name
}

// AddExtGState adds a graphics state resource and returns its resource name.
//
// Graphics states are named sequentially: GS1, GS2, GS3, etc.
//
// Parameters:
//   - objNum: PDF object number of the ExtGState dictionary
//
// Returns:
//   - Resource name (e.g., "GS1")
//
// Example:
//
//	rd := NewResourceDictionary()
//	name := rd.AddExtGState(15)  // Returns "GS1"
//	// In content stream: /GS1 gs (apply graphics state GS1)
func (rd *ResourceDictionary) AddExtGState(objNum int) string {
	name := fmt.Sprintf("GS%d", len(rd.extgstates)+1)
	rd.extgstates[name] = objNum
	return name
}

// GetOrCreateExtGState returns an existing or creates a new ExtGState for the given opacity.
//
// This method caches ExtGState objects by opacity value to avoid creating duplicates.
// Multiple drawing operations with the same opacity will share the same ExtGState object.
//
// Parameters:
//   - opacity: Opacity value (0.0 = transparent, 1.0 = opaque)
//
// Returns:
//   - Resource name (e.g., "GS1")
//   - needsCreation: true if this is a new ExtGState that needs object creation
//
// Example:
//
//	rd := NewResourceDictionary()
//	name1, needsCreate := rd.GetOrCreateExtGState(0.5)
//	// name1 = "GS1", needsCreate = true (first time)
//
//	name2, needsCreate := rd.GetOrCreateExtGState(0.5)
//	// name2 = "GS1", needsCreate = false (cached)
//
//	name3, needsCreate := rd.GetOrCreateExtGState(0.3)
//	// name3 = "GS2", needsCreate = true (different opacity)
func (rd *ResourceDictionary) GetOrCreateExtGState(opacity float64) (string, bool) {
	// Check if ExtGState for this opacity already exists
	if name, exists := rd.extgstateCache[opacity]; exists {
		return name, false // Already exists, no need to create
	}

	// Create new resource name
	name := fmt.Sprintf("GS%d", len(rd.extgstates)+1)

	// Cache by opacity
	rd.extgstateCache[opacity] = name

	// Add to extgstates map with placeholder object number (0)
	// The actual object number will be set later via SetExtGStateObjNum
	rd.extgstates[name] = 0
	rd.extgstateObjMap[name] = 0

	return name, true // New ExtGState, needs creation
}

// SetExtGStateObjNum sets the object number for an ExtGState resource.
//
// This is called after the ExtGState PDF object has been created.
//
// Parameters:
//   - name: ExtGState resource name (e.g., "GS1")
//   - objNum: PDF object number
//
// Returns:
//   - true if the ExtGState was found and updated, false otherwise
func (rd *ResourceDictionary) SetExtGStateObjNum(name string, objNum int) bool {
	if _, exists := rd.extgstates[name]; !exists {
		return false
	}
	rd.extgstates[name] = objNum
	rd.extgstateObjMap[name] = objNum
	return true
}

// GetExtGStateObjNum returns the object number for an ExtGState resource.
//
// Parameters:
//   - name: ExtGState resource name (e.g., "GS1")
//
// Returns:
//   - Object number (0 if not found or not yet set)
func (rd *ResourceDictionary) GetExtGStateObjNum(name string) int {
	return rd.extgstates[name]
}

// HasResources returns true if any resources are registered.
//
// Use this to check if the resource dictionary is empty before writing.
func (rd *ResourceDictionary) HasResources() bool {
	return len(rd.fonts) > 0 || len(rd.xobjects) > 0 || len(rd.extgstates) > 0
}

// Bytes returns the resource dictionary as PDF bytes.
//
// Format:
//
//	<< /Font << /F1 5 0 R >> /XObject << /Im1 10 0 R >> /ProcSet [/PDF /Text /ImageB /ImageC /ImageI] >>
//
// Returns empty dictionary if no resources are registered:
//
//	<< >>
//
// Note: Resource names are sorted alphabetically for consistent output.
func (rd *ResourceDictionary) Bytes() []byte {
	var buf bytes.Buffer
	buf.WriteString("<<")

	// Font resources.
	if len(rd.fonts) > 0 {
		buf.WriteString(" /Font <<")
		rd.writeSortedResources(&buf, rd.fonts)
		buf.WriteString(" >>")
	}

	// XObject resources (images, forms).
	if len(rd.xobjects) > 0 {
		buf.WriteString(" /XObject <<")
		rd.writeSortedResources(&buf, rd.xobjects)
		buf.WriteString(" >>")
	}

	// ExtGState resources (graphics states).
	if len(rd.extgstates) > 0 {
		buf.WriteString(" /ExtGState <<")
		rd.writeSortedResources(&buf, rd.extgstates)
		buf.WriteString(" >>")
	}

	// ProcSet (procedure set) - required for compatibility with old PDF readers.
	// Modern readers ignore this, but it's recommended for maximum compatibility.
	if rd.HasResources() {
		buf.WriteString(" /ProcSet [/PDF /Text /ImageB /ImageC /ImageI]")
	}

	buf.WriteString(" >>")

	return buf.Bytes()
}

// String returns the resource dictionary as a PDF string.
//
// Convenience method for debugging and testing.
func (rd *ResourceDictionary) String() string {
	return string(rd.Bytes())
}

// writeSortedResources writes resources to buffer in sorted order.
//
// Resources are sorted by name (F1, F2, F3, ...) for consistent output.
// This makes diffs more readable and tests more reliable.
//
// Format: /Name ObjNum 0 R.
func (rd *ResourceDictionary) writeSortedResources(buf *bytes.Buffer, resources map[string]int) {
	// Sort resource names for consistent output.
	names := make([]string, 0, len(resources))
	for name := range resources {
		names = append(names, name)
	}
	sort.Strings(names)

	// Write resources in sorted order.
	for _, name := range names {
		objNum := resources[name]
		fmt.Fprintf(buf, " /%s %d 0 R", name, objNum)
	}
}
