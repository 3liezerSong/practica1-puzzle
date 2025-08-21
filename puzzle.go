package main

import (
	"container/heap"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"time"
)

const (
	gridSize         = 3
	boardLen         = gridSize * gridSize // 9
	blankTile        = 0
	defaultMaxExpand = 0 // 0 = sin límite para A*
)

var (
	errInvalidSteps     = errors.New("steps must be >= 0")
	errNoSolution       = errors.New("solution not found")
	errUnknownHeuristic = errors.New("unknown heuristic")
)

type Heuristic int

const (
	heuristicManhattan Heuristic = iota
	heuristicMisplaced
)

var heuristicDisplayName = map[Heuristic]string{
	heuristicManhattan: "Manhattan",
	heuristicMisplaced: "Misplaced",
}

type State [boardLen]int

func Goal() State { return State{1, 2, 3, 4, 5, 6, 7, 8, blankTile} }

// String serializa el estado (útil como clave)
func (s State) String() string {
	var b strings.Builder
	for i, v := range s {
		if v == blankTile {
			b.WriteString("_")
		} else {
			b.WriteString(fmt.Sprintf("%d", v))
		}
		if i%gridSize == gridSize-1 && i != boardLen-1 {
			b.WriteString("|")
		} else if i != boardLen-1 {
			b.WriteString(",")
		}
	}
	return b.String()
}

// Neighbors genera estados vecinos moviendo el espacio vacío
func (s State) Neighbors() []State {
	zeroIndex := 0
	for i := 0; i < boardLen; i++ {
		if s[i] == blankTile {
			zeroIndex = i
			break
		}
	}
	row := zeroIndex / gridSize
	col := zeroIndex % gridSize

	type delta struct{ dr, dc int }
	allowedMoves := [...]delta{
		{dr: -1, dc: 0}, // arriba
		{dr: 1, dc: 0},  // abajo
		{dr: 0, dc: -1}, // izquierda
		{dr: 0, dc: 1},  // derecha
	}

	out := make([]State, 0, 4)
	for _, mv := range allowedMoves {
		newRow := row + mv.dr
		newCol := col + mv.dc
		if newRow < 0 || newRow >= gridSize || newCol < 0 || newCol >= gridSize {
			continue
		}
		newIndex := newRow*gridSize + newCol
		next := s
		next[zeroIndex], next[newIndex] = next[newIndex], next[zeroIndex]
		out = append(out, next)
	}
	return out
}

// Heurísticas

func heuristicCost(s State, kind Heuristic) (int, error) {
	switch kind {
	case heuristicMisplaced:
		count := 0
		for i, v := range s {
			if v == blankTile {
				continue
			}
			if v != i+1 {
				count++
			}
		}
		return count, nil
	case heuristicManhattan:
		sum := 0
		for i, v := range s {
			if v == blankTile {
				continue
			}
			target := v - 1
			x1, y1 := i%gridSize, i/gridSize
			x2, y2 := target%gridSize, target/gridSize
			sum += int(math.Abs(float64(x1-x2)) + math.Abs(float64(y1-y2)))
		}
		return sum, nil
	default:
		return 0, errUnknownHeuristic
	}
}

// A*

type node struct {
	state  State
	g, h   int
	index  int
	parent *node
}

type minQueue []*node

func (p minQueue) Len() int { return len(p) }
func (p minQueue) Less(i, j int) bool {
	fi := p[i].g + p[i].h
	fj := p[j].g + p[j].h
	if fi == fj {
		return p[i].h < p[j].h // desempate
	}
	return fi < fj
}
func (p minQueue) Swap(i, j int) { p[i], p[j] = p[j], p[i]; p[i].index = i; p[j].index = j }
func (p *minQueue) Push(x any)   { n := x.(*node); n.index = len(*p); *p = append(*p, n) }
func (p *minQueue) Pop() any {
	old := *p
	n := len(old)
	item := old[n-1]
	item.index = -1
	*p = old[:n-1]
	return item
}

type SearchResult struct {
	path     []State
	expanded int
	found    bool
}

// maxExpand = 0 ⇒ sin límite.
func Puzzle(start State, kind Heuristic, maxExpand int) (SearchResult, error) {
	open := &minQueue{}
	heap.Init(open)

	h0, err := heuristicCost(start, kind)
	if err != nil {
		return SearchResult{}, err
	}
	startNode := &node{state: start, g: 0, h: h0}
	heap.Push(open, startNode)

	cameFrom := map[string]*node{start.String(): startNode}
	closed := map[string]bool{}
	expanded := 0

	for open.Len() > 0 {
		current := heap.Pop(open).(*node)

		if current.state == Goal() {
			// reconstruir ruta
			reversed := make([]State, 0, current.g+1)
			for n := current; n != nil; n = n.parent {
				reversed = append(reversed, n.state)
			}
			// invertir
			for i, j := 0, len(reversed)-1; i < j; i, j = i+1, j-1 {
				reversed[i], reversed[j] = reversed[j], reversed[i]
			}
			return SearchResult{path: reversed, expanded: expanded, found: true}, nil
		}

		closed[current.state.String()] = true
		expanded++
		if maxExpand > 0 && expanded > maxExpand {
			break
		}

		for _, nb := range current.state.Neighbors() {
			key := nb.String()
			if closed[key] {
				continue
			}
			gScore := current.g + 1
			prev, seen := cameFrom[key]
			if seen && gScore >= prev.g {
				continue
			}
			hScore, err := heuristicCost(nb, kind)
			if err != nil {
				return SearchResult{}, err
			}
			next := &node{state: nb, g: gScore, h: hScore, parent: current}
			cameFrom[key] = next
			heap.Push(open, next)
		}
	}
	return SearchResult{found: false, expanded: expanded}, errNoSolution
}

// ShuffleFromGoal desordena con un “random walk” de 'steps' movimientos válidos
func ShuffleFromGoal(steps int) (State, error) {
	if steps < 0 {
		return State{}, errInvalidSteps
	}
	seed := time.Now().UnixNano()
	rng := rand.New(rand.NewSource(seed))

	state := Goal()
	for i := 0; i < steps; i++ {
		neighbors := state.Neighbors()
		state = neighbors[rng.Intn(len(neighbors))]
	}
	return state, nil
}

const (
	GridSize  = gridSize
	BoardLen  = boardLen
	BlankTile = blankTile
)
