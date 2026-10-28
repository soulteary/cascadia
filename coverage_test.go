package cascadia

import (
	"reflect"
	"testing"

	"golang.org/x/net/html"
)

type coverageMatcher struct {
	match func(*html.Node) bool
}

func (m coverageMatcher) Match(n *html.Node) bool { return m.match(n) }

func TestQueryEvaluatorCombinators(t *testing.T) {
	root := MustParseHTML(`
		<main>
			<div class="parent"><p class="first"></p>text<!-- comment --><span class="target"></span><em></em></div>
			<section><span class="target"></span></section>
		</main>`)
	doc := NewIndexedDocument(root)
	tests := map[string]int{
		"div span":                   1,
		"div > span":                 1,
		"p + span":                   1,
		"p ~ em":                     1,
		"section > span.target":      1,
		"div:has(+ section)":         1,
		"div:has(~ section)":         1,
		"div:has(> span.target)":     1,
		"main:has(span.target)":      1,
		"span:nth-last-child(2)":     1,
		"span:nth-last-of-type(1)":   2,
		"span:only-of-type":          2,
		"section span:not(.missing)": 1,
		"section span:not(.target)":  0,
	}
	for selector, want := range tests {
		t.Run(selector, func(t *testing.T) {
			matcher := mustParseGroup(selector)
			got := doc.QueryAll(matcher)
			baseline := QueryAll(root, matcher)
			if len(got) != want || !reflect.DeepEqual(got, baseline) {
				t.Fatalf("got %d matches, want %d", len(got), want)
			}
		})
	}
}

func TestQueryEvaluatorInternalFallbacks(t *testing.T) {
	root := MustParseHTML(`<main><div></div><span></span></main>`)
	doc := NewIndexedDocument(root)
	evaluator := newQueryEvaluator(doc)
	div := Query(root, mustParseGroup("div"))
	main := Query(root, mustParseGroup("main"))

	custom := coverageMatcher{match: func(n *html.Node) bool { return n == div }}
	if !evaluator.match(custom, div) {
		t.Fatal("custom Matcher fallback did not match")
	}
	if !evaluator.match(tagSelector{tag: "div"}, div) {
		t.Fatal("Sel matcher path did not match")
	}
	if !evaluator.matchSel(compoundSelector{}, div) || evaluator.matchSel(compoundSelector{}, root) {
		t.Fatal("empty compound selector element semantics changed")
	}
	if evaluator.matchSel(combinedSelector{}, div) {
		t.Fatal("combined selector with nil first selector matched")
	}
	if !evaluator.matchSel(combinedSelector{first: tagSelector{tag: "div"}}, div) {
		t.Fatal("zero-combinator selector did not match")
	}
	if evaluator.matchSel(combinedSelector{first: tagSelector{tag: "main"}, combinator: '>'}, div) {
		t.Fatal("combined selector with nil second selector matched")
	}
	if evaluator.matchSel(combinedSelector{
		first: tagSelector{tag: "main"}, combinator: '!', second: tagSelector{tag: "div"},
	}, div) {
		t.Fatal("unknown evaluator combinator matched")
	}

	plainEvaluator := newQueryEvaluator(nil)
	nth := nthPseudoClassSelector{a: 0, b: 1}
	if !plainEvaluator.matchSel(nth, div) {
		t.Fatal("nth selector fallback did not match")
	}
	only := onlyChildPseudoClassSelector{}
	if plainEvaluator.matchSel(only, div) {
		t.Fatal("only-child fallback unexpectedly matched")
	}
	if _, ok := plainEvaluator.siblingMetadata(div); ok {
		t.Fatal("nil document returned sibling metadata")
	}
	if _, ok := evaluator.siblingMetadata(root); ok {
		t.Fatal("unindexed root returned sibling metadata")
	}
	if evaluator.matchRelative(relativePseudoClassSelector{name: "has"}, &html.Node{Type: html.TextNode}) {
		t.Fatal("relative selector matched a text node")
	}
	if !evaluator.matchRelative(relativePseudoClassSelector{
		name: "haschild", rels: []relativeSelector{{sel: tagSelector{tag: "div"}}},
	}, main) {
		t.Fatal("haschild default combinator did not match")
	}
	if evaluator.matchRelativeSelector(div, ' ', tagSelector{tag: "missing"}) {
		t.Fatal("missing descendant unexpectedly matched")
	}
	nested := MustParseHTML(`<div><section><span></span></section></div>`)
	nestedDiv := Query(nested, mustParseGroup("div"))
	if !newQueryEvaluator(nil).matchRelativeSelector(nestedDiv, ' ', tagSelector{tag: "span"}) {
		t.Fatal("recursive descendant relative selector did not match")
	}
	relativeCandidate := relativeSelector{sel: compoundSelector{selectors: []Sel{
		tagSelector{tag: "span"}, attrSelector{key: "data-missing"},
	}}}
	if matched, handled := evaluator.matchRelativeFromIndex(main, ' ', &relativeCandidate); !handled || matched {
		t.Fatal("indexed non-matching relative candidate was not handled")
	}
}

