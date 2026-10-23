package platform

import (
	"reflect"
	"testing"
)

func TestUTF16LEWithBOM(t *testing.T) {
	got := utf16LEWithBOM("aé😀")
	want := []byte{0xFF, 0xFE, 'a', 0x00, 0xE9, 0x00, 0x3D, 0xD8, 0x00, 0xDE}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("utf16LEWithBOM = % X, want % X", got, want)
	}
}
