package tools

import (
	"fmt"
	"strings"
	"time"
)

// analyzeCSS is the complete CSS for analysis reports.
// Extracted from the reference report style with hero, stat-grid, cards,
// quote-cards, tag-cloud, progress-bars, insight-boxes, tables, and footer.
const analyzeCSS = `
:root {
  --primary: #1a6fc4;
  --primary-dark: #0d4a8a;
  --secondary: #27ae60;
  --accent: #f39c12;
  --danger: #e74c3c;
  --dark: #2c3e50;
  --gray: #7f8c8d;
  --light-bg: #f8fafc;
  --card-bg: #ffffff;
  --border: #e1e8ed;
  --text: #2c3e50;
  --text-light: #5a6a7a;
}
* { box-sizing: border-box; margin: 0; padding: 0; }
body {
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', 'PingFang SC', 'Hiragino Sans GB', 'Microsoft YaHei', sans-serif;
  background: var(--light-bg);
  color: var(--text);
  line-height: 1.7;
}
html { scroll-behavior: smooth; }

/* HERO */
.hero {
  background: linear-gradient(135deg, #0d4a8a 0%, #1a6fc4 50%, #2980b9 100%);
  color: white;
  padding: 60px 40px 50px;
  text-align: center;
  position: relative;
  overflow: hidden;
}
.hero::before {
  content: '';
  position: absolute;
  top: -50%; left: -50%;
  width: 200%; height: 200%;
  background: radial-gradient(ellipse at center, rgba(255,255,255,0.05) 0%, transparent 60%);
}
.hero .badge {
  display: inline-block;
  background: rgba(255,255,255,0.2);
  border: 1px solid rgba(255,255,255,0.4);
  border-radius: 20px;
  padding: 5px 18px;
  font-size: 13px;
  letter-spacing: 1px;
  text-transform: uppercase;
  margin-bottom: 18px;
}
.hero h1 {
  font-size: 2.4em;
  font-weight: 800;
  margin-bottom: 14px;
  line-height: 1.3;
}
.hero .subtitle {
  font-size: 1.1em;
  opacity: 0.85;
  max-width: 680px;
  margin: 0 auto 28px;
}
.hero .meta-row {
  display: flex;
  justify-content: center;
  gap: 30px;
  flex-wrap: wrap;
  margin-top: 10px;
}
.hero .meta-item {
  background: rgba(255,255,255,0.15);
  border-radius: 10px;
  padding: 10px 22px;
  text-align: center;
}
.hero .meta-item .num {
  font-size: 1.9em;
  font-weight: 800;
  display: block;
}
.hero .meta-item .label {
  font-size: 0.82em;
  opacity: 0.85;
}

/* LAYOUT */
.container {
  max-width: 1200px;
  margin: 0 auto;
  padding: 0 24px;
}

/* NAV */
.toc {
  background: white;
  border-bottom: 2px solid var(--border);
  position: sticky;
  top: 0;
  z-index: 100;
  box-shadow: 0 2px 8px rgba(0,0,0,0.06);
}
.toc-inner {
  max-width: 1200px;
  margin: 0 auto;
  padding: 0 24px;
  display: flex;
  gap: 0;
  overflow-x: auto;
}
.toc a {
  display: block;
  padding: 14px 18px;
  text-decoration: none;
  color: var(--text-light);
  font-size: 13.5px;
  font-weight: 500;
  white-space: nowrap;
  border-bottom: 3px solid transparent;
  transition: all 0.2s;
}
.toc a:hover {
  color: var(--primary);
  border-bottom-color: var(--primary);
}

/* SECTIONS */
.section {
  padding: 50px 0;
  border-bottom: 1px solid var(--border);
}
.section:last-child { border-bottom: none; }
.section-header {
  display: flex;
  align-items: center;
  gap: 14px;
  margin-bottom: 28px;
}
.section-icon {
  width: 44px;
  height: 44px;
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 20px;
  flex-shrink: 0;
}
/* Section icon background colors — use these instead of inline styles */
.section-icon.bg-blue { background: #ebf5fb; }
.section-icon.bg-green { background: #eafaf1; }
.section-icon.bg-orange { background: #fef9e7; }
.section-icon.bg-red { background: #fdedec; }
.section-icon.bg-purple { background: #f4ecf7; }
.section-header h2 {
  font-size: 1.65em;
  font-weight: 700;
  color: var(--dark);
}
.section-header p {
  color: var(--text-light);
  font-size: 0.95em;
  margin-top: 3px;
}

/* CARDS */
.card {
  background: var(--card-bg);
  border-radius: 14px;
  padding: 28px 30px;
  box-shadow: 0 2px 12px rgba(0,0,0,0.06);
  border: 1px solid var(--border);
  margin-bottom: 24px;
}
.card h3 {
  font-size: 1.15em;
  font-weight: 700;
  color: var(--dark);
  margin-bottom: 14px;
  padding-bottom: 10px;
  border-bottom: 2px solid var(--border);
}
.grid-2 { display: grid; grid-template-columns: 1fr 1fr; gap: 24px; }
.grid-3 { display: grid; grid-template-columns: 1fr 1fr 1fr; gap: 20px; }
@media (max-width: 900px) {
  .grid-2, .grid-3 { grid-template-columns: 1fr; }
  .hero h1 { font-size: 1.7em; }
}

/* STAT BOXES */
.stat-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 18px;
  margin-bottom: 32px;
}
@media (max-width: 800px) { .stat-grid { grid-template-columns: repeat(2, 1fr); } }
.stat-box {
  background: white;
  border-radius: 14px;
  padding: 22px 20px;
  text-align: center;
  box-shadow: 0 2px 10px rgba(0,0,0,0.06);
  border: 1px solid var(--border);
  border-top: 4px solid var(--primary);
}
.stat-box.green { border-top-color: var(--secondary); }
.stat-box.orange { border-top-color: var(--accent); }
.stat-box.red { border-top-color: var(--danger); }
.stat-box .number {
  font-size: 2.2em;
  font-weight: 800;
  color: var(--primary);
  display: block;
}
.stat-box.green .number { color: var(--secondary); }
.stat-box.orange .number { color: var(--accent); }
.stat-box.red .number { color: var(--danger); }
.stat-box .label {
  font-size: 0.88em;
  color: var(--text-light);
  margin-top: 4px;
}

/* QUOTE CARDS */
.quote-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 18px;
  margin-top: 8px;
}
@media (max-width: 750px) { .quote-grid { grid-template-columns: 1fr; } }
.quote-card {
  border-radius: 12px;
  padding: 18px 20px;
  border-left: 4px solid;
  background: #fafbfc;
  position: relative;
}
.quote-card.positive { border-left-color: var(--secondary); background: #f0faf4; }
.quote-card.negative { border-left-color: var(--danger); background: #fef5f5; }
.quote-card.neutral { border-left-color: var(--primary); background: #f0f7ff; }
.quote-card.orange { border-left-color: var(--accent); background: #fffbf0; }
.quote-card .quote-text {
  font-size: 0.94em;
  color: var(--text);
  line-height: 1.65;
  font-style: italic;
}
.quote-card .quote-author {
  font-size: 0.82em;
  color: var(--text-light);
  margin-top: 10px;
  font-weight: 600;
}
.quote-card .quote-tag {
  display: inline-block;
  font-size: 0.75em;
  padding: 2px 10px;
  border-radius: 10px;
  font-weight: 600;
  margin-bottom: 8px;
}
.positive .quote-tag { background: #d4efdf; color: #1e8449; }
.negative .quote-tag { background: #fde8e8; color: #c0392b; }
.neutral .quote-tag { background: #d6eaf8; color: #1a5276; }
.orange .quote-tag { background: #fdebd0; color: #d35400; }

/* TAG CLOUD */
.tag-cloud {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  margin-top: 12px;
}
.tag {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 6px 14px;
  border-radius: 20px;
  font-size: 0.88em;
  font-weight: 600;
  border: 1px solid;
}
.tag-blue { background: #ebf5fb; color: #1a5276; border-color: #aed6f1; }
.tag-green { background: #eafaf1; color: #1e8449; border-color: #a9dfbf; }
.tag-red { background: #fdedec; color: #922b21; border-color: #f5b7b1; }
.tag-orange { background: #fef9e7; color: #784212; border-color: #f9e79f; }
.tag-purple { background: #f4ecf7; color: #6c3483; border-color: #d2b4de; }
.tag .count {
  background: rgba(0,0,0,0.12);
  border-radius: 10px;
  padding: 1px 7px;
  font-size: 0.85em;
}

/* CHART WRAPPER */
.chart-wrap {
  background: white;
  border-radius: 14px;
  padding: 24px;
  box-shadow: 0 2px 12px rgba(0,0,0,0.06);
  border: 1px solid var(--border);
  margin-bottom: 24px;
  text-align: center;
}
.chart-wrap img { max-width: 100%; border-radius: 8px; }
.chart-caption {
  font-size: 0.88em;
  color: var(--text-light);
  margin-top: 10px;
  font-style: italic;
}

/* TABLE */
.data-table, .homelab-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.93em;
}
.data-table th, .homelab-table th {
  background: var(--dark);
  color: white;
  padding: 12px 16px;
  text-align: left;
  font-weight: 600;
}
.data-table td, .homelab-table td {
  padding: 11px 16px;
  border-bottom: 1px solid var(--border);
  vertical-align: top;
}
.data-table tr:nth-child(even) td, .homelab-table tr:nth-child(even) td { background: #f8fafc; }
.data-table tr:hover td, .homelab-table tr:hover td { background: #edf4ff; }

/* HOMELAB SHOWCASE CARDS */
.homelab-card {
  background: white;
  border-radius: 12px;
  padding: 18px 20px;
  border: 1px solid var(--border);
  box-shadow: 0 2px 8px rgba(0,0,0,0.05);
  margin-bottom: 14px;
}
.homelab-card .user {
  font-weight: 700;
  color: var(--primary);
  font-size: 0.95em;
  margin-bottom: 6px;
}
.homelab-card .setup-text {
  font-size: 0.91em;
  color: var(--text);
  line-height: 1.6;
}
.homelab-card .tags {
  margin-top: 10px;
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}
.mini-tag {
  font-size: 0.78em;
  padding: 2px 10px;
  border-radius: 10px;
  font-weight: 600;
}
.mt-blue { background: #d6eaf8; color: #1a5276; }
.mt-green { background: #d5f5e3; color: #1e8449; }
.mt-orange { background: #fdebd0; color: #784212; }
.mt-purple { background: #e8daef; color: #6c3483; }
.mt-gray { background: #eaecee; color: #566573; }

/* INSIGHT BOXES */
.insight-box {
  background: linear-gradient(135deg, #ebf5fb, #d6eaf8);
  border: 1px solid #aed6f1;
  border-radius: 12px;
  padding: 20px 24px;
  margin-bottom: 16px;
}
.insight-box.green { background: linear-gradient(135deg, #eafaf1, #d5f5e3); border-color: #a9dfbf; }
.insight-box.orange { background: linear-gradient(135deg, #fef9e7, #fdebd0); border-color: #f9e79f; }
.insight-box.red { background: linear-gradient(135deg, #fdedec, #fadbd8); border-color: #f5b7b1; }
.insight-box .insight-title {
  font-weight: 700;
  font-size: 1.02em;
  color: var(--dark);
  margin-bottom: 8px;
  display: flex;
  align-items: center;
  gap: 8px;
}
.insight-box p {
  font-size: 0.93em;
  color: var(--text);
  line-height: 1.65;
}

/* PROGRESS BARS */
.progress-item { margin-bottom: 14px; }
.progress-label {
  display: flex;
  justify-content: space-between;
  font-size: 0.9em;
  margin-bottom: 5px;
  font-weight: 500;
}
.progress-bar {
  height: 10px;
  background: #e8ecf0;
  border-radius: 5px;
  overflow: hidden;
}
.progress-fill {
  height: 100%;
  border-radius: 5px;
  transition: width 0.3s;
}
.fill-blue { background: linear-gradient(90deg, #2980b9, #5dade2); }
.fill-green { background: linear-gradient(90deg, #27ae60, #58d68d); }
.fill-orange { background: linear-gradient(90deg, #e67e22, #f8c471); }
.fill-red { background: linear-gradient(90deg, #e74c3c, #f1948a); }
.fill-purple { background: linear-gradient(90deg, #8e44ad, #c39bd3); }

/* HIGHLIGHT STRIP */
.highlight-strip {
  background: linear-gradient(90deg, var(--primary), var(--secondary));
  color: white;
  padding: 16px 30px;
  border-radius: 12px;
  font-size: 1.02em;
  font-weight: 600;
  margin-bottom: 24px;
  display: flex;
  align-items: center;
  gap: 12px;
}

/* DIVIDER */
.divider {
  height: 3px;
  background: linear-gradient(90deg, var(--primary), var(--secondary), transparent);
  border-radius: 2px;
  margin: 6px 0 24px;
}

/* FOOTER */
.footer {
  background: var(--dark);
  color: rgba(255,255,255,0.7);
  text-align: center;
  padding: 30px 24px;
  font-size: 0.88em;
}
.footer strong { color: white; }
.footer a { color: #5dade2; }
`

// buildAnalyzeHTML assembles a complete self-contained HTML report.
func buildAnalyzeHTML(title, lang, bodyContent string) string {
	if lang == "" {
		lang = "zh-CN"
	}
	date := time.Now().Format("2006-01-02")

	var b strings.Builder
	b.Grow(len(analyzeCSS) + len(bodyContent) + 512)

	fmt.Fprintf(&b, `<!DOCTYPE html>
<html lang="%s">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>%s</title>
<style>%s</style>
</head>
<body>
`, lang, title, analyzeCSS)

	b.WriteString(bodyContent)

	fmt.Fprintf(&b, `
<div class="footer">
  <p><strong>%s</strong></p>
  <p style="margin-top:8px;">Generated on %s by Blue AI Assistant</p>
</div>
</body>
</html>`, title, date)

	return b.String()
}
