package melt

import (
	"fmt"
	"go/format"
	"maps"
	"os"
	"path/filepath"
	"strings"
)

type templates map[string]map[string]string // template -> variables
type calls map[string]map[string][]string   // parent template -> child template -> variables

type generationData struct {
	imports map[string]string
	templates
	calls
}

func Generate(file string, extentions []string, paths ...string) {

	m := New(WithGeneration(file), WithPrintRenderOutput(true))

	for _, path := range paths {

		filepath.Walk(path, func(path string, info os.FileInfo, _ error) error {
			if !info.IsDir() {
				path := formatPath(path)

				if !hasExtention(path, extentions) {
					return nil
				}

				name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
				root := name == "root" || strings.HasPrefix(name, "root_") || strings.HasPrefix(name, "root-")

				if hasExtention(path, []string{".scss", ".css"}) {
					return nil
				}

				if !root {
					m.GetComponent(path, true)
				} else {
					m.GetRoot(path, true)
				}
			}

			return nil
		})
	}

	m.generate()
}

func SplitIgnoreType(s string, sep rune) []string {
	var result []string
	var part string

	for i, r := range s {

		if strings.Index(part, "@type(") == 0 {
			part += string(r)

			if r == ')' && i+1 <= len(s)-1 && s[i+1] == byte(sep) {
				result = append(result, part)
				part = ""
			}

			continue
		}

		if r == sep {
			result = append(result, part)
			part = ""
			continue
		}

		part += string(r)
	}

	result = append(result, part)

	return result
}

func (f *Furnace) generate() {
	path := formatPath(f.GenerationOutputFile)
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))

	components := make(map[string]string)
	dataTypes := make(map[string]string)
	writeFuncs := make(map[string]string)

	imports := "\n"
	importMap := map[string]string{
		"github.com/0-mqix/melt": "",
		"io":                     "",
		"net/http":               "",
	}

	componentNames := make([]string, 0)
	rootNames := make([]string, 0)

	for path, component := range f.Components {
		components[path] = component.Name
		componentNames = append(componentNames, component.Name)

		maps.Copy(component.generationData.imports, importMap)

		for name, path := range component.generationData.imports {
			if path == "" {
				imports += fmt.Sprintf("\"%s\"\n", name)
				continue
			}
			imports += fmt.Sprintf("%s \"%s\"\n", name, path)
		}

		for k, v := range createTypes(component.Name, path, component.generationData) {
			dataTypes[k] = v
		}

		for k, v := range createWriteFuncs(component.Name, path, component.generationData) {
			writeFuncs[k] = v
		}
	}

	for path := range f.Roots {
		rootNames = append(rootNames, ComponentName(path))
	}

	code := fmt.Sprintf(`// Code generated by melt; DO NOT EDIT.
	
	package %s

	import (%s)
	
	`, name, imports)

	code += "// root instances\n"
	code += createVariables("*melt.Root", rootNames) + "\n\n"

	code += "// component instances\n"
	code += createVariables("*melt.Component", componentNames) + "\n\n"

	code += createLoad(f.Components, f.Roots) + "\n\n"

	for name, declaration := range writeFuncs {

		if funcDeclaration, exists := dataTypes[name]; exists {
			code += funcDeclaration + "\n\n"
		}

		code += declaration + "\n\n"
	}

	bytes, err := format.Source([]byte(code))

	if err == nil {
		writeOutputFile(f.GenerationOutputFile, bytes)
	} else {
		fmt.Println("[MELT] generation error:", err)
	}
}

func extractGenerationData(templateString string) *generationData {
	matches := TemplateFunctionRegex.FindAllStringSubmatch(templateString, -1)
	blocks := []string{""}

	templates := templates{"": make(map[string]string)}
	imports := make(map[string]string)
	calls := make(calls)

	for _, match := range matches {
		tokens := SplitIgnoreType(match[1], ' ')

		for i, token := range tokens {

			template := blocks[len(blocks)-1]

			if strings.Index(token, ".") == 0 {
				token := "." + strings.Split(token, ".")[1]

				if template == "range" || template == "with" {
					continue
				}

				dataType := "any"

				if i+1 <= len(tokens)-1 {
					result := TypeRegex.FindStringSubmatch(tokens[i+1])

					if len(result) != 0 {
						dataType = result[1]
						source := strings.Split(strings.TrimSpace(result[2]), " ")
						name := source[0]

						if len(result) == 3 && name != "" {
							if len(source) > 1 {
								imports[name] = source[1]
							} else {
								imports[name] = ""
							}
						}
					}
				}

				if _, ok := templates[template]; !ok {
					templates[template] = map[string]string{token: dataType}
				} else {
					templates[template][token] = dataType
				}
			}
		}

		switch tokens[0] {
		case "define":
			if len(tokens) > 1 {
				template := tokens[1]
				blocks = append(blocks, template)

				if _, ok := templates[template]; !ok {
					templates[template] = make(map[string]string)
				}
			}

		case "block", "template":

			if len(tokens) < 3 {
				continue
			}

			block := blocks[len(blocks)-1]
			template := tokens[1]
			variable := tokens[2]

			if _, ok := calls[block]; !ok {
				calls[block] = map[string][]string{variable: {template}}
			} else {
				calls[block][variable] = append(calls[block][variable], template)
			}

			if _, ok := templates[template]; !ok {
				templates[template] = make(map[string]string)
			}

			blocks = append(blocks, template)

		case "with", "range":
			blocks = append(blocks, tokens[0])

		case "end":
			if len(blocks) <= 1 {
				break
			}
			blocks = blocks[:len(blocks)-1]
		}
	}

	return &generationData{
		templates: templates,
		imports:   imports,
		calls:     calls,
	}
}

