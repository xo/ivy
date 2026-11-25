package review

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"image"
	"image/color"
	"image/png"
	"math"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/emulation"
	cdppage "github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

// chromeNames are the names of the program of Chrome that the review looks
// for on the PATH, in this order.
var chromeNames = []string{"google-chrome-stable", "google-chrome", "chromium", "chromium-browser"}

// chromeProgram is the name of the program of Chrome on the PATH, or the
// first of chromeNames when the PATH has none.
var chromeProgram = func() string {
	for _, name := range chromeNames {
		if _, err := exec.LookPath(name); err == nil {
			return name
		}
	}
	return chromeNames[0]
}()

// chromeRenderer draws a page of a PDF file with Chrome, whose viewer draws
// with PDFium (D69). The tag changes with the way that the review takes the
// picture of the page.
var chromeRenderer = pageRenderer{id: "chrome", name: "Chrome", tag: "pdfium5", tools: []string{chromeProgram}, render: chromeDraw}

// chromeViewer starts the URL of the frame of the PDF viewer of Chrome, a
// component extension.
const chromeViewer = "chrome-extension://mhjfbmdgcfjbbpaeojofohoefgiehjai/"

// chromeBrowser is the one Chrome process of a run. It starts at the first
// page that Chrome draws, and each page opens a tab of its own.
type chromeBrowser struct {
	once sync.Once
	// ctx is the context of the browser, as chromedp makes it. The context of
	// a tab is a child of it. chromedp keeps the browser in the context, so
	// the browser keeps the context for the run.
	ctx    context.Context
	cancel context.CancelFunc
	// dir holds the profile of Chrome and its temporary files.
	dir string
	err error
}

// browser returns the context of the browser of the run, and starts Chrome
// the first time.
func (e *refEnv) browser() (context.Context, error) {
	c := &e.chrome
	c.once.Do(func() {
		c.err = c.start(e.env)
	})
	return c.ctx, c.err
}

// start starts Chrome in headless mode, with the environment env of the
// programs. Chrome makes its socket in TMPDIR, whose path must be short,
// so TMPDIR is a new folder in the temporary folder of the system.
func (c *chromeBrowser) start(env []string) error {
	path, err := exec.LookPath(chromeProgram)
	if err != nil {
		return fmt.Errorf("the program %s is not installed: %w", chromeProgram, err)
	}
	if c.dir, err = os.MkdirTemp("", "ivy-chrome-"); err != nil {
		return fmt.Errorf("making the folder of Chrome: %w", err)
	}
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath(path),
		// the headless mode of Chrome itself, which shows a PDF file in its
		// viewer. The old headless shell downloads the file.
		chromedp.Flag("headless", "new"),
		// the viewer places a page as if it had a scroll bar, so the page must
		// have one
		chromedp.Flag("hide-scrollbars", false),
		chromedp.UserDataDir(filepath.Join(c.dir, "profile")),
		chromedp.NoInheritEnv,
		chromedp.Env(slices.Concat(env, []string{"TMPDIR=" + c.dir}, c.fontsEnv(env))...),
	)
	actx, acancel := chromedp.NewExecAllocator(context.Background(), opts...)
	// chromedp logs the events of the frames of the viewer that it does not
	// follow, such as DOM.documentUpdated. A page that fails returns its
	// error.
	ctx, cancel := chromedp.NewContext(actx, chromedp.WithErrorf(func(string, ...any) {}))
	c.ctx, c.cancel = ctx, func() {
		cancel()
		acancel()
	}
	if err := chromedp.Do(ctx); err != nil {
		return fmt.Errorf("starting Chrome: %w", err)
	}
	return nil
}

