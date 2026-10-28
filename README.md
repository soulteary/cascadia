# cascadia

This fork publishes the Go module `github.com/soulteary/cascadia` from `main`.
The `master` branch tracks `andybalholm/cascadia` and is reserved for upstream
synchronization. Branches intended for upstream pull requests should start
from `master` and retain the upstream module path. Fork development and release
pull requests should target `main`.

[![Go quality](https://github.com/soulteary/cascadia/actions/workflows/quality.yml/badge.svg)](https://github.com/soulteary/cascadia/actions/workflows/quality.yml)
[![Go Test Coverage](./.github/coverage.svg)](./.github/go-test-report.md)
[![Go Report Card](./.github/goreportcard.svg)](./.github/goreportcard-report.md)

The Cascadia package implements CSS selectors for use with the parse trees produced by the html package.

To test CSS selectors without writing Go code, check out [cascadia](https://github.com/suntong/cascadia) the command line tool, a thin wrapper around this package.

[Refer to package documentation here](https://pkg.go.dev/github.com/soulteary/cascadia).

## Tests and quality reports

The Go quality workflow runs on pushes to `main`, pull requests targeting
`main`, and manual dispatches. It uses
[go-test-report-action](https://github.com/soulteary/go-test-report-action)
to run tests with the race detector and enforce 100% statement coverage for
the full module and each package.
[goreportcard-action](https://github.com/soulteary/goreportcard-action)
generates the code quality report.

Each run uploads reports and raw test results as workflow artifacts. Successful
`main` runs also commit the badges and reports linked above, provided `main`
still points to the tested revision. Pull request checks have read-only access.

To run the checks locally:

```sh
go test -race -covermode=atomic -coverprofile=coverage.out ./...
go tool cover -func=coverage.out
go vet ./...
```

## Repeated queries over one document

`IndexedDocument` accelerates workloads that run many selectors over the same
immutable HTML tree. It builds tag, class, ID, and sibling-position indexes
once, then uses the most selective rightmost selector as the candidate set.
Selectors that cannot use an index automatically fall back to a tree scan.

```go
doc, err := html.Parse(reader)
if err != nil {
	log.Fatal(err)
}

indexed := cascadia.NewIndexedDocument(doc)
selector, err := cascadia.ParseGroup("article.featured > a:has(img)")
if err != nil {
	log.Fatal(err)
}

links := indexed.QueryAll(selector)
```

For hot loops, reuse result storage with `QueryAllInto`:

```go
var links []*html.Node
for _, selector := range selectors {
	links = indexed.QueryAllInto(selector, links[:0])
	// Process links before the next iteration.
}
```

The tree must not be mutated while the index is in use. Rebuild the index after
changing nodes or attributes. An `IndexedDocument` supports concurrent queries
when the underlying tree is immutable. For a single query, the regular
`Query`/`QueryAll` functions may be faster because they do not build an index.

## Example

The following is an example of how you can use Cascadia.

```go
package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/soulteary/cascadia"
	"golang.org/x/net/html"
)

var pricingHtml string = `
<div class="card mb-4 box-shadow">
	<div class="card-header">
		<h4 class="my-0 font-weight-normal">Free</h4>
	</div>
	<div class="card-body">
		<h1 class="card-title pricing-card-title">$0/mo</h1>
		<ul class="list-unstyled mt-3 mb-4">
			<li>10 users included</li>
			<li>2 GB of storage</li>
			<li><a href="https://example.com">See more</a></li>
		</ul>
	</div>
</div>

<div class="card mb-4 box-shadow">
	<div class="card-header">
		<h4 class="my-0 font-weight-normal">Pro</h4>
	</div>
	<div class="card-body">
		<h1 class="card-title pricing-card-title">$15/mo</h1>
		<ul class="list-unstyled mt-3 mb-4">
			<li>20 users included</li>
			<li>10 GB of storage</li>
			<li><a href="https://example.com">See more</a></li>
		</ul>
	</div>
</div>

<div class="card mb-4 box-shadow">
	<div class="card-header">
		<h4 class="my-0 font-weight-normal">Enterprise</h4>
	</div>
	<div class="card-body">
		<h1 class="card-title pricing-card-title">$29/mo</h1>
		<ul class="list-unstyled mt-3 mb-4">
			<li>30 users included</li>
			<li>15 GB of storage</li>
			<li><a>See more</a></li>
		</ul>
	</div>
</div>
`

func Query(n *html.Node, query string) *html.Node {
	sel, err := cascadia.Parse(query)
	if err != nil {
		return &html.Node{}
	}
	return cascadia.Query(n, sel)
}

func QueryAll(n *html.Node, query string) []*html.Node {
	sel, err := cascadia.Parse(query)
	if err != nil {
		return []*html.Node{}
	}
	return cascadia.QueryAll(n, sel)
}

func AttrOr(n *html.Node, attrName, or string) string {
	for _, a := range n.Attr {
		if a.Key == attrName {
			return a.Val
		}
	}
	return or
}

func main() {
	doc, err := html.Parse(strings.NewReader(pricingHtml))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("List of pricing plans:\n\n")
	for i, p := range QueryAll(doc, "div.card.mb-4.box-shadow") {
		planName := Query(p, "h4").FirstChild.Data
		price := Query(p, ".pricing-card-title").FirstChild.Data
		usersIncluded := Query(p, "li:first-child").FirstChild.Data
		storage := Query(p, "li:nth-child(2)").FirstChild.Data
		detailsUrl := AttrOr(Query(p, "li:last-child a"), "href", "(No link available)")
		fmt.Printf(
			"Plan #%d\nName: %s\nPrice: %s\nUsers: %s\nStorage: %s\nDetails: %s\n\n",
			i+1,
			planName,
			price,
			usersIncluded,
			storage,
			detailsUrl,
		)
	}
}
```
The output is:
```
List of pricing plans:

Plan #1
Name: Free
Price: $0/mo
Users: 10 users included
Storage: 2 GB of storage
Details: https://example.com

Plan #2
Name: Pro
Price: $15/mo
Users: 20 users included
Storage: 10 GB of storage
Details: https://example.com

Plan #3
Name: Enterprise
Price: $29/mo
Users: 30 users included
Storage: 15 GB of storage
Details: (No link available)
```
