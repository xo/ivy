package review

import (
	"fmt"
	"html/template"
	"image"
	"io"
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
	"hasPrefix": strings.HasPrefix,
}).Parse(pageHTML))

// pageHTML is the template of index.html.
const pageHTML = `{{define "dims"}}{{if .X}} width="{{.X}}" height="{{.Y}}"{{end}}{{end}}<!doctype html>
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
.hidden{display:none}
</style>
</head>
<body>
<h1>ivy review</h1>
<p class="run muted">{{.Created.Format "2006-01-02 15:04 UTC"}} · {{.DPI}} dpi · {{if gt .Pages 0}}the first {{.Pages}} pages and the last page{{else}}each page{{end}} · timeout {{.Timeout}} · {{printf "%.0f" .Elapsed}} s · {{len .Docs}} documents</p>
<p class="counts">{{range statuses .Counts}}<span class="{{.}}">{{.}}: {{index $.Counts .}}</span>{{end}}</p>
{{range .Notes}}<p class="err">{{.}}</p>{{end}}
<details><summary>Programs</summary><pre>{{range .Programs}}{{.Name}}: {{.Version}}
{{end}}</pre></details>
<p class="muted">Worst: for each page, the blurred difference of ivy from its closest reference, and the largest of these over the pages. Blurred: the mean difference of the grey levels after a blur of 5 by 5 pixels, from 0 to 255. Mean: the mean difference of the channels. Under 2 is green, under 5 is amber. ivy PDF vs MuPDF: when the references convert a document to PDF, ivy renders that PDF file too, as the column ivy PDF. The number is the largest blurred difference of that column from MuPDF over the pages. It shows the gaps of the PDF renderer of ivy. It is not a reference, so Worst and Refs disagree leave it out.</p>
<div class="bar">
<label>Kind <select id="fkind"><option value="">all</option>{{range kinds .Docs}}<option>{{.}}</option>{{end}}</select></label>
<label>Status <select id="fstatus"><option value="">all</option>{{range statuses .Counts}}<option>{{.}}</option>{{end}}</select></label>
<label>Name <input id="fname" type="search" size="18"></label>
<label><input id="showdiff" type="checkbox"> show the differences</label>
<span id="shown" class="muted"></span>
</div>
<table class="sum" id="sum">
<thead><tr><th data-k="name">Name</th><th data-k="kind">Kind</th><th data-k="status">Status</th><th data-k="pages" data-num>Pages ivy / ref</th><th data-k="worst" data-num>Worst</th><th data-k="ivypdf" data-num>ivy PDF vs MuPDF</th><th data-k="disagree" data-num>Refs disagree</th><th data-k="time" data-num>Time s</th><th data-k="source">Source</th><th data-k="license">License</th></tr></thead>
<tbody>
{{range $i, $d := .Docs}}<tr data-id="{{.ID}}" data-kind="{{.Kind}}" data-status="{{.Status}}" data-name="{{.Name}}" data-pages="{{.Pages}}" data-worst="{{.Worst}}" data-ivypdf="{{.IvyPDFWorst}}" data-disagree="{{.Disagree}}" data-time="{{.TotalMS}}" data-source="{{.Source}}" data-license="{{.License}}" data-order="{{$i}}">
<td class="name"><a href="#{{.ID}}">{{.Name}}</a></td><td>{{.Kind}}</td><td><span class="badge {{.Status}}">{{.Status}}</span></td><td class="n">{{if .Preview}}preview{{else}}{{pages .Pages}}{{end}} / {{pages .RefPages}}</td><td class="n {{mclass .Worst}}">{{f2 .Worst}}</td><td class="n {{mclass .IvyPDFWorst}}">{{f2 .IvyPDFWorst}}</td><td class="n">{{if .Disagree}}{{.Disagree}}{{end}}</td><td class="n">{{sec .TotalMS}}</td><td>{{.Source}}</td><td>{{.License}}</td></tr>
{{end}}</tbody>
</table>
{{range .Docs}}
<section class="doc" id="{{.ID}}" data-kind="{{.Kind}}" data-status="{{.Status}}" data-name="{{.Name}}">
<h2>{{.Name}} <span class="badge {{.Status}}">{{.Status}}</span></h2>
<p class="meta">{{.Kind}}{{if .Source}} · {{.Source}}{{end}}{{if .License}} · {{.License}}{{end}} · {{if hasPrefix .URL "https://"}}<a href="{{.URL}}">{{.URL}}</a>{{else}}{{.URL}}{{end}}{{if .Note}} · {{.Note}}{{end}}</p>
<p class="meta">ivy: {{if .Preview}}a preview{{else}}{{pages .Pages}} pages{{end}}, open {{msf .OpenMS}}, render {{msf .RenderMS}} · reference{{if .Reference}} through {{.Reference}}{{end}}: {{pages .RefPages}} pages · total {{msf .TotalMS}}{{if ge .IvyPDFWorst 0.0}} · ivy PDF vs MuPDF <span class="{{mclass .IvyPDFWorst}}">{{f2 .IvyPDFWorst}}</span>{{end}}{{if .Disagree}} · <span class="warnbadge">the references disagree on {{.Disagree}} pages</span>{{end}}</p>
{{if .Error}}<p class="err">ivy: {{first .Error}}</p>{{if long .Error}}<details><summary>the whole error</summary><pre>{{.Error}}</pre></details>{{end}}{{end}}
{{if .RefError}}<p class="err">reference: {{first .RefError}}</p>{{if long .RefError}}<details><summary>the whole error</summary><pre>{{.RefError}}</pre></details>{{end}}{{end}}
{{if .IvyPDFError}}<p class="err">ivy PDF: {{first .IvyPDFError}}</p>{{if long .IvyPDFError}}<details><summary>the whole error</summary><pre>{{.IvyPDFError}}</pre></details>{{end}}{{end}}
{{if .Log}}<details{{if le (len .Log) 4}} open{{end}}><summary>{{len .Log}} messages of the log of ivy</summary><pre>{{range .Log}}{{.}}
{{end}}</pre></details>{{end}}
{{if .IvyPDFLog}}<details><summary>{{len .IvyPDFLog}} messages of the log of ivy PDF</summary><pre>{{range .IvyPDFLog}}{{.}}
{{end}}</pre></details>{{end}}
{{$cols := cols .}}{{range .Rows}}
<div class="row" style="--cols:{{$cols}}">
<div class="rowhead">Page {{.Page}}{{if .Disagrees}} <span class="warnbadge">references disagree ({{f2 .RefsDiff}})</span>{{end}}{{if .Refs}} <button type="button" class="rowdiff">differences</button>{{end}}</div>
{{with .Ivy}}<figure class="ivy"><figcaption>{{.Name}}</figcaption>
{{if .Thumb}}<a class="plain" href="{{.Image}}"><img loading="lazy" src="{{.Thumb}}"{{template "dims" .Size}} alt="page of ivy"></a>{{end}}
<div class="m">{{size .Size}}{{if .MS}} · {{msf .MS}}{{end}}{{if and .Status (ne .Status "ok")}} · <span class="{{.Status}}">{{.Status}}</span>{{end}}</div>
{{if .Error}}<div class="ce">{{first .Error}}</div>{{end}}</figure>{{end}}
{{with .IvyPDF}}<figure class="ref ivypdf"><figcaption>{{.Name}}</figcaption>
{{if .Thumb}}<a class="plain" href="{{.Image}}"><img loading="lazy" src="{{.Thumb}}"{{template "dims" .Size}} alt="page of ivy PDF"></a>{{end}}
{{if .DiffThumb}}<a class="diff" href="{{.Diff}}"><img loading="lazy" src="{{.DiffThumb}}"{{template "dims" .Size}} alt="difference of ivy PDF from MuPDF"></a>{{end}}
<div class="m">{{with .Metric}}<span class="{{mclass .Blurred}}">vs MuPDF: blurred {{f2 .Blurred}}</span> · mean {{f2 .Mean}}{{if .Scaled}} · <span class="{{if .SizeOff}}bad{{else}}fair{{end}}">ivy PDF {{size .IvySize}} scaled</span>{{end}} · {{end}}{{size .Size}}{{if .MS}} · {{msf .MS}}{{end}}{{if and .Status (ne .Status "ok")}} · <span class="{{.Status}}">{{.Status}}</span>{{end}}</div>
{{if .Error}}<div class="ce">{{first .Error}}</div>{{end}}</figure>{{end}}
{{range .Refs}}<figure class="ref"><figcaption>{{.Name}}</figcaption>
{{if .Thumb}}<a class="plain" href="{{.Image}}"><img loading="lazy" src="{{.Thumb}}"{{template "dims" .Size}} alt="page of {{.Name}}"></a>{{end}}
{{if .DiffThumb}}<a class="diff" href="{{.Diff}}"><img loading="lazy" src="{{.DiffThumb}}"{{template "dims" .Size}} alt="difference from {{.Name}}"></a>{{end}}
<div class="m">{{with .Metric}}<span class="{{mclass .Blurred}}">blurred {{f2 .Blurred}}</span> · mean {{f2 .Mean}}{{if .Scaled}} · <span class="{{if .SizeOff}}bad{{else}}fair{{end}}">ivy {{size .IvySize}} scaled</span>{{end}} · {{end}}{{size .Size}}{{if .MS}} · {{msf .MS}}{{end}}</div>
{{if .Error}}<div class="ce">{{first .Error}}</div>{{end}}</figure>{{end}}
</div>{{end}}
</section>
{{end}}
<script>
(function(){
var rows=[].slice.call(document.querySelectorAll('#sum tbody tr'));
var secs={};[].forEach.call(document.querySelectorAll('section.doc'),function(s){secs[s.id]=s});
var fk=document.getElementById('fkind'),fs=document.getElementById('fstatus'),fn=document.getElementById('fname');
function filter(){
  var k=fk.value,s=fs.value,n=fn.value.toLowerCase(),shown=0;
  rows.forEach(function(r){
    var ok=(!k||r.dataset.kind===k)&&(!s||r.dataset.status===s)&&(!n||r.dataset.name.toLowerCase().indexOf(n)>=0);
    r.classList.toggle('hidden',!ok);secs[r.dataset.id].classList.toggle('hidden',!ok);if(ok)shown++;
  });
  document.getElementById('shown').textContent=shown+' of '+rows.length+' documents';
}
[fk,fs].forEach(function(e){e.addEventListener('change',filter)});fn.addEventListener('input',filter);
var dir={};
[].forEach.call(document.querySelectorAll('#sum th'),function(th){
  th.addEventListener('click',function(){
    var k=th.dataset.k,num=th.hasAttribute('data-num');dir[k]=dir[k]===undefined?(num?-1:1):-dir[k];
    var tb=document.querySelector('#sum tbody');
    rows.sort(function(a,b){
      var x=a.dataset[k],y=b.dataset[k];
      if(num){x=parseFloat(x);y=parseFloat(y)}
      return (x<y?-1:x>y?1:0)*dir[k]||a.dataset.order-b.dataset.order;
    });
    rows.forEach(function(r){tb.appendChild(r)});
  });
});
document.getElementById('showdiff').addEventListener('change',function(e){document.body.classList.toggle('showdiff',e.target.checked)});
[].forEach.call(document.querySelectorAll('button.rowdiff'),function(b){
  b.addEventListener('click',function(){b.closest('.row').classList.toggle('showdiff')});
});
filter();
})();
</script>
</body>
</html>
`
