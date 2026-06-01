package shortcode

// builtinShortcodes maps a shortcode name to its template body. They are parsed
// with shortcodeFuncMap and can be overridden by dropping a same-named
// templates/shortcodes/<name>.html (or theme layouts/shortcodes/<name>.html).
//
// Authoring notes:
//   - .Get takes an int for positional args or a string for named args.
//   - .Inner is the (already shortcode-expanded) body of a paired shortcode and
//     is HTML-escaped when emitted as text; use `| safeHTML` to emit raw HTML.
var builtinShortcodes = map[string]string{
	// figure: src (required), alt, title, caption, link.
	"figure": `<figure>` +
		`{{- with $.Get "link" }}<a href="{{ . }}">{{ end -}}` +
		`<img src="{{ $.Get "src" }}"` +
		`{{- with $.Get "alt" }} alt="{{ . }}"{{ end -}}` +
		`{{- with $.Get "title" }} title="{{ . }}"{{ end -}}` +
		`>` +
		`{{- with $.Get "link" }}</a>{{ end -}}` +
		`{{- with $.Get "caption" }}<figcaption>{{ . }}</figcaption>{{ end -}}` +
		`</figure>`,

	// youtube: video id, given positionally ({{< youtube ID >}}) or named (id=).
	"youtube": `{{- $id := $.Get "id" -}}` +
		`{{- if not $id }}{{ $id = $.Get 0 }}{{ end -}}` +
		`<div class="gobin-youtube">` +
		`<iframe src="https://www.youtube.com/embed/{{ $id }}" ` +
		`title="YouTube video" loading="lazy" frameborder="0" ` +
		`allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture" ` +
		`allowfullscreen></iframe></div>`,

	// gist: GitHub user and gist id, positional ({{< gist user id >}}) or named.
	"gist": `{{- $user := $.Get "user" -}}` +
		`{{- if not $user }}{{ $user = $.Get 0 }}{{ end -}}` +
		`{{- $id := $.Get "id" -}}` +
		`{{- if not $id }}{{ $id = $.Get 1 }}{{ end -}}` +
		`<script src="https://gist.github.com/{{ $user }}/{{ $id }}.js"></script>`,

	// highlight: paired shortcode wrapping its body in a code block. The first
	// positional argument is the language class.
	"highlight": `{{- $lang := $.Get 0 -}}` +
		`<pre><code{{ with $lang }} class="language-{{ . }}"{{ end }}>{{ .Inner }}</code></pre>`,
}
