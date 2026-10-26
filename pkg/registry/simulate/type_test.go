package simulate

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResult_AddFoldsAssertionsAndSteadyState(t *testing.T) {
	steady := func(at, passed, failed int) *Result {
		r := &Result{Steady: at > 0, SteadyAt: at}
		r.Assertions.Total, r.Assertions.Passed, r.Assertions.Failed = passed+failed, passed, failed
		r.Assertions.Cases = []CaseResult{{Field: "steady"}}
		return r
	}

	suite := &Result{}
	suite.Add(steady(3, 4, 0))
	suite.Add(steady(5, 2, 1))
	assert.True(t, suite.Steady, "every run folded in is steady")
	assert.Equal(t, 5, suite.SteadyAt, "at the latest of their steady cycles")
	assert.Equal(t, AssertionSummary{Total: 7, Passed: 6, Failed: 1, Cases: []CaseResult{{Field: "steady"}, {Field: "steady"}}}, suite.Assertions)

	suite.Add(steady(0, 1, 0))
	assert.False(t, suite.Steady, "one run that never settled makes the fold unsteady")
	assert.Zero(t, suite.SteadyAt)

	suite.Add(nil)
	assert.Equal(t, 8, suite.Assertions.Total, "nil adds nothing")
}

func TestResult_AddImportRecordsTheFile(t *testing.T) {
	suite := &Result{}
	suite.AddImport("a/simulate.yaml", &Result{Steady: true, SteadyAt: 2}, nil)
	suite.AddImport("b/simulate.yaml", nil, errors.New("parsing b"))

	assert.Len(t, suite.Imports, 2)
	assert.Equal(t, "b/simulate.yaml", suite.Imports[1].Path)
	assert.EqualError(t, suite.Imports[1].Err, "parsing b")
	assert.True(t, suite.Steady, "a file that failed to load adds no result")
}
