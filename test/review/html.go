package review

import (
	"fmt"
	"html/template"
	"image"
	"io"
	"slices"
	"sort"
	"strings"
)

// WriteHTML writes the report rep as a page of HTML to w. The page holds
// its style and its script, and links to the images with paths relative to
// its folder, so it works when a browser opens it from the disk.
func WriteHTML(w io.Writer, rep *Report) error {
	if err := pageTemplate.Execute(w, rep); err != nil {
		return fmt.Errorf("writing the html: %w", err)
	}
	return nil
}

// metricClass returns the class of a blurred difference: good, fair or bad.
func metricClass(v float64) string {
	switch {
	case v < 0:
		return "none"
	case v < 2:
		return "good"
	case v < 5:
		return "fair"
	}
	return "bad"
}

// refLabel returns the name of the reference with the id id, such as
// "LibreOffice" for "soffice", "PDF" for a PDF file that the references
// draw as it is, or id when it has no name.
func refLabel(id string) string {
	if id == pdfRoute.id {
		return "PDF"
	}
	if name := refName(id); name != "" {
		return name
	}
	return id
}

// refName returns the name of the conversion of the reference with the id
// id, or an empty string when it has none.
func refName(id string) string {
	for _, x := range externals {
		if x.id == id {
			return x.name
		}
	}
	for r := range routeKinds {
		if r.id == id {
			return r.name
		}
	}
	return ""
}

// rendererName returns the name of the page renderer with the id id, such
// as "MuPDF" for "mupdf", or id when it is not known.
func rendererName(id string) string {
	all := slices.Concat(pdfRenderers, []pageRenderer{chromeRenderer})
	for _, x := range externals {
		all = append(all, x.renderers...)
	}
	for r := range routeKinds {
		all = append(all, r.renderers...)
	}
	for _, pr := range all {
		if pr.id == id {
			return pr.name
		}
	}
	return id
}

// worstByColumn returns the worst numbers of d against each column of a
// reference, in the order of the columns.
func worstByColumn(d *DocReport) []refWorst {
	var out []refWorst
	seen := map[string]bool{}
	add := func(key string) {
		v, ok := d.WorstByRenderer[key]
		if !ok || seen[key] {
			return
		}
		seen[key] = true
		ref, renderer, _ := strings.Cut(key, "/")
		out = append(out, refWorst{ID: key, Name: refColumn(refName(ref), rendererName(renderer)), Worst: v})
	}
	for _, r := range d.Rows {
		for _, c := range r.Refs {
			add(c.Ref + "/" + c.Renderer)
		}
	}
	keys := make([]string, 0, len(d.WorstByRenderer))
	for k := range d.WorstByRenderer {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		add(k)
	}
	return out
}

// kindTab is a tab of the report: a group of kinds.
type kindTab struct {
	id, name string
	kinds    []string
}

// kindTabs are the tabs of the report, in their order. A kind that no tab
// names goes into the tab "Other".
var kindTabs = []kindTab{
	{"pdf", "PDF", []string{"pdf", "ai"}},
	{"word", "Word", []string{"docx", "dotx", "docm", "dotm", "doc", "dot"}},
	{"excel", "Excel", []string{"xlsx", "xltx", "xlsm", "xltm", "xls", "xlt"}},
	{"powerpoint", "PowerPoint", []string{"pptx", "potx", "ppsx", "pptm", "ppsm", "ppt", "pps", "pot"}},
	{"opendocument", "OpenDocument", []string{"odt", "ott", "odm", "ods", "ots", "odp", "otp", "odg", "otg", "fodt", "fods", "fodp", "fodg"}},
	{"office", "Other Office", []string{"vsdx", "vsdm", "vstx", "vstm", "vsd", "vst", "vss", "pub", "pages", "key", "numbers", "rtf", "csv", "tsv", "tab"}},
	{"pagedesc", "Page descriptions", []string{"xps", "oxps", "ps", "eps", "epsi", "epsf", "ept", "emf", "wmf", "emz", "wmz", "djvu", "djv"}},
	{"ebook", "Ebooks and text", []string{"epub", "kepub", "mobi", "prc", "azw", "azw3", "azw4", "fb2", "fbz", "htmlz", "txtz", "chm", "md", "markdown", "txt", "text"}},
	{"html", "HTML", []string{"html", "htm", "xhtml"}},
	{"latex", "LaTeX", texKinds},
	{"image", "Images", []string{"psd", "psb", "ora", "kra", "raw", "cr2", "nef", "nrw", "arw", "srf", "sr2", "dng", "orf", "rw2", "pef", "srw", "raf", "cbz", "cbt"}},
}

// docTab is a tab of the report with its documents.
type docTab struct {
	ID, Name string
	Docs     []*DocReport
	// Failed counts the documents that ivy did not render, see failed.
	Failed int
	// Mean is the mean Worst of the documents with a metric, Measured of
	// them, or -1 when none has one.
	Mean     float64
	Measured int
}

