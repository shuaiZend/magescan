package scanner

import (
	"bytes"
	"regexp"
	"sync"
)

// CompiledRule holds a rule along with its pre-compiled regex pattern (if applicable)
type CompiledRule struct {
	Rule   Rule
	Regexp *regexp.Regexp // nil for literal matches
}

// MatchResult represents a single detection finding
type MatchResult struct {
	Rule        Rule
	LineNumber  int
	MatchedText string // The actual matched text (truncated to 100 chars)
}

// Matcher provides thread-safe malware signature matching against file content.
// All regex patterns are pre-compiled at initialization for performance.
type Matcher struct {
	rules []CompiledRule
	once  sync.Once
}

var (
	defaultMatcher *Matcher
	matcherOnce    sync.Once
)

// NewMatcher creates and returns a Matcher with all rules compiled.
// It uses sync.Once internally to ensure regex compilation happens only once.
func NewMatcher() *Matcher {
	matcherOnce.Do(func() {
		defaultMatcher = &Matcher{}
		defaultMatcher.compile()
	})
	return defaultMatcher
}

// compile pre-compiles all regex patterns from the rule set
func (m *Matcher) compile() {
	allRules := GetAllRules()
	m.rules = make([]CompiledRule, 0, len(allRules))

	for _, rule := range allRules {
		cr := CompiledRule{Rule: rule}
		if rule.IsRegex && rule.Regex != "" {
			compiled, err := regexp.Compile(rule.Regex)
			if err != nil {
				// Skip rules with invalid regex patterns rather than panicking
				continue
			}
			cr.Regexp = compiled
		}
		m.rules = append(m.rules, cr)
	}
}

// Match checks the provided content against all loaded rules and returns
// all matching results with line numbers. This method is safe for concurrent use.
func (m *Matcher) Match(content []byte) []MatchResult {
	if len(content) == 0 {
		return nil
	}

	var results []MatchResult
	lines := bytes.Split(content, []byte("\n"))

	for _, cr := range m.rules {
		if cr.Rule.IsRegex && cr.Regexp != nil {
			results = append(results, matchRegex(cr, content, lines)...)
		} else if cr.Rule.Pattern != "" {
			results = append(results, matchLiteral(cr, content, lines)...)
		}
	}

	return results
}

// matchLiteral performs fast literal string matching using bytes.Contains,
// then identifies the specific line number(s) where the match occurs.
func matchLiteral(cr CompiledRule, content []byte, lines [][]byte) []MatchResult {
	pattern := []byte(cr.Rule.Pattern)

	// Quick check: does the content contain the pattern at all?
	if !bytes.Contains(content, pattern) {
		return nil
	}

	var results []MatchResult
	seen := make(map[int]bool)

	for lineNum, line := range lines {
		if bytes.Contains(line, pattern) {
			if seen[lineNum] {
				continue
			}
			seen[lineNum] = true
			matchedText := truncateString(string(line), 100)
			results = append(results, MatchResult{
				Rule:        cr.Rule,
				LineNumber:  lineNum + 1, // 1-based line numbers
				MatchedText: matchedText,
			})
		}
	}

	return results
}

// matchRegex performs regex-based matching against individual lines,
// reporting the line number and matched text for each finding.
func matchRegex(cr CompiledRule, content []byte, lines [][]byte) []MatchResult {
	// First, quick check if regex matches anywhere in content
	if !cr.Regexp.Match(content) {
		return nil
	}

	var results []MatchResult
	seen := make(map[int]bool)

	for lineNum, line := range lines {
		loc := cr.Regexp.Find(line)
		if loc != nil {
			if seen[lineNum] {
				continue
			}
			seen[lineNum] = true
			matchedText := truncateString(string(line), 100)
			results = append(results, MatchResult{
				Rule:        cr.Rule,
				LineNumber:  lineNum + 1,
				MatchedText: matchedText,
			})
		}
	}

	return results
}

// truncateString limits a string to maxLen characters, appending "..." if truncated
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// RuleCount returns the total number of loaded rules
func (m *Matcher) RuleCount() int {
	return len(m.rules)
}

// RulesByCategory returns rules filtered by the specified category
func (m *Matcher) RulesByCategory(cat RuleCategory) []CompiledRule {
	var filtered []CompiledRule
	for _, cr := range m.rules {
		if cr.Rule.Category == cat {
			filtered = append(filtered, cr)
		}
	}
	return filtered
}
