package melt

import (
	"fmt"
	"html/template"
	"net/http"
	text "text/template"
)

var Functions = template.FuncMap{
	"comment": commentFunction,
	"global":  globalFunction,
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

func globalFunction(name string, r *http.Request) template.HTML {
	globals, ok := r.Context().Value(GLOBALS_CONTEXT_KEY).(map[string]string)

	if !ok {
		fmt.Println("[MELT] gobals context value missing", name)
		return template.HTML("")
	}

	html, exists := globals[name]

	if !exists {
		fmt.Println("[MELT] no globals result html found for", name)
		return template.HTML("")
	}

	return template.HTML(html)
}