func createWriteFuncs(name, path string, data *generationData) map[string]string {
	result := make(map[string]string)

	for template := range data.templates {
		if template == "" {
			continue
		} else {
			result[name+ComponentName(strings.Trim(template, "\""))] = createWriteTemplateFunc(name, path, template)
		}
	}

	result[name] = createWriteFunc(name, path)

	return result
}

func createTypes(name, path string, data *generationData) map[string]string {
	result := make(map[string]string)

	for template, variables := range data.templates {
		fields := ""

		for v, variableType := range variables {

			if v == "." {
				continue
			}

			types := data.calls[template][v]
			variableName := v[1:]

			if variableType != "any" {
				fields += fmt.Sprintf("%s %s\n", variableName, variableType)
			} else if len(types) == 1 {
				fields += fmt.Sprintf("%s %s\n", variableName, name+ComponentName(strings.Trim(types[0], "\""))+"Data")
			} else if len(types) > 1 {
				fields += variableName + " " + createMergedStruct(name, types, data)
			} else {
				fields += fmt.Sprintf("%s any\n", variableName)
			}
		}

		componentName := name + ComponentName(strings.Trim(template, "\""))
		typeName := componentName + "Data"

		if template == "" {
			typeName = name + "Data"
		}

		result[componentName] = fmt.Sprintf("type %s struct { %s }", typeName, fields)
	}

	if len(data.templates) == 0 {
		result[name] = fmt.Sprintf("type %sData struct{}", name)
	}

	return result
}

func createMergedStruct(name string, templates []string, data *generationData) string {
	str := "struct {\n"

	fields := make(map[string]string)

	for _, template := range templates {
		for v := range data.templates[template] {

			if v == "." {
				continue
			}

			types := data.calls[template][v]

			if len(types) == 1 {
				fields[v] = name + ComponentName(strings.Trim(types[0], "\"")) + "Data"
			} else if len(types) > 1 {
				fields[v] = createMergedStruct(name, types, data)
			} else {
				fields[v] = "any"
			}
		}
	}

	for n, t := range fields {
		str += fmt.Sprintf("%s %s\n", n[1:], t)
	}

	return str + "}\n"
}

func createVariables(typeName string, names []string) string {
	str := "var (\n"

	for _, name := range names {
		str += fmt.Sprintf("%s %s\n", name, typeName)
	}

	return str + ")"
}

func createLoad(components map[string]*Component, roots map[string]*Root) string {
	fields := ""
	getters := ""
	setters := ""

	for path := range roots {
		getters += fmt.Sprintf("%s = furnace.MustGetRoot(\"%s\")\n", ComponentName(path), path)
	}

	getters += "\n"

	for path, c := range components {
		getters += fmt.Sprintf("%s = furnace.MustGetComponent(\"%s\")\n", c.Name, path)

		if !c.Global {
			continue
		}

		fields += fmt.Sprintf("%s func(r *http.Request, arguments map[string]any) *%sData\n", c.Name, c.Name)
		setters += fmt.Sprintf(`
		if handlers.%s != nil {
			handler = func(r *http.Request, arguments map[string]any) any { return handlers.%s(r, arguments) } 
		} else {
			handler = func(r *http.Request, _ map[string]any) any { return &%sData{} } 
		}

		globalHandlers["%s"] = handler
 		`, c.Name, c.Name, c.Name, path)
	}

	var handler string

	if setters != "" {
		handler = "var handler melt.GlobalHandler\n"
	}

	return fmt.Sprintf(`
	type GlobalHandlers struct {
		%s}	
	
	  func Load(furnace *melt.Furnace, handlers GlobalHandlers) {
		%s
		globalHandlers := make(map[string]melt.GlobalHandler)
		%s
		%s

		furnace.SetGlobalHandlers(globalHandlers)
	  }
`, fields, getters, handler, setters)

}

func createWriteFunc(name, path string) string {
	return fmt.Sprintf(`
		// generated write function for component
		//
		//	path: "%s"
		func Write%s(w io.Writer, r *http.Request, data *%sData, globalOptions ...melt.GlobalOption) error {
			return %s.Write(w, r, data, globalOptions...)
		}
`, path, name, name, name)
}

func createWriteTemplateFunc(name, path, template string) string {

	source := name
	name += ComponentName(strings.Trim(template, "\""))

	return fmt.Sprintf(`
		// generated write function for a template in a component
		//
		//	path: "%s"
		//	template: %s
		func Write%s(w io.Writer, data *%sData) error {
			return %s.WriteTemplate(w, %s, data)
		}
	`, path, template, name, name, source, template)
}
