package cascadia

import "testing"

func requireParserError(t *testing.T, parse func() error) {
	t.Helper()
	if err := parse(); err == nil {
		t.Fatal("parser unexpectedly succeeded")
	}
}

func TestParserPrimitiveErrorAndEscapePaths(t *testing.T) {
	for name, p := range map[string]*parser{
		"escaped newline":   {s: "\\\n"},
		"escaped carriage":  {s: "\\\r"},
		"escaped form feed": {s: "\\\f"},
	} {
		t.Run(name, func(t *testing.T) {
			requireParserError(t, func() error { _, err := p.parseEscape(); return err })
		})
	}
	for name, input := range map[string]string{
		"hex CRLF":  "\\41\r\n",
		"hex space": "\\41 ",
		"hex tab":   "\\41\t",
		"hex LF":    "\\41\n",
		"hex FF":    "\\41\f",
	} {
		t.Run(name, func(t *testing.T) {
			if got, err := (&parser{s: input}).parseEscape(); err != nil || got != "A" {
				t.Fatalf("parseEscape() = %q, %v", got, err)
			}
		})
	}

	for name, input := range map[string]string{
		"too short":       "'",
		"raw newline":     "'a\n'",
		"dangling escape": "'\\",
	} {
		t.Run(name, func(t *testing.T) {
			requireParserError(t, func() error { _, err := (&parser{s: input}).parseString(); return err })
		})
	}
	for name, input := range map[string]string{
		"escaped CRLF": "'a\\\r\nb'",
		"escaped CR":   "'a\\\rb'",
		"escaped LF":   "'a\\\nb'",
		"escaped FF":   "'a\\\fb'",
	} {
		t.Run(name, func(t *testing.T) {
			if got, err := (&parser{s: input}).parseString(); err != nil || got != "ab" {
				t.Fatalf("parseString() = %q, %v", got, err)
			}
		})
	}

	requireParserError(t, func() error { _, err := (&parser{s: "x"}).parseRegex(); return err })
	requireParserError(t, func() error { _, err := (&parser{s: "abc"}).parseRegex(); return err })
	if (&parser{s: "x"}).consumeParenthesis() {
		t.Fatal("consumeParenthesis accepted a non-parenthesis")
	}
}

func TestAttributeParserErrorPaths(t *testing.T) {
	for _, input := range []string{
		"[foo", "[foo xy]", "[foo= ", "[foo=   ", "[foo=bar", "[foo%=bar]",
	} {
		t.Run(input, func(t *testing.T) {
			requireParserError(t, func() error {
				_, err := (&parser{s: input}).parseAttributeSelector()
				return err
			})
		})
	}
}

func TestNthParserStateMachinePaths(t *testing.T) {
	valid := map[string][2]int{
		"+n+2": {1, 2},
		"-n-2": {-1, -2},
		"3)":   {0, 3},
	}
	for input, want := range valid {
		t.Run(input, func(t *testing.T) {
			a, b, err := (&parser{s: input}).parseNth()
			if err != nil || a != want[0] || b != want[1] {
				t.Fatalf("parseNth() = %d, %d, %v", a, b, err)
			}
		})
	}

	tooLarge := "999999999999999999999999999999999999999"
	for _, input := range []string{
		"+", "+x", "-", "-x", "2", "2n", "2n+", "2n-", "other", "o\\\n",
		tooLarge + "n", "-" + tooLarge + "n",
	} {
		t.Run(input, func(t *testing.T) {
			requireParserError(t, func() error {
				_, _, err := (&parser{s: input}).parseNth()
				return err
			})
		})
	}
}

func TestParserComponentEntryErrors(t *testing.T) {
	tests := []struct {
		name string
		run  func() error
	}{
		{"id EOF", func() error { _, err := (&parser{}).parseIDSelector(); return err }},
		{"id prefix", func() error { _, err := (&parser{s: ".x"}).parseIDSelector(); return err }},
		{"class EOF", func() error { _, err := (&parser{}).parseClassSelector(); return err }},
		{"class prefix", func() error { _, err := (&parser{s: "#x"}).parseClassSelector(); return err }},
		{"attribute EOF", func() error { _, err := (&parser{}).parseAttributeSelector(); return err }},
		{"attribute prefix", func() error { _, err := (&parser{s: "x"}).parseAttributeSelector(); return err }},
		{"pseudo EOF", func() error { _, _, err := (&parser{}).parsePseudoclassSelector(); return err }},
		{"pseudo prefix", func() error { _, _, err := (&parser{s: "x"}).parsePseudoclassSelector(); return err }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) { requireParserError(t, test.run) })
	}
}

func TestParserPseudoclassErrorPaths(t *testing.T) {
	invalid := []string{
		":not", ":not(", ":not(div", ":not(])",
		":has", ":has(", ":has(div", ":has(])",
		":haschild", ":haschild(", ":haschild(div",
		":contains", ":contains(", ":contains(\"unterminated)", ":contains(value", ":contains(value x)",
		":containsown", ":containsown(",
		":matches", ":matches(", ":matches([)", ":matches(foo", ":matches(foo]x)",
		":matchesown", ":matchesown(",
		":nth-child", ":nth-child(", ":nth-child(bad)", ":nth-child(2n+)", ":nth-child(2n+1",
		":nth-last-child", ":nth-of-type", ":nth-last-of-type",
		":lang", ":lang(", ":lang(1)", ":lang(en", ":lang(en x)",
		"::unknown", ":unknown",
	}
	for _, input := range invalid {
		t.Run(input, func(t *testing.T) {
			requireParserError(t, func() error {
				_, _, err := (&parser{s: input}).parsePseudoclassSelector()
				return err
			})
		})
	}
}

func TestParserDepthAndRelativeGroupErrors(t *testing.T) {
	requireParserError(t, func() error {
		_, err := (&parser{s: "div", depth: maxParseDepth + 1}).parseSelectorGroup()
		return err
	})
	requireParserError(t, func() error {
		_, err := (&parser{s: "div", depth: maxParseDepth + 1}).parseRelativeSelectorGroup()
		return err
	})
	requireParserError(t, func() error {
		_, err := (&parser{s: "div,"}).parseRelativeSelectorGroup()
		return err
	})
	group, err := (&parser{s: "div,span"}).parseRelativeSelectorGroup()
	if err != nil || len(group) != 2 {
		t.Fatalf("relative group = %#v, %v", group, err)
	}
}
