package post

import (
	"github.com/intentd/tools/fileio"
	"github.com/intentd/tools/term"
)

var (
	readLocal    = fileio.ReadLocal
	startSpinner = term.StartSpinner
	successMark  = term.SuccessMark
)
