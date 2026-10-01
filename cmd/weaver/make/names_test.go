package make

import "testing"

func TestToSnake(t *testing.T) {
	cases := map[string]string{
		"Chat":      "chat",
		"HTTPApi":   "http_api",
		"UserStore": "user_store",
		"T":         "t",
		"API2Echo":  "api2_echo",
	}
	for in, want := range cases {
		if got := toSnake(in); got != want {
			t.Errorf("toSnake(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestUnexport(t *testing.T) {
	cases := map[string]string{
		"Chat":    "chat",
		"HTTPApi": "hTTPApi",
		"T":       "t",
		"":        "",
	}
	for in, want := range cases {
		if got := unexport(in); got != want {
			t.Errorf("unexport(%q) = %q, want %q", in, got, want)
		}
	}
}
