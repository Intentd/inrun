package post

import (
	"github.com/intentd/inrun/pkg/util/fileio"
	"github.com/intentd/inrun/pkg/util/term"
)

var (
	readLocal    = fileio.ReadLocal
	startSpinner = term.StartSpinner
	successMark  = term.SuccessMark
)
