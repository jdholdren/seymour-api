package v1

import (
	"encoding/json"
	"fmt"
	"time"
)

// Date represents a date block over a time instant.
//
// Used for the API to accept dates instead of full blown timestamps.
// Also reuses it's location, not standardized to a given timezone.
type Date time.Time

// ParseDate parses a string in the 2006-01-02 format into a Date.
func ParseDate(s string) (Date, error) {
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		return Date{}, fmt.Errorf("error parsing date: %w", err)
	}

	return Date(t), nil
}

// MarshalJSON implements [json.Marshaler].
func (d Date) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Time(d).Format(time.DateOnly))
}

// UnmarshalJSON implements [json.Unmarshaler].
func (d *Date) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return fmt.Errorf("error unmarshaling date: %w", err)
	}

	parsed, err := ParseDate(s)
	if err != nil {
		return err
	}

	*d = parsed
	return nil
}

// IsZero reports whether d is the zero Date, useful for treating an unset
// Date as a plain value instead of needing a *Date.
func (d Date) IsZero() bool {
	return time.Time(d).IsZero()
}

// Beginning returns the instant of the date at midnight.
func (d Date) Beginning() time.Time {
	t := time.Time(d)
	y, m, day := t.Date()

	return time.Date(y, m, day, 0, 0, 0, 0, t.Location())
}

// End returns the instant of the date at 23:59:59.999999999.
func (d Date) End() time.Time {
	t := time.Time(d)
	y, m, day := t.Date()

	return time.Date(y, m, day, 23, 59, 59, 999999999, t.Location())
}
