package shortcode

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
)

// maxShortcodeDepth bounds recursive expansion of nested shortcodes.
const maxShortcodeDepth = 16

// node is a top-level shortcode invocation resolved from the token stream.
// For a paired shortcode, inner holds the raw source between the opening and
// closing tags and [start,end) spans both tags.
type node struct {
	name  string
	form  form
	args  *args
	inner string
	start int
	end   int
}

// Render expands the shortcodes in src around a Markdown conversion step.
// convert performs the Markdown-to-HTML rendering (typically goldmark).
//
// Markdown-form ({{% %}}) output is spliced into the source before convert so
// it is rendered as Markdown. HTML-form ({{< >}}) output is replaced with a
// unique alphanumeric sentinel that survives convert untouched and is restored
// afterwards, so the generated HTML is emitted verbatim regardless of the
// Markdown renderer's raw-HTML policy.
func (r *Registry) Render(src string, convert func(string) (string, error)) (string, error) {
	if r == nil {
		return convert(src)
	}

	expanded, sentinels, err := r.expandPre(src)
	if err != nil {
		return "", err
	}

	out, err := convert(expanded)
	if err != nil {
		return "", err
	}

	if len(sentinels) > 0 {
		out = expandPost(out, sentinels)
	}
	return out, nil
}

func (r *Registry) expandPre(src string) (string, map[string]string, error) {
	nodes, err := r.scanNodes(src)
	if err != nil {
		return "", nil, err
	}
	if len(nodes) == 0 {
		return src, nil, nil
	}

	sentinels := make(map[string]string)
	base := newSentinelBase(src)
	seq := 0

	var b strings.Builder
	last := 0
	for _, n := range nodes {
		b.WriteString(src[last:n.start])
		out, err := r.renderNode(n, 0)
		if err != nil {
			return "", nil, err
		}
		if n.form == formMarkdown {
			b.WriteString(out)
		} else {
			token := base + strconv.Itoa(seq) + "x"
			seq++
			sentinels[token] = out
			b.WriteString(token)
		}
		last = n.end
	}
	b.WriteString(src[last:])

	return b.String(), sentinels, nil
}

// expandInline expands every shortcode in src to its HTML output inline,
// without sentinels or a Markdown pass. It is used to resolve nested
// shortcodes inside a paired shortcode's body.
func (r *Registry) expandInline(src string, depth int) (string, error) {
	nodes, err := r.scanNodes(src)
	if err != nil {
		return "", err
	}
	if len(nodes) == 0 {
		return src, nil
	}

	var b strings.Builder
	last := 0
	for _, n := range nodes {
		b.WriteString(src[last:n.start])
		out, err := r.renderNode(n, depth)
		if err != nil {
			return "", err
		}
		b.WriteString(out)
		last = n.end
	}
	b.WriteString(src[last:])

	return b.String(), nil
}

func (r *Registry) scanNodes(src string) ([]node, error) {
	tokens, err := scanTokens(src, protectedRanges(src))
	if err != nil {
		return nil, err
	}
	return buildNodes(tokens, src)
}

func (r *Registry) renderNode(n node, depth int) (string, error) {
	if depth > maxShortcodeDepth {
		return "", fmt.Errorf("shortcode %q nested too deeply (>%d)", n.name, maxShortcodeDepth)
	}

	tmpl, ok := r.Lookup(n.name)
	if !ok {
		return "", fmt.Errorf("unknown shortcode %q", n.name)
	}

	inner := n.inner
	if strings.TrimSpace(inner) != "" {
		expanded, err := r.expandInline(inner, depth+1)
		if err != nil {
			return "", err
		}
		inner = expanded
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, newContext(n.name, n.args, inner)); err != nil {
		return "", fmt.Errorf("render shortcode %q: %w", n.name, err)
	}
	return buf.String(), nil
}

// buildNodes pairs opening and closing tags and returns the top-level
// invocations in source order. Tokens nested inside a paired shortcode are
// left in that shortcode's raw inner body for recursive expansion. An opening
// tag without a matching close is treated as a standalone shortcode (matching
// Hugo); a closing tag without an opening is an error.
func buildNodes(tokens []token, src string) ([]node, error) {
	type pair struct{ open, close int }
	var pairs []pair
	var stack []int

	for idx, tk := range tokens {
		if !tk.closing {
			stack = append(stack, idx)
			continue
		}
		matched := -1
		for s := len(stack) - 1; s >= 0; s-- {
			if tokens[stack[s]].name == tk.name {
				matched = s
				break
			}
		}
		if matched < 0 {
			return nil, fmt.Errorf("closing shortcode {{</ %s >}} has no opening tag", tk.name)
		}
		pairs = append(pairs, pair{open: stack[matched], close: idx})
		stack = stack[:matched]
	}

	nested := make([]bool, len(tokens))
	closeOf := make(map[int]int, len(pairs))
	for _, p := range pairs {
		closeOf[p.open] = p.close
		for i := p.open + 1; i < p.close; i++ {
			nested[i] = true
		}
	}

	var nodes []node
	for idx, tk := range tokens {
		if tk.closing || nested[idx] {
			continue
		}
		n := node{name: tk.name, form: tk.form, args: tk.args, start: tk.rawStart, end: tk.rawEnd}
		if closeIdx, ok := closeOf[idx]; ok {
			n.inner = src[tk.rawEnd:tokens[closeIdx].rawStart]
			n.end = tokens[closeIdx].rawEnd
		}
		nodes = append(nodes, n)
	}

	return nodes, nil
}

// expandPost restores sentinel tokens with their shortcode HTML. A block-level
// shortcode that occupied its own line is wrapped by the Markdown renderer in a
// paragraph; that wrapping <p>…</p> is stripped so the HTML is not nested in a
// stray paragraph. Inline shortcodes are substituted in place.
func expandPost(html string, sentinels map[string]string) string {
	for token, out := range sentinels {
		block := "<p>" + token + "</p>"
		if strings.Contains(html, block) {
			html = strings.ReplaceAll(html, block, out)
			continue
		}
		html = strings.ReplaceAll(html, token, out)
	}
	return html
}

// newSentinelBase returns a random alphanumeric prefix that does not already
// appear in src. Concatenated with a numeric sequence and an 'x' terminator it
// yields collision-free tokens where no token is a prefix of another.
func newSentinelBase(src string) string {
	for {
		buf := make([]byte, 8)
		_, _ = rand.Read(buf)
		base := "gobinsc" + hex.EncodeToString(buf)
		if !strings.Contains(src, base) {
			return base
		}
	}
}
