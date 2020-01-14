package docopt

import (
	"fmt"
	"reflect"
	"strings"
)

type patternType uint

const (
	// leaf
	patternArgument patternType = 1 << iota
	patternCommand
	patternOption

	// branch
	patternRequired
	patternOptionAL
	patternOptionSSHORTCUT // Marker/placeholder for [options] shortcut.
	patternOneOrMore
	patternEither

	patternLeaf = patternArgument +
		patternCommand +
		patternOption
	patternBranch = patternRequired +
		patternOptionAL +
		patternOptionSSHORTCUT +
		patternOneOrMore +
		patternEither
	patternAll     = patternLeaf + patternBranch
	patternDefault = 0
)

func (pt patternType) String() string {
	switch pt {
	case patternArgument:
		return "argument"
	case patternCommand:
		return "command"
	case patternOption:
		return "option"
	case patternRequired:
		return "required"
	case patternOptionAL:
		return "optional"
	case patternOptionSSHORTCUT:
		return "optionsshortcut"
	case patternOneOrMore:
		return "oneormore"
	case patternEither:
		return "either"
	case patternLeaf:
		return "leaf"
	case patternBranch:
		return "branch"
	case patternAll:
		return "all"
	case patternDefault:
		return "default"
	}
	return ""
}

type Pattern struct {
	T patternType

	Children PatternList

	Name  string
	Value interface{}

	Short    string
	Long     string
	Argcount int
}

type PatternList []*Pattern

func newBranchPattern(t patternType, pl ...*Pattern) *Pattern {
	var p Pattern
	p.T = t
	p.Children = make(PatternList, len(pl))
	copy(p.Children, pl)
	return &p
}

func newRequired(pl ...*Pattern) *Pattern {
	return newBranchPattern(patternRequired, pl...)
}

func newEither(pl ...*Pattern) *Pattern {
	return newBranchPattern(patternEither, pl...)
}

func newOneOrMore(pl ...*Pattern) *Pattern {
	return newBranchPattern(patternOneOrMore, pl...)
}

func newOptional(pl ...*Pattern) *Pattern {
	return newBranchPattern(patternOptionAL, pl...)
}

func newOptionsShortcut() *Pattern {
	var p Pattern
	p.T = patternOptionSSHORTCUT
	return &p
}

func newLeafPattern(t patternType, name string, value interface{}) *Pattern {
	// default: value=nil
	var p Pattern
	p.T = t
	p.Name = name
	p.Value = value
	return &p
}

func newArgument(name string, value interface{}) *Pattern {
	// default: value=nil
	return newLeafPattern(patternArgument, name, value)
}

func newCommand(name string, value interface{}) *Pattern {
	// default: value=false
	var p Pattern
	p.T = patternCommand
	p.Name = name
	p.Value = value
	return &p
}

func newOption(short, long string, argcount int, value interface{}) *Pattern {
	// default: "", "", 0, false
	var p Pattern
	p.T = patternOption
	p.Short = short
	p.Long = long
	if long != "" {
		p.Name = long
	} else {
		p.Name = short
	}
	p.Argcount = argcount
	if value == false && argcount > 0 {
		p.Value = nil
	} else {
		p.Value = value
	}
	return &p
}

func (p *Pattern) Flat(types patternType) (PatternList, error) {
	if p.T&patternLeaf != 0 {
		if types == patternDefault {
			types = patternAll
		}
		if p.T&types != 0 {
			return PatternList{p}, nil
		}
		return PatternList{}, nil
	}

	if p.T&patternBranch != 0 {
		if p.T&types != 0 {
			return PatternList{p}, nil
		}
		result := PatternList{}
		for _, child := range p.Children {
			childFlat, err := child.Flat(types)
			if err != nil {
				return nil, err
			}
			result = append(result, childFlat...)
		}
		return result, nil
	}
	return nil, newError("unknown pattern type: %d, %d", p.T, types)
}

func (p *Pattern) fix() error {
	err := p.fixIdentities(nil)
	if err != nil {
		return err
	}
	p.fixRepeatingArguments()
	return nil
}

