package main

import (
	"go/token"
	"strings"
	"sync"
)

func CheckGoDocs(lc <-chan *Lexeme, outc chan<- *CheckedLexeme) {
	var wg sync.WaitGroup
	mux := LexemeMux(lc, 2)
	wg.Add(2)
	go func() {
		ch := Filter(mux[0], DeclRootCommentFilter)
		checkGoDoc(ch, outc)
		wg.Done()
	}()
	go func() {
		ch := Filter(Filter(mux[1], DeclTypeFilter), DeclIdentCommentFilter)
		checkGoDoc(ch, outc)
		wg.Done()
	}()
	wg.Wait()
}

func checkGoDoc(tch <-chan *Lexeme, outc chan<- *CheckedLexeme) {
	for {
		ll := []*Lexeme{}
		for {
			l, ok := <-tch
			if !ok {
				return
			}
			if l.tok == token.ILLEGAL {
				break
			}
			ll = append(ll, l)
		}

		godoc := godocBlock(ll)

		// does the comment line up with the next line?
		after := afterGoDoc(ll)
		if after.pos.Column != godoc[0].pos.Column {
			continue
		}
		// is the comment on the line immediately before the code?
		if after.pos.Line != godoc[len(godoc)-1].pos.Line+1 {
			continue
		}

		// does the comment have a token for documentation?
		fields := strings.Fields(godoc[0].lit)
		if len(fields) < 2 {
			continue
		}

		// is the comment a go-swagger comment? If so ignore.
		// len("swagger") == 7
		if len(fields[1]) >= 7 && fields[1][:7] == "swagger" {
			continue
		}

		// check package
		if ll[len(ll)-2].tok == token.PACKAGE {
			if ll[len(ll)-1].lit == "main" {
				// main exemption for describing command line utilities
				continue
			}

			hasPkg := fields[1] == "Package"
			hasName := fields[2] == ll[len(ll)-1].lit
			switch {
			case !hasPkg && !hasName:
				cw := []CheckedWord{{fields[1], "// Package " + ll[len(ll)-1].lit}}
				cl := &CheckedLexeme{godoc[0], "godoc-export", cw}
				outc <- cl
			case !hasPkg:
				cw := []CheckedWord{{fields[1], "Package"}}
				cl := &CheckedLexeme{godoc[0], "godoc-export", cw}
				outc <- cl
			case !hasName:
				cw := []CheckedWord{{fields[2], ll[len(ll)-1].lit}}
				cl := &CheckedLexeme{godoc[0], "godoc-export", cw}
				outc <- cl
			}
			continue
		}

		// what token should the documentation match?
		cmplex := ll[len(ll)-1]
		if ll[len(ll)-2].tok == token.IDENT {
			cmplex = ll[len(ll)-2]
		}
		if (fields[1] == "A" || fields[1] == "An") && fields[2] == cmplex.lit {
			continue
		}
		if fields[1] == cmplex.lit {
			continue
		}

		// bad godoc
		label := "godoc-local"
		if strings.ToUpper(cmplex.lit)[0] == cmplex.lit[0] {
			label = "godoc-export"
		}
		cw := []CheckedWord{{fields[1], cmplex.lit}}
		cl := &CheckedLexeme{godoc[0], label, cw}
		outc <- cl
	}
}

// godocBlock gets the godoc comment block from a comment prefixed token string
func godocBlock(ll []*Lexeme) (comm []*Lexeme) {
	wantLine := 0
	for _, l := range ll {
		if l.tok != token.COMMENT {
			break
		}
		if l.pos.Line != wantLine {
			comm = []*Lexeme{}
		}
		wantLine = l.pos.Line + 1
		comm = append(comm, l)
	}
	return comm
}

// afterGoDoc gets the first token following the comments
func afterGoDoc(ll []*Lexeme) *Lexeme {
	for _, l := range ll {
		if l.tok != token.COMMENT {
			return l
		}
	}
	return nil
}
