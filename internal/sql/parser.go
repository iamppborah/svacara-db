package sql

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

type Parser struct {
	input []byte
	pos   int
}

func NewParser(input string) *Parser {
	return &Parser{input: []byte(input)}
}

func (p *Parser) peek() byte {
	if p.pos >= len(p.input) {
		return 0
	}
	return p.input[p.pos]
}

func (p *Parser) next() byte {
	if p.pos >= len(p.input) {
		return 0
	}
	b := p.input[p.pos]
	p.pos++
	return b
}

func (p *Parser) skipWS() {
	for p.pos < len(p.input) && (p.input[p.pos] == ' ' || p.input[p.pos] == '\t' || p.input[p.pos] == '\n' || p.input[p.pos] == '\r') {
		p.pos++
	}
}

func (p *Parser) skipComments() {
	p.skipWS()
	for p.pos+1 < len(p.input) && p.input[p.pos] == '-' && p.input[p.pos+1] == '-' {
		for p.pos < len(p.input) && p.input[p.pos] != '\n' {
			p.pos++
		}
		p.skipWS()
	}
}

func pKeyword(p *Parser, keywords ...string) bool {
	saved := p.pos
	p.skipComments()
	for _, kw := range keywords {
		p.skipWS()
		for _, c := range []byte(kw) {
			b := p.peek()
			if b >= 'A' && b <= 'Z' {
				b += 32
			}
			if b != byte(c) {
				p.pos = saved
				return false
			}
			p.next()
		}
	}
	return true
}

func pExpect(p *Parser, keyword string, errMsg string) error {
	if !pKeyword(p, keyword) {
		return fmt.Errorf("%s: got %q", errMsg, p.rest())
	}
	return nil
}

