package render

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	maxEncodedImageBytes = 5 * 1024 * 1024
	minRenderWidth       = 1200
	widthStepRatio       = 0.85
)

type Renderer struct {
	workDir  string
	format   string
	dpi      int
	maxWidth int
}

func New(workDir, format string, dpi, maxWidth int) *Renderer {
	_ = os.MkdirAll(workDir, 0o755)
	return &Renderer{
		workDir:  workDir,
		format:   strings.TrimPrefix(format, "."),
		dpi:      dpi,
		maxWidth: maxWidth,
	}
}

func (r *Renderer) Capture(ctx context.Context, sheetID string, gid int64, captureRange string, bearerToken string) (string, error) {
	stamp := time.Now().Format("20060102-150405")
	pdfPath := filepath.Join(r.workDir, "kpi-"+stamp+".pdf")
	prefix := filepath.Join(r.workDir, "kpi-"+stamp)
	finalPath := prefix + "." + r.ext()

	if err := r.downloadPDF(ctx, sheetID, gid, captureRange, bearerToken, pdfPath); err != nil {
		return "", err
	}
	dpi := fmt.Sprint(r.dpi)
	if err := run(ctx, "pdftoppm", "-png", "-r", dpi, pdfPath, prefix); err != nil {
		return "", err
	}
	renderedPNGs, err := renderedPages(prefix)
	if err != nil {
		return "", err
	}
	var lastEncodedSize int
	for _, width := range renderWidths(r.maxWidth) {
		args := r.magickArgs(renderedPNGs, dpi, width, finalPath)
		if err := run(ctx, "magick", args...); err != nil {
			if fallbackErr := run(ctx, "convert", args...); fallbackErr != nil {
				return "", fmt.Errorf("magick failed: %w; convert fallback failed: %w", err, fallbackErr)
			}
		}
		content, err := os.ReadFile(finalPath)
		if err != nil {
			return "", err
		}
		encoded := base64.StdEncoding.EncodeToString(content)
		if len(encoded) <= maxEncodedImageBytes {
			return encoded, nil
		}
		lastEncodedSize = len(encoded)
	}
	return "", fmt.Errorf("encoded image is %.2f MB, SeaTalk limit is 5 MB", float64(lastEncodedSize)/(1024*1024))
}

func (r *Renderer) magickArgs(renderedPNGs []string, dpi string, maxWidth int, finalPath string) []string {
	args := make([]string, 0, len(renderedPNGs)*6+24)
	for _, page := range renderedPNGs {
		args = append(args,
			"(",
			page,
			"-density", dpi,
			"-fuzz", "2%",
			"-trim", "+repage",
			")",
		)
	}
	args = append(args,
		"-append",
		"-bordercolor", "white",
		"-border", fmt.Sprintf("%dx%d", r.marginPixels(), r.marginPixels()),
		"-filter", "Lanczos",
		"-resize", fmt.Sprintf("%dx>", maxWidth),
		"-unsharp", "0x0.75+0.75+0.008",
		"-quality", "92",
		"-define", "png:compression-level=9",
		"-strip",
	)
	if r.ext() == "jpg" {
		args = append(args, "-background", "white", "-alpha", "remove", "-alpha", "off")
	}
	return append(args, finalPath)
}

func renderWidths(maxWidth int) []int {
	widths := []int{maxWidth}
	for width := int(float64(maxWidth) * widthStepRatio); width >= minRenderWidth; width = int(float64(width) * widthStepRatio) {
		widths = append(widths, width)
	}
	if maxWidth > minRenderWidth && widths[len(widths)-1] != minRenderWidth {
		widths = append(widths, minRenderWidth)
	}
	return widths
}

func (r *Renderer) downloadPDF(ctx context.Context, sheetID string, gid int64, captureRange, bearerToken, path string) error {
	params := url.Values{}
	params.Set("format", "pdf")
	params.Set("gid", fmt.Sprint(gid))
	params.Set("range", captureRange)
	params.Set("size", "A4")
	params.Set("portrait", "false")
	params.Set("fitw", "true")
	params.Set("sheetnames", "false")
	params.Set("printtitle", "false")
	params.Set("pagenumbers", "false")
	params.Set("gridlines", "false")
	params.Set("fzr", "false")
	exportURL := "https://docs.google.com/spreadsheets/d/" + sheetID + "/export?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, exportURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+bearerToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("sheet export status %d: %s", resp.StatusCode, string(body))
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(file, resp.Body)
	return err
}

func (r *Renderer) Cleanup() {}

func (r *Renderer) marginPixels() int {
	pixels := r.dpi / 4
	if pixels < 1 {
		return 1
	}
	return pixels
}

func renderedPages(prefix string) ([]string, error) {
	paths, err := filepath.Glob(prefix + "-*.png")
	if err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		if _, err := os.Stat(prefix + ".png"); err == nil {
			return []string{prefix + ".png"}, nil
		}
		return nil, fmt.Errorf("no rendered png found at %s*.png", prefix)
	}
	sort.Slice(paths, func(i, j int) bool {
		return pageNumber(paths[i], prefix) < pageNumber(paths[j], prefix)
	})
	return paths, nil
}

func pageNumber(path, prefix string) int {
	value := strings.TrimSuffix(strings.TrimPrefix(path, prefix+"-"), ".png")
	page, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return page
}

func (r *Renderer) ext() string {
	if r.format == "jpeg" {
		return "jpg"
	}
	return r.format
}

func run(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %v: %w: %s", name, args, err, string(output))
	}
	return nil
}