func (p *Pattern) fixIdentities(uniq PatternList) error {
	// Make pattern-tree tips point to same object if they are equal.
	if p.T&patternBranch == 0 {
		return nil
	}
	if uniq == nil {
		pFlat, err := p.Flat(patternDefault)
		if err != nil {
			return err
		}
		uniq = pFlat.unique()
	}
	for i, child := range p.Children {
		if child.T&patternBranch == 0 {
			ind, err := uniq.index(child)
			if err != nil {
				return err
			}
			p.Children[i] = uniq[ind]
		} else {
			err := child.fixIdentities(uniq)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (p *Pattern) fixRepeatingArguments() {
	// Fix elements that should accumulate/increment values.
	var either []PatternList

	for _, child := range p.transform().Children {
		either = append(either, child.Children)
	}
	for _, cas := range either {
		casMultiple := PatternList{}
		for _, e := range cas {
			if cas.count(e) > 1 {
				casMultiple = append(casMultiple, e)
			}
		}
		for _, e := range casMultiple {
			if e.T == patternArgument || e.T == patternOption && e.Argcount > 0 {
				switch e.Value.(type) {
				case string:
					e.Value = strings.Fields(e.Value.(string))
				case []string:
				default:
					e.Value = []string{}
				}
			}
			if e.T == patternCommand || e.T == patternOption && e.Argcount == 0 {
				e.Value = 0
			}
		}
	}
}

func (p *Pattern) match(left *PatternList, collected *PatternList) (bool, *PatternList, *PatternList) {
	if collected == nil {
		collected = &PatternList{}
	}
	if p.T&patternRequired != 0 {
		l := left
		c := collected
		for _, p := range p.Children {
			var matched bool
			matched, l, c = p.match(l, c)
			if !matched {
				return false, left, collected
			}
		}
		return true, l, c
	} else if p.T&patternOptionAL != 0 || p.T&patternOptionSSHORTCUT != 0 {
		for _, p := range p.Children {
			_, left, collected = p.match(left, collected)
		}
		return true, left, collected
	} else if p.T&patternOneOrMore != 0 {
		if len(p.Children) != 1 {
			panic("OneOrMore.match(): assert len(p.children) == 1")
		}
		l := left
		c := collected
		var lAlt *PatternList
		matched := true
		times := 0
		for matched {
			// could it be that something didn't match but changed l or c?
			matched, l, c = p.Children[0].match(l, c)
			if matched {
				times++
			}
			if lAlt == l {
				break
			}
			lAlt = l
		}
		if times >= 1 {
			return true, l, c
		}
		return false, left, collected
	} else if p.T&patternEither != 0 {
		type outcomeStruct struct {
			matched   bool
			left      *PatternList
			collected *PatternList
			length    int
		}
		outcomes := []outcomeStruct{}
		for _, p := range p.Children {
			matched, l, c := p.match(left, collected)
			outcome := outcomeStruct{matched, l, c, len(*l)}
			if matched {
				outcomes = append(outcomes, outcome)
			}
		}
		if len(outcomes) > 0 {
			minLen := outcomes[0].length
			minIndex := 0
			for i, v := range outcomes {
				if v.length < minLen {
					minIndex = i
				}
			}
			return outcomes[minIndex].matched, outcomes[minIndex].left, outcomes[minIndex].collected
		}
		return false, left, collected
	} else if p.T&patternLeaf != 0 {
		pos, match := p.singleMatch(left)
		var increment interface{}
		if match == nil {
			return false, left, collected
		}
		leftAlt := make(PatternList, len((*left)[:pos]), len((*left)[:pos])+len((*left)[pos+1:]))
		copy(leftAlt, (*left)[:pos])
		leftAlt = append(leftAlt, (*left)[pos+1:]...)
		sameName := PatternList{}
		for _, a := range *collected {
			if a.Name == p.Name {
				sameName = append(sameName, a)
			}
		}

		switch p.Value.(type) {
		case int, []string:
			switch p.Value.(type) {
			case int:
				increment = 1
			case []string:
				switch match.Value.(type) {
				case string:
					increment = []string{match.Value.(string)}
				default:
					increment = match.Value
				}
			}
			if len(sameName) == 0 {
				match.Value = increment
				collectedMatch := make(PatternList, len(*collected), len(*collected)+1)
				copy(collectedMatch, *collected)
				collectedMatch = append(collectedMatch, match)
				return true, &leftAlt, &collectedMatch
			}
			switch sameName[0].Value.(type) {
			case int:
				sameName[0].Value = sameName[0].Value.(int) + increment.(int)
			case []string:
				sameName[0].Value = append(sameName[0].Value.([]string), increment.([]string)...)
			}
			return true, &leftAlt, collected
		}
		collectedMatch := make(PatternList, len(*collected), len(*collected)+1)
		copy(collectedMatch, *collected)
		collectedMatch = append(collectedMatch, match)
		return true, &leftAlt, &collectedMatch
	}
	panic("unmatched type")
}

func (p *Pattern) singleMatch(left *PatternList) (int, *Pattern) {
	if p.T&patternArgument != 0 {
		for n, pat := range *left {
			if pat.T&patternArgument != 0 {
				return n, newArgument(p.Name, pat.Value)
			}
		}
		return -1, nil
	} else if p.T&patternCommand != 0 {
		for n, pat := range *left {
			if pat.T&patternArgument != 0 {
				if pat.Value == p.Name {
					return n, newCommand(p.Name, true)
				}
				break
			}
		}
		return -1, nil
	} else if p.T&patternOption != 0 {
		for n, pat := range *left {
			if p.Name == pat.Name {
				return n, pat
			}
		}
		return -1, nil
	}
	panic("unmatched type")
}

func (p *Pattern) String() string {
	if p.T&patternOption != 0 {
		return fmt.Sprintf("%s(%s, %s, %d, %+v)", p.T, p.Short, p.Long, p.Argcount, p.Value)
	} else if p.T&patternLeaf != 0 {
		return fmt.Sprintf("%s(%s, %+v)", p.T, p.Name, p.Value)
	} else if p.T&patternBranch != 0 {
		result := ""
		for i, child := range p.Children {
			if i > 0 {
				result += ", "
			}
			result += child.String()
		}
		return fmt.Sprintf("%s(%s)", p.T, result)
	}
	panic("unmatched type")
}

func (p *Pattern) transform() *Pattern {
	/*
		Expand pattern into an (almost) equivalent one, but with single Either.

		Example: ((-a | -b) (-c | -d)) => (-a -c | -a -d | -b -c | -b -d)
		Quirks: [-a] => (-a), (-a...) => (-a -a)
	*/
	result := []PatternList{}
	groups := []PatternList{PatternList{p}}
	parents := patternRequired +
		patternOptionAL +
		patternOptionSSHORTCUT +
		patternEither +
		patternOneOrMore
	for len(groups) > 0 {
		children := groups[0]
		groups = groups[1:]
		var child *Pattern
		for _, c := range children {
			if c.T&parents != 0 {
				child = c
				break
			}
		}
		if child != nil {
			children.remove(child)
			if child.T&patternEither != 0 {
				for _, c := range child.Children {
					r := PatternList{}
					r = append(r, c)
					r = append(r, children...)
					groups = append(groups, r)
				}
			} else if child.T&patternOneOrMore != 0 {
				r := PatternList{}
				r = append(r, child.Children.double()...)
				r = append(r, children...)
				groups = append(groups, r)
			} else {
				r := PatternList{}
				r = append(r, child.Children...)
				r = append(r, children...)
				groups = append(groups, r)
			}
		} else {
			result = append(result, children)
		}
	}
	either := PatternList{}
	for _, e := range result {
		either = append(either, newRequired(e...))
	}
	return newEither(either...)
}

func (p *Pattern) eq(other *Pattern) bool {
	return reflect.DeepEqual(p, other)
}

func (pl PatternList) unique() PatternList {
	table := make(map[string]bool)
	result := PatternList{}
	for _, v := range pl {
		if !table[v.String()] {
			table[v.String()] = true
			result = append(result, v)
		}
	}
	return result
}

func (pl PatternList) index(p *Pattern) (int, error) {
	for i, c := range pl {
		if c.eq(p) {
			return i, nil
		}
	}
	return -1, newError("%s not in list", p)
}

func (pl PatternList) count(p *Pattern) int {
	count := 0
	for _, c := range pl {
		if c.eq(p) {
			count++
		}
	}
	return count
}

func (pl PatternList) diff(l PatternList) PatternList {
	lAlt := make(PatternList, len(l))
	copy(lAlt, l)
	result := make(PatternList, 0, len(pl))
	for _, v := range pl {
		if v != nil {
			match := false
			for i, w := range lAlt {
				if w.eq(v) {
					match = true
					lAlt[i] = nil
					break
				}
			}
			if match == false {
				result = append(result, v)
			}
		}
	}
	return result
}

func (pl PatternList) double() PatternList {
	l := len(pl)
	result := make(PatternList, l*2)
	copy(result, pl)
	copy(result[l:2*l], pl)
	return result
}

func (pl *PatternList) remove(p *Pattern) {
	(*pl) = pl.diff(PatternList{p})
}

func (pl PatternList) dictionary() map[string]interface{} {
	dict := make(map[string]interface{})
	for _, a := range pl {
		dict[a.Name] = a.Value
	}
	return dict
}
