package config

import (
	"crypto/subtle"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"

	json "github.com/oarkflow/fastjson"
)

type ValueKind string

const (
	KindAny         ValueKind = "any"
	KindString      ValueKind = "string"
	KindBool        ValueKind = "bool"
	KindInt         ValueKind = "int"
	KindInt64       ValueKind = "int64"
	KindFloat64     ValueKind = "float64"
	KindDuration    ValueKind = "duration"
	KindSize        ValueKind = "size"
	KindStringSlice ValueKind = "[]string"
	KindSecret      ValueKind = "secret"
	KindMap         ValueKind = "map"
)

type SecretString struct{ value string }

func NewSecretString(v string) SecretString { return SecretString{value: v} }
func (s SecretString) Value() string        { return s.value }
func (s SecretString) String() string {
	if s.value == "" {
		return ""
	}
	return "[REDACTED]"
}
func (s SecretString) GoString() string             { return s.String() }
func (s SecretString) MarshalJSON() ([]byte, error) { return json.Marshal(s.String()) }
func (s *SecretString) UnmarshalJSON(b []byte) error {
	var v string
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	s.value = v
	return nil
}
func (s SecretString) IsZero() bool { return s.value == "" }
func (s SecretString) EqualConstantTime(v string) bool {
	return subtle.ConstantTimeCompare([]byte(s.value), []byte(v)) == 1
}

type Size int64

const (
	KB  Size = 1000
	MB  Size = 1000 * KB
	GB  Size = 1000 * MB
	KiB Size = 1024
	MiB Size = 1024 * KiB
	GiB Size = 1024 * MiB
)

func ParseSize(s string) (Size, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty size")
	}
	lower := strings.ToLower(s)
	units := []struct {
		suffix string
		mult   int64
	}{{"gib", int64(GiB)}, {"mib", int64(MiB)}, {"kib", int64(KiB)}, {"gb", int64(GB)}, {"mb", int64(MB)}, {"kb", int64(KB)}, {"b", 1}}
	for _, u := range units {
		if strings.HasSuffix(lower, u.suffix) {
			n := strings.TrimSpace(s[:len(s)-len(u.suffix)])
			f, err := strconv.ParseFloat(n, 64)
			if err != nil {
				return 0, err
			}
			return Size(f * float64(u.mult)), nil
		}
	}
	n, err := strconv.ParseInt(s, 10, 64)
	return Size(n), err
}
func (s Size) String() string {
	if s >= GiB && s%GiB == 0 {
		return fmt.Sprintf("%dGiB", s/GiB)
	}
	if s >= MiB && s%MiB == 0 {
		return fmt.Sprintf("%dMiB", s/MiB)
	}
	if s >= KiB && s%KiB == 0 {
		return fmt.Sprintf("%dKiB", s/KiB)
	}
	return fmt.Sprintf("%dB", s)
}
func (s Size) MarshalJSON() ([]byte, error) { return json.Marshal(s.String()) }
func (s *Size) UnmarshalJSON(b []byte) error {
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	sz, ok := ToSize(v)
	if !ok {
		return fmt.Errorf("invalid size")
	}
	*s = sz
	return nil
}

func ToString(v any) (string, bool) {
	switch x := v.(type) {
	case nil:
		return "", false
	case string:
		return x, true
	case SecretString:
		return x.Value(), true
	case fmt.Stringer:
		return x.String(), true
	case bool:
		return strconv.FormatBool(x), true
	case int:
		return strconv.Itoa(x), true
	case int64:
		return strconv.FormatInt(x, 10), true
	case int32:
		return strconv.FormatInt(int64(x), 10), true
	case uint:
		return strconv.FormatUint(uint64(x), 10), true
	case uint64:
		return strconv.FormatUint(x, 10), true
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64), true
	case json.Number:
		return x.String(), true
	default:
		return "", false
	}
}
func ToBool(v any) (bool, bool) {
	switch x := v.(type) {
	case bool:
		return x, true
	case string:
		b, e := strconv.ParseBool(strings.TrimSpace(x))
		return b, e == nil
	case int:
		return x != 0, true
	case int64:
		return x != 0, true
	case float64:
		return x != 0, true
	case json.Number:
		i, e := x.Int64()
		return i != 0, e == nil
	default:
		return false, false
	}
}
func ToInt(v any) (int, bool) { i, ok := ToInt64(v); return int(i), ok }
func ToInt64(v any) (int64, bool) {
	switch x := v.(type) {
	case int:
		return int64(x), true
	case int8:
		return int64(x), true
	case int16:
		return int64(x), true
	case int32:
		return int64(x), true
	case int64:
		return x, true
	case uint:
		if uint64(x) > uint64(^uint(0)>>1) {
			return 0, false
		}
		return int64(x), true
	case uint64:
		if x > uint64(^uint64(0)>>1) {
			return 0, false
		}
		return int64(x), true
	case float64:
		return int64(x), true
	case json.Number:
		i, e := x.Int64()
		return i, e == nil
	case string:
		i, e := strconv.ParseInt(strings.TrimSpace(x), 10, 64)
		return i, e == nil
	default:
		return 0, false
	}
}
func ToFloat64(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case float32:
		return float64(x), true
	case int:
		return float64(x), true
	case int64:
		return float64(x), true
	case json.Number:
		f, e := x.Float64()
		return f, e == nil
	case string:
		f, e := strconv.ParseFloat(strings.TrimSpace(x), 64)
		return f, e == nil
	default:
		return 0, false
	}
}
func ToDuration(v any) (time.Duration, bool) {
	switch x := v.(type) {
	case time.Duration:
		return x, true
	case string:
		d, e := time.ParseDuration(strings.TrimSpace(x))
		return d, e == nil
	case int:
		return time.Duration(x), true
	case int64:
		return time.Duration(x), true
	case float64:
		return time.Duration(x), true
	case json.Number:
		i, e := x.Int64()
		return time.Duration(i), e == nil
	default:
		return 0, false
	}
}
func ToSize(v any) (Size, bool) {
	switch x := v.(type) {
	case Size:
		return x, true
	case string:
		s, e := ParseSize(x)
		return s, e == nil
	case int:
		return Size(x), true
	case int64:
		return Size(x), true
	case float64:
		return Size(x), true
	case json.Number:
		i, e := x.Int64()
		return Size(i), e == nil
	default:
		return 0, false
	}
}
func ToStringSlice(v any) ([]string, bool) {
	switch x := v.(type) {
	case []string:
		return append([]string(nil), x...), true
	case []any:
		out := make([]string, 0, len(x))
		for _, v := range x {
			s, ok := ToString(v)
			if !ok {
				return nil, false
			}
			out = append(out, s)
		}
		return out, true
	case string:
		if strings.TrimSpace(x) == "" {
			return []string{}, true
		}
		parts := strings.Split(x, ",")
		out := parts[:0]
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				out = append(out, p)
			}
		}
		return out, true
	default:
		return nil, false
	}
}
func MustURL(s string) url.URL {
	u, err := url.Parse(s)
	if err != nil {
		panic(err)
	}
	return *u
}
func ParseIP(s string) (net.IP, bool)       { ip := net.ParseIP(s); return ip, ip != nil }
func ParseAddr(s string) (netip.Addr, bool) { a, err := netip.ParseAddr(s); return a, err == nil }
