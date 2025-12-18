// Package testing provides utility functions for testing purposes across multiple packages.
package testing

import (
	"reflect"
	"strings"
	"testing"

	mapset "github.com/deckarep/golang-set/v2"
)

// AssertError validates that an error matches expected results.
func AssertError(t *testing.T, err error, wantErr bool) {
	t.Helper()

	if wantErr != (err != nil) {
		t.Fatalf("Expected error = %v, got: %v", wantErr, err)
	}
}

// AssertEqual verifies two values are equal.
func AssertEqual(t *testing.T, got, want any) {
	t.Helper()

	if got != want {
		t.Errorf("got = %q, want: %q", got, want)
	}
}

// AssertLength verifies the length of a string, slice, or map using reflection.
func AssertLength(t *testing.T, got any, want int) {
	t.Helper()

	v := reflect.ValueOf(got)
	kind := v.Kind()

	if kind != reflect.String && kind != reflect.Slice && kind != reflect.Map && kind != reflect.Array && kind != reflect.Chan {
		t.Fatalf("AssertLength called with unsupported type: %T", got)
	}

	if v.Len() != want {
		t.Fatalf("Expected output length %d, got: %d", want, v.Len())
	}
}

// AssertContains verifies that got contains all values in want.
// Supports multiple type combinations:
// - got: string, want: string - checks substring containment
// - got: []string, want: string - checks if any element contains substring
// - got: string, want: []string - checks if string contains all substrings
// - got: []string, want: []string - checks if any element contains each substring
// - got: string/[]string, want: map[string]string - checks containment of all map values
func AssertContains(t *testing.T, got any, want any, index ...int) {
	t.Helper()

	// Build a set-based search function for got
	var searchIn func(string) bool
	switch gotVal := got.(type) {
	case string:
		searchIn = func(needle string) bool {
			return strings.Contains(gotVal, needle)
		}
	case []string:
		gotSet := mapset.NewSet(gotVal...)
		searchIn = func(needle string) bool {
			for item := range gotSet.Iter() {
				if strings.Contains(item, needle) {
					return true
				}
			}
			return false
		}
	default:
		t.Fatalf("AssertContains: got must be string or []string, got %T", got)
		return
	}

	// Check all want values against got
	switch wantVal := want.(type) {
	case string:
		if !searchIn(wantVal) {
			if len(index) > 0 {
				t.Errorf("Expected output[%d] to contain %q, got: %v", index[0], wantVal, got)
			} else {
				t.Errorf("Expected output to contain %q, got: %v", wantVal, got)
			}
		}
	case []string:
		for _, needle := range wantVal {
			if !searchIn(needle) {
				if len(index) > 0 {
					t.Errorf("Expected output[%d] to contain %q, got: %v", index[0], needle, got)
				} else {
					t.Errorf("Expected output to contain %q, got: %v", needle, got)
				}
			}
		}
	case map[string]string:
		for key, needle := range wantVal {
			if !searchIn(needle) {
				t.Errorf("Expected output for %s to contain %q, got: %v", key, needle, got)
			}
		}
	default:
		t.Fatalf("AssertContains: want must be string, []string, or map[string]string, got %T", want)
	}
}

// AssertNotContains verifies a string or slice does not contain unwanted substrings.
// For slices, checks that no element contains any of the unwanted substrings.
func AssertNotContains(t *testing.T, output any, unwanted []string) {
	t.Helper()

	var checkContains func(string) bool
	switch outputVal := output.(type) {
	case string:
		checkContains = func(needle string) bool {
			return strings.Contains(outputVal, needle)
		}
	case []string:
		outputSet := mapset.NewSet(outputVal...)
		checkContains = func(needle string) bool {
			for item := range outputSet.Iter() {
				if strings.Contains(item, needle) {
					return true
				}
			}
			return false
		}
	default:
		t.Fatalf("AssertNotContains: output must be string or []string, got %T", output)
		return
	}

	for _, unwanted := range unwanted {
		if checkContains(unwanted) {
			t.Errorf("Expected output to not contain %q, got: %s", unwanted, output)
		}
	}
}

// AssertNotEmpty verifies a string is not empty.
func AssertNotEmpty(t *testing.T, got string) {
	t.Helper()

	if got == "" {
		t.Error("Expected non-empty string")
	}
}

// AssertOutput validates test output and error against expected values.
func AssertOutput(t *testing.T, got, want []string, err error, wantErr bool) {
	t.Helper()

	AssertError(t, err, wantErr)
	AssertLength(t, got, len(want))

	for i, msg := range want {
		if got[i] != msg {
			t.Errorf("Expected output[%d] = %s, got: %s", i, msg, got[i])
		}
	}
}
