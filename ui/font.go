package ui

import "strings"

const (
	FontGlyphWidth  = 5
	FontGlyphHeight = 7
	FontAdvance     = 6
)

var fontGlyphs = map[rune]string{
	' ': ".....\n.....\n.....\n.....\n.....\n.....\n.....",
	'!': "..#..\n..#..\n..#..\n..#..\n..#..\n.....\n..#..",
	'"': ".#.#.\n.#.#.\n.#.#.\n.....\n.....\n.....\n.....",
	',': ".....\n.....\n.....\n.....\n.....\n..#..\n.#...",
	'-': ".....\n.....\n.....\n.###.\n.....\n.....\n.....",
	'.': ".....\n.....\n.....\n.....\n.....\n.....\n..#..",
	'/': "....#\n...#.\n...#.\n..#..\n.#...\n.#...\n#....",
	'0': ".###.\n#...#\n#..##\n#.#.#\n##..#\n#...#\n.###.",
	'1': "..#..\n.##..\n..#..\n..#..\n..#..\n..#..\n.###.",
	'2': ".###.\n#...#\n....#\n...#.\n..#..\n.#...\n#####",
	'3': ".###.\n#...#\n....#\n..##.\n....#\n#...#\n.###.",
	'4': "...#.\n..##.\n.#.#.\n#..#.\n#####\n...#.\n...#.",
	'5': "#####\n#....\n####.\n....#\n....#\n#...#\n.###.",
	'6': ".###.\n#...#\n#....\n####.\n#...#\n#...#\n.###.",
	'7': "#####\n....#\n...#.\n..#..\n.#...\n.#...\n.#...",
	'8': ".###.\n#...#\n#...#\n.###.\n#...#\n#...#\n.###.",
	'9': ".###.\n#...#\n#...#\n.####\n....#\n#...#\n.###.",
	':': ".....\n..#..\n.....\n.....\n.....\n..#..\n.....",
	'(': "...#.\n..#..\n.#...\n.#...\n.#...\n..#..\n...#.",
	')': ".#...\n..#..\n...#.\n...#.\n...#.\n..#..\n.#...",
	'+': ".....\n..#..\n..#..\n#####\n..#..\n..#..\n.....",
	'=': ".....\n.....\n#####\n.....\n#####\n.....\n.....",
	'?': ".###.\n#...#\n...#.\n..#..\n..#..\n.....\n..#..",
	'A': ".###.\n#...#\n#...#\n#####\n#...#\n#...#\n#...#",
	'B': "####.\n#...#\n#...#\n####.\n#...#\n#...#\n####.",
	'C': ".###.\n#...#\n#....\n#....\n#....\n#...#\n.###.",
	'D': "####.\n#...#\n#...#\n#...#\n#...#\n#...#\n####.",
	'E': "#####\n#....\n#....\n####.\n#....\n#....\n#####",
	'F': "#####\n#....\n#....\n####.\n#....\n#....\n#....",
	'G': ".###.\n#...#\n#....\n#..##\n#...#\n#...#\n.###.",
	'H': "#...#\n#...#\n#...#\n#####\n#...#\n#...#\n#...#",
	'I': ".###.\n..#..\n..#..\n..#..\n..#..\n..#..\n.###.",
	'J': "..###\n...#.\n...#.\n...#.\n...#.\n#..#.\n.##..",
	'K': "#...#\n#..#.\n#.#..\n##...\n#.#..\n#..#.\n#...#",
	'L': "#....\n#....\n#....\n#....\n#....\n#....\n#####",
	'M': "#...#\n##.##\n#.#.#\n#...#\n#...#\n#...#\n#...#",
	'N': "#...#\n##..#\n#.#.#\n#..##\n#...#\n#...#\n#...#",
	'O': ".###.\n#...#\n#...#\n#...#\n#...#\n#...#\n.###.",
	'P': "####.\n#...#\n#...#\n####.\n#....\n#....\n#....",
	'Q': ".###.\n#...#\n#...#\n#...#\n#.#.#\n#..#.\n.##.#",
	'R': "####.\n#...#\n#...#\n####.\n#.#..\n#..#.\n#...#",
	'S': ".####\n#....\n#....\n.###.\n....#\n....#\n####.",
	'T': "#####\n..#..\n..#..\n..#..\n..#..\n..#..\n..#..",
	'U': "#...#\n#...#\n#...#\n#...#\n#...#\n#...#\n.###.",
	'V': "#...#\n#...#\n#...#\n#...#\n#...#\n.#.#.\n..#..",
	'W': "#...#\n#...#\n#...#\n#...#\n#.#.#\n##.##\n#...#",
	'X': "#...#\n#...#\n.#.#.\n..#..\n.#.#.\n#...#\n#...#",
	'Y': "#...#\n#...#\n.#.#.\n..#..\n..#..\n..#..\n..#..",
	'Z': "#####\n....#\n...#.\n..#..\n.#...\n#....\n#####",
}

func glyphFor(r rune) string {
	if g, ok := fontGlyphs[r]; ok {
		return g
	}
	if r >= 'a' && r <= 'z' {
		if g, ok := fontGlyphs[r-('a'-'A')]; ok {
			return g
		}
	}
	return fontGlyphs[' ']
}

type FontAtlas struct {
	Pixels      []byte
	Width       uint32
	Height      uint32
	GlyphOf     map[rune]uint32
	GlyphCount  uint32
	GlyphWidth  uint32
	GlyphHeight uint32
}

func BuildFontAtlas() FontAtlas {
	runes := sortedRunes()
	count := uint32(len(runes)) + 1

	width := count * FontGlyphWidth
	height := uint32(FontGlyphHeight)
	pixels := make([]byte, width*height)
	glyphOf := make(map[rune]uint32, len(runes))

	for i, r := range runes {
		col := uint32(i + 1)
		glyphOf[r] = col
		writeGlyph(pixels, width, col, glyphFor(r))
	}

	return FontAtlas{
		Pixels:      pixels,
		Width:       width,
		Height:      height,
		GlyphOf:     glyphOf,
		GlyphCount:  count,
		GlyphWidth:  FontGlyphWidth,
		GlyphHeight: FontGlyphHeight,
	}
}

func sortedRunes() []rune {
	runes := make([]rune, 0, len(fontGlyphs))
	for r := range fontGlyphs {
		runes = append(runes, r)
	}
	for i := 1; i < len(runes); i++ {
		for j := i; j > 0 && runes[j-1] > runes[j]; j-- {
			runes[j-1], runes[j] = runes[j], runes[j-1]
		}
	}
	return runes
}

func writeGlyph(pixels []byte, atlasWidth uint32, col uint32, pattern string) {
	rows := strings.Split(pattern, "\n")
	if len(rows) > FontGlyphHeight {
		rows = rows[:FontGlyphHeight]
	}
	originX := col * FontGlyphWidth
	for y, row := range rows {
		for x, c := range row {
			if c == '#' {
				offset := uint32(y)*atlasWidth + originX + uint32(x)
				if int(offset) < len(pixels) {
					pixels[offset] = 255
				}
			}
		}
	}
}

func (a *FontAtlas) LookupGlyph(r rune) uint32 {
	if col, ok := a.GlyphOf[r]; ok {
		return col
	}
	if r >= 'a' && r <= 'z' {
		if col, ok := a.GlyphOf[r-('a'-'A')]; ok {
			return col
		}
	}
	return 0
}
