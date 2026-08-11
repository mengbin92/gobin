package generator

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/mengbin92/gobin/internal/config"
)

type staticAssetFile struct {
	SourcePath string
	OutputPath string
}

type staticAssetCopyAction string

const (
	staticAssetCopy staticAssetCopyAction = "copy"
	staticAssetSkip staticAssetCopyAction = "skip"
)

type staticAssetCopyPlan struct {
	Asset    staticAssetFile
	DestPath string
	// ManifestPath is the slash-separated path relative to outputDir that
	// identifies this asset in the static asset manifest. It mirrors what
	// actually lands on disk, which means it includes the content hash when
	// filename-level fingerprinting is enabled.
	ManifestPath string
	Action       staticAssetCopyAction
	Reason       string
}

type staticAssetCopyResult struct {
	Copied  int
	Skipped int
	Deleted int
}

func (r staticAssetCopyResult) stats() AssetCopyStats {
	return AssetCopyStats{
		Copied:  r.Copied,
		Skipped: r.Skipped,
		Deleted: r.Deleted,
	}
}

func copyStaticAssets(cfg *config.Config, outputDir string) error {
	_, err := copyStaticAssetsWithResult(cfg, outputDir)
	return err
}

func copyStaticAssetsWithResult(cfg *config.Config, outputDir string) (staticAssetCopyResult, error) {
	plans, err := planStaticAssetCopies(cfg, outputDir)
	if err != nil {
		return staticAssetCopyResult{}, err
	}

	result, err := executeStaticAssetCopyPlan(plans)
	if err != nil {
		return result, err
	}

	deleted, err := removeStaleManagedStaticAssets(outputDir, plans)
	if err != nil {
		return result, err
	}
	result.Deleted = deleted

	if err := writeStaticAssetManifest(outputDir, plans); err != nil {
		return result, err
	}

	return result, nil
}

func planStaticAssetCopies(cfg *config.Config, outputDir string) ([]staticAssetCopyPlan, error) {
	assets, err := collectStaticAssetFiles(cfg)
	if err != nil {
		return nil, err
	}

	return planStaticAssetCopiesForFiles(assets, outputDir, newAssetFingerprinter(cfg))
}

func planStaticAssetCopiesForFiles(assets []staticAssetFile, outputDir string, fingerprinter *assetFingerprinter) ([]staticAssetCopyPlan, error) {
	plans := make([]staticAssetCopyPlan, 0, len(assets))
	for _, asset := range assets {
		rewritten, err := fingerprinter.resolveFingerprintOutput(asset)
		if err != nil {
			return nil, err
		}
		destPath, err := safeOutputPath(outputDir, filepath.ToSlash(rewritten))
		if err != nil {
			return nil, err
		}
		action, reason, err := decideStaticAssetCopy(asset.SourcePath, destPath)
		if err != nil {
			return nil, err
		}
		plans = append(plans, staticAssetCopyPlan{
			Asset:        asset,
			DestPath:     destPath,
			ManifestPath: manifestAssetPath(rewritten),
			Action:       action,
			Reason:       reason,
		})
	}

	return plans, nil
}

func executeStaticAssetCopyPlan(plans []staticAssetCopyPlan) (staticAssetCopyResult, error) {
	var result staticAssetCopyResult
	for _, plan := range plans {
		if plan.Action == staticAssetSkip {
			result.Skipped++
			continue
		}

		if err := os.MkdirAll(filepath.Dir(plan.DestPath), 0755); err != nil {
			return result, err
		}
		if err := copyFile(plan.Asset.SourcePath, plan.DestPath); err != nil {
			return result, err
		}
		result.Copied++
	}

	return result, nil
}

const staticAssetManifestName = ".gobin-assets.json"

type staticAssetManifest struct {
	Assets []string `json:"assets"`
}

func removeStaleManagedStaticAssets(outputDir string, plans []staticAssetCopyPlan) (int, error) {
	previous, err := readStaticAssetManifest(outputDir)
	if err != nil {
		return 0, err
	}
	if len(previous) == 0 {
		return 0, nil
	}

	current := make(map[string]struct{}, len(plans))
	for _, plan := range plans {
		current[plan.ManifestPath] = struct{}{}
	}

	deleted := 0
	for assetPath := range previous {
		if _, ok := current[assetPath]; ok {
			continue
		}

		cleanPath, ok := cleanManifestAssetPath(assetPath)
		if !ok {
			continue
		}
		targetPath, err := safeOutputPath(outputDir, filepath.ToSlash(cleanPath))
		if err != nil {
			return deleted, err
		}
		info, err := os.Stat(targetPath)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return deleted, err
		}
		if info.IsDir() {
			continue
		}
		if err := os.Remove(targetPath); err != nil {
			return deleted, err
		}
		deleted++
	}

	return deleted, nil
}

