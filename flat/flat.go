package flat

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/goccy/go-reflect"
	"github.com/sujit-baniya/config/mergo"
)

// Options the flatten options.
// By default: Delimiter = "."
type Options struct {
	Prefix    string
	Delimiter string
	Safe      bool
	MaxDepth  int
}

// Flatten the map, it returns a map one level deep
// regardless of how nested the original map was.
// By default, the flatten has Delimiter = ".", and
// no limitation of MaxDepth
func Flatten(nested map[string]any, opts *Options) (m map[string]any, err error) {
	if opts == nil {
		opts = &Options{
			Delimiter: ".",
		}
	}

	m, err = flatten(opts.Prefix, 0, nested, opts)

	return
}

func removeEmpty(s []string) []string {
	var r []string
	for _, str := range s {
		if str != "" {
			r = append(r, str)
		}
	}
	return r
}

func FlattenEnv(opts *Options) map[string]any {
	m := make(map[string]any)
	if opts == nil {
		opts = &Options{
			Delimiter: ".",
		}
	}
	for _, env := range os.Environ() {
		envKeyValue := strings.Split(strings.ToLower(env), "=")
		if len(envKeyValue) == 2 {
			key := envKeyValue[0]
			value := envKeyValue[1]
			keyParts := removeEmpty(strings.Split(key, "_"))
			fn := func(s []string) (t []any) {
				for _, a := range s {
					t = append(t, a)
				}
				return
			}
			keys := GenerateKeys(len(keyParts) - 1)
			for _, r := range keys {
				m[fmt.Sprintf(r, fn(keyParts)...)] = value
			}
		}
	}
	return m
}

func MapReplace(dst, src map[string]any) map[string]any {
	for key, value := range src {
		if _, ok := dst[key]; ok {
			dst[key] = value
		}
	}
	return dst
}
func GenerateKeys(length int) (b []string) {
	for combination := range GenerateCombinations("._", length) {
		if len(combination) == length {
			b = append(b, "%s"+strings.Join(strings.Split(combination, ""), "%s")+"%s")
		}
	}

	return
}

func GenerateCombinations(alphabet string, length int) <-chan string {
	c := make(chan string)

	go func(c chan string) {
		defer close(c) // Once the iteration function is finished, we close the channel

		AddLetter(c, "", alphabet, length) // We start by feeding it an empty string
	}(c)

	return c // Return the channel to the calling function
}

// AddLetter adds a letter to the combination to create a new combination.
// This new combination is passed on to the channel before we call AddLetter once again
// to add yet another letter to the new combination in case length allows it
func AddLetter(c chan string, combo string, alphabet string, length int) {
	// Check if we reached the length limit
	// If so, we just return without adding anything
	if length <= 0 {
		return
	}

	var newCombo string
	for _, ch := range alphabet {
		newCombo = combo + string(ch)
		c <- newCombo
		AddLetter(c, newCombo, alphabet, length-1)
	}
}

func flatten(prefix string, depth int, nested interface{}, opts *Options) (flatmap map[string]any, err error) {
	flatmap = make(map[string]any)

	switch nested := nested.(type) {
	case map[string]any:
		if opts.MaxDepth != 0 && depth >= opts.MaxDepth {
			flatmap[prefix] = nested
			return
		}
		if reflect.DeepEqual(nested, map[string]any{}) {
			flatmap[prefix] = nested
			return
		}
		for k, v := range nested {
			// create new key
			newKey := k
			if prefix != "" {
				newKey = prefix + opts.Delimiter + newKey
			}
			fm1, fe := flatten(newKey, depth+1, v, opts)
			if fe != nil {
				err = fe
				return
			}
			MergeMap(flatmap, fm1)
		}
	case []interface{}:
		if opts.Safe {
			flatmap[prefix] = nested
			return
		}
		if reflect.DeepEqual(nested, []interface{}{}) {
			flatmap[prefix] = nested
			return
		}
		for i, v := range nested {
			newKey := strconv.Itoa(i)
			if prefix != "" {
				newKey = prefix + opts.Delimiter + newKey
			}
			fm1, fe := flatten(newKey, depth+1, v, opts)
			if fe != nil {
				err = fe
				return
			}
			MergeMap(flatmap, fm1)
		}
	default:
		flatmap[prefix] = nested
	}
	return
}

// MergeMap is the function that MergeMap to map with from
// example:
// to = {"hi": "there"}
// from = {"foo": "bar"}
// then, to = {"hi": "there", "foo": "bar"}
func MergeMap(to map[string]any, from map[string]any) {
	for kt, vt := range from {
		to[kt] = vt
	}
}

// Unflatten the map, it returns a nested map of a map
// By default, the flatten has Delimiter = "."
func Unflatten(flat map[string]any, opts *Options) (nested map[string]any, err error) {
	if opts == nil {
		opts = &Options{
			Delimiter: ".",
		}
	}
	nested, err = unflatten(flat, opts)
	return
}

func unflatten(flat map[string]any, opts *Options) (nested map[string]any, err error) {
	nested = make(map[string]any)

	for k, v := range flat {
		temp := uf(k, v, opts).(map[string]any)
		err = mergo.Merge(&nested, temp, func(c *mergo.Config) { c.Overwrite = true })
		if err != nil {
			return
		}
	}

	return
}

func uf(k string, v interface{}, opts *Options) (n interface{}) {
	n = v

	if opts.Prefix != "" {
		k = strings.TrimPrefix(k, opts.Prefix+opts.Delimiter)
	}
	keys := strings.Split(k, opts.Delimiter)

	for i := len(keys) - 1; i >= 0; i-- {
		temp := make(map[string]any)
		temp[keys[i]] = n
		n = temp
	}

	return
}
