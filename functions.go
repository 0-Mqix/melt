package melt

import (
	"html/template"
	text "text/template"
)

var Functions = template.FuncMap{
	"comment": commentFunction,
}

var RootFunctions = text.FuncMap{
	"html": htmlFunction,
}

func commentFunction(s string) template.HTML {
	return template.HTML("<!--" + s + "-->")
}

func htmlFunction(s string) template.HTML {
	return template.HTML(s)
}