func TestEvaluatorTraversalHelpers(t *testing.T) {
	root := MustParseHTML(`<main><p></p><p class="match"></p></main>`)
	matcher := mustParseGroup("p.match")
	evaluator := newQueryEvaluator(NewIndexedDocument(root))

	got := queryAllWithEvaluator(root, matcher, evaluator)
	want := QueryAll(root, matcher)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("queryAllWithEvaluator mismatch")
	}
	if first := queryWithEvaluator(root, matcher, evaluator); first != want[0] {
		t.Fatalf("queryWithEvaluator returned %p, want %p", first, want[0])
	}
	if first := queryWithEvaluator(root, mustParseGroup("aside"), evaluator); first != nil {
		t.Fatalf("queryWithEvaluator returned unexpected node %p", first)
	}
}

func TestIndexedDocumentCandidatePaths(t *testing.T) {
	root := MustParseHTML(`
		<main>
			<div id="one" class="item selected"></div>
			<span id="two" class="item"></span>
		</main>`)
	doc := NewIndexedDocument(root)

	for _, selector := range []string{
		"#one, #two", ".selected, span", "div.item, span.item",
		"main > #one", "[id]", "*:first-child",
	} {
		matcher := mustParseGroup(selector)
		got := doc.QueryAll(matcher)
		want := QueryAll(root, matcher)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("candidate path mismatch for %q", selector)
		}
	}

	if got := doc.Query(coverageMatcher{match: func(n *html.Node) bool {
		return n.Type == html.ElementNode && n.Data == "span"
	}}); got == nil || got.Data != "span" {
		t.Fatalf("custom Matcher Query fallback returned %v", got)
	}
	if got := doc.Query(mustParseGroup("#missing")); got != nil {
		t.Fatalf("missing indexed candidate returned %v", got)
	}

	if _, ok := doc.candidates(tagSelector{tag: "div"}); !ok {
		t.Fatal("single Sel candidate path fell back")
	}
	if _, ok := doc.candidates(SelectorGroup{tagSelector{tag: "div"}, attrSelector{key: "id"}}); ok {
		t.Fatal("unanchored selector group did not fall back")
	}
	if _, ok := doc.bestCandidates(combinedSelector{first: tagSelector{tag: "main"}}); !ok {
		t.Fatal("combined selector without second did not use first")
	}
	if _, ok := doc.bestCandidates(compoundSelector{selectors: []Sel{
		attrSelector{key: "id"}, tagSelector{tag: "div"}, idSelector{id: "one"}, classSelector{class: "item"},
	}}); !ok {
		t.Fatal("compound selector anchors were not found")
	}
	missingRelative := relativeSelector{sel: attrSelector{key: "data-missing"}}
	if matched, handled := newQueryEvaluator(doc).matchRelativeFromIndex(root, ' ', &missingRelative); handled || matched {
		t.Fatal("unanchored relative selector did not fall back")
	}
	for _, selector := range []Sel{
		combinedSelector{first: tagSelector{tag: "main"}},
		compoundSelector{selectors: []Sel{idSelector{id: "one"}, classSelector{class: "item"}, tagSelector{tag: "div"}}},
		attrSelector{key: "id"},
	} {
		_ = rightmostAnchors(selector)
	}
}

