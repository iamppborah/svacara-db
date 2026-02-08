package sql

import (
	"github.com/ppborah/svacara-db/internal/relational"
)

func selectIndex(tdef *relational.TableDef, filter QLNode) *relational.IndexDef {
	if filter.Type == 0 {
		return nil
	}
	cols := filterColumns(filter)
	if cols == nil {
		return nil
	}
	var best *relational.IndexDef
	bestMatch := 0
	for i := range tdef.Indexes {
		idx := &tdef.Indexes[i]
		match := indexMatch(idx.Keys, cols)
		if match > bestMatch {
			bestMatch = match
			best = idx
		}
	}
	if bestMatch > 0 {
		return best
	}
	return nil
}

func filterColumns(node QLNode) []string {
	if node.Type == QLAnd {
		left := filterColumns(node.Kids[0])
		right := filterColumns(node.Kids[1])
		return append(left, right...)
	}
	if node.Type >= QLLT && node.Type <= QLNE {
		if node.Kids[0].Type == QLIdent {
			return []string{string(node.Kids[0].Str)}
		}
	}
	return nil
}

func indexMatch(indexKeys, filterCols []string) int {
	match := 0
	for _, ik := range indexKeys {
		found := false
		for _, fc := range filterCols {
			if ik == fc {
				match++
				found = true
				break
			}
		}
		if !found {
			break
		}
	}
	return match
}
