package main

import (
	"github.com/midbel/cli"

	tea "charm.land/bubbletea/v2"
	"github.com/midbel/codecs/terminal"
)

var terminalQueryCmd = cli.Command{
	Name:    "query",
	Alias:   []string{"exec"},
	Summary: "Find nodes in xml document",
	Handler: &TerminalQueryCmd{},
}

type TerminalQueryCmd struct{}

func (c *TerminalQueryCmd) Run(args []string) error {
	set := cli.NewFlagSet("query")
	if err := set.Parse(args); err != nil {
		return err
	}
	p := tea.NewProgram(terminal.NewQueryModel(set.Arg(0)))
	if _, err := p.Run(); err != nil {
		return err
	}
	return nil
}
