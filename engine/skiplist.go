package engine

import (
	"math"
	"math/rand"
	"time"

	"github.com/rs/zerolog/log"
)

// const p = 0.5

const DefaultMaxLevel = 32
const DefaultProbability float64 = 1 / math.E

type node struct {
	forward  []*node
	backward *node
	key      uint64
	value    *PricePoint
}

func (n *node) next() *node {
	if len(n.forward) == 0 {
		return nil
	}
	return n.forward[0]
}

func (n *node) previous() *node {
	return n.backward
}

func (n *node) hasNext() bool {
	return n.next() != nil
}

func (n *node) hasPrevious() bool {
	return n.previous() != nil
}

type SkipList struct {
	lessThan    func(l, r uint64) bool
	header      *node
	footer      *node
	length      int
	randSource  rand.Source
	probability float64
	probTable   []float64
	MaxLevel    int
}

func (list *SkipList) SetProbability(newProbability float64) {
	list.probability = newProbability
	list.probTable = probabilityTable(list.probability, list.MaxLevel)
}

func probabilityTable(probability float64, maxLevel int) (table []float64) {
	for i := 1; i <= maxLevel; i++ {
		prob := math.Pow(probability, float64(i-1))
		table = append(table, prob)
	}
	return table
}

func (s *SkipList) Len() int {
	return s.length
}

type Iterator interface {
	Next() (ok bool)
	Previous() (ok bool)
	Key() uint64
	Value() *PricePoint
	Seek(key uint64) (ok bool)
	Close()
}

type iter struct {
	current *node
	key     uint64
	list    *SkipList
	value   *PricePoint
}

func (i iter) Key() uint64 {
	return i.key
}

func (i iter) Value() *PricePoint {
	return i.value
}

func (i *iter) Next() bool {
	if !i.current.hasNext() {
		return false
	}

	i.current = i.current.next()
	i.key = i.current.key
	i.value = i.current.value

	return true
}

func (i *iter) Previous() bool {
	if !i.current.hasPrevious() {
		return false
	}

	i.current = i.current.previous()
	i.key = i.current.key
	i.value = i.current.value

	return true
}

func (i *iter) Seek(key uint64) (ok bool) {
	current := i.current
	list := i.list

	if current == nil {
		current = list.header
	}

	if current.key != 0 && key < current.key {
		current = list.header
	}

	if current.backward == nil {
		current = list.header
	} else {
		current = current.backward
	}

	current = list.getPath(current, nil, key)

	if current == nil {
		return
	}

	i.current = current
	i.key = current.key
	i.value = current.value

	return true
}

func (i *iter) Close() {
	i.key = 0
	i.value = nil
	i.current = nil
	i.list = nil
}

type rangeIterator struct {
	iter
	upperLimit uint64
	lowerLimit uint64
}

func (i *rangeIterator) Next() bool {
	if !i.current.hasNext() {
		return false
	}

	next := i.current.next()

	if next.key >= i.upperLimit {
		return false
	}

	i.current = i.current.next()
	i.key = i.current.key
	i.value = i.current.value
	return true
}

func (i *rangeIterator) Previous() bool {
	if !i.current.hasPrevious() {
		return false
	}

	previous := i.current.previous()

	if previous.key < i.lowerLimit {
		return false
	}

	i.current = i.current.previous()
	i.key = i.current.key
	i.value = i.current.value
	return true
}

func (i *rangeIterator) Seek(key uint64) (ok bool) {
	if key < i.lowerLimit {
		return
	} else if key >= i.upperLimit {
		return
	}

	return i.iter.Seek(key)
}

func (i *rangeIterator) Close() {
	i.iter.Close()
	i.upperLimit = 0
	i.lowerLimit = 0
}

func (s *SkipList) Iterator() Iterator {
	return &iter{
		current: s.header,
		list:    s,
	}
}

func (s *SkipList) Seek(key uint64) Iterator {
	current := s.getPath(s.header, nil, key)
	if current == nil {
		return nil
	}

	return &iter{
		current: current,
		key:     current.key,
		list:    s,
		value:   current.value,
	}
}

func (s *SkipList) SeekToFirst() Iterator {
	if s.length == 0 {
		return nil
	}

	current := s.header.next()

	return &iter{
		current: current,
		key:     current.key,
		list:    s,
		value:   current.value,
	}
}

func (s *SkipList) SeekToLast() Iterator {
	current := s.footer
	if current == nil {
		return nil
	}

	return &iter{
		current: current,
		key:     current.key,
		list:    s,
		value:   current.value,
	}
}

