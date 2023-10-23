//go:build ignore

package main

import (
	"github.com/0-mqix/melt"
)

func main() {
	melt.Generate("./templates/templates.go", []string{".html"}, "./templates")
}
