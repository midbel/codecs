package main

import (
	"fmt"
	"time"

	"github.com/midbel/codecs/cmd/cli"
)

func main() {
	fmt.Println("start")
	spin := cli.NewSpinner()
	spin.SetMessage("in progress...")
	spin.Start()
	time.Sleep(3 * time.Second)
	spin.Stop()
	fmt.Print("done")
}