// cjkAliases are the CJK families that a PDF file names without embedding
// them, with the font that Chrome draws them with. The viewer of Chrome asks
// fontconfig for the family by its name, and fontconfig answers with a font
// that has none of the glyphs, so the page shows no CJK text. Droid Sans
// Fallback is a font of ivy, which fontconfig finds in the fonts of the
// review (D28).
var cjkAliases = []string{
	// Japanese
	"KozMinPr6N-Regular", "KozMinPro-Regular", "KozGoPr6N-Medium", "KozGoPro-Medium", "HeiseiMin-W3",
	"HeiseiKakuGo-W5", "Ryumin-Light", "GothicBBB-Medium", "MS-Mincho", "MS-Gothic", "MSMincho", "MSGothic",
	// Simplified Chinese
	"STSong-Light", "STSongStd-Light", "STSong", "SimSun", "SimHei", "MHei-Medium", "AdobeSongStd-Light",
	"AdobeHeitiStd-Regular", "AdobeFangsongStd-Regular", "AdobeKaitiStd-Regular",
	// Traditional Chinese
	"MSung-Light", "MSungStd-Light", "MingLiU", "AdobeMingStd-Light",
	// Korean
	"HYSMyeongJo-Medium", "HYGoThic-Medium", "HYSMyeongJoStd-Medium", "HYGothic-Medium", "Batang", "Gulim",
	"AdobeMyungjoStd-Medium", "AdobeGothicStd-Bold",
}

// fontsEnv returns the variable FONTCONFIG_FILE for Chrome, or none. The
// file that it names includes the configuration of fontconfig in env, and
// maps cjkAliases to Droid Sans Fallback. A failure leaves Chrome with the
// configuration of the other references.
func (c *chromeBrowser) fontsEnv(env []string) []string {
	var base string
	for _, kv := range env {
		if v, ok := strings.CutPrefix(kv, "FONTCONFIG_FILE="); ok {
			base = v
		}
	}
	if base == "" {
		return nil
	}
	var b strings.Builder
	b.WriteString(`<?xml version="1.0"?>
<!DOCTYPE fontconfig SYSTEM "urn:fontconfig:fonts.dtd">
<fontconfig>
  <include ignore_missing="yes">` + html.EscapeString(base) + `</include>
`)
	for _, name := range cjkAliases {
		b.WriteString(`  <alias binding="strong"><family>` + html.EscapeString(name) +
			`</family><prefer><family>Droid Sans Fallback</family></prefer></alias>` + "\n")
	}
	b.WriteString("</fontconfig>\n")
	conf := filepath.Join(c.dir, "fonts.conf")
	if err := os.WriteFile(conf, []byte(b.String()), 0o644); err != nil {
		return nil
	}
	return []string{"FONTCONFIG_FILE=" + conf}
}

// closeChrome ends the Chrome process of the run, if it started, and
// removes its folder.
func (e *refEnv) closeChrome() {
	c := &e.chrome
	if c.cancel != nil {
		c.cancel()
	}
	if c.dir != "" {
		_ = os.RemoveAll(c.dir)
	}
}

// pageBox is the size of a page of a PDF file in points, from pdfinfo: the
// size of its CropBox, with the rotation of the page.
func pageBox(ctx context.Context, j *refJob, in string, n int) (float64, float64, error) {
	p := strconv.Itoa(n)
	args := slices.Concat([]string{"-f", p, "-l", p}, j.passwordArgs(), []string{in})
	out, err := runTool(ctx, j.env, "", "pdfinfo", args...)
	if err != nil {
		return 0, 0, err
	}
	var w, h float64
	rot, sized := 0, false
	for line := range strings.SplitSeq(out, "\n") {
		f := strings.Fields(line)
		if len(f) < 4 || f[0] != "Page" || f[1] != p {
			continue
		}
		switch {
		// Page    1 size:  489.6 x 734.4 pts
		case f[2] == "size:" && len(f) >= 6:
			var errW, errH error
			w, errW = strconv.ParseFloat(f[3], 64)
			h, errH = strconv.ParseFloat(f[5], 64)
			if err := errors.Join(errW, errH); err != nil {
				return 0, 0, fmt.Errorf("reading the size of page %d from pdfinfo: %w", n, err)
			}
			sized = true
		// Page    1 rot:   90
		case f[2] == "rot:":
			if rot, err = strconv.Atoi(f[3]); err != nil {
				return 0, 0, fmt.Errorf("reading the rotation of page %d from pdfinfo: %w", n, err)
			}
		}
	}
	if !sized || w <= 0 || h <= 0 {
		return 0, 0, &toolError{fmt.Errorf("pdfinfo gives no size of page %d", n)}
	}
	if rot%180 != 0 {
		w, h = h, w
	}
	return w, h, nil
}

// mupdfSize returns the size in pixels of a page of w by h points that MuPDF
// draws at dpi. MuPDF rounds the sides of the page up.
func mupdfSize(w, h float64, dpi int) image.Point {
	side := func(v float64) int { return int(math.Ceil(v*float64(dpi)/72 - 0.001)) }
	return image.Pt(side(w), side(h))
}