// docTabs returns the tabs of the report that hold documents, with the
// documents of each one in the order of docs.
func docTabs(docs []*DocReport) []docTab {
	tabs := make([]docTab, len(kindTabs)+1)
	for i, k := range kindTabs {
		tabs[i] = docTab{ID: k.id, Name: k.name}
	}
	tabs[len(kindTabs)] = docTab{ID: "other", Name: "Other"}
	for _, d := range docs {
		i := slices.IndexFunc(kindTabs, func(k kindTab) bool { return contains(k.kinds, d.Kind) })
		if i < 0 {
			i = len(kindTabs)
		}
		tabs[i].Docs = append(tabs[i].Docs, d)
	}
	out := tabs[:0]
	for _, t := range tabs {
		if len(t.Docs) == 0 {
			continue
		}
		t.Failed = failed(t.Docs)
		sum := 0.0
		for _, d := range t.Docs {
			if d.Worst >= 0 {
				sum += d.Worst
				t.Measured++
			}
		}
		t.Mean = -1
		if t.Measured > 0 {
			t.Mean = sum / float64(t.Measured)
		}
		out = append(out, t)
	}
	return out
}

// failed counts the documents of docs that ivy did not render: each status
// but ok and partial.
func failed(docs []*DocReport) int {
	n := 0
	for _, d := range docs {
		if d.Status != StatusOK && d.Status != StatusPartial {
			n++
		}
	}
	return n
}

// refWorst is the worst number of a document against one reference.
type refWorst struct {
	ID, Name string
	Worst    float64
}

// worstBy returns the worst numbers of d against each reference, the
// primary references first.
func worstBy(d *DocReport) []refWorst {
	out := make([]refWorst, 0, len(d.WorstBy))
	for id, v := range d.WorstBy {
		out = append(out, refWorst{ID: id, Name: refLabel(id), Worst: v})
	}
	rank := func(id string) int {
		for _, x := range externals {
			if x.id == id && contains(x.primary, d.Kind) {
				return 0
			}
			if x.id == id {
				return 2
			}
		}
		return 1
	}
	sort.Slice(out, func(i, j int) bool {
		if a, b := rank(out[i].ID), rank(out[j].ID); a != b {
			return a < b
		}
		return out[i].ID < out[j].ID
	})
	return out
}

var pageTemplate = template.Must(template.New("page").Funcs(template.FuncMap{
	"f2": func(v float64) string {
		if v < 0 {
			return "-"
		}
		return fmt.Sprintf("%.2f", v)
	},
	"sec": func(ms float64) string { return fmt.Sprintf("%.1f", ms/1000) },
	"msf": func(ms float64) string {
		if ms >= 1000 {
			return fmt.Sprintf("%.1f s", ms/1000)
		}
		return fmt.Sprintf("%.0f ms", ms)
	},
	"size": func(p image.Point) string {
		if p.X == 0 && p.Y == 0 {
			return ""
		}
		return fmt.Sprintf("%d×%d", p.X, p.Y)
	},
	"first": func(s string) string {
		line, _, _ := strings.Cut(s, "\n")
		if len(line) > 300 {
			line = line[:300] + "..."
		}
		return line
	},
	"long":   func(s string) bool { return strings.Contains(s, "\n") || len(s) > 300 },
	"mclass": metricClass,
	"pages": func(n int) string {
		if n < 0 {
			return "?"
		}
		return fmt.Sprint(n)
	},
	"cols": func(d *DocReport) int {
		n := 1 + len(d.RefNames)
		if len(d.Rows) != 0 && d.Rows[0].IvyPDF != nil {
			n++
		}
		return n
	},
	"kinds": func(docs []*DocReport) []string {
		seen := map[string]bool{}
		var out []string
		for _, d := range docs {
			if !seen[d.Kind] {
				seen[d.Kind] = true
				out = append(out, d.Kind)
			}
		}
		return out
	},
	"statuses": func(counts map[string]int) []string {
		var out []string
		for _, s := range []string{StatusPanic, StatusTimeout, StatusError, StatusMissing, StatusUnsupported, StatusPartial, StatusOK} {
			if counts[s] != 0 {
				out = append(out, s)
			}
		}
		return out
	},
	"hasPrefix":     strings.HasPrefix,
	"worstBy":       worstBy,
	"refLabel":      refLabel,
	"refColumn":     refColumn,
	"tabs":          docTabs,
	"failed":        failed,
	"worstByColumn": worstByColumn,
	"multi": func(d *DocReport) bool {
		return len(d.WorstBy) > 1 || len(d.Externals) > 0
	},
}).Parse(pageHTML))

