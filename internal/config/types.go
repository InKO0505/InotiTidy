package config

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Duration is a time.Duration that (de)serializes as a Go duration string
// ("500ms", "24h", "30m") in YAML. The zero value means "unset".
type Duration struct{ D time.Duration }

func (d Duration) MarshalYAML() (any, error) {
	if d.D == 0 {
		return nil, nil
	}
	return d.D.String(), nil
}

func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	var s string
	if err := value.Decode(&s); err != nil {
		return err
	}
	s = strings.TrimSpace(s)
	if s == "" {
		d.D = 0
		return nil
	}
	parsed, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", s, err)
	}
	d.D = parsed
	return nil
}

// Size is a byte count that (de)serializes as a human string ("100MB", "2G",
// "1024"). Units are powers of 1024; a bare number is bytes.
type Size int64

var sizeUnits = []struct {
	suffix string
	factor int64
}{
	{"TB", 1 << 40}, {"T", 1 << 40},
	{"GB", 1 << 30}, {"G", 1 << 30},
	{"MB", 1 << 20}, {"M", 1 << 20},
	{"KB", 1 << 10}, {"K", 1 << 10},
	{"B", 1},
}

func (s Size) MarshalYAML() (any, error) {
	if s == 0 {
		return nil, nil
	}
	return s.String(), nil
}

func (s Size) String() string {
	n := int64(s)
	for _, u := range sizeUnits {
		if u.factor > 1 && n%u.factor == 0 && n >= u.factor {
			return fmt.Sprintf("%d%s", n/u.factor, u.suffix)
		}
	}
	return strconv.FormatInt(n, 10)
}

func (s *Size) UnmarshalYAML(value *yaml.Node) error {
	// Accept both a bare integer and a quoted human string.
	var raw string
	if err := value.Decode(&raw); err != nil {
		var n int64
		if err2 := value.Decode(&n); err2 != nil {
			return err
		}
		*s = Size(n)
		return nil
	}
	parsed, err := ParseSize(raw)
	if err != nil {
		return err
	}
	*s = Size(parsed)
	return nil
}

// ParseSize converts a human size string to bytes.
func ParseSize(in string) (int64, error) {
	in = strings.TrimSpace(strings.ToUpper(in))
	if in == "" {
		return 0, nil
	}
	for _, u := range sizeUnits {
		if rest, ok := strings.CutSuffix(in, u.suffix); ok {
			numPart := strings.TrimSpace(rest)
			if numPart == "" {
				continue
			}
			f, err := strconv.ParseFloat(numPart, 64)
			if err != nil {
				return 0, fmt.Errorf("invalid size %q: %w", in, err)
			}
			return int64(f * float64(u.factor)), nil
		}
	}
	n, err := strconv.ParseInt(in, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid size %q", in)
	}
	return n, nil
}