// chromePage is the state of page n of a PDF file in the viewer of Chrome.
type chromePage struct {
	// Pages is the number of pages of the file, or 0 before the viewer
	// loaded it. Password reports that the viewer asks for a password.
	Pages    int  `json:"pages"`
	Password bool `json:"password"`
	// Width and Height are the size of the page at the zoom 1, in CSS
	// pixels.
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// chromeRect is the place of a page on the screen of the viewer, in CSS
// pixels, and the zoom of the viewer.
type chromeRect struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
	Zoom   float64 `json:"zoom"`
	// ViewWidth and ViewHeight are the size of the viewer, in CSS pixels.
	ViewWidth  float64 `json:"viewWidth"`
	ViewHeight float64 `json:"viewHeight"`
}

// The scripts that run in the frame of the viewer. They read the element
// pdf-viewer of the viewer of Chrome and its viewport.
const (
	chromeStateJS = `(() => {
  const v = document.querySelector('pdf-viewer');
  if (!v) return {pages: 0, password: false};
  const pw = !!(v.shadowRoot && v.shadowRoot.querySelector('viewer-password-dialog'));
  const d = v.documentDimensions;
  if (!v.viewport_ || !d || !d.pageDimensions) return {pages: 0, password: pw};
  const p = d.pageDimensions[%d];
  return {pages: d.pageDimensions.length, password: pw, width: p ? p.width : 0, height: p ? p.height : 0};
})()`
	chromePasswordJS = `(() => {
  const d = document.querySelector('pdf-viewer').shadowRoot.querySelector('viewer-password-dialog');
  d.dispatchEvent(new CustomEvent('password-submitted', {detail: {password: %s}}));
})()`
	chromeRectJS = `(() => {
  const vp = document.querySelector('pdf-viewer').viewport_;
  if (vp.getZoom() !== 1) vp.setZoom(1);
  vp.goToPage(%d);
  const r = vp.getPageScreenRect(%d);
  const s = vp.size;
  return {x: r.x, y: r.y, width: r.width, height: r.height, zoom: vp.getZoom(), viewWidth: s.width, viewHeight: s.height};
})()`
)

// chromeMargin is the space in CSS pixels that the viewport of a tab gives
// a page at each side, for the shadow of the page and the scroll bar.
const chromeMargin = 40

// chromeDraw draws page n, from 1, of the PDF file in with Chrome. Chrome
// opens the file in a tab at the zoom 100%, which gives a point 4/3 CSS
// pixels, with a scale of the device of dpi/96. The picture is the page
// alone, cut from the screen of the viewer without its shadow and its
// background, at the size of the CropBox that MuPDF draws.
func chromeDraw(ctx context.Context, j *refJob, in string, n int, out string) error {
	w, h, err := pageBox(ctx, j, in, n)
	if err != nil {
		return err
	}
	size := mupdfSize(w, h, j.dpi)
	bctx, err := j.browser()
	if err != nil {
		return err
	}
	tab, cancel := chromedp.NewContext(bctx)
	defer cancel()
	if d, ok := ctx.Deadline(); ok {
		var cancelTime context.CancelFunc
		tab, cancelTime = context.WithDeadline(tab, d)
		defer cancelTime()
	}
	stop := context.AfterFunc(ctx, cancel)
	defer stop()
	img, err := chromeShot(tab, in, n, w, h, float64(j.dpi)/96, j.password)
	if err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("the time ended before Chrome drew page %d: %w", n, ctx.Err())
		}
		return err
	}
	return writePNG(out, fitEdges(img, size))
}

// fitEdges returns img at the size size, without the pixels of its edges.
// Each pixel at an edge of the result, or out of img, takes the closest
// pixel inside the edges of img. Chrome places a page at a fraction of a
// pixel when the scale of the device is not whole, so the pixels at its
// edges mix the page with its shadow. Its page can also be a pixel smaller
// or larger than the page of MuPDF, because Chrome rounds the size of a page
// to whole CSS pixels and MuPDF rounds it up to whole pixels.
func fitEdges(img image.Image, size image.Point) *image.RGBA {
	src := flatten(img)
	b := src.Rect
	dst := image.NewRGBA(image.Rectangle{Max: size})
	clamp := func(v, lo, hi int) int { return min(max(v, lo), hi) }
	for y := range size.Y {
		sy := b.Min.Y + clamp(y, 1, max(1, b.Dy()-2))
		for x := range size.X {
			sx := b.Min.X + clamp(x, 1, max(1, b.Dx()-2))
			if !image.Pt(sx, sy).In(b) {
				dst.Set(x, y, image.White)
				continue
			}
			dst.SetRGBA(x, y, src.RGBAAt(sx, sy))
		}
	}
	return dst
}