// pageHTML is the template of index.html.
const pageHTML = `{{define "doc"}}
<section class="doc" id="{{.ID}}" data-kind="{{.Kind}}" data-status="{{.Status}}" data-name="{{.Name}}">
<h2>{{.Name}} <span class="badge {{.Status}}">{{.Status}}</span></h2>
<p class="meta">{{.Kind}}{{if .Source}} · {{.Source}}{{end}}{{if .License}} · {{.License}}{{end}} · {{if hasPrefix .URL "https://"}}<a href="{{.URL}}">{{.URL}}</a>{{else}}{{.URL}}{{end}}{{if .Note}} · {{.Note}}{{end}}</p>
<p class="meta">ivy: {{if .Preview}}a preview{{else}}{{pages .Pages}} pages{{end}}, open {{msf .OpenMS}}, render {{msf .RenderMS}} · reference{{if .Reference}} through {{.Reference}}{{end}}: {{pages .RefPages}} pages · total {{msf .TotalMS}}{{if ge .IvyPDFWorst 0.0}} · ivy PDF vs MuPDF <span class="{{mclass .IvyPDFWorst}}">{{f2 .IvyPDFWorst}}</span>{{end}}{{if .Disagree}} · <span class="warnbadge">the references disagree on {{.Disagree}} pages</span>{{end}}{{with .MediaBoxWorst}} · MediaBox: worst <span class="{{mclass .}}">{{f2 .}}</span>{{end}}{{if .SizeDiffers}} · <span class="bad">{{.SizeDiffers}} pages of another size</span>{{end}}</p>
{{if .Paper}}<p class="meta">paper: {{.Paper.Name}}{{if .Paper.Width}}, {{.Paper.Width}} × {{.Paper.Height}} pt{{end}}</p>{{end}}
{{if multi .}}<p class="meta">worst by reference: {{range $j, $w := worstBy .}}{{if $j}} · {{end}}{{$w.Name}} <span class="{{mclass $w.Worst}}">{{f2 $w.Worst}}</span>{{end}}{{range .Externals}} · {{.Name}}: {{pages .Pages}} pages, {{.File}}{{end}}</p>{{end}}
{{with worstByColumn .}}{{if gt (len .) 1}}<p class="meta">worst by column: {{range $j, $w := .}}{{if $j}} · {{end}}{{$w.Name}} <span class="{{mclass $w.Worst}}">{{f2 $w.Worst}}</span>{{end}}</p>{{end}}{{end}}
{{range .Externals}}{{if .Error}}<p class="err">{{.Name}}: {{first .Error}}</p>{{end}}{{end}}
{{if .Error}}<p class="err">ivy: {{first .Error}}</p>{{if long .Error}}<details><summary>the whole error</summary><pre>{{.Error}}</pre></details>{{end}}{{end}}
{{if .RefError}}<p class="err">reference: {{first .RefError}}</p>{{if long .RefError}}<details><summary>the whole error</summary><pre>{{.RefError}}</pre></details>{{end}}{{end}}
{{if .IvyPDFError}}<p class="err">ivy PDF: {{first .IvyPDFError}}</p>{{if long .IvyPDFError}}<details><summary>the whole error</summary><pre>{{.IvyPDFError}}</pre></details>{{end}}{{end}}
{{if .Log}}<details{{if le (len .Log) 4}} open{{end}}><summary>{{len .Log}} messages of the log of ivy</summary><pre>{{range .Log}}{{.}}
{{end}}</pre></details>{{end}}
{{if .IvyPDFLog}}<details><summary>{{len .IvyPDFLog}} messages of the log of ivy PDF</summary><pre>{{range .IvyPDFLog}}{{.}}
{{end}}</pre></details>{{end}}
{{$cols := cols .}}{{$d := .}}{{range .Rows}}{{$row := .}}
<div class="row" style="--cols:{{$cols}}">
<div class="rowhead">Page {{.Page}}{{if eq .Box "media"}}, MediaBox{{if ge .RefsDiff 0.0}} <span class="m">the references differ by {{f2 .RefsDiff}}</span>{{end}}{{end}}{{if .Disagrees}} <span class="warnbadge">references disagree ({{f2 .RefsDiff}})</span>{{end}}{{if and .Primary (multi $d)}} <span class="m">primary: {{refLabel .Primary}}</span>{{end}}{{if .SizeDiffers}} <span class="bad">the size differs from {{refLabel .Primary}}</span>{{end}}{{if .Refs}} <button type="button" class="rowdiff">differences</button>{{end}}</div>
{{with .Ivy}}<figure class="ivy{{if $row.SizeDiffers}} sizediff{{end}}"><figcaption>{{.Name}}</figcaption>
{{if .Thumb}}<a class="plain" href="{{.Image}}"><img loading="lazy" src="{{.Thumb}}"{{template "dims" .Size}} alt="page of ivy"></a>{{end}}
<div class="m">{{size .Size}}{{if .MS}} · {{msf .MS}}{{end}}{{if and .Status (ne .Status "ok")}} · <span class="{{.Status}}">{{.Status}}</span>{{end}}</div>
{{if .Error}}<div class="ce">{{first .Error}}</div>{{end}}</figure>{{end}}
{{with .IvyPDF}}<figure class="ref ivypdf"><figcaption>{{.Name}}</figcaption>
{{if .Thumb}}<a class="plain" href="{{.Image}}"><img loading="lazy" src="{{.Thumb}}"{{template "dims" .Size}} alt="page of ivy PDF"></a>{{end}}
{{if .DiffThumb}}<a class="diff" href="{{.Diff}}"><img loading="lazy" src="{{.DiffThumb}}"{{template "dims" .Size}} alt="difference of ivy PDF from MuPDF"></a>{{end}}
<div class="m">{{with .Metric}}<span class="{{mclass .Blurred}}">vs MuPDF: blurred {{f2 .Blurred}}</span> · mean {{f2 .Mean}}{{if .Scaled}} · <span class="{{if .SizeOff}}bad{{else}}fair{{end}}">ivy PDF {{size .IvySize}} scaled</span>{{end}} · {{end}}{{size .Size}}{{if .MS}} · {{msf .MS}}{{end}}{{if and .Status (ne .Status "ok")}} · <span class="{{.Status}}">{{.Status}}</span>{{end}}</div>
{{if .Error}}<div class="ce">{{first .Error}}</div>{{end}}</figure>{{end}}
{{range .Refs}}<figure class="ref{{if .Fault}} fault{{end}}"><figcaption>{{refColumn .RefName .Name}}</figcaption>
{{if .Thumb}}<a class="plain" href="{{.Image}}"><img loading="lazy" src="{{.Thumb}}"{{template "dims" .Size}} alt="page of {{.Name}}"></a>{{end}}
{{if .DiffThumb}}<a class="diff" href="{{.Diff}}"><img loading="lazy" src="{{.DiffThumb}}"{{template "dims" .Size}} alt="difference from {{.Name}}"></a>{{end}}
<div class="m">{{with .Metric}}<span class="{{mclass .Blurred}}">blurred {{f2 .Blurred}}</span> · mean {{f2 .Mean}}{{if .Scaled}} · <span class="{{if .SizeOff}}bad{{else}}fair{{end}}">ivy {{size .IvySize}} scaled</span>{{end}} · {{end}}{{size .Size}}{{if .MS}} · {{msf .MS}}{{end}}</div>
{{if .Fault}}<div class="fnote">known fault of the reference, not counted: {{.Fault}}</div>{{end}}
{{if .Error}}<div class="ce">{{first .Error}}</div>{{end}}</figure>{{end}}
</div>{{end}}
</section>
{{end}}{{define "dims"}}{{if .X}} width="{{.X}}" height="{{.Y}}"{{end}}{{end}}<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>ivy review</title>
<style>
:root{--fg:#1d2125;--muted:#5f6b76;--line:#d8dee4;--bg:#fff;--panel:#f5f7f9;--good:#1a7f37;--fair:#9a6700;--bad:#cf222e;--link:#0b5cad}
@media (prefers-color-scheme:dark){:root{--fg:#e6edf3;--muted:#9aa7b3;--line:#30363d;--bg:#0d1117;--panel:#161b22;--good:#3fb950;--fair:#d29922;--bad:#f85149;--link:#58a6ff}}
*{box-sizing:border-box}
body{margin:0;padding:16px 20px 60px;font:14px/1.45 system-ui,-apple-system,"Segoe UI",sans-serif;color:var(--fg);background:var(--bg)}
a{color:var(--link)}
h1{font-size:20px;margin:0 0 4px}
h2{font-size:16px;margin:0;word-break:break-all}
.muted{color:var(--muted)}
.run{margin:0 0 12px}
.counts span{margin-right:12px}
.bar{position:sticky;top:0;z-index:2;background:var(--bg);padding:8px 0;border-bottom:1px solid var(--line);display:flex;gap:14px;flex-wrap:wrap;align-items:center}
.bar label{white-space:nowrap}
table.sum{border-collapse:collapse;width:100%;margin:10px 0 24px;font-size:13px}
.sum th,.sum td{padding:3px 8px;border-bottom:1px solid var(--line);text-align:left;vertical-align:top}
.sum th{cursor:pointer;user-select:none;background:var(--panel);position:sticky;top:44px}
.sum td.n{text-align:right;font-variant-numeric:tabular-nums}
.sum td.name{word-break:break-all}
.badge{display:inline-block;padding:0 6px;border-radius:9px;font-size:12px;font-weight:600;border:1px solid currentColor}
.ok{color:var(--good)}.partial,.unsupported,.missing{color:var(--fair)}.error,.panic,.timeout{color:var(--bad)}
.good{color:var(--good)}.fair{color:var(--fair)}.bad{color:var(--bad)}.none{color:var(--muted)}
section.doc{border-top:2px solid var(--line);padding:14px 0 6px}
.meta{margin:4px 0;color:var(--muted)}
pre{white-space:pre-wrap;word-break:break-word;background:var(--panel);padding:8px;border-radius:4px;max-height:360px;overflow:auto;font-size:12px;margin:4px 0}
p.err{color:var(--bad);margin:4px 0;word-break:break-word}
details{margin:4px 0}
.row{display:grid;grid-template-columns:repeat(var(--cols),minmax(0,1fr));gap:10px;margin:12px 0}
.rowhead{grid-column:1/-1;display:flex;gap:12px;align-items:baseline;font-weight:600}
.rowhead button{font-size:12px}
figure{margin:0;min-width:0}
figcaption{font-weight:600;font-size:13px}
figure img{display:block;width:100%;height:auto;border:1px solid var(--line);background:#fff}
.m{font-size:12px;color:var(--muted);font-variant-numeric:tabular-nums}
.ce{font-size:12px;color:var(--bad);word-break:break-word;max-height:6em;overflow:auto}
.diff{display:none}
body.showdiff .diff,.row.showdiff .diff{display:block}
body.showdiff .ref .plain,.row.showdiff .ref .plain{display:none}
.warnbadge{color:var(--fair);font-weight:600}
figure.fault img{opacity:.55;border:2px dashed var(--fair)}
.fnote{font-size:12px;color:var(--fair);word-break:break-word}
figure.sizediff img{border:2px solid var(--bad)}
table.kinds{border-collapse:collapse;margin:6px 0 12px;font-size:13px}
.kinds th,.kinds td{padding:2px 10px;border-bottom:1px solid var(--line);text-align:right}
.kinds th:first-child,.kinds td:first-child{text-align:left}
.hidden{display:none}
.tabs{display:flex;flex-wrap:wrap;gap:6px;margin:12px 0 4px}
.tabs[hidden]{display:none}
.tabs button{font:inherit;color:var(--fg);background:var(--panel);border:1px solid var(--line);border-radius:6px;padding:4px 10px;cursor:pointer}
.tabs button[aria-selected=true]{border-color:var(--link);box-shadow:inset 0 -2px 0 var(--link)}
.tabs .n{color:var(--muted);font-size:12px}
h2.tabhead{font-size:18px;margin:18px 0 2px}
section.doc figure a{cursor:zoom-in}
.gal{position:fixed;inset:0;z-index:10;display:flex;flex-direction:column;background:rgba(10,12,16,.94);color:#e6edf3}
.gal.hidden{display:none}
.galtop{display:flex;gap:12px;align-items:center;justify-content:space-between;padding:10px 16px;border-bottom:1px solid #30363d;flex-wrap:wrap}
.galtitle{min-width:0;flex:1 1 320px}
.galtitle h2{font-size:15px;color:#e6edf3}
.galcap{font-size:13px;color:#9aa7b3;margin-top:2px}
.galtools{display:flex;gap:14px;align-items:center;font-size:13px;white-space:nowrap}
.galtools a{color:#58a6ff}
.gal button{font:inherit;color:#e6edf3;background:#21262d;border:1px solid #30363d;border-radius:6px;padding:4px 10px;cursor:pointer}
.gal button:hover{background:#30363d}
.gal kbd{font-size:11px;border:1px solid #30363d;border-radius:4px;padding:0 4px;color:#9aa7b3}
.galmain{flex:1;min-height:0;display:flex;align-items:center;gap:8px;padding:10px}
.galimg{flex:1;min-width:0;height:100%;display:flex;align-items:center;justify-content:center}
.galimg img{max-width:100%;max-height:100%;object-fit:contain;background:#fff;box-shadow:0 2px 16px rgba(0,0,0,.5)}
.galnav{font-size:26px!important;line-height:1;padding:10px 12px!important;flex:none}
.galnav:disabled{opacity:.3;cursor:default}
.galinfo{max-height:32vh;overflow:auto;overflow-wrap:anywhere;padding:8px 16px 14px;border-top:1px solid #30363d;font-size:13px}
.galinfo .meta,.galinfo .m{color:#9aa7b3}
.galinfo .ce,.galinfo .err{color:#f85149}
.galinfo .fnote{color:#d29922}
.galinfo .good{color:#3fb950}.galinfo .fair{color:#d29922}.galinfo .bad{color:#f85149}
.galinfo pre{background:#161b22;color:#e6edf3}
@media (max-width:640px){.galnav{display:none}.galtools{flex-wrap:wrap}}
</style>
</head>
<body>
<h1>ivy review</h1>
<p class="run muted">{{.Created.Format "2006-01-02 15:04 UTC"}} · {{.DPI}} dpi · {{if gt .Pages 0}}the first {{.Pages}} pages and the last page{{else}}each page{{end}} · timeout {{.Timeout}} · {{printf "%.0f" .Elapsed}} s · {{len .Docs}} documents</p>
<p class="counts">{{range statuses .Counts}}<span class="{{.}}">{{.}}: {{index $.Counts .}}</span>{{end}}</p>
{{range .Notes}}<p class="err">{{.}}</p>{{end}}
<details><summary>Programs</summary><pre>{{range .Programs}}{{.Name}}: {{.Version}}
{{end}}</pre></details>
<p class="muted">Worst: for each page, the blurred difference of ivy from its primary reference, and the largest of these over the pages. Blurred: the mean difference of the grey levels after a blur of 5 by 5 pixels, from 0 to 255. Mean: the mean difference of the channels. Under 2 is green, under 5 is amber. ivy PDF vs MuPDF: when the references convert a document to PDF, ivy renders that PDF file too, as the column ivy PDF. The number is the largest blurred difference of that column from MuPDF over the pages. It shows the gaps of the PDF renderer of ivy. It is not a reference, so Worst and Refs disagree leave it out. MediaBox: if the MediaBox of a page of a PDF file differs from its CropBox, the page has a second row that shows the MediaBox. The other rows show the CropBox. Worst and Refs disagree leave the rows of the MediaBox out, and the header of the document gives their worst.</p>
<p class="muted">References: {{if eq .Refs "all"}}every reference, with each PDF file drawn by MuPDF, Poppler Splash, Poppler Cairo and Chrome (-refs all){{else}}the primary oracle of each kind, with its PDF file drawn by MuPDF and Chrome, and at most one secondary oracle, with its PDF file drawn by MuPDF (D69){{end}}. A column names the conversion, then the renderer, such as Microsoft Office - MuPDF. Primary reference: a file of Microsoft Office is compared with Microsoft Office first, when its PDF file is in the cache, an XPS file with Windows XPS, a metafile with GDI+, and an iWork file with Apple iWork. Worst uses the primary reference of each page, drawn by MuPDF for a PDF file. Chrome does not count for Worst. By reference gives the worst against each reference, and worst by column the worst against each column. Fault: a cell with a dashed border is a known fault of the reference, from reference-faults.tsv, and no number counts it. Size: a page of ivy whose size differs by more than one pixel from the page of the primary reference has a red border. Paper: a document whose file gives no paper renders on each paper of -papers, with the paper in its name (D32).</p>
{{if .Papers}}<p class="muted">Papers: {{range $i, $p := .Papers}}{{if $i}}, {{end}}{{$p.Name}}{{end}}. Fonts of the system: {{if .SystemFonts}}yes{{else}}no{{end}}{{range .FontDirs}} · {{.}}{{end}}.</p>{{end}}
<table class="kinds"><thead><tr><th>Kind</th><th>Documents</th><th>Pages</th><th>Pages of another size</th></tr></thead><tbody>
{{range .Kinds}}<tr><td>{{.Kind}}</td><td>{{.Documents}}</td><td>{{.Pages}}</td><td{{if .SizeDiffers}} class="bad"{{end}}>{{.SizeDiffers}}</td></tr>
{{end}}</tbody></table>
<div class="bar">
<label>Kind <select id="fkind"><option value="">all</option>{{range kinds .Docs}}<option>{{.}}</option>{{end}}</select></label>
<label>Status <select id="fstatus"><option value="">all</option>{{range statuses .Counts}}<option>{{.}}</option>{{end}}</select></label>
<label>Name <input id="fname" type="search" size="18"></label>
<label><input id="showdiff" type="checkbox"> show the differences</label>
<span id="shown" class="muted"></span>
</div>
{{$tabs := tabs .Docs}}<nav class="tabs" role="tablist" aria-label="the kinds of the documents" hidden>
<button type="button" role="tab" data-tab="all" aria-selected="true">All <span class="n">{{len .Docs}}{{with failed .Docs}}, {{.}} failed{{end}}</span></button>
{{range $tabs}}<button type="button" role="tab" data-tab="{{.ID}}" aria-controls="tab-{{.ID}}" aria-selected="false">{{.Name}} <span class="n">{{len .Docs}}{{if .Failed}}, {{.Failed}} failed{{end}}</span></button>
{{end}}</nav>
{{range $tabs}}<section class="tab" id="tab-{{.ID}}" data-tab="{{.ID}}" role="tabpanel">
<h2 class="tabhead">{{.Name}}</h2>
<p class="muted">{{len .Docs}} documents{{if .Failed}} · <span class="bad">{{.Failed}} failed</span>{{end}} · mean worst <span class="{{mclass .Mean}}">{{f2 .Mean}}</span>{{if .Measured}} over {{.Measured}} documents{{end}}</p>
<table class="sum">
<thead><tr><th data-k="name">Name</th><th data-k="kind">Kind</th><th data-k="status">Status</th><th data-k="pages" data-num>Pages ivy / ref</th><th data-k="worst" data-num>Worst</th><th data-k="byref">By reference</th><th data-k="sizediff" data-num>Size differs</th><th data-k="ivypdf" data-num>ivy PDF vs MuPDF</th><th data-k="disagree" data-num>Refs disagree</th><th data-k="time" data-num>Time s</th><th data-k="source">Source</th><th data-k="license">License</th></tr></thead>
<tbody>
{{range $i, $d := .Docs}}<tr data-id="{{.ID}}" data-kind="{{.Kind}}" data-status="{{.Status}}" data-name="{{.Name}}" data-pages="{{.Pages}}" data-worst="{{.Worst}}" data-byref="{{range worstBy .}}{{.Name}} {{end}}" data-sizediff="{{.SizeDiffers}}" data-ivypdf="{{.IvyPDFWorst}}" data-disagree="{{.Disagree}}" data-time="{{.TotalMS}}" data-source="{{.Source}}" data-license="{{.License}}" data-order="{{$i}}">
<td class="name"><a href="#{{.ID}}">{{.Name}}</a></td><td>{{.Kind}}</td><td><span class="badge {{.Status}}">{{.Status}}</span></td><td class="n">{{if .Preview}}preview{{else}}{{pages .Pages}}{{end}} / {{pages .RefPages}}</td><td class="n {{mclass .Worst}}">{{f2 .Worst}}</td><td>{{if multi .}}{{range $j, $w := worstBy .}}{{if $j}} · {{end}}{{$w.Name}} <span class="{{mclass $w.Worst}}">{{f2 $w.Worst}}</span>{{end}}{{end}}</td><td class="n{{if .SizeDiffers}} bad{{end}}">{{if .SizeDiffers}}{{.SizeDiffers}}{{end}}</td><td class="n {{mclass .IvyPDFWorst}}">{{f2 .IvyPDFWorst}}</td><td class="n">{{if .Disagree}}{{.Disagree}}{{end}}</td><td class="n">{{sec .TotalMS}}</td><td>{{.Source}}</td><td>{{.License}}</td></tr>
{{end}}</tbody>
</table>
{{range .Docs}}{{template "doc" .}}{{end}}
</section>
{{end}}<div id="gal" class="gal hidden" role="dialog" aria-modal="true" aria-label="gallery of the images of a document">
<div class="galtop">
<div class="galtitle"><h2 id="galdoc"></h2><div id="galcap" class="galcap"></div></div>
<div class="galtools"><span id="galpos"></span><label><input id="galdiff" type="checkbox"> differences <kbd>d</kbd></label><a id="galopen" href="#" target="_blank" rel="noopener">open the file</a><button type="button" id="galclose" aria-label="close the gallery">✕</button></div>
</div>
<div class="galmain"><button type="button" class="galnav" id="galprev" aria-label="previous image">‹</button><div class="galimg"><img id="galimg" alt=""></div><button type="button" class="galnav" id="galnext" aria-label="next image">›</button></div>
<div id="galinfo" class="galinfo"></div>
</div>
<script>
(function(){
var rows=[].slice.call(document.querySelectorAll('table.sum tbody tr'));
var tabs=[].slice.call(document.querySelectorAll('[role=tab]')),panes=[].slice.call(document.querySelectorAll('section.tab'));
var secs={};[].forEach.call(document.querySelectorAll('section.doc'),function(s){secs[s.id]=s});
var fk=document.getElementById('fkind'),fs=document.getElementById('fstatus'),fn=document.getElementById('fname');
function filter(){
  var k=fk.value,s=fs.value,n=fn.value.toLowerCase(),shown=0;
  rows.forEach(function(r){
    var ok=(!k||r.dataset.kind===k)&&(!s||r.dataset.status===s)&&(!n||r.dataset.name.toLowerCase().indexOf(n)>=0);
    r.classList.toggle('hidden',!ok);secs[r.dataset.id].classList.toggle('hidden',!ok);
    if(ok&&!r.closest('section.tab').classList.contains('hidden'))shown++;
  });
  document.getElementById('shown').textContent=shown+' of '+rows.length+' documents';
}
[fk,fs].forEach(function(e){e.addEventListener('change',filter)});fn.addEventListener('input',filter);
// A click on a heading sorts the table of the heading.
[].forEach.call(document.querySelectorAll('table.sum'),function(table){
  var dir={},tb=table.querySelector('tbody'),trs=[].slice.call(tb.rows);
  [].forEach.call(table.querySelectorAll('th'),function(th){
    th.addEventListener('click',function(){
      var k=th.dataset.k,num=th.hasAttribute('data-num');dir[k]=dir[k]===undefined?(num?-1:1):-dir[k];
      trs.sort(function(a,b){
        var x=a.dataset[k],y=b.dataset[k];
        if(num){x=parseFloat(x);y=parseFloat(y)}
        return (x<y?-1:x>y?1:0)*dir[k]||a.dataset.order-b.dataset.order;
      });
      trs.forEach(function(r){tb.appendChild(r)});
    });
  });
});
// The tabs show the documents of one group of kinds, or all of them. The
// hash of the URL keeps the tab, such as #tab=word, so that a reload or a
// link opens it. The hash names no element, so the browser does not scroll.
// Without the script, the nav stays hidden and every group shows.
function showTab(id,keep){
  if(id!=='all'&&!panes.some(function(p){return p.dataset.tab===id}))id='all';
  tabs.forEach(function(b){b.setAttribute('aria-selected',b.dataset.tab===id?'true':'false')});
  panes.forEach(function(p){p.classList.toggle('hidden',id!=='all'&&p.dataset.tab!==id)});
  if(!keep&&location.hash!=='#tab='+id)history.replaceState(null,'','#tab='+id);
  filter();
}
tabs.forEach(function(b){b.addEventListener('click',function(){showTab(b.dataset.tab)})});
if(tabs.length)tabs[0].parentNode.hidden=false;
(function(){
  var h=decodeURIComponent(location.hash.slice(1));
  if(h.indexOf('tab=')===0){showTab(h.slice(4));window.scrollTo(0,0);return}
  // a link to a document opens the tab of the document
  var el=h&&document.getElementById(h),pane=el&&el.closest&&el.closest('section.tab');
  showTab(pane?pane.dataset.tab:'all',true);
  if(pane)el.scrollIntoView();
})();
document.getElementById('showdiff').addEventListener('change',function(e){document.body.classList.toggle('showdiff',e.target.checked)});
[].forEach.call(document.querySelectorAll('button.rowdiff'),function(b){
  b.addEventListener('click',function(){b.closest('.row').classList.toggle('showdiff')});
});
// The gallery shows the images of one document, in the order of the page:
// ivy, ivy PDF and each reference, row by row.
var gal=document.getElementById('gal'),gimg=document.getElementById('galimg'),gdoc=document.getElementById('galdoc'),
  gcap=document.getElementById('galcap'),gpos=document.getElementById('galpos'),gdiff=document.getElementById('galdiff'),
  gopen=document.getElementById('galopen'),ginfo=document.getElementById('galinfo'),
  gprev=document.getElementById('galprev'),gnext=document.getElementById('galnext'),items=[],cur=0,gsec=null;
function rowLabel(row){
  var h=row&&row.querySelector('.rowhead');if(!h)return '';
  var c=h.cloneNode(true);[].forEach.call(c.querySelectorAll('button'),function(b){b.remove()});
  return c.textContent.replace(/\s+/g,' ').trim();
}
function itemsOf(sec){
  var out=[];
  [].forEach.call(sec.querySelectorAll('figure'),function(f){
    var p=f.querySelector('a.plain'),d=f.querySelector('a.diff');if(!p&&!d)return;
    var cap=f.querySelector('figcaption');
    out.push({fig:f,plain:p?p.getAttribute('href'):'',diff:d?d.getAttribute('href'):'',
      cap:cap?cap.textContent.trim():'',page:rowLabel(f.closest('.row'))});
  });
  return out;
}
function copyOf(el){var c=el.cloneNode(true);c.removeAttribute('id');return c}
function show(){
  var it=items[cur],diff=gdiff.checked&&!!it.diff,src=diff||!it.plain?it.diff:it.plain;
  gimg.src=src;gimg.alt=it.page+' · '+it.cap;gopen.href=src;
  gcap.textContent=it.page+' · '+it.cap+(diff?' · difference':gdiff.checked?' · no image of the differences':'');
  gpos.textContent=(cur+1)+' / '+items.length;
  gprev.disabled=cur===0;gnext.disabled=cur===items.length-1;
  ginfo.textContent='';
  [].forEach.call(it.fig.querySelectorAll('.m,.fnote,.ce'),function(e){ginfo.appendChild(copyOf(e))});
  [].forEach.call(gsec.children,function(e){
    if(e.matches('p.meta,p.err')||(e.tagName==='DETAILS'))ginfo.appendChild(copyOf(e));
  });
  [cur-1,cur+1].forEach(function(i){var n=items[i];if(n){(new Image()).src=gdiff.checked&&n.diff?n.diff:(n.plain||n.diff)}});
}
function openGallery(sec,fig,diff){
  items=itemsOf(sec);if(!items.length)return;gsec=sec;
  cur=Math.max(0,items.findIndex(function(it){return it.fig===fig}));
  var h=sec.querySelector('h2');gdoc.innerHTML=h?h.innerHTML:'';
  var row=fig.closest('.row');
  gdiff.checked=diff||document.body.classList.contains('showdiff')||!!(row&&row.classList.contains('showdiff'));
  gal.classList.remove('hidden');document.body.style.overflow='hidden';show();
  document.getElementById('galclose').focus();
}
function closeGallery(){gal.classList.add('hidden');document.body.style.overflow='';gimg.removeAttribute('src')}
function step(n){var i=cur+n;if(i>=0&&i<items.length){cur=i;show()}}
document.addEventListener('click',function(e){
  var a=e.target.closest&&e.target.closest('section.doc figure a');
  if(!a||e.button!==0||e.metaKey||e.ctrlKey||e.shiftKey||e.altKey)return;
  e.preventDefault();openGallery(a.closest('section.doc'),a.closest('figure'),a.classList.contains('diff'));
});
gprev.addEventListener('click',function(){step(-1)});gnext.addEventListener('click',function(){step(1)});
document.getElementById('galclose').addEventListener('click',closeGallery);
gdiff.addEventListener('change',show);
gal.addEventListener('click',function(e){if(e.target===gal||e.target.classList.contains('galmain')||e.target.classList.contains('galimg'))closeGallery()});
document.addEventListener('keydown',function(e){
  if(gal.classList.contains('hidden')||e.metaKey||e.ctrlKey||e.altKey)return;
  switch(e.key){
  case 'Escape':closeGallery();break;
  case 'ArrowRight':step(1);break;
  case 'ArrowLeft':step(-1);break;
  case 'Home':cur=0;show();break;
  case 'End':cur=items.length-1;show();break;
  case 'd':case 'D':if(e.target!==gdiff){gdiff.checked=!gdiff.checked;show()}break;
  default:return;
  }
  e.preventDefault();
});
var tx=null;
gal.addEventListener('touchstart',function(e){tx=e.touches.length===1?e.touches[0].clientX:null},{passive:true});
gal.addEventListener('touchend',function(e){
  if(tx===null)return;var dx=e.changedTouches[0].clientX-tx;tx=null;
  if(Math.abs(dx)>50)step(dx<0?1:-1);
});
})();
</script>
</body>
</html>
`
