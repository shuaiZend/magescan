package scanner

import (
	"bytes"
	"context"
	"log"
	"regexp"
	"sync"
	"time"
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
	rules    []CompiledRule
	once     sync.Once
	debugLog *log.Logger
}

var (
	defaultMatcher *Matcher
	matcherOnce    sync.Once
)

// NewMatcher creates and returns a Matcher with all rules compiled.
// It uses sync.Once internally to ensure regex compilation happens only once.
// An optional debugLog can be passed for debug-mode logging.
func NewMatcher(debugLog *log.Logger) *Matcher {
	matcherOnce.Do(func() {
		defaultMatcher = &Matcher{}
		defaultMatcher.compile()
	})
	// Always set debugLog (may be nil) on returned instance
	m := &Matcher{
		rules:    defaultMatcher.rules,
		debugLog: debugLog,
	}
	return m
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
func (m *Matcher) Match(ctx context.Context, content []byte) []MatchResult {
	if len(content) == 0 {
		return nil
	}

	var results []MatchResult
	lines := bytes.Split(content, []byte("\n"))

	for _, cr := range m.rules {
		// Check context cancellation between rules
		select {
		case <-ctx.Done():
			return results
		default:
		}

		var ruleStart time.Time
		if m.debugLog != nil {
			ruleStart = time.Now()
		}

		if cr.Rule.IsRegex && cr.Regexp != nil {
			results = append(results, matchRegex(ctx, cr, content, lines)...)
		} else if cr.Rule.Pattern != "" {
			results = append(results, matchLiteral(ctx, cr, content, lines)...)
		}

		if m.debugLog != nil {
			elapsed := time.Since(ruleStart)
			if elapsed > 100*time.Millisecond {
				m.debugLog.Printf("SLOW RULE: %s took %v (content size: %d bytes)",
					cr.Rule.ID, elapsed, len(content))
			}
		}
	}

	return results
}

// isCommentLine returns true if the line (after trimming whitespace) is a comment
func isCommentLine(line []byte) bool {
	trimmed := bytes.TrimLeft(line, " \t")
	if len(trimmed) == 0 {
		return false
	}
	if bytes.HasPrefix(trimmed, []byte("//")) {
		return true
	}
	if bytes.HasPrefix(trimmed, []byte("#")) {
		return true
	}
	if bytes.HasPrefix(trimmed, []byte("/*")) {
		return true
	}
	if trimmed[0] == '*' {
		return true
	}
	return false
}

// matchLiteral performs fast literal string matching using bytes.Contains,
// then identifies the specific line number(s) where the match occurs.
func matchLiteral(ctx context.Context, cr CompiledRule, content []byte, lines [][]byte) []MatchResult {
	pattern := []byte(cr.Rule.Pattern)

	// Quick check: does the content contain the pattern at all?
	if !bytes.Contains(content, pattern) {
		return nil
	}

	// Maximum line length to process (64KB)
	const maxLineLen = 64 * 1024

	skipComments := cr.Rule.Severity != SeverityCritical

	var results []MatchResult
	seen := make(map[int]bool)

	for lineNum, line := range lines {
		// Check for cancellation periodically
		if lineNum%50 == 0 && lineNum > 0 {
			select {
			case <-ctx.Done():
				return results
			default:
			}
		}

		if len(line) > maxLineLen {
			continue
		}
		if skipComments && isCommentLine(line) {
			continue
		}
		if bytes.Contains(line, pattern) {
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

// matchRegex performs regex-based matching against individual lines,
// reporting the line number and matched text for each finding.
func matchRegex(ctx context.Context, cr CompiledRule, content []byte, lines [][]byte) []MatchResult {
	var results []MatchResult
	seen := make(map[int]bool)

	// Maximum line length to process with regex (64KB)
	const maxLineLen = 64 * 1024

	skipComments := cr.Rule.Severity != SeverityCritical

	for lineNum, line := range lines {
		// Check for cancellation periodically
		if lineNum%50 == 0 && lineNum > 0 {
			select {
			case <-ctx.Done():
				return results
			default:
			}
		}

		if len(line) > maxLineLen {
			continue
		}
		if skipComments && isCommentLine(line) {
			continue
		}
		loc := cr.Regexp.Find(line)
		if loc != nil {
			if !seen[lineNum] {
				seen[lineNum] = true
				results = append(results, MatchResult{
					Rule:        cr.Rule,
					LineNumber:  lineNum + 1,
					MatchedText: truncateString(string(loc), 100),
				})
			}
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
