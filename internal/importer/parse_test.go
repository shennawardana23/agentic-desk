package importer_test

import (
	"os"
	"reflect"
	"testing"

	"github.com/shennawardana23/agentic-desk/internal/importer"
)

func TestParse_Fixture(t *testing.T) {
	content, err := os.ReadFile("testdata/sample.md")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	got, err := importer.Parse("testdata/sample.md", content)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	want := []importer.Rule{
		{
			SourceFile: "testdata/sample.md", Heading: "Sample Rules",
			LineStart: 1, LineEnd: 4,
			Content: "Intro text before any heading — not captured.",
		},
		{
			SourceFile: "testdata/sample.md", Heading: "Workflow Rules",
			LineStart: 5, LineEnd: 9,
			Content: "Always check status first.\nThen plan before executing.",
		},
		{
			SourceFile: "testdata/sample.md", Heading: "Priority System",
			LineStart: 10, LineEnd: 13,
			Content: "Critical items win over convenience.",
		},
		{
			SourceFile: "testdata/sample.md", Heading: "Another Section",
			LineStart: 16, LineEnd: 18,
			Content: "This one has content.",
		},
	}
	for i := range want {
		want[i].ContentHash = importer.HashContent(want[i].Content)
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Parse() =\n%+v\nwant\n%+v", got, want)
	}
}

func TestParse_IgnoresHeadingsInsideFencedCode(t *testing.T) {
	content := "## Real Heading\n\nSome intro.\n\n```bash\n# not a heading, just a shell comment\necho hi\n```\n\nMore text after the fence.\n"

	got, err := importer.Parse("f.md", []byte(content))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 rule, got %d: %+v", len(got), got)
	}
	if got[0].Heading != "Real Heading" {
		t.Fatalf("expected heading %q, got %q", "Real Heading", got[0].Heading)
	}
	wantContent := "Some intro.\n\n```bash\n# not a heading, just a shell comment\necho hi\n```\n\nMore text after the fence."
	if got[0].Content != wantContent {
		t.Fatalf("Content =\n%q\nwant\n%q", got[0].Content, wantContent)
	}
}

func TestParse_Deterministic(t *testing.T) {
	content := []byte("## Heading\n\nSame content every time.\n")

	first, err := importer.Parse("f.md", content)
	if err != nil {
		t.Fatalf("first parse: %v", err)
	}
	second, err := importer.Parse("f.md", content)
	if err != nil {
		t.Fatalf("second parse: %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("Parse is not deterministic:\n%+v\nvs\n%+v", first, second)
	}
}

func TestParse_NoHeadings(t *testing.T) {
	got, err := importer.Parse("f.md", []byte("just plain text, no headings at all\n"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected no rules, got %+v", got)
	}
}

func TestHashContent(t *testing.T) {
	a := importer.HashContent("hello")
	b := importer.HashContent("hello")
	c := importer.HashContent("world")

	if a != b {
		t.Error("expected identical content to hash identically")
	}
	if a == c {
		t.Error("expected different content to hash differently")
	}
	if len(a) != 64 {
		t.Errorf("expected a 64-char hex sha256 digest, got %d chars", len(a))
	}
}
