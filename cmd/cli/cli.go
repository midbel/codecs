package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"maps"
	"os"
	"slices"

	"github.com/midbel/distance"
)

func NewFlagSet(name string) *flag.FlagSet {
	set := flag.NewFlagSet(name, flag.ContinueOnError)
	set.SetOutput(io.Discard)
	return set
}

type SuggestionError struct {
	Name   string
	Others []string
}

func (e SuggestionError) Error() string {
	return fmt.Sprintf("%s: unknown sub command", e.Name)
}

type DelegateError struct {
	Command string
	Args    []string
}

func (e DelegateError) Error() string {
	return fmt.Sprintf("delegate to command %s", e.Command)
}

type Command struct {
	Name    string
	Alias   []string
	Summary string
	Help    string
	Handler
}

func Help(summary, help string) *Command {
	return &Command{
		Summary: summary,
		Help:    help,
		Handler: helpHandler{},
	}
}

func Delegate(to string) *Command {
	return &Command{
		Handler: delegateHandler{},
	}
}

func (c *Command) getHelp() string {
	return c.Help
}

func (c *Command) getSummary() string {
	return c.Summary
}

func (c *Command) getAliases() []string {
	return c.Alias
}

func printHelp(w io.Writer, summary, help string) {
	if summary != "" {
		fmt.Fprintln(w, summary)
		fmt.Fprintln(w)
	}
	if help != "" {
		fmt.Fprintln(w, help)
		fmt.Fprintln(w)
	}
}

type Handler interface {
	Run([]string) error
}

type helpHandler struct{}

func (helpHandler) Run(_ []string) error {
	return flag.ErrHelp
}

type delegateHandler struct {
	To string
}

func (d delegateHandler) Run(args []string) error {
	err := DelegateError{
		Command: d.To,
		Args:    args,
	}
	return err
}

type CommandNode struct {
	Name     string
	Children map[string]*CommandNode
	cmd      *Command
}

func createNode(name string) *CommandNode {
	return &CommandNode{
		Name:     name,
		Children: make(map[string]*CommandNode),
	}
}

func (c CommandNode) Help() {
	printHelp(os.Stderr, c.cmd.getSummary(), c.cmd.getHelp())
	fmt.Fprintln(os.Stderr, "available sub command(s)")
	for s, n := range c.Children {
		fmt.Printf("- %s: %s", s, n.cmd.getSummary())
		fmt.Fprintln(os.Stderr)
	}
}

type CommandTrie struct {
	root    *CommandNode
	summary string
	help    string
}

func New() *CommandTrie {
	trie := CommandTrie{
		root: createNode(""),
	}
	return &trie
}

func (t *CommandTrie) SetSummary(summary string) {
	t.summary = summary
}

func (t *CommandTrie) SetHelp(help string) {
	t.help = help
}

func (t *CommandTrie) Register(paths []string, cmd *Command) error {
	node := t.root
	for _, name := range paths {
		if node.Children[name] == nil {
			node.Children[name] = createNode(name)
		}
		node = node.Children[name]
	}
	node.cmd = cmd
	return nil
}

func (t *CommandTrie) Help() {
	if t.summary != "" {
		fmt.Fprintln(os.Stderr, t.summary)
		fmt.Fprintln(os.Stderr)
	}
	if t.help != "" {
		fmt.Fprintln(os.Stderr, t.help)
		fmt.Fprintln(os.Stderr)
	}
	if len(t.root.Children) == 0 {
		return
	}
	fmt.Fprintln(os.Stderr, "Available commands")
	for k, n := range t.root.Children {
		fmt.Printf("- %s: %s", k, n.cmd.getSummary())
		fmt.Fprintln(os.Stderr)
	}
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
	if node.cmd == nil {
		var found bool
		for _, c := range node.Children {
			if c.cmd == nil {
				continue
			}
			found = slices.Contains(c.cmd.getAliases(), args[ix])
			if found {
				ix++
				node = c
				break
			}
		}
		if !found {
			list := slices.Collect(maps.Keys(node.Children))
			return t.sugget(args[ix], list)
		}
	}
	err := node.cmd.Run(args[ix:])
	if errors.Is(err, flag.ErrHelp) {
		node.Help()
		return nil
	}
	return err
}

func (t *CommandTrie) sugget(name string, others []string) error {
	return SuggestionError{
		Name:   name,
		Others: distance.Levenshtein(name, others),
	}
}
