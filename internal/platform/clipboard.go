package platform

import "unicode/utf16"

// utf16LEWithBOM encodes s as UTF-16LE prefixed with a byte-order mark. clip.exe
// reads stdin in the OEM code page unless it starts with this BOM, so both the
// Windows and WSL clipboard paths send it to keep non-ASCII values intact.
func utf16LEWithBOM(s string) []byte {
	units := utf16.Encode([]rune(s))
	b := make([]byte, 2, 2+2*len(units))
	b[0], b[1] = 0xFF, 0xFE
	for _, u := range units {
		b = append(b, byte(u), byte(u>>8))
	}
	return b
}
