package skillmarket

import (
	"strings"
	"testing"
)

var rewrittenSearchBenchmark string

func BenchmarkRewriteSearchQuery(b *testing.B) {
	for _, tc := range []struct{ name, query string }{
		{"chinese", "请帮我找一个技能，查找重复文件，不要上传云端"},
		{"english", "Could you please help me find a skill for local OCR without cloud uploads"},
		{"literal", "::std::find"},
		{"long_content", "帮我找一个技能，" + strings.Repeat("PDF OCR 本地处理，不要上传；", 100)},
	} {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				rewrittenSearchBenchmark = RewriteSearchQuery(tc.query)
			}
		})
	}
}
