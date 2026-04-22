package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMinifyHTMLContent_PreservesWhitespaceSensitiveBlocks(t *testing.T) {
	input := `
<!DOCTYPE html>
<html>
  <body>
    <!-- remove me -->
    <p>
      Hello   world
    </p>
    <pre>  keep
  spacing  </pre>
    <script>
      const msg = "a   b";
    </script>
  </body>
</html>
`

	got := minifyContent(input, ".html")

	if contains := `<!-- remove me -->`; strings.Contains(got, contains) {
		t.Fatalf("expected html comments to be removed, got %q", got)
	}
	if !strings.Contains(got, "<p>Hello world</p>") {
		t.Fatalf("expected paragraph whitespace to be collapsed, got %q", got)
	}
	if !strings.Contains(got, "<pre>  keep\n  spacing  </pre>") {
		t.Fatalf("expected pre block whitespace to be preserved, got %q", got)
	}
	if !strings.Contains(got, "const msg = \"a   b\";") {
		t.Fatalf("expected script contents to be preserved, got %q", got)
	}
}

func TestMinifyCSSContent_RemovesSafeWhitespaceAndComments(t *testing.T) {
	input := `
/* comment */
body {
  color : red ;
  margin : 0 ;
}
`

	got := minifyContent(input, ".css")
	want := "body{color:red;margin:0}"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestMinifyJSContent_PreservesInlineSemantics(t *testing.T) {
	input := `
const msg = "a   b";
const pattern = /a b/;
`

	got := minifyContent(input, ".js")
	want := "const msg = \"a   b\";\nconst pattern = /a b/;"
	if got != want {
		t.Fatalf("expected js content to remain intact, got %q", got)
	}
}

func TestMinifyOutput_PreservesFileMode(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "index.html")
	if err := os.WriteFile(path, []byte("<p> hello </p>"), 0600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	if err := minifyOutput(tmpDir); err != nil {
		t.Fatalf("minifyOutput failed: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("expected file mode 0600, got %o", info.Mode().Perm())
	}
}
