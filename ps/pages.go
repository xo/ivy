package ps

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"math"
	"strings"
	"sync"

	"github.com/xo/okage"
	"github.com/xo/okage/svg"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/display"
	"github.com/xo/ivy/internal/fontdb"
	"github.com/xo/ivy/internal/hayro/pdffont"
	"github.com/xo/ivy/ps/internal/graphics"
	"github.com/xo/ivy/ps/internal/interp"
)

// run is the result of a run of the program of a document.
type run struct {
	pages []graphics.Page
}

// pages runs the program on the first call, and returns its pages. A run
// that the context ends is not kept, so that a later call runs again.
func (d *Document) pages(ctx context.Context) ([]graphics.Page, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.run != nil {
		return d.run.pages, nil
	}
	pages, err := d.interpret(ctx)
	if err != nil {
		return nil, err
	}
	d.run = &run{pages: pages}
	return pages, nil
}

// setup returns the page device at the start of the program.
func (d *Document) setup() graphics.Setup {
	if d.eps && d.hasBBox {
		b := d.bbox
		return graphics.Setup{Width: b[2] - b[0], Height: b[3] - b[1], X: b[0], Y: b[1], FirstPageOnly: true}
	}
	s := graphics.Setup{Width: 612, Height: 792, FirstPageOnly: d.eps}
	if d.media[0] > 0 {
		s.Width, s.Height = d.media[0], d.media[1]
	}
	return s
}

// interpret runs the program and returns its pages. It returns an error
// only when the context ends it. Another error ends the last page, which
// keeps the error.
func (d *Document) interpret(ctx context.Context) (pages []graphics.Page, err error) {
	if d.ps.Size() > d.lim.MaxMemory {
		return nil, fmt.Errorf("postscript of %d bytes: %w", d.ps.Size(), ivy.ErrLimit)
	}
	text := make([]byte, d.ps.Size())
	if _, err := io.ReadFull(io.NewSectionReader(d.ps, 0, d.ps.Size()), text); err != nil {
		return nil, fmt.Errorf("reading postscript: %w", err)
	}
	log := d.cfg.Log()
	in := interp.New(interp.Config{Limits: d.lim, Log: log})
	m := graphics.New(in, graphics.Config{Setup: d.setup(), Fonts: d.fonts.find, Log: log})
	in.Finish()
	defer func() {
		if r := recover(); r != nil {
			err = nil
			m.Finish(fmt.Errorf("postscript: panic: %v: %w", r, ivy.ErrFormat))
			pages = m.Pages()
		}
	}()
	runErr := in.Run(ctx, interp.NewFile("program", text))
	if ctxErr := ctx.Err(); ctxErr != nil {
		return nil, fmt.Errorf("running postscript: %w", ctxErr)
	}
	if runErr != nil {
		log.Debug("postscript: the program ended with an error", "error", runErr)
	}
	m.Finish(runErr)
	return m.Pages(), nil
}

// PageCount satisfies the ivy.Pager interface. It runs the whole program
// on its first call.
func (d *Document) PageCount(ctx context.Context) (int, error) {
	pages, err := d.pages(ctx)
	if err != nil {
		return 0, err
	}
	return len(pages), nil
}

// Page satisfies the ivy.Pager interface.
func (d *Document) Page(ctx context.Context, n int) (ivy.Page, error) {
	pages, err := d.pages(ctx)
	if err != nil {
		return nil, err
	}
	if n < 0 || n >= len(pages) {
		return nil, ivy.ErrPageRange
	}
	return &page{p: pages[n]}, nil
}

// page is a page of a document.
type page struct {
	p graphics.Page
}

// Size satisfies the ivy.Page interface.
func (p *page) Size() (float32, float32) {
	return float32(p.p.Width), float32(p.p.Height)
}

// The interfaces that a page implements.
var (
	_ ivy.Page       = (*page)(nil)
	_ display.Drawer = (*page)(nil)
)

// Render satisfies the ivy.Page interface. It returns a *ivy.PartialError
// for a page that an error of the program ended early.
func (p *page) Render(ctx context.Context, dst *okage.Pixmap, t okage.Transform) error {
	return p.draw(ctx, display.NewOkage(dst, t), t)
}

// Draw satisfies the display.Drawer interface (D58).
func (p *page) Draw(ctx context.Context, d display.Device, res float32) error {
	return p.draw(ctx, d, okage.TransformFromScale(res, res))
}

// draw replays the page onto dev, whose user space is the space of the
// page in points. t maps the page onto the pixels of the target, and sets
// the resolution of the parts that the page draws into a pixmap.
func (p *page) draw(ctx context.Context, dev display.Device, t okage.Transform) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = &ivy.PartialError{Err: fmt.Errorf("rendering postscript page: panic: %v: %w", r, ivy.ErrFormat)}
		}
	}()
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("rendering postscript page: %w", err)
	}
	s := float32(1 / graphics.ResolutionScale)
	dev.Save()
	defer dev.Restore()
	dev.Concat(okage.TransformFromScale(s, s))
	pixels := math.Sqrt(math.Abs(float64(t.SX*t.SY-t.KX*t.KY))) / graphics.ResolutionScale
	p.p.List.Replay(dev, pixels)
	if p.p.Err != nil {
		return &ivy.PartialError{Err: p.p.Err}
	}
	return nil
}

// fontSource finds the fonts that a program names and does not define, in
// the fonts of ivy.Config (D23, D24).
type fontSource struct {
	fsys   fs.FS
	system bool
	lim    ivy.Limits
	log    *slog.Logger

	once sync.Once
	db   *svg.FontDatabase
	mu   sync.Mutex
	// found keeps the program of each name, or nil.
	found map[string]*graphics.FontProgram
}

// newFontSource returns the font source of cfg, or nil when cfg gives no
// font.
func newFontSource(cfg ivy.Config, lim ivy.Limits) *fontSource {
	if cfg.Fonts == nil && !cfg.SystemFonts {
		return nil
	}
	return &fontSource{fsys: cfg.Fonts, system: cfg.SystemFonts, lim: lim, log: cfg.Log(), found: map[string]*graphics.FontProgram{}}
}

// find is a graphics.FontSource. A nil source finds no font.
func (f *fontSource) find(name string) (*graphics.FontProgram, bool) {
	if f == nil {
		return nil, false
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if fp, ok := f.found[name]; ok {
		return fp, fp != nil
	}
	f.once.Do(func() {
		f.db, _ = fontdb.Load(f.fsys, f.system, f.lim, f.log)
	})
	fp := f.lookup(name)
	f.found[name] = fp
	return fp, fp != nil
}

// lookup finds the face of a name in the database.
func (f *fontSource) lookup(name string) *graphics.FontProgram {
	if f.db == nil {
		return nil
	}
	lower := strings.ToLower(name)
	weight := uint16(400)
	if strings.Contains(lower, "bold") || strings.Contains(lower, "black") || strings.Contains(lower, "heavy") {
		weight = 700
	}
	italic := strings.Contains(lower, "italic") || strings.Contains(lower, "oblique")
	family, _, _ := strings.Cut(name, "-")
	for _, n := range []string{name, family} {
		id, ok := fontdb.Find(f.db, n, weight, italic)
		if !ok {
			continue
		}
		data, index, ok := f.db.FaceData(id)
		if !ok {
			continue
		}
		prog, err := pdffont.NewProgram(data, index)
		if err != nil {
			continue
		}
		return &graphics.FontProgram{Program: prog}
	}
	return nil
}
