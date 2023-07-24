package melt

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"strings"

	"golang.org/x/net/html"
)

func (f *Furnace) pasteComponent(n *html.Node, component *Component, attributes string) {
	buffer := bytes.NewBufferString("")
	arguments := make(map[string]string)
	declarations := ""

	for _, attribute := range strings.Split(attributes, " ") {
		attribute = strings.TrimSpace(attribute)

		if len(attribute) == 0 {
			continue
		}

		pair := strings.Split(attribute, "=")
		if len(pair) != 2 {
			continue
		}

		name := pair[0]
		value := pair[1]

		id := f.lastArgumentId.Load()
		argument := fmt.Sprintf("$arg%d", id)
		arguments[name] = argument

		declaration := fmt.Sprintf("%s := %s", argument, value)
		encoded := hex.EncodeToString([]byte(declaration))
		declarations += fmt.Sprintf("{{%s}}", encoded)

		f.lastArgumentId.Add(1)
	}

	for _, part := range component.Nodes {
		html.Render(buffer, part)
	}

	argumented := TemplateFunctionRegex.ReplaceAllStringFunc(buffer.String(), func(s string) string {
		b, _ := hex.DecodeString(s[2 : len(s)-2])
		content := string(b)

		fmt.Println("original:", content)
		content = replaceTemplateVariables(content, arguments)
		fmt.Println("replaced:", content)
		content = hex.EncodeToString([]byte(content))
		fmt.Println("hex:", content)

		return "{{" + content + "}}"
	})

	//prepend the declarations to the arugmented component html
	buffer = bytes.NewBufferString(declarations + argumented)

	nodes := make([]*html.Node, 0)
	element, _ := html.Parse(buffer)
	extractFromFakeBody(element, &nodes)

	for _, part := range nodes {
		part.Parent.RemoveChild(part)
		n.Parent.InsertBefore(part, n)
	}
}

func replaceTemplateVariables(s string, arguments map[string]string) string {
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
			return s[:begin] + replacement

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
					return replacement
				}

				s = s[:begin] + replacement + s[end+1:]
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
	delete := false
	if n.Type == html.NodeType(html.TextNode) {
		matches := transformedComponentRegex.FindAllStringSubmatch(n.Data, -1)

		if len(matches) == 0 {
			goto Skip
		}

		delete = true

		type Paste struct {
			Component  *Component
			Attributes string
			Index      int
			Raw        string
			Name       string
		}

		paste := make([]*Paste, 0)
		position := 0

		for _, result := range matches {
			raw, err := hex.DecodeString(result[2])

			if err != nil {
				fmt.Println("[MELT]", err)
				continue
			}

			attributes := string(raw)
			name := result[1]
			source, ok := imports[name]

			var component *Component

			if !ok {
				goto Paste
			}

			fmt.Println("\nimport: ", source.Path)
			fmt.Println(name, attributes)
			component, ok = f.GetComponent(source.Path)

			if !ok {
				goto Paste
			}

			styles[source.Path] = component.Style

		Paste:
			i := len(paste) - 1

			if i > -1 {
				offset := position + len(paste[i].Raw)
				position = offset + strings.Index(n.Data[offset:], result[0])
			} else {
				position = strings.Index(n.Data, result[0])
			}

			fmt.Println(position, strings.Index(n.Data, result[0]))

			paste = append(paste, &Paste{
				Attributes: attributes,
				Component:  component,
				Index:      position,
				Name:       name,
				Raw:        result[0],
			})
		}

		lenght := len(n.Data)

		for i, p := range paste {
			fmt.Println(i, p.Index, p.Raw, p.Name, p.Attributes)

			p.Index -= lenght - len(n.Data)
			before := n.Data[:p.Index]
			n.Data = n.Data[p.Index+len(p.Raw):]

			n.Parent.InsertBefore(textNode(before), n)

			if p.Component != nil {
				f.pasteComponent(n, p.Component, p.Attributes)
			}
		}

	}

Skip:
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		f.useComponents(c, imports, styles)
	}

	if delete {
	}
}
