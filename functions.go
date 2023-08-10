package melt

import "html/template"

var Functions = template.FuncMap{
	"for":     forFunction,
	"comment": commentFunction,
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
