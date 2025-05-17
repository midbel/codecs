package main

type TransformCmd struct {
}

func (c TransformCmd) Run(args []string) error {
	set := flag.NewFlagSet("transform", flag.ContinueOnError)

	if err := set.Parse(args); err != nil {
		return err
	}
	return nil
}
