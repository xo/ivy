package msole

import (
	"context"
	"fmt"
	"iter"
	"strings"
	"time"

	"github.com/xo/magic/cfb"

	"github.com/xo/ivy"
)

// Summary is what the stream "\x05SummaryInformation" says about a
// document.
type Summary struct {
	// Info holds the title, the author, the subject, the dates and the name
	// of the program.
	Info ivy.Info
	// Thumbnail is the thumbnail, or nil.
	Thumbnail *cfb.ClipboardData
}

// ReadSummary reads the summary information of f. A file without the stream
// has an empty summary, and so does a stream that is damaged: the summary
// is a help, and a fault in it does not stop the document. It returns an
// error only for a stream over the limits.
func ReadSummary(f *cfb.File, lim ivy.Limits) (Summary, error) {
	if !Has(f, SummaryName) {
		return Summary{}, nil
	}
	buf, err := ReadStream(f, SummaryName, lim)
	if err != nil {
		return Summary{}, err
	}
	return ParseSummary(buf), nil
}

// ParseSummary reads the summary information in buf, the bytes of the stream
// "\x05SummaryInformation". It returns an empty summary for a stream that is
// damaged.
func ParseSummary(buf []byte) Summary {
	ps, err := cfb.ParsePropertySet(buf)
	if err != nil {
		return Summary{}
	}
	sec := ps.Section(cfb.FMTIDSummaryInformation)
	if sec == nil {
		return Summary{}
	}
	var s Summary
	s.Info.Title = stringProperty(sec, cfb.PIDSITitle)
	s.Info.Subject = stringProperty(sec, cfb.PIDSISubject)
	if author := stringProperty(sec, cfb.PIDSIAuthor); author != "" {
		s.Info.Authors = []string{author}
	}
	s.Info.Producer = stringProperty(sec, cfb.PIDSIAppName)
	s.Info.Created = timeProperty(sec, cfb.PIDSICreateTime)
	s.Info.Modified = timeProperty(sec, cfb.PIDSILastSave)
	if cd, ok := ps.Thumbnail(); ok && len(cd.Data) > 0 {
		s.Thumbnail = &cd
	}
	return s
}

// stringProperty returns the text of the string property id of sec, without
// the space around it, or "".
func stringProperty(sec *cfb.Section, id uint32) string {
	p, ok := sec.Property(id)
	if !ok {
		return ""
	}
	var text string
	switch v := p.Value.(type) {
	case string:
		text = v
	case cfb.CodePageString:
		var ok bool
		if text, ok = v.Text(); !ok {
			b := v.Bytes
			if i := strings.IndexByte(string(b), 0); i >= 0 {
				b = b[:i]
			}
			text = Decode(b, v.CodePage)
		}
	}
	text = strings.TrimRight(text, "\x00")
	return strings.TrimSpace(strings.ToValidUTF8(text, "�"))
}

// timeProperty returns the time of the property id of sec, or the zero time.
// A time before 1980 is not the time of an Office document, and some writers
// put the duration of the edit there, so it is the zero time too.
func timeProperty(sec *cfb.Section, id uint32) time.Time {
	p, ok := sec.Property(id)
	if !ok {
		return time.Time{}
	}
	ft, ok := p.Value.(cfb.Filetime)
	if !ok {
		return time.Time{}
	}
	t := ft.Time()
	if t.Year() < 1980 {
		return time.Time{}
	}
	return t
}

// Previews lists the thumbnail of the summary, if it has one that the
// package decodes. A thumbnail that it cannot read is left out, and logged.
func (s Summary) Previews(ctx context.Context, cfg ivy.Config) iter.Seq2[ivy.Preview, error] {
	return func(yield func(ivy.Preview, error) bool) {
		if err := ctx.Err(); err != nil {
			yield(ivy.Preview{}, fmt.Errorf("listing the previews: %w", err))
			return
		}
		if s.Thumbnail == nil {
			return
		}
		p, err := ThumbnailPreview(*s.Thumbnail, cfg)
		if err != nil {
			cfg.Log().Debug("msole: thumbnail left out", "err", err)
			return
		}
		yield(p, nil)
	}
}