func (s *SkipList) Range(from, to uint64) Iterator {
	start := s.getPath(s.header, nil, from)
	return &rangeIterator{
		iter: iter{
			current: &node{
				forward:  []*node{start},
				backward: start,
			},
			list: s,
		},
		upperLimit: to,
		lowerLimit: from,
	}
}

func (s *SkipList) level() int {
	return len(s.header.forward) - 1
}

func maxInt(x, y int) int {
	if x > y {
		return x
	}
	return y
}

func (s *SkipList) effectiveMaxLevel() int {
	return maxInt(s.level(), s.MaxLevel)
}

func (list *SkipList) randomLevel() (level int) {
	r := float64(list.randSource.Int63()) / (1 << 63)

	level = 1
	for level < list.MaxLevel && r < list.probTable[level] {
		level++
	}
	return
}

func (s *SkipList) Get(key uint64) (value *PricePoint, ok bool) {
	candidate := s.getPath(s.header, nil, key)

	if candidate == nil || candidate.key != key {
		return nil, false
	}

	return candidate.value, true
}

func (s *SkipList) getPath(current *node, update []*node, key uint64) *node {
	depth := len(current.forward) - 1

	for i := depth; i >= 0; i-- {
		for current.forward[i] != nil && current.forward[i].key < key {
			current = current.forward[i]
		}
		if update != nil {
			update[i] = current
		}
	}
	return current.next()
}

func (s *SkipList) Set(key uint64, value *PricePoint) {
	if key == 0 {
		orderID := uint64(0)
		if value != nil && len(value.Entries) >= 1 {
			orderID = value.Entries[0].ID
		}
		log.Error().
			Str("section", "internal:data").
			Str("action", "set").
			Uint64("key", key).
			Uint64("order_id", orderID).
			Msg("nil/0 keys are not supported")
		return
	}
	update := make([]*node, s.level()+1, s.effectiveMaxLevel()+1)
	candidate := s.getPath(s.header, update, key)

	if candidate != nil && candidate.key == key {
		candidate.value = value
		return
	}

	newLevel := s.randomLevel()

	if currentLevel := s.level(); newLevel > currentLevel {
		for i := currentLevel + 1; i <= newLevel; i++ {
			update = append(update, s.header)
			s.header.forward = append(s.header.forward, nil)
		}
	}

	newNode := &node{
		forward: make([]*node, newLevel+1, s.effectiveMaxLevel()+1),
		key:     key,
		value:   value,
	}

	if previous := update[0]; previous.key != 0 {
		newNode.backward = previous
	}

	for i := 0; i <= newLevel; i++ {
		newNode.forward[i] = update[i].forward[i]
		update[i].forward[i] = newNode
	}

	s.length++

	if newNode.forward[0] != nil {
		if newNode.forward[0].backward != newNode {
			newNode.forward[0].backward = newNode
		}
	}

	if s.footer == nil || s.footer.key < key {
		s.footer = newNode
	}
}

func (s *SkipList) Delete(key uint64) (value *PricePoint, ok bool) {
	if key == 0 {
		log.Error().
			Str("section", "internal:data").
			Str("action", "delete").
			Uint64("key", key).
			Msg("nil/0 keys are not supported")
		return
	}
	update := make([]*node, s.level()+1, s.effectiveMaxLevel())
	candidate := s.getPath(s.header, update, key)

	if candidate == nil || candidate.key != key {
		return nil, false
	}

	previous := candidate.backward
	if s.footer == candidate {
		s.footer = previous
	}

	next := candidate.next()
	if next != nil {
		next.backward = previous
	}

	for i := 0; i <= s.level() && update[i].forward[i] == candidate; i++ {
		update[i].forward[i] = candidate.forward[i]
	}

	for s.level() > 0 && s.header.forward[s.level()] == nil {
		s.header.forward = s.header.forward[:s.level()]
	}
	s.length--

	return candidate.value, true
}

func NewCustomMap(lessThan func(l, r uint64) bool) *SkipList {
	return &SkipList{
		lessThan: lessThan,
		header: &node{
			forward: []*node{nil},
		},
		randSource:  rand.New(rand.NewSource(time.Now().UnixNano())),
		MaxLevel:    DefaultMaxLevel,
		probability: DefaultProbability,
		probTable:   probabilityTable(DefaultProbability, DefaultMaxLevel),
	}
}

// type Ordered interface {
// 	LessThan(other Ordered) bool
// }
