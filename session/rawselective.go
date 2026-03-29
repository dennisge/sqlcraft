// Copyright 2025 Dennis Ge
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package session

import (
	"strings"
)

type selectiveExpr struct {
	op           string
	text         string
	children     []*selectiveExpr
	wrapped      bool
	placeholders []string
	values       []any
}

func pruneNamedSelectiveSQL(rawSQL string, arg any) (string, bool, error) {
	prefix, body := splitSelectivePrefix(rawSQL)
	expr := parseSelectiveExpr(body)
	if expr == nil {
		return "", false, nil
	}

	pruned, err := pruneNamedSelectiveExpr(expr, arg)
	if err != nil {
		return "", false, err
	}
	if pruned == nil {
		return "", false, nil
	}

	return renderSelectiveSQL(prefix, pruned), true, nil
}

func prunePositionalSelectiveSQL(rawSQL string, args []any) (string, []any, bool, error) {
	prefix, body := splitSelectivePrefix(rawSQL)
	expr := parseSelectiveExpr(body)
	if expr == nil {
		return "", nil, false, nil
	}

	consumed := annotateSelectiveArgs(expr, args, 0)
	if consumed < len(args) {
		return "", nil, false, sessionErrorf("append raw selective: placeholder count %d != arg count %d in %q", consumed, len(args), rawSQL)
	}

	pruned := prunePositionalSelectiveExpr(expr)
	if pruned == nil {
		return "", nil, false, nil
	}

	return renderSelectiveSQL(prefix, pruned), collectSelectiveArgs(pruned), true, nil
}

func splitSelectivePrefix(rawSQL string) (string, string) {
	trimmed := strings.TrimSpace(rawSQL)
	for _, keyword := range []string{"WHERE", "HAVING", "AND", "OR"} {
		if hasKeywordPrefix(trimmed, keyword) {
			return keyword, strings.TrimSpace(trimmed[len(keyword):])
		}
	}
	return "", trimmed
}

func hasKeywordPrefix(s string, keyword string) bool {
	if len(s) < len(keyword) || !strings.EqualFold(s[:len(keyword)], keyword) {
		return false
	}
	if len(s) == len(keyword) {
		return true
	}
	return isSelectiveBoundary(rune(s[len(keyword)]))
}

func parseSelectiveExpr(s string) *selectiveExpr {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}

	if inner, ok := unwrapSelectiveParens(s); ok {
		expr := parseSelectiveExpr(inner)
		if expr == nil {
			return nil
		}
		expr.wrapped = true
		return expr
	}

	if parts := splitSelectiveExpr(s, "OR"); len(parts) > 1 {
		return &selectiveExpr{
			op:       "OR",
			children: parseSelectiveChildren(parts),
		}
	}
	if parts := splitSelectiveExpr(s, "AND"); len(parts) > 1 {
		return &selectiveExpr{
			op:       "AND",
			children: parseSelectiveChildren(parts),
		}
	}

	return &selectiveExpr{
		text:         s,
		placeholders: getPlaceholders(s),
	}
}

func parseSelectiveChildren(parts []string) []*selectiveExpr {
	children := make([]*selectiveExpr, 0, len(parts))
	for _, part := range parts {
		expr := parseSelectiveExpr(part)
		if expr != nil {
			children = append(children, expr)
		}
	}
	return children
}

func splitSelectiveExpr(s string, operator string) []string {
	parts := make([]string, 0)
	start := 0
	depth := 0
	quote := byte(0)
	wordStart := -1
	betweenPending := false

	for i := 0; i < len(s); i++ {
		ch := s[i]
		if quote != 0 {
			if ch == quote {
				if quote == '\'' && i+1 < len(s) && s[i+1] == '\'' {
					i++
					continue
				}
				quote = 0
			}
			continue
		}

		switch ch {
		case '\'', '"', '`':
			quote = ch
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		}

		if depth == 0 && isSelectiveWordChar(ch) {
			if wordStart == -1 {
				wordStart = i
			}
		} else if wordStart != -1 {
			if strings.EqualFold(s[wordStart:i], "BETWEEN") {
				betweenPending = true
			}
			wordStart = -1
		}

		if operator == "AND" && betweenPending && depth == 0 && matchesSelectiveOperator(s, i, operator) {
			betweenPending = false
			i += len(operator) - 1
			continue
		}

		if depth != 0 || !matchesSelectiveOperator(s, i, operator) {
			continue
		}

		parts = append(parts, strings.TrimSpace(s[start:i]))
		start = i + len(operator)
		i += len(operator) - 1
	}

	if len(parts) == 0 {
		return nil
	}
	parts = append(parts, strings.TrimSpace(s[start:]))
	return parts
}

func isSelectiveWordChar(ch byte) bool {
	return (ch >= '0' && ch <= '9') ||
		(ch >= 'a' && ch <= 'z') ||
		(ch >= 'A' && ch <= 'Z') ||
		ch == '_'
}

func matchesSelectiveOperator(s string, index int, operator string) bool {
	end := index + len(operator)
	if end > len(s) || !strings.EqualFold(s[index:end], operator) {
		return false
	}
	if index > 0 && !isSelectiveBoundary(rune(s[index-1])) {
		return false
	}
	if end < len(s) && !isSelectiveBoundary(rune(s[end])) {
		return false
	}
	return true
}

