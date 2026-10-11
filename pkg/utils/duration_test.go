// Tests for ParseTimeDuration (duration.go).
package utils_test

import (
	"testing"
	"time"

	"github.com/intentd/inrun/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseTimeDuration_Empty(t *testing.T) {
	_, err := utils.ParseTimeDuration("")
	assert.Error(t, err)
}

func TestParseTimeDuration_Seconds(t *testing.T) {
	d, err := utils.ParseTimeDuration("30s")
	require.NoError(t, err)
	assert.Equal(t, 30*time.Second, d)
}

func TestParseTimeDuration_Minutes(t *testing.T) {
	d, err := utils.ParseTimeDuration("5m")
	require.NoError(t, err)
	assert.Equal(t, 5*time.Minute, d)
}

func TestParseTimeDuration_Hours(t *testing.T) {
	d, err := utils.ParseTimeDuration("12h")
	require.NoError(t, err)
	assert.Equal(t, 12*time.Hour, d)
}

func TestParseTimeDuration_Days(t *testing.T) {
	d, err := utils.ParseTimeDuration("10d")
	require.NoError(t, err)
	assert.Equal(t, 10*24*time.Hour, d)
}

func TestParseTimeDuration_Weeks(t *testing.T) {
	d, err := utils.ParseTimeDuration("2w")
	require.NoError(t, err)
	assert.Equal(t, 14*24*time.Hour, d)
}

func TestParseTimeDuration_Months(t *testing.T) {
	d, err := utils.ParseTimeDuration("3mo")
	require.NoError(t, err)
	assert.Equal(t, 90*24*time.Hour, d)
}

func TestParseTimeDuration_Years(t *testing.T) {
	d, err := utils.ParseTimeDuration("1y")
	require.NoError(t, err)
	assert.Equal(t, 365*24*time.Hour, d)
}

func TestParseTimeDuration_FractionalYear(t *testing.T) {
	d, err := utils.ParseTimeDuration("0.5y")
	require.NoError(t, err)
	// 0.5 * 365 days
	assert.Equal(t, time.Duration(0.5*float64(365*24*time.Hour)), d)
}

func TestParseTimeDuration_FractionalMonths(t *testing.T) {
	d, err := utils.ParseTimeDuration("1.5mo")
	require.NoError(t, err)
	assert.Equal(t, time.Duration(1.5*float64(30*24*time.Hour)), d)
}

func TestParseTimeDuration_InvalidYear(t *testing.T) {
	_, err := utils.ParseTimeDuration("xy")
	assert.Error(t, err)
}

func TestParseTimeDuration_InvalidDay(t *testing.T) {
	_, err := utils.ParseTimeDuration("xd")
	assert.Error(t, err)
}

func TestParseTimeDuration_InvalidGoSyntax(t *testing.T) {
	_, err := utils.ParseTimeDuration("not-a-duration")
	assert.Error(t, err)
}

func TestParseTimeDuration_Never(t *testing.T) {
	d, err := utils.ParseTimeDuration("never")
	require.NoError(t, err)
	assert.Equal(t, time.Duration(0), d)
}
