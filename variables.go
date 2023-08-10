package melt

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

func prefixTemplateVariables(s, target, prefix string) string {
	lenTarget := len(target)
	lenPrefix := len(prefix)

Start:
	begin := 0
	selecting := false
	last := rune(0)

	for i, c := range s {
		if !selecting && (c == '.' || c == '$' || c == '%') {

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
			name := s[begin : i+1]

			if name != target {
				goto Continue
			}

			return s[:begin] + prefix + s[begin+lenTarget:]

		} else {
			name := s[begin : i+1]
			done := false

			if len(s) >= begin+lenPrefix {
				done = s[begin:begin+lenPrefix] == prefix
			}

			if name == target && !done {
				offset := -0

				if target[0] == '.' {
					offset = -1
				}

				s = s[:begin] + prefix + s[begin+lenTarget+offset:]
				goto Start
			}

			next := s[i+1]

			if next == '.' || next == ',' || next == ' ' {
				selecting = false
			}
		}

	Continue:
		last = c
	}

	return s
}
