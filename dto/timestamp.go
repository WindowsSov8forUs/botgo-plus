package dto

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Timestamp preserves QQ RFC3339 values and legacy Unix-second timestamps without guessing milliseconds.
type Timestamp string

func (t *Timestamp) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*t = ""
		return nil
	}
	var value string
	if len(data) > 0 && data[0] == '"' {
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
	} else {
		var number json.Number
		if err := json.Unmarshal(data, &number); err != nil {
			return err
		}
		value = number.String()
	}
	*t = Timestamp(value)
	return nil
}

func (t Timestamp) Time() (time.Time, error) {
	value := string(t)
	if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return parsed, nil
	}
	seconds, fraction, hasFraction := strings.Cut(value, ".")
	sec, err := strconv.ParseInt(seconds, 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid QQ timestamp: %w", err)
	}
	var nsec int64
	if hasFraction {
		if fraction == "" {
			return time.Time{}, fmt.Errorf("invalid fractional timestamp")
		}
		for _, digit := range fraction {
			if digit < '0' || digit > '9' {
				return time.Time{}, fmt.Errorf("invalid fractional timestamp")
			}
		}
		fraction = (fraction + "000000000")[:9]
		nsec, _ = strconv.ParseInt(fraction, 10, 64)
		if strings.HasPrefix(seconds, "-") {
			nsec = -nsec
		}
	}
	return time.Unix(sec, nsec).UTC(), nil
}
