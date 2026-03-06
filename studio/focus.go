package studio

import (
	tea "charm.land/bubbletea/v2"
)

type Stack struct {
	list []Screen
}

func NewStack() *Stack {
	return &Stack{}
}

func (k *Stack) Update(msg tea.Msg) tea.Cmd {
	if len(k.list) == 0 {
		return nil
	}
	curr := k.Current()
	if curr == nil {
		return nil
	}
	m, c := curr.Update(msg)
	k.list[len(k.list)-1] = m
	return c
}

func (k *Stack) Current() Screen {
	z := len(k.list)
	if z == 0 {
		return nil
	}
	return k.list[z-1]
}

func (k *Stack) Push(s Screen) {
	k.list = append(k.list, s)
}

func (k *Stack) Pop() {
	z := len(k.list)
	if z > 1 {
		k.list = k.list[:z-1]
	}
}

func (k *Stack) Len() int {
	return len(k.list)
}

type History[T any] struct {
	elements []T
	ptr      int
	count    int
}

func NewHistory[T any](size int) *History[T] {
	return &History[T]{
		elements: make([]T, size),
	}
}

func (h *History[T]) Push(elem T) {
	h.elements[h.ptr] = elem
	h.ptr++
	if h.ptr >= len(h.elements) {
		h.ptr = 0
	}
	if h.count < len(h.elements) {
		h.count++
	}
}

func (h *History[T]) Pop() (T, bool) {
	if h.count == 0 {
		var z T
		return z, false
	}
	h.ptr--
	if h.ptr < 0 {
		h.ptr = len(h.elements) - 1
	}
	h.count--
	return h.elements[h.ptr], true
}

func (h *History[T]) Count() int {
	return h.count
}

func (h *History[T]) Size() int {
	return len(h.elements)
}

func (h *History[T]) All() []T {
	if h.count == 0 {
		return nil
	}
	var list []T
	for i := h.ptr - h.count; i < len(h.elements); i++ {
		list = append(list, h.elements[i])
		if len(list) == h.count {
			break
		}
	}
	if len(list) == h.count {
		return list
	}
	for i := 0; i < h.ptr-1; i++ {
		list = append(list, h.elements[i])
		if len(list) == h.count {
			break
		}
	}
	return list
}

type Focusable interface {
	Focus() tea.Cmd
	Blur()
	Update(tea.Msg) tea.Cmd
}

type FocusRing struct {
	elements []Focusable
	current  int
}

func (r *FocusRing) Push(f Focusable) {
	r.elements = append(r.elements, f)
}

func (r *FocusRing) Next() tea.Cmd {
	if len(r.elements) == 0 {
		return nil
	}
	r.elements[r.current].Blur()
	r.current++
	if r.current >= len(r.elements) {
		r.current = 0
	}
	return r.elements[r.current].Focus()
}

func (r *FocusRing) Prev() tea.Cmd {
	if len(r.elements) == 0 {
		return nil
	}
	r.elements[r.current].Blur()
	r.current--
	if r.current < 0 {
		r.current = len(r.elements) - 1
	}
	return r.elements[r.current].Focus()
}

func (r *FocusRing) Update(msg tea.Msg) tea.Cmd {
	if len(r.elements) == 0 {
		return nil
	}
	cmd := r.elements[r.current].Update(msg)
	return cmd
}
