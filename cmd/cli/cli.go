package cli

import (
	"fmt"
	"maps"
	"slices"

	"github.com/midbel/distance"
)

type SuggestionError struct {
	Name   string
	Others []string
}

func (e SuggestionError) Error() string {
	return fmt.Sprintf("%s: unknown subcommand", e.Name)
}

type Command struct {
	Name    string
	Alias   []string
	Summary string
	Help    string
	Handler
}

type Handler interface {
	Run([]string) error
}

type HandlerFunc func([]string) error

func (f HandlerFunc) Run(args []string) error {
	return f(args)
}

type CommandNode struct {
	Name     string
	Children map[string]*CommandNode
	Handler
}

func createNode(name string) *CommandNode {
	return &CommandNode{
		Name:     name,
		Children: make(map[string]*CommandNode),
	}
}

type CommandTrie struct {
	root *CommandNode
}

func New() *CommandTrie {
	trie := CommandTrie{
		root: createNode(""),
	}
	return &trie
}

func (t *CommandTrie) Register(paths []string, handler Handler) error {
	node := t.root
	for _, name := range paths {
		if node.Children[name] == nil {
			node.Children[name] = createNode(name)
		}
		node = node.Children[name]
	}
	node.Handler = handler
	return nil
}

func (t *CommandTrie) Execute(args []string) error {
	var (
		node = t.root
		ix   int
	)
	for _, name := range args {
		child := node.Children[name]
		if child == nil {
			break
		}
		node = child
		ix++
	}
	if node.Handler == nil {
		list := slices.Collect(maps.Keys(node.Children))
		return t.sugget(args[ix], list)
	}
	return node.Handler.Run(args[ix:])
}

func (t *CommandTrie) sugget(name string, others []string) error {
	return SuggestionError{
		Name:   name,
		Others: distance.Levenshtein(name, others),
	}
}
