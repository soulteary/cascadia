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

	custom := coverageMatcher{match: func(n *html.Node) bool { return n == div }}
	if !evaluator.match(custom, div) {
		t.Fatal("custom Matcher fallback did not match")
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
	t.Run("MustCompile panic", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Fatal("MustCompile did not panic for an invalid selector")
			}
		}()
		MustCompile("div[")
	})
}