// chromeShot opens the PDF file in in the tab of ctx, and returns the
// picture of page n, from 1, of w by h points, at the scale scale of the
// device. If the viewer asks for a password, it gives password.
func chromeShot(ctx context.Context, in string, n int, w, h, scale float64, password string) (image.Image, error) {
	if err := chromedp.Do(ctx); err != nil {
		return nil, fmt.Errorf("opening a tab of Chrome: %w", err)
	}
	// the tab gets the size of the page with its margins before the viewer
	// opens the file. The viewer does not draw the page again when the size
	// of the tab changes later.
	tw, th := math.Ceil(w*4/3+2*chromeMargin), math.Ceil(h*4/3+2*chromeMargin)
	_, err := chromedp.Call(ctx, emulation.SetDeviceMetricsOverride, emulation.SetDeviceMetricsOverrideParams{
		Width: int64(tw), Height: int64(th), DeviceScaleFactor: scale, ScreenWidth: int64(tw), ScreenHeight: int64(th),
	})
	if err != nil {
		return nil, fmt.Errorf("setting the size of the tab: %w", err)
	}
	u := url.URL{Scheme: "file", Path: in, Fragment: "toolbar=0&zoom=100"}
	if err := chromedp.Do(ctx, chromedp.Navigate(u.String())); err != nil {
		return nil, fmt.Errorf("opening the file in Chrome: %w", err)
	}
	viewer, cancel, err := chromeViewerFrame(ctx)
	if err != nil {
		return nil, err
	}
	defer cancel()
	// the viewer loads the file
	var st chromePage
	var asked time.Time
	for {
		st, err = chromedp.Run(viewer, chromedp.Evaluate[chromePage](fmt.Sprintf(chromeStateJS, n-1)))
		if err != nil {
			return nil, fmt.Errorf("reading the viewer of Chrome: %w", err)
		}
		switch {
		case st.Pages > 0:
		case st.Password && password == "":
			return nil, &toolError{errors.New("the viewer of Chrome asks for a password, and the file has none in the manifest")}
		case st.Password && asked.IsZero():
			// the dialog of the password gives the password to the viewer with
			// this event
			pw, err := json.Marshal(password)
			if err != nil {
				return nil, fmt.Errorf("encoding the password: %w", err)
			}
			if _, err := chromedp.Run(viewer, chromedp.Evaluate[chromedp.Void](fmt.Sprintf(chromePasswordJS, pw))); err != nil {
				return nil, fmt.Errorf("giving the password to the viewer of Chrome: %w", err)
			}
			asked = time.Now()
		case st.Password && time.Since(asked) > 5*time.Second:
			return nil, &toolError{errors.New("the viewer of Chrome did not open the file with the password")}
		}
		if st.Pages > 0 {
			break
		}
		if err := sleep(ctx, 100*time.Millisecond); err != nil {
			return nil, err
		}
	}
	if n > st.Pages || st.Width <= 0 || st.Height <= 0 {
		return nil, &toolError{fmt.Errorf("the viewer of Chrome has no page %d", n)}
	}
	// the viewer shows the page. The place of the page counts when it stays
	// the same.
	var r, prev chromeRect
	for try := 0; ; try++ {
		if err := sleep(ctx, 100*time.Millisecond); err != nil {
			return nil, err
		}
		r, err = chromedp.Run(viewer, chromedp.Evaluate[chromeRect](fmt.Sprintf(chromeRectJS, n-1, n-1)))
		if err != nil {
			return nil, fmt.Errorf("finding the page in the viewer of Chrome: %w", err)
		}
		if r == prev || try >= 50 {
			break
		}
		prev = r
	}
	if r.Zoom != 1 || r.Width <= 0 || r.Height <= 0 {
		return nil, &toolError{fmt.Errorf("the viewer of Chrome shows page %d at the zoom %g, at %gx%g", n, r.Zoom, r.Width, r.Height)}
	}
	// the whole page must be on the screen of the viewer
	if r.X < 0 || r.Y < 0 || r.X+r.Width > r.ViewWidth || r.Y+r.Height > r.ViewHeight {
		return nil, &toolError{fmt.Errorf("the viewer of Chrome, of %gx%g, shows page %d in part, at %g,%g", r.ViewWidth, r.ViewHeight, n, r.X, r.Y)}
	}
	// the viewer can place a page at half a CSS pixel, and Chrome draws it
	// from the whole device pixel before that place
	snap := func(v float64) float64 { return math.Floor(v*scale) / scale }
	return chromeStable(ctx, &cdppage.Viewport{X: snap(r.X), Y: snap(r.Y), Width: r.Width, Height: r.Height, Scale: 1})
}

