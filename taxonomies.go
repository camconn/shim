// SHIM - A web front end for the Hugo site generator
// Copyright (C) 2016        Cameron Conn

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.

// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package main

import (
	"container/list"
	"fmt"
)

// TaxonomyKinds - All of the kinds of taxonomies available. Only one of these
// should exist for a given site. Moreover, this should be reloaded and changed
// based on which site is currently being used.
// NOTE: Taxonomies are referred to by their plural term.
type TaxonomyKinds map[string]*Taxonomy

// GetTaxonomy - Get the Taxonomy struct that is associated with `name`. If
// there is no Taxonomy, return an error.
func (tk TaxonomyKinds) GetTaxonomy(name string) (*Taxonomy, error) {
	if t, isPlural := tk[name]; isPlural {
		return t, nil
	}

	// Name must be singular, so find the Taxonomy associated with the singular
	// term `name`.
	for _, t := range tk {
		if t.Singular() == name {
			return t, nil
		}
	}

	return nil, fmt.Errorf("No taxonomy found!")
}

// GetKinds - Get the plural names of all taxonomies which exist.
func (tk TaxonomyKinds) GetKinds() []*Taxonomy {
	kinds := make([]*Taxonomy, len(tk))

	i := 0
	for _, t := range tk {
		kinds[i] = t
		i++
	}

	return kinds
}

// NewTaxonomy - Create a new Taxonomy
func (tk TaxonomyKinds) NewTaxonomy(singular, plural string) {
	t := new(Taxonomy)
	t.singular = singular
	t.plural = plural

	t.terms = list.New()
	tk[plural] = t
}

// Taxonomy - A Kind of Taxonomy which includes two keywords, one
// which is singular, one which is plural (e.g. "tag" and "tags").
type Taxonomy struct {
	// Terms used to refer to the taxonomy
	singular string `desc:"How this taxonomy is referred to in the singular case."`
	plural   string `desc:"How this taxonomy is referred to in the plural case"`

	// A list of all terms in the taxonomy
	// TODO: Make this a BST or something similar
	terms *list.List
}

// Clear - Clear all terms from this Taxonomy
func (t Taxonomy) Clear() {
	l := t.terms
	front := l.Front()
	if front != nil {
		l.Init()
	}
	t.terms = list.New()
}

// Singular - Getter method for Taxonomy singular term
func (t Taxonomy) Singular() string {
	return t.singular
}

// Plural - Getter method for Taxonomy plural term
func (t Taxonomy) Plural() string {
	return t.plural
}

// NumTerms - The number of terms in this Taxonomy
func (t Taxonomy) NumTerms() int {
	return t.Terms().Len()
}

// GetTerm - lol
func (t Taxonomy) GetTerm(name string) (*Term, error) {
	for elem := t.terms.Front(); elem != nil; elem = elem.Next() {
		value := (*elem).Value
		term, ok := value.(Term)

		if ok {
			if term.Name() == name {
				return &term, nil
			}
		}
	}

	return nil, fmt.Errorf("Term not found")
}

// AddTerm - Add a Term to a Taxonomy referred by the name `name`. If the Term
// already exists, return an error.
func (t Taxonomy) AddTerm(name string) error {
	// Try to find the term. If it doesn't exist, then add it.
	_, err := t.GetTerm(name)
	if err != nil {
		term := Term{}
		term.name = name
		t.terms.PushBack(term)
	} else {
		// Term already exists
	}

	return nil
}

// TermNames - Get a string slice of all terms in this Taxonomy.
func (t Taxonomy) TermNames() []string {
	terms := make([]string, t.Terms().Len())
	i := 0
	for elem := t.terms.Front(); elem != nil; elem = elem.Next() {
		value, ok := elem.Value.(Term)
		if ok {
			terms[i] = value.Name()
			i++
		}
	}

	return terms
}

// Terms - This taxonomy's terms
func (t Taxonomy) Terms() *list.List {
	return t.terms
}

// Term - A term within a Taxonomy
type Term struct {
	name string
}

// Name - Get a term's name
func (t Term) Name() string {
	return t.name
}
