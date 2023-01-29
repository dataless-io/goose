package links

import (
	"fmt"
	"testing"

	"github.com/fulldump/biff"
)

func TestLinks(t *testing.T) {

	all := ParseLinks(`#here llal he#re Hello @fulanez here is a website https://inceptiondb.io you should check with @menganez, email me at me@gmail.com #do #it #now`)

	for _, link := range all {
		fmt.Println(link, "|"+link.Text+"|")
	}
}

func TestLinksMultibyte(t *testing.T) {

	message := `@one lala @two ñaña @three lulululuuuu`
	links := ParseLinks(message)

	for _, link := range links {
		biff.AssertEqual(link.Text, string([]rune(message)[link.Begin:link.End]))
	}
}

func TestLinksCode(t *testing.T) {

	message := "Prueba el tag `<b>bold</b>` para indicar negrita"
	links := ParseLinks(message)

	for _, link := range links {
		fmt.Println(link)
		biff.AssertEqual(link.Text, message[link.Begin:link.End])
	}
}