// chromeViewerFrame returns a context of the frame of the PDF viewer of the
// tab of ctx, and the function that detaches from it. The frame is a target
// of its own, a child of the tab.
func chromeViewerFrame(ctx context.Context) (context.Context, context.CancelFunc, error) {
	tab := chromedp.FromContext(ctx).Target.TargetID
	for {
		infos, err := chromedp.Targets(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("listing the targets of Chrome: %w", err)
		}
		for _, info := range infos {
			if info.Type == "iframe" && info.ParentID == tab && strings.HasPrefix(info.URL, chromeViewer) {
				viewer, cancel := chromedp.NewContext(ctx, chromedp.WithTargetID(info.TargetID), chromedp.WithDetachOnCancel())
				return viewer, cancel, nil
			}
		}
		if err := sleep(ctx, 100*time.Millisecond); err != nil {
			return nil, nil, err
		}
	}
}

// Limits of the wait for the drawing of a page by the viewer of Chrome.
const (
	// chromeSame is the number of pictures in a row that must be the same.
	chromeSame = 4
	// chromeStep is the time between two pictures.
	chromeStep = 200 * time.Millisecond
	// chromeBlank is the time after which a page with the color of the
	// background of the viewer alone counts as drawn.
	chromeBlank = 20 * time.Second
)

// chromeStable takes pictures of the part clip of the tab of ctx until
// chromeSame pictures in a row are the same, because the viewer draws a
// page in steps, and returns the last one. The viewer shows its background
// in place of a page that it has not drawn yet, and a page can take seconds
// before its first step. So a picture of the color of the background alone
// does not count, until chromeBlank passes.
func chromeStable(ctx context.Context, clip *cdppage.Viewport) (image.Image, error) {
	shot := func(clip *cdppage.Viewport) ([]byte, image.Image, error) {
		res, err := chromedp.Call(ctx, cdppage.CaptureScreenshot, cdppage.CaptureScreenshotParams{Clip: clip})
		if err != nil {
			return nil, nil, fmt.Errorf("taking the picture of the page in Chrome: %w", err)
		}
		img, err := png.Decode(bytes.NewReader(res.Data))
		if err != nil {
			return nil, nil, fmt.Errorf("decoding the picture of Chrome: %w", err)
		}
		return res.Data, img, nil
	}
	// a pixel at the corner of the tab, out of the page, has the color of
	// the background
	_, corner, err := shot(&cdppage.Viewport{X: 1, Y: 1, Width: 1, Height: 1, Scale: 1})
	if err != nil {
		return nil, err
	}
	bg := corner.At(corner.Bounds().Min.X, corner.Bounds().Min.Y)
	start := time.Now()
	var last []byte
	count := 0
	for {
		data, img, err := shot(clip)
		if err != nil {
			return nil, err
		}
		switch {
		case uniform(img, bg) && time.Since(start) < chromeBlank:
			last, count = nil, 0
		case bytes.Equal(data, last):
			count++
		default:
			last, count = data, 1
		}
		if count == chromeSame {
			return img, nil
		}
		if err := sleep(ctx, chromeStep); err != nil {
			return nil, err
		}
	}
}

// uniform reports whether each pixel of img has the color c.
func uniform(img image.Image, c color.Color) bool {
	r0, g0, b0, a0 := c.RGBA()
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if r, g, bl, a := img.At(x, y).RGBA(); r != r0 || g != g0 || bl != b0 || a != a0 {
				return false
			}
		}
	}
	return true
}

// sleep waits for d, or for the end of ctx.
func sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return fmt.Errorf("waiting for Chrome: %w", ctx.Err())
	case <-t.C:
		return nil
	}
}
