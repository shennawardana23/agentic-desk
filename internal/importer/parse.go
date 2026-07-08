// Package importer deterministically extracts profile rules from
// markdown files (CLAUDE.md, RULES.md, PRINCIPLES.md by default) and
// diffs them against what's already in the Second Brain, so re-imports
// are idempotent and respect rules the user has hand-edited
// (ProfileRule.Overridden). No LLM calls anywhere in this package —
// parsing must stay 100% deterministic.
package importer

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
)

// Rule is one heading-delimited section extracted from a source file,
// not yet persisted — see secondbrain.ProfileRule for the stored form.
type Rule struct {
	SourceFile  string
	Heading     string
	LineStart   int
	LineEnd     int
	ContentHash string
	Content     string
}

var (
	headingPattern = regexp.MustCompile(`^#{1,6}\s+(.+?)\s*$`)
	fencePattern   = regexp.MustCompile("^```")
)

// HashContent returns the deterministic hash Parse assigns to a rule's
// body — exported so callers (and tests) can compute the same hash
// independently.
func HashContent(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

// Parse extracts one Rule per markdown heading (any level, ATX style
// "#".."######") in content. A heading's body runs from the line after
// it to the line before the next heading or end of file; headings with
// no body text are skipped since there's nothing to embed or diff.
// Lines inside fenced code blocks (```) are never treated as headings,
// so a "# comment" inside an example shell block doesn't fragment the
// section it lives in.
//
// Two headings with identical text in the same file collide on the
// same (source_file, heading) key the Second Brain stores by — that's
// inherent to the schema this importer writes to (migrations/0001), not
// something this parser resolves; the later one simply wins on Apply.
func Parse(sourceFile string, content []byte) ([]Rule, error) {
	scanner := bufio.NewScanner(bytes.NewReader(content))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	type headingLine struct {
		heading string
		lineNum int
	}
	var headings []headingLine
	var lines []string

	inFence := false
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		lines = append(lines, line)

		if fencePattern.MatchString(strings.TrimSpace(line)) {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		if m := headingPattern.FindStringSubmatch(line); m != nil {
			headings = append(headings, headingLine{heading: m[1], lineNum: lineNum})
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	var rules []Rule
	for i, h := range headings {
		bodyStart := h.lineNum + 1
		bodyEnd := len(lines)
		if i+1 < len(headings) {
			bodyEnd = headings[i+1].lineNum - 1
		}
		if bodyStart > bodyEnd {
			continue // heading immediately followed by another heading or EOF
		}

		body := strings.TrimSpace(strings.Join(lines[bodyStart-1:bodyEnd], "\n"))
		if body == "" {
			continue
		}

		rules = append(rules, Rule{
			SourceFile:  sourceFile,
			Heading:     h.heading,
			LineStart:   h.lineNum,
			LineEnd:     bodyEnd,
			ContentHash: HashContent(body),
			Content:     body,
		})
	}
	return rules, nil
}
