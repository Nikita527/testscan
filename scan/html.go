package scan

import (
	"fmt"
	"html"
	"io"
	"sort"
	"strings"
)

// WriteHTML encodes findings as a self-contained interactive HTML report
// (summary + filter by severity/rule + groups by rule then file). Not a score dashboard.
func WriteHTML(w io.Writer, findings []Finding) error {
	byRule := groupByRule(findings)
	ruleIDs := make([]string, 0, len(byRule))
	for id := range byRule {
		ruleIDs = append(ruleIDs, id)
	}
	sort.Strings(ruleIDs)

	var errors, warnings, notes int
	for _, f := range findings {
		switch strings.ToLower(f.Severity) {
		case "error":
			errors++
		case "warning":
			warnings++
		default:
			notes++
		}
	}

	var b strings.Builder
	b.WriteString("<!DOCTYPE html>\n<html lang=\"en\">\n<head>\n<meta charset=\"utf-8\">\n")
	b.WriteString("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">\n")
	b.WriteString("<title>testscan report</title>\n<style>\n")
	b.WriteString(htmlCSS)
	b.WriteString("</style>\n</head>\n<body>\n")
	b.WriteString("<header class=\"hero\">\n")
	b.WriteString("<p class=\"brand\">testscan</p>\n")
	b.WriteString("<h1>Findings report</h1>\n")
	fmt.Fprintf(&b, "<p class=\"sub\">%d finding(s) · %d error · %d warning · %d other</p>\n",
		len(findings), errors, warnings, notes)
	b.WriteString("</header>\n")

	b.WriteString("<section class=\"filters\" aria-label=\"Filters\">\n")
	b.WriteString("<label><input type=\"checkbox\" data-sev=\"error\" checked> error</label>\n")
	b.WriteString("<label><input type=\"checkbox\" data-sev=\"warning\" checked> warning</label>\n")
	b.WriteString("<label><input type=\"checkbox\" data-sev=\"note\" checked> other</label>\n")
	b.WriteString("<input type=\"search\" id=\"q\" placeholder=\"Filter by rule / file / message…\" autocomplete=\"off\">\n")
	b.WriteString("</section>\n")

	b.WriteString("<nav class=\"toc\" aria-label=\"Rules\">\n<ul>\n")
	for _, id := range ruleIDs {
		fs := byRule[id]
		sev := dominantSeverity(fs)
		fmt.Fprintf(&b, "<li><a href=\"#rule-%s\" data-rule=\"%s\" data-sev=\"%s\">%s <span class=\"count\">%d</span></a></li>\n",
			html.EscapeString(id), html.EscapeString(id), sev, html.EscapeString(id), len(fs))
	}
	if len(ruleIDs) == 0 {
		b.WriteString("<li class=\"empty\">No findings</li>\n")
	}
	b.WriteString("</ul>\n</nav>\n")

	b.WriteString("<main>\n")
	for _, id := range ruleIDs {
		fs := byRule[id]
		fmt.Fprintf(&b, "<section class=\"rule\" id=\"rule-%s\" data-rule=\"%s\">\n",
			html.EscapeString(id), html.EscapeString(id))
		fmt.Fprintf(&b, "<h2>%s <span class=\"count\">%d</span></h2>\n",
			html.EscapeString(id), len(fs))

		byFile := groupByFile(fs)
		files := make([]string, 0, len(byFile))
		for f := range byFile {
			files = append(files, f)
		}
		sort.Strings(files)

		for _, file := range files {
			items := byFile[file]
			sort.SliceStable(items, func(i, j int) bool {
				if items[i].Line != items[j].Line {
					return items[i].Line < items[j].Line
				}
				return items[i].Message < items[j].Message
			})
			fmt.Fprintf(&b, "<article class=\"file\">\n<h3>%s</h3>\n<ul>\n", html.EscapeString(file))
			for _, f := range items {
				sev := normalizeSev(f.Severity)
				fmt.Fprintf(&b,
					"<li class=\"finding\" data-sev=\"%s\" data-rule=\"%s\" data-file=\"%s\" data-msg=\"%s\">"+
						"<span class=\"sev sev-%s\">%s</span> "+
						"<span class=\"loc\">:%d</span> "+
						"<span class=\"msg\">%s</span></li>\n",
					sev,
					html.EscapeString(f.Rule),
					html.EscapeString(strings.ToLower(f.File)),
					html.EscapeString(strings.ToLower(f.Message)),
					sev, html.EscapeString(f.Severity),
					f.Line,
					html.EscapeString(f.Message),
				)
			}
			b.WriteString("</ul>\n</article>\n")
		}
		b.WriteString("</section>\n")
	}
	b.WriteString("</main>\n")
	b.WriteString("<script>\n")
	b.WriteString(htmlJS)
	b.WriteString("</script>\n</body>\n</html>\n")

	_, err := io.WriteString(w, b.String())
	return err
}

func groupByRule(findings []Finding) map[string][]Finding {
	out := make(map[string][]Finding)
	for _, f := range findings {
		out[f.Rule] = append(out[f.Rule], f)
	}
	return out
}

func groupByFile(findings []Finding) map[string][]Finding {
	out := make(map[string][]Finding)
	for _, f := range findings {
		out[f.File] = append(out[f.File], f)
	}
	return out
}

func normalizeSev(s string) string {
	switch strings.ToLower(s) {
	case "error":
		return "error"
	case "warning":
		return "warning"
	default:
		return "note"
	}
}

func dominantSeverity(fs []Finding) string {
	hasErr, hasWarn := false, false
	for _, f := range fs {
		switch normalizeSev(f.Severity) {
		case "error":
			hasErr = true
		case "warning":
			hasWarn = true
		}
	}
	if hasErr {
		return "error"
	}
	if hasWarn {
		return "warning"
	}
	return "note"
}

const htmlCSS = `
:root {
  --bg: #0f1419;
  --panel: #1a222c;
  --text: #e7ecf1;
  --muted: #8b9aab;
  --border: #2a3542;
  --accent: #3d9a78;
  --error: #e85d5d;
  --warning: #d4a017;
  --note: #6b8cae;
  --font: "IBM Plex Sans", "Segoe UI", system-ui, sans-serif;
  --mono: "IBM Plex Mono", "Cascadia Code", ui-monospace, monospace;
}
* { box-sizing: border-box; }
body {
  margin: 0;
  font-family: var(--font);
  background: var(--bg);
  color: var(--text);
  line-height: 1.45;
}
.hero {
  padding: 2rem 1.5rem 1.25rem;
  border-bottom: 1px solid var(--border);
  background:
    radial-gradient(ellipse 80% 60% at 10% -20%, rgba(61,154,120,.18), transparent),
    var(--bg);
}
.brand {
  margin: 0;
  font-size: .85rem;
  letter-spacing: .12em;
  text-transform: uppercase;
  color: var(--accent);
  font-weight: 600;
}
.hero h1 { margin: .35rem 0 .5rem; font-size: 1.75rem; font-weight: 600; }
.sub { margin: 0; color: var(--muted); }
.filters {
  display: flex;
  flex-wrap: wrap;
  gap: .75rem 1.25rem;
  align-items: center;
  padding: 1rem 1.5rem;
  border-bottom: 1px solid var(--border);
  background: var(--panel);
  position: sticky;
  top: 0;
  z-index: 2;
}
.filters label { color: var(--muted); font-size: .9rem; cursor: pointer; }
.filters input[type="search"] {
  flex: 1 1 14rem;
  min-width: 12rem;
  padding: .45rem .7rem;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--bg);
  color: var(--text);
  font: inherit;
}
.toc { padding: 1rem 1.5rem; border-bottom: 1px solid var(--border); }
.toc ul { list-style: none; margin: 0; padding: 0; display: flex; flex-wrap: wrap; gap: .5rem; }
.toc a {
  display: inline-flex;
  gap: .4rem;
  align-items: baseline;
  padding: .35rem .65rem;
  border-radius: 999px;
  background: var(--panel);
  border: 1px solid var(--border);
  color: var(--text);
  text-decoration: none;
  font-family: var(--mono);
  font-size: .8rem;
}
.toc a:hover { border-color: var(--accent); }
.toc .count, .rule h2 .count {
  color: var(--muted);
  font-weight: 500;
}
.toc .empty { color: var(--muted); }
main { padding: 1.25rem 1.5rem 3rem; max-width: 72rem; }
.rule { margin-bottom: 2rem; scroll-margin-top: 4.5rem; }
.rule h2 {
  margin: 0 0 1rem;
  font-family: var(--mono);
  font-size: 1.1rem;
  font-weight: 600;
}
.file {
  background: var(--panel);
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: .85rem 1rem;
  margin-bottom: .75rem;
}
.file h3 {
  margin: 0 0 .6rem;
  font-family: var(--mono);
  font-size: .85rem;
  font-weight: 500;
  color: var(--muted);
  word-break: break-all;
}
.file ul { list-style: none; margin: 0; padding: 0; }
.finding {
  display: grid;
  grid-template-columns: auto auto 1fr;
  gap: .5rem .75rem;
  align-items: start;
  padding: .4rem 0;
  border-top: 1px solid var(--border);
  font-size: .92rem;
}
.finding:first-child { border-top: 0; }
.sev {
  font-size: .72rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: .04em;
  padding: .15rem .4rem;
  border-radius: 4px;
}
.sev-error { background: rgba(232,93,93,.15); color: var(--error); }
.sev-warning { background: rgba(212,160,23,.15); color: var(--warning); }
.sev-note { background: rgba(107,140,174,.15); color: var(--note); }
.loc { font-family: var(--mono); color: var(--muted); font-size: .85rem; }
.msg { word-break: break-word; }
.rule.is-hidden, .file.is-hidden, .finding.is-hidden, .toc li.is-hidden { display: none; }
@media (max-width: 640px) {
  .finding { grid-template-columns: auto 1fr; }
  .loc { grid-column: 2; }
  .msg { grid-column: 1 / -1; }
}
`

const htmlJS = `
(function () {
  const sevBoxes = [...document.querySelectorAll('.filters input[data-sev]')];
  const q = document.getElementById('q');
  function apply() {
    const on = new Set(sevBoxes.filter(b => b.checked).map(b => b.dataset.sev));
    const needle = (q.value || '').trim().toLowerCase();
    document.querySelectorAll('.finding').forEach(li => {
      const sevOk = on.has(li.dataset.sev);
      const text = (li.dataset.rule + ' ' + li.dataset.file + ' ' + li.dataset.msg);
      const qOk = !needle || text.includes(needle);
      li.classList.toggle('is-hidden', !(sevOk && qOk));
    });
    document.querySelectorAll('.file').forEach(art => {
      const any = [...art.querySelectorAll('.finding')].some(li => !li.classList.contains('is-hidden'));
      art.classList.toggle('is-hidden', !any);
    });
    document.querySelectorAll('.rule').forEach(sec => {
      const any = [...sec.querySelectorAll('.finding')].some(li => !li.classList.contains('is-hidden'));
      sec.classList.toggle('is-hidden', !any);
      const toc = document.querySelector('.toc a[data-rule="' + sec.dataset.rule + '"]');
      if (toc && toc.parentElement) toc.parentElement.classList.toggle('is-hidden', !any);
    });
  }
  sevBoxes.forEach(b => b.addEventListener('change', apply));
  q.addEventListener('input', apply);
})();
`
