package melt

import (
	"html/template"
	text "text/template"
)

var Functions = template.FuncMap{
	"for":     forFunction,
	"comment": commentFunction,
}

var RootFunctions = text.FuncMap{
	"html": htmlFunction,
}

func forFunction(from, to int) <-chan int {
	ch := make(chan int)
	go func() {
		for i := from; i <= to; i++ {
			ch <- i
		}
		close(ch)
	}()
	return ch
}

func commentFunction(s string) template.HTML {
	return template.HTML("<!--" + s + "-->")
}

func htmlFunction(s string) template.HTML {
	return template.HTML(s)
}