func TestCompatibilityHelpersAndErrorPaths(t *testing.T) {
	root := MustParseHTML(`<main><p></p><span></span></main>`)
	nodes := []*html.Node{Query(root, mustParseGroup("p")), Query(root, mustParseGroup("span"))}
	got := Filter(nodes, coverageMatcher{match: func(n *html.Node) bool { return n.Data == "span" }})
	if len(got) != 1 || got[0].Data != "span" {
		t.Fatalf("Filter returned %#v", got)
	}

	if (neverMatchSelector{}).Specificity() != (Specificity{}) {
		t.Fatal("never-match specificity changed")
	}
	if (combinedSelector{first: tagSelector{tag: "p"}}).PseudoElement() != "" {
		t.Fatal("combined selector without second returned a pseudo-element")
	}
	for name, run := range map[string]func(){
		"relative pseudo": func() {
			relativePseudoClassSelector{name: "invalid"}.Match(nodes[0])
		},
		"attribute operation": func() {
			attrSelector{operation: "invalid"}.Match(root)
		},
		"combined operation": func() {
			combinedSelector{first: tagSelector{tag: "main"}, combinator: '!'}.Match(root)
		},
	} {
		t.Run(name+" panic", func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("expected panic")
				}
			}()
			run()
		})
	}
	if attributeNotEqualMatch("id", "x", &html.Node{Type: html.TextNode}, false) {
		t.Fatal("attributeNotEqualMatch matched a text node")
	}
	if (combinedSelector{first: nil}).Match(root) {
		t.Fatal("combined selector with nil first matched")
	}
	if !(combinedSelector{first: tagSelector{tag: "p"}}).Match(nodes[0]) {
		t.Fatal("zero-combinator combined selector did not match")
	}
	if relativeMatch(nodes[0], '~', tagSelector{tag: "missing"}) {
		t.Fatal("general sibling relative selector unexpectedly matched")
	}

	detachedElement := &html.Node{Type: html.ElementNode, Data: "p"}
	detachedText := &html.Node{Type: html.TextNode}
	parent := &html.Node{Type: html.ElementNode, Data: "div"}
	parent.AppendChild(&html.Node{Type: html.ElementNode, Data: "span"})
	detachedElement.Parent = parent
	if nthChildMatch(2, 1, false, false, detachedText) ||
		nthChildMatch(2, 1, false, false, &html.Node{Type: html.ElementNode}) ||
		nthChildMatch(2, 1, false, false, detachedElement) ||
		!nthChildMatch(0, 1, false, false, parent.FirstChild) {
		t.Fatal("nth-child defensive paths changed")
	}
	if simpleNthChildMatch(1, false, &html.Node{Type: html.ElementNode}) ||
		simpleNthChildMatch(3, false, detachedElement) ||
		simpleNthLastChildMatch(1, false, &html.Node{Type: html.ElementNode}) ||
		simpleNthLastChildMatch(3, false, detachedElement) {
		t.Fatal("simple nth-child defensive paths changed")
	}
	if (onlyChildPseudoClassSelector{}).Match(&html.Node{Type: html.ElementNode}) {
		t.Fatal("detached element matched :only-child")
	}
	if (rootPseudoClassSelector{}).Match(&html.Node{Type: html.ElementNode}) {
		t.Fatal("detached element matched :root")
	}
	relative := relativePseudoClassSelector{
		name: "has",
		rels: []relativeSelector{
			{sel: tagSelector{tag: "p"}},
			{sel: idSelector{id: "specific"}},
		},
	}
	if relative.Specificity() != (Specificity{1, 0, 0}) {
		t.Fatalf("unexpected relative-selector specificity: %v", relative.Specificity())
	}
	if relative.PseudoElement() != "" {
		t.Fatal("relative selector returned a pseudo-element")
	}
	if relativeMatch(root, '!', tagSelector{tag: "p"}) {
		t.Fatal("unknown relative combinator matched")
	}

	for _, selector := range []string{
		"div\\", `div[title="unterminated]`, "div:nth-child(", "div:has(",
		"#", ".", "div,,span", "div[foo=]",
	} {
		if _, err := ParseGroup(selector); err == nil {
			t.Errorf("ParseGroup(%q) unexpectedly succeeded", selector)
		}
	}
	if _, err := Compile("div["); err == nil {
		t.Fatal("Compile unexpectedly accepted invalid selector")
	}
	for name, parse := range map[string]func() error{
		"Parse leftovers":                  func() error { _, err := Parse("div,span"); return err },
		"ParseWithPseudoElement error":     func() error { _, err := ParseWithPseudoElement("div["); return err },
		"ParseWithPseudoElement leftovers": func() error { _, err := ParseWithPseudoElement("div,span"); return err },
		"ParseGroup leftovers":             func() error { _, err := ParseGroup("div)"); return err },
		"two pseudo-elements":              func() error { _, err := ParseWithPseudoElement("div::before::after"); return err },
		"pseudo-element not last":          func() error { _, err := ParseWithPseudoElement("div::before.foo"); return err },
	} {
		t.Run(name, func(t *testing.T) { requireParserError(t, parse) })
	}
	t.Run("MustCompile panic", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Fatal("MustCompile did not panic for an invalid selector")
			}
		}()
		MustCompile("div[")
	})
}
