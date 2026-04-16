package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tools "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func main() {
	outDir := flag.String("out", "/Users/orca/.zimaos-blue/data/workspace/pptx_theme_samples_20260416", "output directory for generated pptx theme samples")
	flag.Parse()

	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		fail("create output dir: %v", err)
	}

	tool := tools.NewPPTXTool([]string{*outDir}, nil, nil)
	ctx := context.Background()

	for _, theme := range tools.ListThemes() {
		fileName := fmt.Sprintf("Qwen3_5_优化介绍_%s_现代宽屏.pptx", theme)
		if _, err := tool.Execute(ctx, sampleDeckArgs(theme, fileName)); err != nil {
			fail("generate %s: %v", theme, err)
		}
		fmt.Println(filepath.Join(*outDir, fileName))
	}
}

func sampleDeckArgs(theme, path string) map[string]interface{} {
	return map[string]interface{}{
		"action":   "create",
		"path":     path,
		"theme":    theme,
		"title":    "Qwen3.5 优化介绍",
		"subtitle": themeLabel(theme) + " · 现代宽屏样例",
		"summary":  "面向 Pages / Office 的现代宽屏主题验证样例。",
		"sections": []interface{}{
			map[string]interface{}{
				"heading": "核心亮点",
				"bullets": []interface{}{
					"混合思考模式",
					"128K 长上下文",
					"部署效率提升",
				},
			},
			map[string]interface{}{
				"heading": "能力概览",
				"table": map[string]interface{}{
					"columns": []interface{}{
						map[string]interface{}{"header": "维度", "width": 1.6},
						map[string]interface{}{"header": "表现", "width": 1.0, "align": "right"},
						map[string]interface{}{"header": "说明", "width": 2.2},
					},
					"rows": []interface{}{
						[]interface{}{"数学推理", "97%", "复杂链路更稳定"},
						[]interface{}{"多语言理解", "119", "覆盖更多输入场景"},
						[]interface{}{"工具调用", "92%", "结构化结果更一致"},
					},
				},
			},
			map[string]interface{}{
				"heading": "趋势对比",
				"body": strings.Join([]string{
					"准确率：93%",
					"延迟：180ms",
					"重点：首轮响应更快",
				}, "\n"),
				"chart": map[string]interface{}{
					"type":                       "combo",
					"title":                      "关键指标趋势",
					"value_axis_title":           "准确率",
					"secondary_value_axis_title": "延迟(ms)",
					"categories":                 []interface{}{"Base", "v1", "v2", "v3"},
					"series": []interface{}{
						map[string]interface{}{
							"name":   "准确率",
							"type":   "bar",
							"values": []interface{}{72, 81, 88, 93},
						},
						map[string]interface{}{
							"name":   "延迟",
							"type":   "line",
							"axis":   "secondary",
							"values": []interface{}{310, 250, 210, 180},
						},
					},
				},
			},
		},
	}
}

func themeLabel(theme string) string {
	switch theme {
	case "analysis":
		return "分析蓝"
	case "ui_review":
		return "审阅石墨"
	case "executive":
		return "高管海军蓝"
	case "clean":
		return "极简留白"
	case "midnight":
		return "深夜蓝"
	case "editorial":
		return "高对比编辑"
	case "terracotta":
		return "暖陶土"
	case "forest":
		return "森系绿"
	case "coral":
		return "珊瑚橙"
	default:
		return theme
	}
}

func fail(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
