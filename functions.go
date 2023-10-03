package melt

import (
	"fmt"
	"html/template"
	text "text/template"
	"time"
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

func globalFunction(name string, vars ...any) template.HTML {
	args := make(map[string]any)
	for i := 0; i < len(vars)-1; i += 2 {
		args[vars[i].(string)] = vars[i+1]
	}
	fmt.Println(args)

	result := make(chan template.HTML)

	go func() {
		time.Sleep(time.Second * 1)
		result <- template.HTML("delayed")
	}()

	return <-result
}
