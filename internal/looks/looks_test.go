package looks

import (
	"image"
	"image/color"
	"testing"
	"time"

	"github.com/blu9a/shipwake/internal/model"
)

func TestParse(t *testing.T) {
	for _, c := range []struct{ name, subject, variant string }{
		{"look: Button", "Button", ""},
		{"look: Button [dark]", "Button", "dark"},
		{"look:  List row  [ dark ,phone ]", "List row", "dark, phone"},
		{"look: Price [EUR] [light]", "Price [EUR]", "light"},
		{"look: Empty []", "Empty", ""},
	} {
		s, v, ok := Parse(c.name)
		if !ok || s != c.subject || v != c.variant {
			t.Errorf("Parse(%q) = %q, %q, %v; want %q, %q", c.name, s, v, ok, c.subject, c.variant)
		}
	}
	for _, name := range []string{"Button", "decisive: look: Button", "look:", "look: [dark]", "screenshot"} {
		if _, _, ok := Parse(name); ok {
			t.Errorf("Parse(%q) should not be a look", name)
		}
	}
	if got := Name("Button", "dark, phone"); got != "Button [dark, phone]" {
		t.Errorf("Name = %q", got)
	}
	if s, v, _ := Parse("look: " + Name("Card", "light, desktop")); s != "Card" || v != "light, desktop" {
		t.Errorf("a name reads back as itself: %q %q", s, v)
	}
}

func TestID(t *testing.T) {
	a, b := ID("List row", "dark, phone"), ID("List row", "dark phone")
	if a == b {
		t.Errorf("distinct variants share an ID: %s", a)
	}
	if a != ID("List row", "dark, phone") {
		t.Error("an ID must be stable")
	}
	if want := "list-row-dark-phone-"; a[:len(want)] != want {
		t.Errorf("ID = %q, want it to start with %q", a, want)
	}
}

