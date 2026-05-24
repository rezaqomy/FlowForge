package kernel

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

type Evaluator interface {
	EvalBool(expr string, scope *Scope) (bool, error)
}

type ExpressionEvaluator struct{}

func NewEvaluator() *ExpressionEvaluator {
	return &ExpressionEvaluator{}
}

func (e *ExpressionEvaluator) EvalBool(expr string, scope *Scope) (bool, error) {
	value, err := e.eval(expr, scope)
	if err != nil {
		return false, err
	}
	boolean, ok := value.(bool)
	if !ok {
		return false, fmt.Errorf("%w: expression %q did not evaluate to bool", ErrInvalidExpression, expr)
	}
	return boolean, nil
}

func (e *ExpressionEvaluator) Parse(expr string) error {
	_, err := parseExpression(tokenize(expr))
	return err
}

func (e *ExpressionEvaluator) Identifiers(expr string) ([]string, error) {
	node, err := parseExpression(tokenize(expr))
	if err != nil {
		return nil, err
	}
	seen := make(map[string]struct{})
	var ids []string
	node.walk(func(n exprNode) {
		if ident, ok := n.(*identifierNode); ok {
			if _, exists := seen[ident.path]; exists {
				return
			}
			seen[ident.path] = struct{}{}
			ids = append(ids, ident.path)
		}
	})
	return ids, nil
}

func (e *ExpressionEvaluator) eval(expr string, scope *Scope) (any, error) {
	node, err := parseExpression(tokenize(expr))
	if err != nil {
		return nil, err
	}
	return node.eval(scope)
}

type token struct {
	kind string
	text string
}

func tokenize(input string) []token {
	var tokens []token
	for i := 0; i < len(input); {
		switch ch := rune(input[i]); {
		case unicode.IsSpace(ch):
			i++
		case ch == '(':
			tokens = append(tokens, token{kind: "lparen", text: "("})
			i++
		case ch == ')':
			tokens = append(tokens, token{kind: "rparen", text: ")"})
			i++
		case ch == '"':
			j := i + 1
			for j < len(input) && rune(input[j]) != '"' {
				if input[j] == '\\' && j+1 < len(input) {
					j += 2
					continue
				}
				j++
			}
			if j >= len(input) {
				tokens = append(tokens, token{kind: "invalid", text: input[i:]})
				return tokens
			}
			tokens = append(tokens, token{kind: "string", text: input[i : j+1]})
			i = j + 1
		case strings.HasPrefix(input[i:], "==") || strings.HasPrefix(input[i:], "!=") || strings.HasPrefix(input[i:], ">=") || strings.HasPrefix(input[i:], "<="):
			tokens = append(tokens, token{kind: "op", text: input[i : i+2]})
			i += 2
		case ch == '>' || ch == '<':
			tokens = append(tokens, token{kind: "op", text: string(ch)})
			i++
		default:
			j := i
			for j < len(input) {
				r := rune(input[j])
				if unicode.IsSpace(r) || r == '(' || r == ')' || r == '>' || r == '<' || r == '=' || r == '!' {
					break
				}
				j++
			}
			word := input[i:j]
			kind := "ident"
			switch word {
			case "and", "or", "not", "in", "contains", "true", "false":
				kind = "keyword"
			default:
				if _, err := strconv.ParseFloat(word, 64); err == nil {
					kind = "number"
				}
			}
			tokens = append(tokens, token{kind: kind, text: word})
			i = j
		}
	}
	return tokens
}

type parser struct {
	tokens []token
	pos    int
}

func parseExpression(tokens []token) (exprNode, error) {
	p := &parser{tokens: tokens}
	node, err := p.parseOr()
	if err != nil {
		return nil, err
	}
	if p.pos != len(p.tokens) {
		return nil, fmt.Errorf("%w: unexpected token %q", ErrInvalidExpression, p.tokens[p.pos].text)
	}
	return node, nil
}

func (p *parser) parseOr() (exprNode, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	for p.matchKeyword("or") {
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		left = &binaryNode{op: "or", left: left, right: right}
	}
	return left, nil
}

func (p *parser) parseAnd() (exprNode, error) {
	left, err := p.parseNot()
	if err != nil {
		return nil, err
	}
	for p.matchKeyword("and") {
		right, err := p.parseNot()
		if err != nil {
			return nil, err
		}
		left = &binaryNode{op: "and", left: left, right: right}
	}
	return left, nil
}

func (p *parser) parseNot() (exprNode, error) {
	if p.matchKeyword("not") {
		node, err := p.parseNot()
		if err != nil {
			return nil, err
		}
		return &unaryNode{op: "not", value: node}, nil
	}
	return p.parseComparison()
}

func (p *parser) parseComparison() (exprNode, error) {
	left, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}
	if p.pos >= len(p.tokens) {
		return left, nil
	}
	current := p.tokens[p.pos]
	if current.kind == "op" || (current.kind == "keyword" && (current.text == "in" || current.text == "contains")) {
		p.pos++
		right, err := p.parsePrimary()
		if err != nil {
			return nil, err
		}
		return &binaryNode{op: current.text, left: left, right: right}, nil
	}
	return left, nil
}

