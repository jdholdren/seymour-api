package seymour

import (
	"database/sql/driver"
	"fmt"
	"time"
)

// DBTime is a sqlite-acceptable implementation of a time that can be marshaled in and out of
// a sqlite db.
type DBTime struct {
	Time time.Time
}

// Value implements [driver.Valuer].
func (t DBTime) Value() (driver.Value, error) {
	if t.Time.IsZero() {
		return nil, nil
	}

	return t.Time.Format(time.RFC3339), nil
}

// Scan implements the [sql.Scanner] interface.
func (t *DBTime) Scan(value any) error {
	if value == nil {
		return nil
	}

	switch v := value.(type) {
	case time.Time:
		t.Time = v
	case string:
		// Try to parse in the correct format:
		parsed, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return fmt.Errorf("error parsing time format: %s", v)
		}

		t.Time = parsed
	default:
		return fmt.Errorf("unsupported type for Time.Scan: %T", value)
	}

	return nil
}
