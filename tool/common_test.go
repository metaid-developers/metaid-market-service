package tool

import (
	"fmt"
	"testing"
)

func TestParseDataUrl(t *testing.T) {
	dataUrl := "eyJwIjoiYnJjLTIwIiwib3AiOiJ0cmFuc2ZlciIsInRpY2siOiJPUlhDIiwiYW10IjoiMTAwMCJ9"
	fmt.Println(string(ParseDataUrl(dataUrl)))
}