func readStaticAssetManifest(outputDir string) (map[string]struct{}, error) {
	content, err := os.ReadFile(filepath.Join(outputDir, staticAssetManifestName))
	if os.IsNotExist(err) {
		return map[string]struct{}{}, nil
	}
	if err != nil {
		return nil, err
	}

	var manifest staticAssetManifest
	if err := json.Unmarshal(content, &manifest); err != nil {
		return nil, fmt.Errorf("read static asset manifest: %w", err)
	}

	assets := make(map[string]struct{}, len(manifest.Assets))
	for _, assetPath := range manifest.Assets {
		if cleanPath, ok := cleanManifestAssetPath(assetPath); ok {
			assets[manifestAssetPath(cleanPath)] = struct{}{}
		}
	}
	return assets, nil
}

func writeStaticAssetManifest(outputDir string, plans []staticAssetCopyPlan) error {
	assets := make([]string, 0, len(plans))
	for _, plan := range plans {
		assets = append(assets, plan.ManifestPath)
	}
	sort.Strings(assets)

	content, err := json.MarshalIndent(staticAssetManifest{Assets: assets}, "", "  ")
	if err != nil {
		return err
	}
	content = append(content, '\n')

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outputDir, staticAssetManifestName), content, 0644)
}

func manifestAssetPath(path string) string {
	return filepath.ToSlash(filepath.Clean(path))
}

func cleanManifestAssetPath(path string) (string, bool) {
	cleanPath := filepath.Clean(filepath.FromSlash(path))
	if cleanPath == "." || cleanPath == staticAssetManifestName || filepath.IsAbs(cleanPath) || cleanPath == ".." || strings.HasPrefix(cleanPath, ".."+string(filepath.Separator)) {
		return "", false
	}
	return cleanPath, true
}

func decideStaticAssetCopy(sourcePath, destPath string) (staticAssetCopyAction, string, error) {
	sourceInfo, err := os.Stat(sourcePath)
	if err != nil {
		return "", "", err
	}

	destInfo, err := os.Stat(destPath)
	if os.IsNotExist(err) {
		return staticAssetCopy, "missing", nil
	}
	if err != nil {
		return "", "", err
	}
	if !destInfo.Mode().IsRegular() {
		return staticAssetCopy, "not-regular", nil
	}
	if sourceInfo.Size() != destInfo.Size() {
		return staticAssetCopy, "size", nil
	}
	if sourceInfo.Mode().Perm() != destInfo.Mode().Perm() {
		return staticAssetCopy, "mode", nil
	}
	if sourceInfo.ModTime().After(destInfo.ModTime()) {
		return staticAssetCopy, "source-newer", nil
	}

	sameContent, err := filesHaveSameContent(sourcePath, destPath)
	if err != nil {
		return "", "", err
	}
	if !sameContent {
		return staticAssetCopy, "content", nil
	}

	return staticAssetSkip, "current", nil
}

func filesHaveSameContent(sourcePath, destPath string) (bool, error) {
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return false, err
	}
	defer sourceFile.Close()

	destFile, err := os.Open(destPath)
	if err != nil {
		return false, err
	}
	defer destFile.Close()

	sourceBuffer := make([]byte, 32*1024)
	destBuffer := make([]byte, 32*1024)
	for {
		sourceRead, sourceErr := sourceFile.Read(sourceBuffer)
		destRead, destErr := destFile.Read(destBuffer)
		if sourceRead != destRead || !bytes.Equal(sourceBuffer[:sourceRead], destBuffer[:destRead]) {
			return false, nil
		}
		if sourceErr == io.EOF && destErr == io.EOF {
			return true, nil
		}
		if sourceErr != nil && sourceErr != io.EOF {
			return false, sourceErr
		}
		if destErr != nil && destErr != io.EOF {
			return false, destErr
		}
	}
}

func copyFile(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}

	_, err = dstFile.ReadFrom(srcFile)
	closeErr := dstFile.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err := os.Chmod(dst, srcInfo.Mode().Perm()); err != nil {
		return err
	}
	return os.Chtimes(dst, srcInfo.ModTime(), srcInfo.ModTime())
}

