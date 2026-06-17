package cli

import (
	"encoding/json"
	"strings"
	"testing"

	apiv1 "github.com/juanfont/headscale/gen/api/v1"
)

// TestFormatOutputValueWithUnsetOptional guards against a regression where the
// generated API types, passed by value with unset optional fields, fail to
// marshal because stdlib reflection calls Opt*.MarshalJSON directly.
func TestFormatOutputValueWithUnsetOptional(t *testing.T) {
	// DisplayName, Email, CreatedAt and PictureURL are intentionally unset.
	user := apiv1.User{
		ID:   apiv1.NewOptUint64(1),
		Name: apiv1.NewOptString("alice"),
	}

	for _, format := range []string{outputFormatJSON, outputFormatJSONLine, outputFormatYAML} {
		out, err := formatOutput(user, "", format)
		if err != nil {
			t.Fatalf("formatOutput(%s) on value type: %v", format, err)
		}

		if !strings.Contains(out, "alice") {
			t.Errorf("formatOutput(%s) missing name: %q", format, out)
		}
	}

	// A slice of value types must work too (e.g. "users list").
	listOut, err := formatOutput([]apiv1.User{user}, "", outputFormatJSON)
	if err != nil {
		t.Fatalf("formatOutput(json) on slice of value type: %v", err)
	}

	var decoded []map[string]any

	err = json.Unmarshal([]byte(listOut), &decoded)
	if err != nil {
		t.Fatalf("list output is not valid JSON: %v\n%s", err, listOut)
	}

	if len(decoded) != 1 || decoded[0]["name"] != "alice" {
		t.Errorf("unexpected list output: %s", listOut)
	}
}
