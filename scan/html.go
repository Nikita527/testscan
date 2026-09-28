package scan

import (
	"fmt"
	"html"
	"io"
	"sort"
	"strings"
)

// WriteHTML encodes findings as a self-contained interactive HTML report
// with Health Score in the hero, filters, and groups by rule then file.
func WriteHTML(w io.Writer, findings []Finding, score Score) error {
	byRule := groupByRule(findings)
	ruleIDs := sortedRuleIDs(byRule)

	var b strings.Builder
	b.WriteString("<!DOCTYPE html>\n<html lang=\"en\">\n<head>\n<meta charset=\"utf-8\">\n")
	b.WriteString("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">\n")
	b.WriteString("<title>testscan report</title>\n<style>\n")
	b.WriteString(htmlCSS)
	b.WriteString("</style>\n</head>\n<body>\n")
	b.WriteString("<header class=\"hero\">\n")
	b.WriteString("<div class=\"hero-top\">\n")
	b.WriteString("<div class=\"hero-brand\">\n")
	b.WriteString("<p class=\"brand\">testscan</p>\n")
	b.WriteString("<h1>Findings report</h1>\n")
	subParts := fmt.Sprintf("%d finding(s) · %d error · %d warning · %d note",
		len(findings), score.Errors, score.Warnings, score.Notes)
	if score.ParseSkipped > 0 {
		subParts += fmt.Sprintf(" · %d tool error", score.ParseSkipped)
	}
	subParts += fmt.Sprintf(" · %d file(s)", score.Files)
	fmt.Fprintf(&b, "<p class=\"sub\">%s</p>\n", subParts)
	b.WriteString("</div>\n")
	writeScoreHero(&b, score)
	b.WriteString("</div>\n")
	writeScoreCaption(&b, score)
	b.WriteString("<p class=\"signal-caption\">Showing trusted warnings first; enable notes for low-precision heuristics</p>\n")
	b.WriteString("<div class=\"sev-breakdown\" aria-label=\"Severity breakdown\">\n")
	fmt.Fprintf(&b, "<span class=\"chip chip-error\"><strong>%d</strong> error</span>\n", score.Errors)
	fmt.Fprintf(&b, "<span class=\"chip chip-warning\"><strong>%d</strong> warning</span>\n", score.Warnings)
	fmt.Fprintf(&b, "<span class=\"chip chip-note\"><strong>%d</strong> note</span>\n", score.Notes)
	if score.ParseSkipped > 0 {
		fmt.Fprintf(&b, "<span class=\"chip chip-tool-error\"><strong>%d</strong> tool error</span>\n", score.ParseSkipped)
	}
	b.WriteString("</div>\n")
	b.WriteString("</header>\n")

	b.WriteString("<section class=\"filters\" aria-label=\"Filters\">\n")
	b.WriteString("<label><input type=\"checkbox\" data-sev=\"error\" checked> error</label>\n")
	b.WriteString("<label><input type=\"checkbox\" data-sev=\"warning\" checked> warning</label>\n")
	b.WriteString("<label><input type=\"checkbox\" data-sev=\"note\"> other</label>\n")
	b.WriteString("<label><input type=\"checkbox\" data-sev=\"tool-error\"> tool error</label>\n")
	b.WriteString("<input type=\"search\" id=\"q\" placeholder=\"Filter by rule / file / message…\" autocomplete=\"off\">\n")
	b.WriteString("</section>\n")

	b.WriteString("<nav class=\"toc\" aria-label=\"Rules\">\n<ul>\n")
	for _, id := range ruleIDs {
		fs := byRule[id]
		sev := dominantSeverity(fs)
		prec := rulePrecisionDisplay(id)
		fmt.Fprintf(&b, "<li><a href=\"#rule-%s\" data-rule=\"%s\" data-sev=\"%s\">%s <span class=\"count\">%d</span> <span class=\"prec\">%.2f</span></a></li>\n",
			html.EscapeString(id), html.EscapeString(id), sev, html.EscapeString(id), len(fs), prec)
	}
	if len(ruleIDs) == 0 {
		b.WriteString("<li class=\"empty\">No findings</li>\n")
	}
	b.WriteString("</ul>\n</nav>\n")

	b.WriteString("<main>\n")
	for _, id := range ruleIDs {
		fs := byRule[id]
		prec := rulePrecisionDisplay(id)
		fmt.Fprintf(&b, "<section class=\"rule\" id=\"rule-%s\" data-rule=\"%s\">\n",
			html.EscapeString(id), html.EscapeString(id))
		fmt.Fprintf(&b, "<h2>%s <span class=\"count\">%d</span> <span class=\"prec\">%.2f</span></h2>\n",
			html.EscapeString(id), len(fs), prec)

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
			seenMsg := map[string]bool{}
			fmt.Fprintf(&b, "<article class=\"file\">\n<h3>%s</h3>\n<ul>\n", html.EscapeString(file))
			for _, f := range items {
				sev := findingDataSev(f)
				msgHTML := html.EscapeString(f.Message)
				dup := seenMsg[f.Message]
				if dup {
					msgHTML = `<span class="msg-dup" title="` + html.EscapeString(f.Message) + `">same message as above</span>`
				} else {
					seenMsg[f.Message] = true
				}
				related := ""
				if f.RelatedQualName != "" || f.RelatedLine > 0 {
					twin := f.RelatedQualName
					if twin == "" {
						twin = "?"
					}
					if f.RelatedLine > 0 {
						related = fmt.Sprintf(` <span class="related">twin %s:%d</span>`,
							html.EscapeString(twin), f.RelatedLine)
					} else {
						related = fmt.Sprintf(` <span class="related">twin %s</span>`,
							html.EscapeString(twin))
					}
				}
				snippet := ""
				if f.Snippet != "" {
					snippet = fmt.Sprintf(`<pre class="snippet">%s</pre>`, html.EscapeString(f.Snippet))
				}
				fmt.Fprintf(&b,
					"<li class=\"finding\" data-sev=\"%s\" data-rule=\"%s\" data-file=\"%s\" data-msg=\"%s\">"+
						"<span class=\"sev sev-%s\">%s</span> "+
						"<span class=\"loc\">:%d</span> "+
						"<span class=\"msg\">%s%s</span>%s</li>\n",
					sev,
					html.EscapeString(f.Rule),
					html.EscapeString(strings.ToLower(f.File)),
					html.EscapeString(strings.ToLower(f.Message)),
					sev, html.EscapeString(displaySevLabel(f, sev)),
					f.Line,
					msgHTML, related, snippet,
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

// sortedRuleIDs orders rules by precision desc, then count desc, then id.
func sortedRuleIDs(byRule map[string][]Finding) []string {
	ruleIDs := make([]string, 0, len(byRule))
	for id := range byRule {
		ruleIDs = append(ruleIDs, id)
	}
	sort.SliceStable(ruleIDs, func(i, j int) bool {
		pi, pj := rulePrecisionDisplay(ruleIDs[i]), rulePrecisionDisplay(ruleIDs[j])
		if pi != pj {
			return pi > pj
		}
		ci, cj := len(byRule[ruleIDs[i]]), len(byRule[ruleIDs[j]])
		if ci != cj {
			return ci > cj
		}
		return ruleIDs[i] < ruleIDs[j]
	})
	return ruleIDs
}

// rulePrecisionDisplay returns the catalog precision (default 1.0 for unlisted rules).
func rulePrecisionDisplay(ruleID string) float64 {
	if p, ok := RulePrecision[ruleID]; ok {
		return p
	}
	return 1.0
}

func writeScoreHero(b *strings.Builder, score Score) {
	// Circumference of r=15.9155 ≈ 100 so dasharray percent maps 1:1.
	pct := score.Value
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	grade := html.EscapeString(score.Grade)
	fmt.Fprintf(b, "<div class=\"score\" data-grade=\"%s\" title=\"Health Score (precision-weighted)\">\n", grade)
	b.WriteString("<svg viewBox=\"0 0 36 36\" aria-hidden=\"true\">\n")
	b.WriteString("<path class=\"ring-bg\" d=\"M18 2.0845 a 15.9155 15.9155 0 0 1 0 31.831 a 15.9155 15.9155 0 0 1 0 -31.831\"/>\n")
	fmt.Fprintf(b, "<path class=\"ring-fg\" stroke-dasharray=\"%d, 100\" d=\"M18 2.0845 a 15.9155 15.9155 0 0 1 0 31.831 a 15.9155 15.9155 0 0 1 0 -31.831\"/>\n", pct)
	b.WriteString("</svg>\n")
	fmt.Fprintf(b, "<div class=\"score-label\"><span class=\"score-value\">%d</span><span class=\"score-grade\">%s</span></div>\n",
		score.Value, grade)
	b.WriteString("</div>\n")
}

func writeScoreCaption(b *strings.Builder, score Score) {
	b.WriteString("<p class=\"score-caption\">")
	b.WriteString("Grade is precision-weighted warning density over scanned files — not a claim that tests are excellent. Notes are UI-only.")
	if score.WarningsIgnored > 0 {
		fmt.Fprintf(b, " %d of %d warning(s) excluded (low-precision rules).",
			score.WarningsIgnored, score.Warnings)
	} else if score.WarningsInGrade > 0 {
		fmt.Fprintf(b, " %d warning(s) counted toward grade.", score.WarningsInGrade)
	}
	b.WriteString("</p>\n")
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

func findingDataSev(f Finding) string {
	if f.Rule == "parse-error" {
		return "tool-error"
	}
	return normalizeSev(f.Severity)
}

func displaySevLabel(f Finding, dataSev string) string {
	if dataSev == "tool-error" {
		return "tool error"
	}
	return f.Severity
}

func dominantSeverity(fs []Finding) string {
	hasErr, hasWarn, hasTool, onlyTool := false, false, false, true
	for _, f := range fs {
		if f.Rule == "parse-error" {
			hasTool = true
			continue
		}
		onlyTool = false
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
	if hasTool && onlyTool {
		return "tool-error"
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
.hero-top {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 1.25rem 2rem;
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
.score-caption {
  margin: .85rem 0 0;
  max-width: 40rem;
  font-size: .82rem;
  line-height: 1.45;
  color: var(--muted);
}
.signal-caption {
  margin: .45rem 0 0;
  max-width: 40rem;
  font-size: .82rem;
  line-height: 1.45;
  color: var(--muted);
}
.score {
  position: relative;
  width: 7.5rem;
  height: 7.5rem;
  flex: 0 0 auto;
}
.score svg {
  display: block;
  width: 100%;
  height: 100%;
  transform: rotate(-90deg);
}
.ring-bg {
  fill: none;
  stroke: var(--border);
  stroke-width: 2.8;
}
.ring-fg {
  fill: none;
  stroke: var(--accent);
  stroke-width: 2.8;
  stroke-linecap: round;
  transition: stroke-dasharray .4s ease;
}
.score[data-grade="A"] .ring-fg { stroke: var(--accent); }
.score[data-grade="B"] .ring-fg { stroke: #4aaf8a; }
.score[data-grade="C"] .ring-fg { stroke: var(--warning); }
.score[data-grade="D"] .ring-fg { stroke: #d4783a; }
.score[data-grade="F"] .ring-fg { stroke: var(--error); }
.score-label {
  position: absolute;
  inset: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  line-height: 1.1;
}
.score-value {
  font-size: 1.65rem;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
}
.score-grade {
  font-size: .85rem;
  font-weight: 600;
  letter-spacing: .08em;
  color: var(--muted);
}
.sev-breakdown {
  display: flex;
  flex-wrap: wrap;
  gap: .5rem;
  margin-top: 1.1rem;
}
.chip {
  font-size: .8rem;
  padding: .3rem .65rem;
  border-radius: 6px;
  border: 1px solid var(--border);
  background: var(--panel);
  color: var(--muted);
}
.chip strong { color: var(--text); margin-right: .25rem; }
.chip-error { border-color: rgba(232,93,93,.35); }
.chip-warning { border-color: rgba(212,160,23,.35); }
.chip-note { border-color: rgba(107,140,174,.35); }
.chip-tool-error { border-color: rgba(139,154,171,.45); color: var(--muted); }
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
.toc .prec, .rule h2 .prec {
  color: var(--accent);
  font-weight: 500;
  font-variant-numeric: tabular-nums;
  opacity: .85;
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
.sev-tool-error { background: rgba(139,154,171,.12); color: var(--muted); }
.loc { font-family: var(--mono); color: var(--muted); font-size: .85rem; }
.msg { word-break: break-word; }
.msg-dup { color: var(--muted); font-style: italic; }
.related {
  display: inline-block;
  margin-left: .35rem;
  font-family: var(--mono);
  font-size: .8rem;
  color: var(--note);
}
.snippet {
  grid-column: 1 / -1;
  margin: .35rem 0 0;
  padding: .55rem .7rem;
  border-radius: 6px;
  border: 1px solid var(--border);
  background: var(--bg);
  font-family: var(--mono);
  font-size: .78rem;
  line-height: 1.4;
  overflow-x: auto;
  white-space: pre;
  color: var(--muted);
}
.rule.is-hidden, .file.is-hidden, .finding.is-hidden, .toc li.is-hidden { display: none; }
@media (max-width: 640px) {
  .finding { grid-template-columns: auto 1fr; }
  .loc { grid-column: 2; }
  .msg { grid-column: 1 / -1; }
  .snippet { grid-column: 1 / -1; }
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
      const text = (li.dataset.rule + ' ' + li.dataset.file + ' ' + li.dataset.msg).toLowerCase();
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
  apply();
})();
`