func collectStaticAssetFiles(cfg *config.Config) ([]staticAssetFile, error) {
	cfg = config.Normalize(cfg)

	type assetSource struct {
		rootDir string
		// prefix is preserved in the output path. The primary static dir
		// (and the theme asset dir) ship their contents to the site root,
		// so prefix is empty. Additional staticDirs (e.g. img, images)
		// keep their directory name as the output prefix so that
		// img/cover.jpg -> public/img/cover.jpg.
		prefix string
	}

	sources := make([]assetSource, 0, 2)
	if cfg.Theme != "" && cfg.ThemesDir != "" {
		themeStaticDir := filepath.Join(cfg.ThemesDir, cfg.Theme, "assets")
		if _, err := os.Stat(themeStaticDir); err == nil {
			sources = append(sources, assetSource{rootDir: themeStaticDir})
		} else if err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("stat theme assets: %w", err)
		}
	}

	staticDirs := cfg.StaticDirs
	if len(staticDirs) == 0 {
		staticDirs = []string{cfg.StaticDir}
	}
	for i, staticDir := range staticDirs {
		if _, err := os.Stat(staticDir); err == nil {
			prefix := ""
			if i > 0 {
				prefix = filepath.Base(staticDir)
			}
			sources = append(sources, assetSource{rootDir: staticDir, prefix: prefix})
		} else if err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("stat static assets %q: %w", staticDir, err)
		}
	}

	overlay := make(map[string]staticAssetFile)
	order := make([]string, 0)

	for _, source := range sources {
		err := filepath.WalkDir(source.rootDir, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				return nil
			}

			relPath, err := filepath.Rel(source.rootDir, path)
			if err != nil {
				return err
			}

			outputPath := filepath.Clean(relPath)
			if source.prefix != "" {
				outputPath = filepath.Join(source.prefix, outputPath)
			}
			if _, seen := overlay[outputPath]; !seen {
				order = append(order, outputPath)
			}
			overlay[outputPath] = staticAssetFile{
				SourcePath: filepath.Clean(path),
				OutputPath: outputPath,
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	sort.Strings(order)

	assets := make([]staticAssetFile, 0, len(order))
	for _, outputPath := range order {
		assets = append(assets, overlay[outputPath])
	}

	return assets, nil
}

func minifyOutput(outputDir string) error {
	return filepath.WalkDir(outputDir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".html" && ext != ".css" && ext != ".js" {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		minified := []byte(minifyContent(string(content), ext))
		if bytes.Equal(content, minified) {
			return nil
		}

		info, err := entry.Info()
		if err != nil {
			return err
		}

		return os.WriteFile(path, minified, info.Mode())
	})
}

func minifyContent(content string, ext string) string {
	switch ext {
	case ".css":
		return minifyCSSContent(content)
	case ".html":
		return minifyHTMLContent(content)
	case ".js":
		// Keep JavaScript source unchanged apart from surrounding whitespace.
		// Regex-based "minification" is too risky for string literals and regexes.
		return strings.TrimSpace(content)
	default:
		return strings.TrimSpace(content)
	}
}

var cssCommentPattern = regexp.MustCompile(`(?s)/\*.*?\*/`)
var cssWhitespacePattern = regexp.MustCompile(`\s+`)
var cssPunctuationPattern = regexp.MustCompile(`\s*([{}:;,>+~])\s*`)

func minifyCSSContent(content string) string {
	content = cssCommentPattern.ReplaceAllString(content, "")
	content = cssWhitespacePattern.ReplaceAllString(content, " ")
	content = cssPunctuationPattern.ReplaceAllString(content, "$1")
	content = strings.ReplaceAll(content, ";}", "}")
	return strings.TrimSpace(content)
}

func minifyHTMLContent(content string) string {
	var out bytes.Buffer
	var text bytes.Buffer
	preserveTag := ""

	flushText := func() {
		if text.Len() == 0 {
			return
		}
		if preserveTag != "" {
			out.Write(text.Bytes())
		} else {
			out.WriteString(collapseHTMLWhitespace(text.String()))
		}
		text.Reset()
	}

	for i := 0; i < len(content); {
		if strings.HasPrefix(content[i:], "<!--") {
			flushText()
			end := strings.Index(content[i+4:], "-->")
			if end < 0 {
				break
			}
			i += 4 + end + 3
			continue
		}

		if content[i] != '<' {
			text.WriteByte(content[i])
			i++
			continue
		}

		flushText()
		tagEnd := strings.IndexByte(content[i:], '>')
		if tagEnd < 0 {
			out.WriteString(content[i:])
			break
		}

		tag := content[i : i+tagEnd+1]
		tagName, closing, selfClosing := parseHTMLTag(tag)
		if tagName != "" && isHTMLWhitespaceSensitiveTag(tagName) {
			if !closing && !selfClosing {
				preserveTag = tagName
			} else if closing && preserveTag == tagName {
				preserveTag = ""
			}
		}

		out.WriteString(strings.TrimSpace(tag))
		i += tagEnd + 1
	}

	flushText()
	return strings.TrimSpace(out.String())
}

func collapseHTMLWhitespace(s string) string {
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return ""
	}
	return strings.Join(fields, " ")
}

func parseHTMLTag(tag string) (name string, closing bool, selfClosing bool) {
	trimmed := strings.TrimSpace(tag)
	trimmed = strings.TrimPrefix(trimmed, "<")
	trimmed = strings.TrimSuffix(trimmed, ">")
	trimmed = strings.TrimSpace(trimmed)
	if trimmed == "" {
		return "", false, false
	}

	if strings.HasPrefix(trimmed, "/") {
		closing = true
		trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "/"))
	}
	if strings.HasSuffix(trimmed, "/") {
		selfClosing = true
		trimmed = strings.TrimSpace(strings.TrimSuffix(trimmed, "/"))
	}
	if trimmed == "" {
		return "", closing, selfClosing
	}

	for i, r := range trimmed {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			return strings.ToLower(trimmed[:i]), closing, selfClosing
		}
	}

	return strings.ToLower(trimmed), closing, selfClosing
}

func isHTMLWhitespaceSensitiveTag(tag string) bool {
	switch tag {
	case "pre", "script", "style", "textarea":
		return true
	default:
		return false
	}
}
