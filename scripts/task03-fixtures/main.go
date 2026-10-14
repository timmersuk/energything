// Creates new isolated synthetic databases; never accepts an existing data path.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/timmersuk/energything/internal/storage"
	"github.com/timmersuk/energything/internal/testsupport"
)

func main() {
	if err := os.MkdirAll(".cache", 0755); err != nil {
		panic(err)
	}
	root, err := os.MkdirTemp(".cache", "task03-browser-")
	if err != nil {
		panic(err)
	}
	for _, name := range []string{"empty", "fixtures"} {
		dir := filepath.Join(root, name)
		s, err := storage.Open(context.Background(), dir)
		if err != nil {
			panic(err)
		}
		if name == "fixtures" {
			err = testsupport.SeedBrowsing(context.Background(), s)
			if err != nil {
				panic(err)
			}
		}
		if err = s.Close(); err != nil {
			panic(err)
		}
	}
	absolute, _ := filepath.Abs(root)
	fmt.Println(absolute)
}
