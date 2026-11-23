package skillmarket

import (
	"strconv"
	"testing"
)

func TestRewriteSearchQueryPhrasingAndContent(t *testing.T) {
	for _, tc := range []struct{ name, raw, want string }{
		{"zh_short_counter", "请帮我找个技能，PDF OCR", "PDF OCR"},
		{"zh_polite", "麻烦你帮我找一款技能：本地备份", "本地备份"},
		{"zh_recommend", "劳驾给我推荐一个技能：RAG", "RAG"},
		{"zh_question", "有没有一个不联网的技能？", "不联网"},
		{"zh_suffix_period", "帮我找一个不会上传数据的技能。", "不会上传数据"},
		{"zh_search", "请为我搜索一种技能，图片去重", "图片去重"},
		{"zh_whitespace", "\t请 帮我 找 一个 技能 ：\tPDF OCR\n", "PDF OCR"},
		{"zh_no_counter", "搜索技能：解析 XLSX", "解析 XLSX"},
		{"zh_ascii_comma", "找一个技能, 用来处理 CSV", "用来处理 CSV"},
		{"en_recommend_me", "Recommend me a skill for SQL migration", "SQL migration"},
		{"en_search_for", "Search for a skill to parse PDFs", "parse PDFs"},
		{"en_look_for", "Look for some skills for local backup", "local backup"},
		{"en_case", "PLEASE HELP ME FIND A SKILL FOR GitHub Actions", "GitHub Actions"},
		{"en_polite_question", "Can you please find a skill for OCR", "OCR"},
		{"en_polite_comma", "Please, help me find a skill for OCR", "OCR"},
		{"en_multiple_politeness", "Could you please help me find a skill that can parse PDFs", "parse PDFs"},
		{"en_colon", "Find a skill: Rust borrow checker", "Rust borrow checker"},
		{"en_contraction", "Find a skill for Rust's borrow checker", "Rust's borrow checker"},
		{"cpp_global_namespace", "帮我找一个技能，::std::find", "::std::find"},
		{"css_pseudo_class", "Help me find a skill for :root CSS selectors", ":root CSS selectors"},
		{"yaml_key", "帮我找一个技能，schema:", "schema:"},
		{"dotnet", "帮我找一个技能，.NET 8/C#", ".NET 8/C#"},
		{"windows_path", `帮我找一个技能，C:\work\find_skill.py`, `C:\work\find_skill.py`},
		{"identifier_and_version", "帮我找一个技能，@scope/find-skills@2.0", "@scope/find-skills@2.0"},
		{"quoted_capability", "帮我找一个技能，“查找重复文件”", "“查找重复文件”"},
		{"negation_and_scope", "帮我找一个技能，只搜索本地文件，不联网，不删除原文件", "只搜索本地文件，不联网，不删除原文件"},
		{"en_negation", "Find a skill for offline OCR without cloud uploads", "offline OCR without cloud uploads"},
		{"multiline_constraints", "帮我找一个技能，解析 PDF\n不要执行脚本\n不要上传数据", "解析 PDF\n不要执行脚本\n不要上传数据"},
		{"literal_zh_quote", "“帮我找一个技能，PDF”", "“帮我找一个技能，PDF”"},
		{"literal_en_quote", `'Help me find a skill for PDF'`, `'Help me find a skill for PDF'`},
		{"literal_skill_name", "Find Skills", "Find Skills"},
		{"content_verb", "search local files without deleting them", "search local files without deleting them"},
		{"compound_zh_word", "查找技能树依赖", "查找技能树依赖"},
		{"metadata_in_body", "PDF 帮我找一个技能", "PDF 帮我找一个技能"},
		{"empty", "", ""},
		{"unicode_whitespace", "\u3000\u00a0\t", ""},
		{"incomplete_zh_separator", "帮我找一个技能：", "帮我找一个技能："},
		{"incomplete_zh_space", "帮我找一个技能   ", "帮我找一个技能"},
		{"incomplete_en_to", "find a skill to", "find a skill to"},
		{"incomplete_en_which_can", "find a skill which can", "find a skill which can"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := RewriteSearchQuery(tc.raw); got != tc.want {
				t.Errorf("RewriteSearchQuery(%q) = %q, want %q", tc.raw, got, tc.want)
			}
			if got := RewriteSearchQuery(tc.want); got != tc.want {
				t.Errorf("rewriting canonical content changes meaning: %q -> %q", tc.want, got)
			}
		})
	}
}

func FuzzRewriteSearchQueryPreservesQuotedCapability(f *testing.F) {
	for _, seed := range []string{"", "find-skills", "::std::find", ":root", "不会上传云端", "帮我找一个技能，PDF", "a\nb\t\"c\"", "🙂"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, capability string) {
		quoted := strconv.Quote(capability)
		for _, prefix := range []string{"帮我找一个技能，", "Help me find a skill for "} {
			if got := RewriteSearchQuery(prefix + quoted); got != quoted {
				t.Fatalf("quoted capability changed: %q -> %q", quoted, got)
			}
		}
		if got := RewriteSearchQuery(quoted); got != quoted {
			t.Fatalf("standalone literal changed: %q -> %q", quoted, got)
		}
	})
}
