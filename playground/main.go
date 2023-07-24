package main

import (
	"bytes"
	"fmt"

	"github.com/0-mqix/melt"
)

func main() {
	m := melt.New()
	index, ok := m.GetComponent("index.html")

	if !ok {
		panic("meltdown")
	}

	data := struct {
		User struct {
			Name string
		}
		Yeet int
	}{Yeet: 1, User: struct{ Name string }{Name: "max "}}
	htmlBuffer := bytes.NewBufferString("")

	err := index.Template.Execute(htmlBuffer, data)

	if err != nil {
		panic(err)
	}

	fmt.Printf("---html:---\n%s\n", htmlBuffer.String())
	fmt.Printf("---style:---\n%s\n", index.Style)
	fmt.Printf("---nodes:---\n")
	for _, n := range index.Nodes {
		fmt.Printf("%s\n", n.Data)
	}

}