func isSelectiveBoundary(r rune) bool {
	return (r < '0' || r > '9') &&
		(r < 'a' || r > 'z') &&
		(r < 'A' || r > 'Z') &&
		r != '_'
}

func unwrapSelectiveParens(s string) (string, bool) {
	if len(s) < 2 || s[0] != '(' || s[len(s)-1] != ')' {
		return "", false
	}

	depth := 0
	quote := byte(0)
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if quote != 0 {
			if ch == quote {
				if quote == '\'' && i+1 < len(s) && s[i+1] == '\'' {
					i++
					continue
				}
				quote = 0
			}
			continue
		}

		switch ch {
		case '\'', '"', '`':
			quote = ch
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 && i != len(s)-1 {
				return "", false
			}
		}
	}

	if depth != 0 {
		return "", false
	}
	return strings.TrimSpace(s[1 : len(s)-1]), true
}

func pruneNamedSelectiveExpr(expr *selectiveExpr, arg any) (*selectiveExpr, error) {
	if expr == nil {
		return nil, nil
	}
	if len(expr.children) == 0 {
		if len(expr.placeholders) == 0 {
			return expr, nil
		}
		for _, ph := range expr.placeholders {
			value, ok, err := lookupNamedArgValue(arg, placeholderName(ph))
			if err != nil {
				return nil, sessionWrapError("append raw selective", err)
			}
			if !ok {
				return nil, nil
			}
			if actual, present, null, wrapped := unwrapSelectivePresence(value); wrapped {
				if !present || null {
					return nil, nil
				}
				value = actual
			} else if !isNotZero(value) {
				return nil, nil
			}
		}
		return expr, nil
	}

	children := make([]*selectiveExpr, 0, len(expr.children))
	for _, child := range expr.children {
		pruned, err := pruneNamedSelectiveExpr(child, arg)
		if err != nil {
			return nil, err
		}
		if pruned != nil {
			children = append(children, pruned)
		}
	}
	return rebuildSelectiveExpr(expr, children), nil
}

func annotateSelectiveArgs(expr *selectiveExpr, args []any, index int) int {
	if expr == nil {
		return index
	}
	if len(expr.children) == 0 {
		count := len(expr.placeholders)
		if count == 0 {
			return index
		}
		end := index + count
		if end > len(args) {
			end = len(args)
		}
		expr.values = append([]any(nil), args[index:end]...)
		return end
	}
	for _, child := range expr.children {
		index = annotateSelectiveArgs(child, args, index)
	}
	return index
}

func prunePositionalSelectiveExpr(expr *selectiveExpr) *selectiveExpr {
	if expr == nil {
		return nil
	}
	if len(expr.children) == 0 {
		if len(expr.placeholders) == 0 {
			return expr
		}
		if len(expr.values) < len(expr.placeholders) {
			return nil
		}
		for i, value := range expr.values {
			if actual, present, null, wrapped := unwrapSelectivePresence(value); wrapped {
				if !present || null {
					return nil
				}
				expr.values[i] = actual
				continue
			}
			if !isNotZero(value) {
				return nil
			}
		}
		return expr
	}

	children := make([]*selectiveExpr, 0, len(expr.children))
	for _, child := range expr.children {
		if pruned := prunePositionalSelectiveExpr(child); pruned != nil {
			children = append(children, pruned)
		}
	}
	return rebuildSelectiveExpr(expr, children)
}

func rebuildSelectiveExpr(expr *selectiveExpr, children []*selectiveExpr) *selectiveExpr {
	switch len(children) {
	case 0:
		return nil
	case 1:
		child := children[0]
		if expr.wrapped {
			child.wrapped = true
		}
		return child
	default:
		return &selectiveExpr{
			op:       expr.op,
			children: children,
			wrapped:  expr.wrapped,
		}
	}
}

func collectSelectiveArgs(expr *selectiveExpr) []any {
	if expr == nil {
		return nil
	}
	if len(expr.children) == 0 {
		return append([]any(nil), expr.values...)
	}
	args := make([]any, 0)
	for _, child := range expr.children {
		args = append(args, collectSelectiveArgs(child)...)
	}
	return args
}

func renderSelectiveSQL(prefix string, expr *selectiveExpr) string {
	body := renderSelectiveExpr(expr)
	if body == "" {
		return ""
	}
	if prefix == "" {
		return body
	}
	return prefix + " " + body
}

func renderSelectiveExpr(expr *selectiveExpr) string {
	if expr == nil {
		return ""
	}

	var body string
	if len(expr.children) == 0 {
		body = strings.TrimSpace(expr.text)
	} else {
		parts := make([]string, 0, len(expr.children))
		for _, child := range expr.children {
			rendered := renderSelectiveExpr(child)
			if rendered != "" {
				parts = append(parts, rendered)
			}
		}
		body = strings.Join(parts, " "+expr.op+" ")
	}

	if body == "" {
		return ""
	}
	if expr.wrapped {
		return "(" + body + ")"
	}
	return body
}
