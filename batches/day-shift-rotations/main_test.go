package main

import (
	"testing"
	"time"
)

func day(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 15, 30, 0, 0, time.UTC)
}

func TestConnectionDefaults(t *testing.T) {
	for _, tc := range []struct{ host, port, want string }{
		{"", "", "localhost:3306"},
		{"db", "", "db:3306"},
		{"db", "3307", "db:3307"},
	} {
		if got := hostPort(tc.host, tc.port); got != tc.want {
			t.Errorf("hostPort(%q, %q) = %q, want %q", tc.host, tc.port, got, tc.want)
		}
	}
	if got := valueOr("  ", "fallback"); got != "fallback" {
		t.Fatalf("valueOr blank = %q", got)
	}
	if got := valueOr("value", "fallback"); got != "value" {
		t.Fatalf("valueOr value = %q", got)
	}
}

func TestDateHelpers(t *testing.T) {
	friday := day(2026, time.September, 25)
	holiday := day(2026, time.September, 28)
	isHoliday := func(d time.Time) bool { return dateOnly(d).Equal(dateOnly(holiday)) }
	if got := nextBusinessDayAfter(friday, isHoliday); !got.Equal(day(2026, time.September, 29).Truncate(24 * time.Hour)) {
		t.Fatalf("next business day = %v", got)
	}
	if got := dateOnly(friday); got.Hour() != 0 || got.Minute() != 0 {
		t.Fatalf("dateOnly = %v", got)
	}
	if got := maxDate(friday, holiday); !got.Equal(holiday) {
		t.Fatalf("maxDate = %v", got)
	}
	if got := maxDate(holiday, friday); !got.Equal(holiday) {
		t.Fatalf("maxDate reverse = %v", got)
	}
	if got := weekdayIndex(day(2026, time.September, 21)); got != 0 {
		t.Fatalf("Monday index = %d", got)
	}
	if got := weekdayIndex(day(2026, time.September, 25)); got != 4 {
		t.Fatalf("Friday index = %d", got)
	}
	if got := weekdayIndex(day(2026, time.September, 27)); got != 0 {
		t.Fatalf("Sunday index = %d", got)
	}
}

func TestBusinessDaysBetween(t *testing.T) {
	neverHoliday := func(time.Time) bool { return false }
	if got := businessDaysBetween(day(2026, time.September, 21), day(2026, time.September, 25), neverHoliday); got != 4 {
		t.Fatalf("weekday distance = %d, want 4", got)
	}
	if got := businessDaysBetween(day(2026, time.September, 25), day(2026, time.September, 28), neverHoliday); got != 1 {
		t.Fatalf("weekend distance = %d, want 1", got)
	}
	if got := businessDaysBetween(day(2026, time.September, 28), day(2026, time.September, 25), neverHoliday); got != 0 {
		t.Fatalf("reverse distance = %d, want 0", got)
	}
	allHoliday := func(time.Time) bool { return true }
	if got := businessDaysBetween(day(2026, time.September, 21), day(2026, time.September, 25), allHoliday); got != 0 {
		t.Fatalf("holiday distance = %d, want 0", got)
	}
}

func TestFetchHolidaysReturnsEmptyCalendar(t *testing.T) {
	holidays, err := fetchHolidays(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(holidays) != 0 {
		t.Fatalf("holidays = %#v", holidays)
	}
}
