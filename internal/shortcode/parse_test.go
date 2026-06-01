package shortcode

import (
	"reflect"
	"testing"
)

func TestParseArgs(t *testing.T) {
	tests := []struct {
		name       string
		inner      string
		wantName   string
		wantClose  bool
		positional []string
		named      map[string]string
		wantErr    bool
	}{
		{
			name:       "positional only",
			inner:      "youtube dQw4w9WgXcQ",
			wantName:   "youtube",
			positional: []string{"dQw4w9WgXcQ"},
		},
		{
			name:     "named quoted with spaces",
			inner:    `figure src="/img/a.png" caption="hello world"`,
			wantName: "figure",
			named:    map[string]string{"src": "/img/a.png", "caption": "hello world"},
		},
		{
			name:       "quoted positional kept whole",
			inner:      `say "a = b"`,
			wantName:   "say",
			positional: []string{"a = b"},
		},
		{
			name:     "closing tag",
			inner:    "/highlight",
			wantName: "highlight",
			wantClose: true,
		},
		{
			name:       "mixed positional and named",
			inner:      `figure /img/x.png alt="x"`,
			wantName:   "figure",
			positional: []string{"/img/x.png"},
			named:      map[string]string{"alt": "x"},
		},
		{
			name:    "empty tag",
			inner:   "   ",
			wantErr: true,
		},
		{
			name:    "closing tag with args",
			inner:   "/highlight go",
			wantErr: true,
		},
		{
			name:    "unterminated quote",
			inner:   `figure src="oops`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, closing, a, err := parseArgs(tt.inner)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got none")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if name != tt.wantName {
				t.Errorf("name = %q, want %q", name, tt.wantName)
			}
			if closing != tt.wantClose {
				t.Errorf("closing = %v, want %v", closing, tt.wantClose)
			}
			if tt.wantClose {
				return
			}
			if len(a.positional) == 0 && len(tt.positional) == 0 {
				// both empty, ok
			} else if !reflect.DeepEqual(a.positional, tt.positional) {
				t.Errorf("positional = %#v, want %#v", a.positional, tt.positional)
			}
			wantNamed := tt.named
			if wantNamed == nil {
				wantNamed = map[string]string{}
			}
			if !reflect.DeepEqual(a.named, wantNamed) {
				t.Errorf("named = %#v, want %#v", a.named, wantNamed)
			}
		})
	}
}

func TestScanTokens(t *testing.T) {
	src := `intro {{< youtube id123 >}} mid
{{% note key="a > b" %}}body{{% /note %}}
done`

	tokens, err := scanTokens(src, protectedRanges(src))
	if err != nil {
		t.Fatalf("scanTokens: %v", err)
	}
	if len(tokens) != 3 {
		t.Fatalf("expected 3 tokens, got %d: %#v", len(tokens), tokens)
	}

	if tokens[0].name != "youtube" || tokens[0].form != formHTML || tokens[0].closing {
		t.Errorf("token0 = %#v", tokens[0])
	}
	if got := src[tokens[0].rawStart:tokens[0].rawEnd]; got != "{{< youtube id123 >}}" {
		t.Errorf("token0 raw = %q", got)
	}

	if tokens[1].name != "note" || tokens[1].form != formMarkdown || tokens[1].closing {
		t.Errorf("token1 = %#v", tokens[1])
	}
	// '>' inside the quoted named arg must not terminate the tag early.
	if got := tokens[1].args.named["key"]; got != "a > b" {
		t.Errorf("note key = %q, want %q", got, "a > b")
	}

	if tokens[2].name != "note" || !tokens[2].closing {
		t.Errorf("token2 = %#v", tokens[2])
	}
}

func TestScanTokensUnterminated(t *testing.T) {
	if _, err := scanTokens("text {{< figure src=x", nil); err == nil {
		t.Fatal("expected error for unterminated tag")
	}
}

func TestScanTokensIgnoresPlainBraces(t *testing.T) {
	src := "code {{ .Title }} and {{end}} are not shortcodes"
	tokens, err := scanTokens(src, protectedRanges(src))
	if err != nil {
		t.Fatalf("scanTokens: %v", err)
	}
	if len(tokens) != 0 {
		t.Fatalf("expected 0 tokens, got %d", len(tokens))
	}
}
