package xproc

type Pipeline struct{}

func Load() (*Pipeline, error) {
	var pipe Pipeline
	return &pipe, nil
}
