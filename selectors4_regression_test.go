package cascadia

import (
	"reflect"
	"testing"

	"golang.org/x/net/html"
)

func TestIndexedSelectors4MatchScan(t *testing.T) {
	root := MustParseHTML(nthOfTestHTML)
	for _, indexedRoot := range []*html.Node{root, Query(root, mustParseGroup("ul"))} {
		doc := NewIndexedDocument(indexedRoot)
		for _, query := range []string{
			"li:nth-child(2 of .a)",
			"li:nth-child(odd of .a)",
			"li:nth-last-child(1 of .a)",
			"li:nth-last-child(-n+2 of .a)",
			"li:nth-child(1 of .b, #l5)",
			"li:nth-child(2 of :is(.a, .b))",
			"li:is(.a, :unknown):not(.b)",
			"ul:has(> li:nth-child(2 of .a))",
			"ul:has(> :is(.a, .b)) > li:where(.a)",
			"ul:has(> li:nth-child(2 of .a)) > li",
		} {
			t.Run(query, func(t *testing.T) {
				matcher := mustParseGroup(query)
				want := QueryAll(indexedRoot, matcher)
				if got := doc.QueryAll(matcher); !reflect.DeepEqual(got, want) {
					t.Fatalf("indexed nodes %p, scan nodes %p", got, want)
				}
				if got := doc.Query(matcher); got != firstNode(want) {
					t.Fatalf("indexed first node %p, scan first node %p", got, firstNode(want))
				}
			})
		}
	}
}

func TestForgivingSelectorArgumentBoundaries(t *testing.T) {
	root := MustParseHTML(isTestHTML)
	for _, query := range []string{
		`:is(:unknown(\)), .a)`,
		`:where(:unknown(")"), .a)`,
		`:is(:unknown([title=")"]), .a)`,
	} {
		if got := matchIDs(t, root, query); got != "d1" {
			t.Errorf("%s: got %q, want d1", query, got)
		}
	}
	parser := parser{s: ".a)", depth: maxParseDepth + 1}
	if _, err := parser.parseForgivingSelectorGroup(); err == nil {
		t.Fatal("forgiving parsing exceeded the nesting limit")
	}
}

func TestNthOfDetachedAndOrdinarySpecificity(t *testing.T) {
	detached := &html.Node{Type: html.ElementNode, Data: "li", Attr: []html.Attribute{{Key: "class", Val: "a"}}}
	if mustParseGroup(":nth-child(1 of .a)").Match(detached) {
		t.Fatal("detached element matched a filtered sibling position")
	}
	selector, err := Parse("li:nth-child(2n+1)")
	if err != nil {
		t.Fatal(err)
	}
	if got := selector.Specificity(); got != (Specificity{0, 1, 1}) {
		t.Fatalf("ordinary nth-child specificity = %v", got)
	}
}
