package report

import (
	_ "embed"

	"github.com/go-pdf/fpdf"
)

//go:embed fonts/NotoSans-Regular.ttf
var notoRegular []byte

//go:embed fonts/NotoSans-Bold.ttf
var notoBold []byte

//go:embed fonts/NotoSans-Italic.ttf
var notoItalic []byte

//go:embed fonts/NotoSansMono-Regular.ttf
var notoMono []byte

const (
	fontSans = "Noto"
	fontMono = "NotoMono"
)

// registerFonts adds the embedded Noto fonts to the document
func registerFonts(pdf *fpdf.Fpdf) {
	pdf.AddUTF8FontFromBytes(fontSans, "", notoRegular)
	pdf.AddUTF8FontFromBytes(fontSans, "B", notoBold)
	pdf.AddUTF8FontFromBytes(fontSans, "I", notoItalic)
	pdf.AddUTF8FontFromBytes(fontMono, "", notoMono)
}