func pMustSym(p *Parser) string {
	p.skipComments()
	start := p.pos
	for {
		b := p.peek()
		if (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '_' || b == '@' {
			p.next()
		} else {
			break
		}
	}
	if p.pos == start {
		return ""
	}
	return string(p.input[start:p.pos])
}

func (p *Parser) pNum() (int64, bool) {
	p.skipComments()
	start := p.pos
	neg := false
	if p.peek() == '-' {
		neg = true
		p.next()
	}
	for p.peek() >= '0' && p.peek() <= '9' {
		p.next()
	}
	if p.pos == start || (neg && p.pos == start+1) {
		p.pos = start
		return 0, false
	}
	n, _ := strconv.ParseInt(string(p.input[start:p.pos]), 10, 64)
	return n, true
}

func (p *Parser) pStr() ([]byte, bool) {
	p.skipComments()
	if p.peek() != '\'' && p.peek() != '"' {
		return nil, false
	}
	quote := p.next()
	var out []byte
	for p.pos < len(p.input) {
		b := p.next()
		if b == quote {
			return out, true
		}
		if b == '\\' && p.pos < len(p.input) {
			out = append(out, p.next())
		} else {
			out = append(out, b)
		}
	}
	return nil, false
}

func (p *Parser) rest() string {
	if p.pos >= len(p.input) {
		return "<EOF>"
	}
	r := string(p.input[p.pos:])
	if len(r) > 30 {
		r = r[:30] + "..."
	}
	return r
}

func Parse(input string) (interface{}, error) {
	p := NewParser(input)
	result := pStmt(p)
	p.skipWS()
	if p.pos < len(p.input) {
		return nil, fmt.Errorf("unexpected trailing input: %q", p.rest())
	}
	return result, nil
}

func pStmt(p *Parser) interface{} {
	switch {
	case pKeyword(p, "create", "table"):
		return pCreateTable(p)
	case pKeyword(p, "create", "index"):
		return pCreateIndex(p)
	case pKeyword(p, "select"):
		return pSelect(p)
	case pKeyword(p, "insert", "into"):
		return pInsert(p)
	case pKeyword(p, "update"):
		return pUpdate(p)
	case pKeyword(p, "delete", "from"):
		return pDelete(p)
	case pKeyword(p, "explain"):
		stmt := pSelect(p)
		return fmt.Sprintf("EXPLAIN: %+v", stmt)
	case pKeyword(p, "begin"):
		return "BEGIN"
	case pKeyword(p, "commit"):
		return "COMMIT"
	case pKeyword(p, "abort"):
		return "ABORT"
	default:
		return fmt.Errorf("unknown statement at: %q", p.rest())
	}
}

func pCreateTable(p *Parser) *QLCreateTable {
	stmt := &QLCreateTable{}
	stmt.Table = pMustSym(p)
	if err := pExpect(p, "(", "expect '('"); err != nil {
		return stmt
	}
	for {
		if pKeyword(p, "primary", "key") {
			pExpect(p, "(", "expect '(' after PRIMARY KEY")
			for {
				stmt.PKeys = append(stmt.PKeys, pMustSym(p))
				if !pKeyword(p, ",") {
					break
				}
			}
			pExpect(p, ")", "expect ')'")
			if pKeyword(p, ",") {
				continue
			}
			break
		}
		col := ColSpec{}
		col.Name = pMustSym(p)
		col.Type = pType(p)
		stmt.Cols = append(stmt.Cols, col)
		if !pKeyword(p, ",") {
			break
		}
	}
	pExpect(p, ")", "expect ')'")
	return stmt
}

func pType(p *Parser) uint32 {
	if pKeyword(p, "int64") || pKeyword(p, "int") {
		return QLI64
	}
	if pKeyword(p, "bytes") || pKeyword(p, "text") || pKeyword(p, "string") {
		return QLStr
	}
	if pKeyword(p, "bool") {
		return QLAnd
	}
	if pKeyword(p, "float64") || pKeyword(p, "float") {
		return 999
	}
	return 0
}

func pCreateIndex(p *Parser) *QLCreateIndex {
	stmt := &QLCreateIndex{}
	if pKeyword(p, "unique") {
		stmt.Unique = true
	}
	stmt.Name = pMustSym(p)
	if err := pExpect(p, "on", "expect ON"); err != nil {
		return stmt
	}
	stmt.Table = pMustSym(p)
	if err := pExpect(p, "(", "expect '('"); err != nil {
		return stmt
	}
	for {
		stmt.Keys = append(stmt.Keys, pMustSym(p))
		if !pKeyword(p, ",") {
			break
		}
	}
	pExpect(p, ")", "expect ')'")
	return stmt
}

func pSelect(p *Parser) *QLSelect {
	stmt := &QLSelect{}
	pSelectExprList(p, stmt)
	if err := pExpect(p, "from", "expect FROM"); err != nil {
		return stmt
	}
	stmt.Table = pMustSym(p)
	pScan(p, &stmt.QLScan)
	return stmt
}

func pSelectExprList(p *Parser, node *QLSelect) {
	pSelectExpr(p, node)
	for pKeyword(p, ",") {
		pSelectExpr(p, node)
	}
}

func pSelectExpr(p *Parser, node *QLSelect) {
	if pKeyword(p, "*") {
		node.Names = append(node.Names, "*")
		node.Output = append(node.Output, QLNode{Type: QLIdent, Str: []byte("*")})
		return
	}
	name := pMustSym(p)
	if pKeyword(p, "as") {
		name = pMustSym(p)
	}
	node.Names = append(node.Names, name)
	node.Output = append(node.Output, QLNode{Type: QLIdent, Str: []byte(name)})
}

func pScan(p *Parser, node *QLScan) {
	if pKeyword(p, "index", "by") {
		pIndexBy(p, node)
	}
	if pKeyword(p, "filter") {
		expr := pExprOr(p)
		node.Filter = expr
	}
	node.Offset = 0
	node.Limit = math.MaxInt64
	if pKeyword(p, "limit") {
		p.pLimit(node)
	}
}

func pIndexBy(p *Parser, node *QLScan) {
	node.Key1 = pExprCmp(p)
	if pKeyword(p, "and") || pKeyword(p, ",") {
		node.Key2 = pExprCmp(p)
	}
}

func (p *Parser) pLimit(node *QLScan) {
	n, ok := p.pNum()
	if ok {
		node.Limit = n
	}
	if pKeyword(p, "offset") {
		n, ok := p.pNum()
		if ok {
			node.Offset = n
		}
	}
}

func pInsert(p *Parser) *QLInsert {
	stmt := &QLInsert{}
	stmt.Table = pMustSym(p)
	if pKeyword(p, "(") {
		for {
			stmt.Names = append(stmt.Names, pMustSym(p))
			if !pKeyword(p, ",") {
				break
			}
		}
		pExpect(p, ")", "expect ')'")
	}
	pExpect(p, "values", "expect VALUES")
	pExpect(p, "(", "expect '('")
	for {
		stmt.Values = append(stmt.Values, pExprOr(p))
		if !pKeyword(p, ",") {
			break
		}
	}
	pExpect(p, ")", "expect ')'")
	return stmt
}

func pUpdate(p *Parser) *QLUpdate {
	stmt := &QLUpdate{}
	stmt.Table = pMustSym(p)
	pExpect(p, "set", "expect SET")
	for {
		name := pMustSym(p)
		pExpect(p, "=", "expect =")
		stmt.Names = append(stmt.Names, name)
		stmt.Values = append(stmt.Values, pExprOr(p))
		if !pKeyword(p, ",") {
			break
		}
	}
	pExpect(p, "where", "expect WHERE")
	pScan(p, &stmt.QLScan)
	return stmt
}

func pDelete(p *Parser) *QLDelete {
	stmt := &QLDelete{}
	stmt.Table = pMustSym(p)
	if pKeyword(p, "where") {
		pScan(p, &stmt.QLScan)
	}
	return stmt
}

func pExprOr(p *Parser) QLNode {
	left := pExprAnd(p)
	for pKeyword(p, "or") {
		right := pExprAnd(p)
		left = QLNode{Type: QLOr, Kids: []QLNode{left, right}}
	}
	return left
}

func pExprAnd(p *Parser) QLNode {
	left := pExprCmp(p)
	for pKeyword(p, "and") {
		right := pExprCmp(p)
		left = QLNode{Type: QLAnd, Kids: []QLNode{left, right}}
	}
	return left
}

func pExprCmp(p *Parser) QLNode {
	left := pExprAdd(p)
	op := uint32(0)
	switch {
	case pKeyword(p, "<="):
		op = QLLE
	case pKeyword(p, ">="):
		op = QLGE
	case pKeyword(p, "!="), pKeyword(p, "<>"):
		op = QLNE
	case pKeyword(p, "="), pKeyword(p, "=="):
		op = QLEQ
	case pKeyword(p, "<"):
		op = QLLT
	case pKeyword(p, ">"):
		op = QLGT
	case pKeyword(p, "like"):
		op = QLLike
	}
	if op != 0 {
		right := pExprAdd(p)
		left = QLNode{Type: op, Kids: []QLNode{left, right}}
	}
	return left
}

func pExprAdd(p *Parser) QLNode {
	left := pExprMul(p)
	for {
		op := uint32(0)
		switch {
		case pKeyword(p, "+"):
			op = QLAdd
		case pKeyword(p, "-"):
			op = QLSub
		default:
			return left
		}
		right := pExprMul(p)
		left = QLNode{Type: op, Kids: []QLNode{left, right}}
	}
}

func pExprMul(p *Parser) QLNode {
	left := pExprAtom(p)
	for {
		op := uint32(0)
		switch {
		case pKeyword(p, "*"):
			op = QLMul
		case pKeyword(p, "/"):
			op = QLDiv
		case pKeyword(p, "%"):
			op = QLMod
		default:
			return left
		}
		right := pExprAtom(p)
		left = QLNode{Type: op, Kids: []QLNode{left, right}}
	}
}

func pExprAtom(p *Parser) QLNode {
	switch {
	case pKeyword(p, "not"):
		return QLNode{Type: QLNot, Kids: []QLNode{pExprAtom(p)}}
	case pKeyword(p, "("):
		expr := pExprOr(p)
		pExpect(p, ")", "expect ')'")
		return expr
	}

	if n, ok := p.pNum(); ok {
		return QLNode{Type: QLI64, I64: n}
	}

	if s, ok := p.pStr(); ok {
		return QLNode{Type: QLStr, Str: s}
	}

	name := pMustSym(p)
	if name != "" {
		if pKeyword(p, "(") {
			kids := []QLNode{}
			if !pKeyword(p, ")") {
				for {
					kids = append(kids, pExprOr(p))
					if !pKeyword(p, ",") {
						break
					}
				}
				pExpect(p, ")", "expect ')'")
			}
			return QLNode{Type: QLFunc, Str: []byte(strings.ToLower(name)), Kids: kids}
		}
		return QLNode{Type: QLIdent, Str: []byte(name)}
	}

	return QLNode{}
}