func (p *parser) parsePrimary() (exprNode, error) {
	if p.pos >= len(p.tokens) {
		return nil, fmt.Errorf("%w: unexpected end of expression", ErrInvalidExpression)
	}
	current := p.tokens[p.pos]
	p.pos++

	switch current.kind {
	case "lparen":
		node, err := p.parseOr()
		if err != nil {
			return nil, err
		}
		if !p.match("rparen", ")") {
			return nil, fmt.Errorf("%w: expected ')'", ErrInvalidExpression)
		}
		return node, nil
	case "string":
		value, err := strconv.Unquote(current.text)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidExpression, err)
		}
		return &literalNode{value: value}, nil
	case "number":
		value, err := strconv.ParseFloat(current.text, 64)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidExpression, err)
		}
		return &literalNode{value: value}, nil
	case "keyword":
		switch current.text {
		case "true":
			return &literalNode{value: true}, nil
		case "false":
			return &literalNode{value: false}, nil
		default:
			return nil, fmt.Errorf("%w: unexpected keyword %q", ErrInvalidExpression, current.text)
		}
	case "ident":
		return &identifierNode{path: current.text}, nil
	default:
		return nil, fmt.Errorf("%w: unexpected token %q", ErrInvalidExpression, current.text)
	}
}

func (p *parser) matchKeyword(text string) bool {
	return p.match("keyword", text)
}

func (p *parser) match(kind, text string) bool {
	if p.pos >= len(p.tokens) {
		return false
	}
	current := p.tokens[p.pos]
	if current.kind == kind && current.text == text {
		p.pos++
		return true
	}
	return false
}

type exprNode interface {
	eval(scope *Scope) (any, error)
	walk(func(exprNode))
}

type literalNode struct {
	value any
}

func (n *literalNode) eval(_ *Scope) (any, error) { return n.value, nil }
func (n *literalNode) walk(fn func(exprNode))     { fn(n) }

type identifierNode struct {
	path string
}

func (n *identifierNode) eval(scope *Scope) (any, error) {
	value, ok := scope.GetPath(n.path)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrPathNotFound, n.path)
	}
	return value, nil
}
func (n *identifierNode) walk(fn func(exprNode)) { fn(n) }

type unaryNode struct {
	op    string
	value exprNode
}

func (n *unaryNode) eval(scope *Scope) (any, error) {
	value, err := n.value.eval(scope)
	if err != nil {
		return nil, err
	}
	boolean, ok := value.(bool)
	if !ok {
		return nil, fmt.Errorf("%w: operator %s expects boolean", ErrInvalidExpression, n.op)
	}
	return !boolean, nil
}
func (n *unaryNode) walk(fn func(exprNode)) {
	fn(n)
	n.value.walk(fn)
}

type binaryNode struct {
	op          string
	left, right exprNode
}

func (n *binaryNode) eval(scope *Scope) (any, error) {
	left, err := n.left.eval(scope)
	if err != nil {
		return nil, err
	}
	right, err := n.right.eval(scope)
	if err != nil {
		return nil, err
	}

	switch n.op {
	case "and", "or":
		lb, lok := left.(bool)
		rb, rok := right.(bool)
		if !lok || !rok {
			return nil, fmt.Errorf("%w: logical operator %s expects booleans", ErrInvalidExpression, n.op)
		}
		if n.op == "and" {
			return lb && rb, nil
		}
		return lb || rb, nil
	case "==":
		return compareEqual(left, right), nil
	case "!=":
		return !compareEqual(left, right), nil
	case ">", "<", ">=", "<=":
		lf, lok := toFloat(left)
		rf, rok := toFloat(right)
		if !lok || !rok {
			return nil, fmt.Errorf("%w: comparison %s expects numbers", ErrInvalidExpression, n.op)
		}
		switch n.op {
		case ">":
			return lf > rf, nil
		case "<":
			return lf < rf, nil
		case ">=":
			return lf >= rf, nil
		default:
			return lf <= rf, nil
		}
	case "in":
		return inList(left, right), nil
	case "contains":
		return contains(left, right), nil
	default:
		return nil, fmt.Errorf("%w: unsupported operator %s", ErrInvalidExpression, n.op)
	}
}
func (n *binaryNode) walk(fn func(exprNode)) {
	fn(n)
	n.left.walk(fn)
	n.right.walk(fn)
}

func compareEqual(left, right any) bool {
	switch l := left.(type) {
	case float64:
		r, ok := toFloat(right)
		return ok && l == r
	default:
		return fmt.Sprint(left) == fmt.Sprint(right)
	}
}

func toFloat(value any) (float64, bool) {
	switch typed := value.(type) {
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	default:
		return 0, false
	}
}

func inList(value, collection any) bool {
	items, ok := collection.([]any)
	if ok {
		for _, item := range items {
			if compareEqual(value, item) {
				return true
			}
		}
		return false
	}

	switch typed := collection.(type) {
	case []string:
		for _, item := range typed {
			if fmt.Sprint(value) == item {
				return true
			}
		}
	}
	return false
}

func contains(left, right any) bool {
	return strings.Contains(fmt.Sprint(left), fmt.Sprint(right))
}