func TestCollect(t *testing.T) {
	png := func(name, path string) model.Attachment {
		return model.Attachment{Name: name, Type: "image/png", Path: path}
	}
	runs := []*model.Run{
		{ID: "a", Tests: []model.Test{
			{Attachments: []model.Attachment{png("look: Button [light]", "1.png"), png("decisive: it shows", "2.png"), {Name: "look: Log", Type: "text/plain", Path: "x.txt"}}},
			{Steps: []model.Step{{Steps: []model.Step{{Attachments: []model.Attachment{png("look: Card", "3.png")}}}}}},
			{Attachments: []model.Attachment{png("look: Button [light]", "4.png")}}, // taken again in the same run: the newer one
		}},
		{ID: "b", Tests: []model.Test{{Attachments: []model.Attachment{png("look: Button [light]", "5.png"), png("look: Button [dark]", "6.png")}}}},
	}
	got := Collect(runs)
	want := []Shot{
		{Subject: "Button", Variant: "light", Run: "a", Path: "4.png", Type: "image/png"},
		{Subject: "Card", Run: "a", Path: "3.png", Type: "image/png"},
		{Subject: "Button", Variant: "dark", Run: "b", Path: "6.png", Type: "image/png"},
	}
	if len(got) != len(want) {
		t.Fatalf("Collect = %+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("shot %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestBaseline(t *testing.T) {
	day := func(d int) time.Time { return time.Date(2026, 1, d, 0, 0, 0, 0, time.UTC) }
	approvals := []model.LookApproval{
		{ID: "1", Subject: "Button", Commit: "c1", At: day(1)},
		{ID: "2", Subject: "Button", Commit: "c2", At: day(3)},
		{ID: "3", Subject: "Button", Commit: "elsewhere", At: day(4)}, // on another branch
		{ID: "4", Subject: "Button", Variant: "dark", Commit: "c1", At: day(5)},
	}
	before := map[string]bool{"c1": true, "c2": true}
	got := Baseline(approvals, "Button", "", func(a model.LookApproval) bool { return before[a.Commit] })
	if got == nil || got.ID != "2" {
		t.Errorf("Baseline = %+v, want the newest approval that applies", got)
	}
	if got := Baseline(approvals, "Card", "", func(model.LookApproval) bool { return true }); got != nil {
		t.Errorf("a look nobody approved has no baseline: %+v", got)
	}
}

// button draws a flat button: a rounded-ish rectangle on a background.
func button(w, h int, fill color.NRGBA) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.SetNRGBA(x, y, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
			if x >= 4 && x < w-4 && y >= 4 && y < h-4 {
				img.SetNRGBA(x, y, fill)
			}
		}
	}
	return img
}

var blue = color.NRGBA{R: 0x25, G: 0x63, B: 0xEB, A: 0xFF}

func TestCompareSame(t *testing.T) {
	a, b := button(40, 20, blue), button(40, 20, blue)
	if r := Compare(a, b); r.Pixels != 0 {
		t.Errorf("identical images differ in %d pixels", r.Pixels)
	}
	// a shade below the threshold is the same look
	near := button(40, 20, color.NRGBA{R: 0x26, G: 0x63, B: 0xEB, A: 0xFF})
	if r := Compare(near, a); r.Pixels != 0 {
		t.Errorf("an imperceptible shade differs in %d pixels", r.Pixels)
	}
}

func TestCompareChanged(t *testing.T) {
	was := button(40, 20, blue)
	now := button(40, 20, color.NRGBA{R: 0x16, G: 0xA3, B: 0x4A, A: 0xFF})
	r := Compare(now, was)
	if want := 32 * 12; r.Pixels != want {
		t.Errorf("a recolored button differs in %d pixels, want %d", r.Pixels, want)
	}
	if r.Diff.Bounds() != was.Bounds() {
		t.Errorf("diff bounds %v", r.Diff.Bounds())
	}
	if c := r.Diff.NRGBAAt(10, 8); c != Tint {
		t.Errorf("a changed pixel is tinted, got %v", c)
	}
	if c := r.Diff.NRGBAAt(4, 10); c != Highlight {
		t.Errorf("the change is outlined, got %v", c)
	}
	if c := r.Diff.NRGBAAt(1, 1); c.A != 0 {
		t.Errorf("an unchanged pixel is transparent, got %v", c)
	}
	again := Compare(now, was)
	if again.Pixels != r.Pixels || string(again.Diff.Pix) != string(r.Diff.Pix) {
		t.Error("comparing the same images must give the same result")
	}
}

func TestCompareSize(t *testing.T) {
	r := Compare(button(40, 24, blue), button(40, 20, blue))
	if r.Diff.Bounds().Dy() != 24 {
		t.Errorf("diff is as large as the larger image: %v", r.Diff.Bounds())
	}
	if r.Pixels < 40*4 {
		t.Errorf("the rows only one image has differ: %d", r.Pixels)
	}
}

func TestCompareAntialiasing(t *testing.T) {
	// A vertical edge between white and black, its edge pixel shaded grey: moving the shade is
	// anti-aliasing, not a change.
	edge := func(shade uint8) *image.NRGBA {
		img := image.NewNRGBA(image.Rect(0, 0, 9, 9))
		for y := range 9 {
			for x := range 9 {
				v := uint8(255)
				switch {
				case x > 4:
					v = 0
				case x == 4:
					v = shade
				}
				img.SetNRGBA(x, y, color.NRGBA{R: v, G: v, B: v, A: 255})
			}
		}
		return img
	}
	if r := Compare(edge(90), edge(170)); r.Pixels != 0 {
		t.Errorf("an anti-aliased edge that shifts its shade differs in %d pixels", r.Pixels)
	}
}

func TestEncodePNG(t *testing.T) {
	data, err := EncodePNG(button(8, 8, blue))
	if err != nil || len(data) < 8 || string(data[1:4]) != "PNG" {
		t.Fatalf("EncodePNG: %v", err)
	}
}
