package melt

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"strings"

	"golang.org/x/net/html"
)

func SplitIgnoreString(s string, sep rune) []string {
	var result []string
	var part string
	var in bool

	for _, c := range s {

		if c == '"' {
			in = !in
			part += string(c)
			continue
		}

		if !in && c == sep {
			result = append(result, part)
			part = ""
			continue
		}

		part += string(c)
	}

	result = append(result, part)

	return result
}

type Argument struct {
	Value string
	Type  int
}

const (
	ArgumentTypeVariable = iota
	ArgumentTypeConstant
)

func (f *Furnace) pasteComponent(n *html.Node, component *Component, attributes string) {
	buffer := bytes.NewBufferString("")
	declarations := ""

	arguments := make(map[string]Argument)

	for _, attribute := range SplitIgnoreString(attributes, ' ') {
		attribute = strings.TrimSpace(attribute)

		if len(attribute) == 0 {
			continue
		}

		pair := SplitIgnoreString(attribute, '=')
		if len(pair) != 2 {
			continue
		}

		name := pair[0]
		value := pair[1]

		switch name[0] {

		case '.', '$':

			if !(value[0] == '.' || value[1] == '$') {
				value = strings.Trim(value, "\"")

				value = TemplateFunctionRegex.ReplaceAllStringFunc(value, func(s string) string {
					return "{{" + hex.EncodeToString([]byte(s[2:len(s)-2])) + "}}"
				})

				arguments[name] = Argument{Value: value, Type: ArgumentTypeConstant}
				continue
			}

			id := f.lastArgumentId.Load()
			argument := fmt.Sprintf("$arg%d", id)
			arguments[name] = Argument{Value: argument, Type: ArgumentTypeVariable}

			declaration := fmt.Sprintf("%s := %s", argument, value)
			encoded := hex.EncodeToString([]byte(declaration))
			declarations += fmt.Sprintf("{{%s}}", encoded)

			f.lastArgumentId.Add(1)

		case '&':
			// fmt.Println(name, value)
		}

	}

	for _, part := range component.Nodes {
		html.Render(buffer, part)
	}

	argumented := TemplateFunctionRegex.ReplaceAllStringFunc(buffer.String(), func(s string) string {
		b, _ := hex.DecodeString(s[2 : len(s)-2])

		content := strings.TrimSpace(string(b))

		argument, ok := arguments[content]

		if ok && argument.Type == ArgumentTypeConstant {
			fmt.Println("argument", content, argument.Value)
			return argument.Value
		}

		content = replaceTemplateVariables(content, arguments)
		content = hex.EncodeToString([]byte(content))

		return "{{" + content + "}}"
	})

	//prepend the declarations to the arugmented component html
	buffer = bytes.NewBufferString(declarations + argumented)

	nodes := make([]*html.Node, 0)
	element, _ := html.Parse(buffer)
	extractFromFakeBody(element, &nodes)

	for _, part := range nodes {
		part.Parent.RemoveChild(part)
		n.AppendChild(part)
	}
}

func replaceTemplateVariables(s string, arguments map[string]Argument) string {
Start:
	begin := 0
	selecting := false
	last := rune(0)

	for i, c := range s {
		if !selecting && (c == '.' || c == '$') {

			if i > 0 && !(last == ' ' || last == ',') {
				goto Continue
			}

			selecting = true
			begin = i
		}

		if !selecting {
			goto Continue
		}

		if len(s)-1 == i {
			name := s[begin:]
			replacement, ok := arguments[name]
			if !ok {
				goto Continue
			}
			return s[:begin] + replacement.Value

		} else {
			end := i
			next := s[i+1]

			if next == '.' || next == ',' || next == ' ' {
				selecting = false
				name := s[begin : end+1]
				replacement, ok := arguments[name]

				if !ok {
					goto Continue
				}

				if end-begin-1 == len(s) {
					return replacement.Value
				}

				s = s[:begin] + replacement.Value + s[end+1:]
				goto Start
			}
		}

	Continue:
		last = c
	}

	return s
}

func textNode(data string) *html.Node {
	return &html.Node{Type: html.TextNode, Data: data}
}

func (f *Furnace) useComponents(n *html.Node, imports map[string]*Import, styles map[string]string) {
	if n.Type == html.ElementNode && strings.Index(n.Data, "melt-") == 0 {

		data := strings.Split(n.Data, "-")

		result, _ := encoder.DecodeString(data[1])
		name := string(result)

		result, _ = encoder.DecodeString(data[2])
		attributes := string(result)

		source, ok := imports[name]

		var component *Component

		if !ok {
			goto Next
		}

		component, ok = f.GetComponent(source.Path)

		if !ok {
			goto Next
		}

		styles[source.Path] = component.Style
		f.pasteComponent(n, component, attributes)

	}
Next:
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		f.useComponents(c, imports, styles)
	}
}
