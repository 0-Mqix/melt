package melt

import (
	"testing"
)

func TestReplaceTemplateVariables(t *testing.T) {

	type Case struct {
		Input     string
		Expected  string
		Arguments map[string]string
	}

	tests := []Case{
		{
			Input:     ".Foo",
			Expected:  "$arg0",
			Arguments: map[string]string{".Foo": "$arg0"},
		},
		{
			Input:     ".Foo .Foo .Foo",
			Expected:  "$arg0 $arg0 $arg0",
			Arguments: map[string]string{".Foo": "$arg0"},
		},
		{
			Input:     " . ",
			Expected:  " $arg0 ",
			Arguments: map[string]string{".": "$arg0"},
		},
		{
			Input:     " . .Foo .Bar ",
			Expected:  " $arg0 .Foo .Bar ",
			Arguments: map[string]string{".": "$arg0"},
		},
		{
			Input:     "   .Foo  ",
			Expected:  "   $arg0  ",
			Arguments: map[string]string{".Foo": "$arg0"},
		},
		{
			Input:     "range $foo, $bar := $value",
			Expected:  "range $foo, $bar := $arg0",
			Arguments: map[string]string{"$value": "$arg0"},
		},
		{
			Input:     ".Foo := .Bar",
			Expected:  ".Foo := $arg0",
			Arguments: map[string]string{".Bar": "$arg0"},
		},
		{
			Input:     "  .Foo :=  .LongWithWhiteSpace $bar  ",
			Expected:  "  .Foo :=  $arg0 $arg1  ",
			Arguments: map[string]string{".LongWithWhiteSpace": "$arg0", "$bar": "$arg1"},
		},
		{
			Input:     ".Foo := .Foo.Id",
			Expected:  "$arg0 := $arg0.Id",
			Arguments: map[string]string{".Foo": "$arg0"},
		},
		{
			Input:     " .Foo.Id .Bar ",
			Expected:  " $arg0.Id $arg1 ",
			Arguments: map[string]string{".Foo": "$arg0", ".Bar": "$arg1"},
		},
		{
			Input:     ".Foo.Id = .Bar",
			Expected:  "$arg0.Id = $arg1",
			Arguments: map[string]string{".Foo": "$arg0", ".Bar": "$arg1"},
		},
		{
			Input:     "$Test.Foo, .Foo",
			Expected:  "$Test.Foo, $arg0",
			Arguments: map[string]string{".Foo": "$arg0"},
		},
		{
			Input:     ".Foo.Id .Bar .Monke.Tree.Id .Tree, $melt $jungle .Jungle.Tree.Leave",
			Expected:  "$arg0.Id .Bar $arg1.Tree.Id $arg5, $arg3 $jungle $arg4.Tree.Leave",
			Arguments: map[string]string{".Foo": "$arg0", ".Monke": "$arg1", "$melt": "$arg3", ".Jungle": "$arg4", ".Tree": "$arg5"},
		},
	}

	for _, c := range tests {

		arguments := make(map[string]Argument)
		for k, v := range c.Arguments {
			arguments[k] = Argument{Value: v, Type: ArgumentTypeVariable}
		}

		result := replaceTemplateVariables(c.Input, arguments)

		if c.Expected != result {
			t.Fatalf("failed case:\n Input:\t\t%s\n Expected:\t%s\n Output:\t%s\n Arguments: %v\n", c.Input, c.Expected, result, c.Arguments)
		}
	}
}

func TestPrefixTemplateVariables(t *testing.T) {

	type Case struct {
		Input    string
		Expected string
		Target   string
		Prefix   string
	}

	tests := []Case{
		{
			Input:    ".Foo",
			Expected: "$root.Foo",
			Target:   ".",
			Prefix:   "$root",
		},
		{
			Input:    ".",
			Expected: "$root",
			Target:   ".",
			Prefix:   "$root",
		},
		{
			Input:    "$bar",
			Expected: "$Index_bar",
			Target:   "$",
			Prefix:   "$Index_",
		},
		{
			Input:    "{{ range $x $y := .Coordinates }}",
			Expected: "{{ range $Index_x $Index_y := .Coordinates }}",
			Target:   "$",
			Prefix:   "$Index_",
		},
		{
			Input:    "{{ range $x $y $z := .Coordinates }}",
			Expected: "{{ range $Index_x $Index_y $Index_z := .Coordinates }}",
			Target:   "$",
			Prefix:   "$Index_",
		},
		{
			Input:    "{{ range $x $y $z := .Coordinates }}",
			Expected: "{{ range $x $y $z := $root.Coordinates }}",
			Target:   ".",
			Prefix:   "$root",
		},
		{
			Input:    "{{ %test }}",
			Expected: "{{ $Index_test }}",
			Target:   "%",
			Prefix:   "$Index_",
		},
		{
			Input:    "{{ %test }}",
			Expected: "{{ $Index_test }}",
			Target:   "%",
			Prefix:   "$Index_",
		},
		{
			Input:    "%x",
			Expected: "$Index_x",
			Target:   "%",
			Prefix:   "$Index_",
		},
		{
			Input:    "%x",
			Expected: "$Index_x",
			Target:   "%",
			Prefix:   "$Index_",
		},
		{
			Input:    "{{ %x }}",
			Expected: "{{ $Index_x }}",
			Target:   "%",
			Prefix:   "$Index_",
		},
		{
			Input:    "{{ %test %x %y }}",
			Expected: "{{ $Index_test $Index_x $Index_y }}",
			Target:   "%",
			Prefix:   "$Index_",
		},
	}

	for _, c := range tests {

		result := prefixTemplateVariables(c.Input, c.Target, c.Prefix)

		if c.Expected != result {
			t.Fatalf("failed case:\n Input:\t\t%s\n Expected:\t%s\n Output:\t%s\n", c.Input, c.Expected, result)
		}
	}
}
