// pkg/note/cron_test.go
package note

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCronFieldExtractors(t *testing.T) {
	tests := []struct {
		name                                              string
		expr                                              string
		wantMinute, wantHour, wantDom, wantMonth, wantDow string
		wantErr                                           bool
	}{
		{"standard 5 fields", "*/5 0 * * 1", "*/5", "0", "*", "*", "1", false},
		{"@hourly macro", "@hourly", "0", "*", "*", "*", "*", false},
		{"@daily macro", "@daily", "0", "0", "*", "*", "*", false},
		{"@weekly macro", "@weekly", "0", "0", "*", "*", "0", false},
		{"@monthly macro", "@monthly", "0", "0", "1", "*", "*", false},
		{"@yearly macro", "@yearly", "0", "0", "1", "1", "*", false},
		{"empty string", "", "*", "*", "*", "*", "*", false},
		{"invalid field count", "0 0 * *", "*", "*", "*", "*", "*", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			min, err := cronMinute(tt.expr)
			if (err != nil) != tt.wantErr {
				t.Errorf("cronMinute() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if min != tt.wantMinute {
				t.Errorf("cronMinute() = %q, want %q", min, tt.wantMinute)
			}
			hour, _ := cronHour(tt.expr)
			if hour != tt.wantHour {
				t.Errorf("cronHour() = %q, want %q", hour, tt.wantHour)
			}
			dom, _ := cronDom(tt.expr)
			if dom != tt.wantDom {
				t.Errorf("cronDom() = %q, want %q", dom, tt.wantDom)
			}
			month, _ := cronMonth(tt.expr)
			if month != tt.wantMonth {
				t.Errorf("cronMonth() = %q, want %q", month, tt.wantMonth)
			}
			dow, _ := cronDow(tt.expr)
			if dow != tt.wantDow {
				t.Errorf("cronDow() = %q, want %q", dow, tt.wantDow)
			}
		})
	}
}

func TestCronField(t *testing.T) {
	tests := []struct {
		expr string
		pos  int
		want string
		err  bool
	}{
		{"*/5 0 * * 1", 0, "*/5", false},
		{"*/5 0 * * 1", 1, "0", false},
		{"*/5 0 * * 1", 2, "*", false},
		{"*/5 0 * * 1", 3, "*", false},
		{"*/5 0 * * 1", 4, "1", false},
		{"", 0, "*", false},
		{"@hourly", 0, "0", false},
		{"  invalid count  ", 0, "*", true},
		{"*/5 0 * * 1", 5, "*", true},
	}
	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			got, err := cronField(tt.expr, tt.pos)
			if (err != nil) != tt.err {
				t.Errorf("cronField() error = %v, wantErr %v", err, tt.err)
				return
			}
			if got != tt.want {
				t.Errorf("cronField() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCronExpr(t *testing.T) {
	tests := []struct {
		min, hour, dom, month, dow string
		want                       string
	}{
		{"*/5", "0", "*", "*", "1", "*/5 0 * * 1"},
		{"", "0", "*", "*", "1", "* 0 * * 1"},
		{"*/5", "", "*", "*", "1", "*/5 * * * 1"},
	}
	for _, tt := range tests {
		got := cronExpr(tt.min, tt.hour, tt.dom, tt.month, tt.dow)
		if got != tt.want {
			t.Errorf("cronExpr(%q,%q,%q,%q,%q) = %q, want %q", tt.min, tt.hour, tt.dom, tt.month, tt.dow, got, tt.want)
		}
	}
}

func TestCronValid(t *testing.T) {
	tests := []struct {
		expr string
		want bool
	}{
		{"*/5 0 * * 1", true},
		{"@hourly", true},
		{"", false},
		{"0 0 * *", false},
		{"invalid", false},
	}
	for _, tt := range tests {
		if got := cronValid(tt.expr); got != tt.want {
			t.Errorf("cronValid(%q) = %v, want %v", tt.expr, got, tt.want)
		}
	}
}

func TestCronFromMapStrict(t *testing.T) {
	tests := []struct {
		name string
		v    interface{}
		want string
		err  bool
	}{
		{"valid map", map[string]interface{}{"minute": "*/5", "hour": "0", "dayOfMonth": "*", "month": "*", "dayOfWeek": "1"}, "*/5 0 * * 1", false},
		{"missing fields", map[string]interface{}{"minute": "*/5"}, "*/5 * * * *", false},
		{"not a map", "string", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := cronFromMapStrict(tt.v)
			if (err != nil) != tt.err {
				t.Errorf("cronFromMapStrict() error = %v, wantErr %v", err, tt.err)
				return
			}
			if got != tt.want {
				t.Errorf("cronFromMapStrict() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCronFromMapAny(t *testing.T) {
	tests := []struct {
		name string
		v    interface{}
		want string
	}{
		{"map", map[string]interface{}{"minute": "*/5", "hour": "0", "dayOfMonth": "*", "month": "*", "dayOfWeek": "1"}, "*/5 0 * * 1"},
		{"cron string", "*/5 0 * * 1", "*/5 0 * * 1"},
		{"@hourly", "@hourly", "0 * * * *"},
		{"empty", nil, "* * * * *"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cronFromMapAny(tt.v); got != tt.want {
				t.Errorf("cronFromMapAny(%v) = %q, want %q", tt.v, got, tt.want)
			}
		})
	}
}

func TestCronToMapTemplate(t *testing.T) {
	expr := "*/5 0 * * 1"
	got, err := cronToMapTemplate(expr)
	if err != nil {
		t.Fatalf("cronToMapTemplate() error = %v", err)
	}
	if !strings.HasPrefix(got, CronMapSentinel) {
		t.Errorf("cronToMapTemplate() missing sentinel prefix: %q", got)
	}
	jsonPart := strings.TrimPrefix(got, CronMapSentinel)
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(jsonPart), &m); err != nil {
		t.Errorf("cronToMapTemplate() JSON unmarshal error: %v", err)
	}
	if m["minute"] != "*/5" {
		t.Errorf("minute = %v, want */5", m["minute"])
	}
}

func TestCronNormalize(t *testing.T) {
	tests := []struct {
		expr string
		want string
	}{
		{"*/5 0 * * 1", "*/5 0 * * 1"},
		{"@hourly", "0 * * * *"},
		{"", "* * * * *"},
		{"0 0 * *", "* * * * *"},
		{"  0 0 * * *  ", "0 0 * * *"},
	}
	for _, tt := range tests {
		if got := cronNormalize(tt.expr); got != tt.want {
			t.Errorf("cronNormalize(%q) = %q, want %q", tt.expr, got, tt.want)
		}
	}
}

func TestCronDescribe(t *testing.T) {
	tests := []struct {
		expr string
		want string
	}{
		{"*/5 * * * *", "Every 5 minutes"},
		{"0 * * * *", "Every hour"},
		{"30 2 * * *", "At 2:30 every day"},
		{"0 2 * * 1", "At 2:0 on day-of-week 1"},
		{"0 2 15 * *", "At 2:0 on day 15 of every month"},
		{"invalid", "Invalid cron expression"},
	}
	for _, tt := range tests {
		if got := cronDescribe(tt.expr); got != tt.want {
			t.Errorf("cronDescribe(%q) = %q, want %q", tt.expr, got, tt.want)
		}
	}
}
