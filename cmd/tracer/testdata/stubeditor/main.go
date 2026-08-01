// Command stubeditor stands in for $EDITOR in tests: it overwrites the file
// named in its one argument with the content of the file named by the
// TRACER_TEST_EDITOR_CONTENT environment variable, so a test controls
// exactly what "the learner typed" without a real interactive editor.
package main

import (
	"io"
	"os"
)

func main() {
	src, err := os.Open(os.Getenv("TRACER_TEST_EDITOR_CONTENT"))
	if err != nil {
		panic(err)
	}
	defer src.Close()

	dst, err := os.Create(os.Args[1])
	if err != nil {
		panic(err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		panic(err)
	}
}
