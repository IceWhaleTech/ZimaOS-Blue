/**
 * Performance benchmark for typeless rendering pipeline.
 * Run: npx vitest bench --run src/utils/__tests__/typeless-perf.bench.ts
 */
import { bench, describe } from 'vitest'
import {
  parseTypelessContent,
  parseTypelessContentIncremental,
  hasTypelessCards,
} from '../typeless'

const SMALL = `Here is a simple response.

\`\`\`javascript
function hello() {
  console.log("Hello, world!");
}
\`\`\`

That's all.`

const MEDIUM = `# Analysis Results

\`\`\`python
import pandas as pd
df = pd.read_csv("data.csv")
result = df.groupby("category").agg({"value": ["mean", "std", "count"]})
print(result)
\`\`\`

| Category | Count | Mean | Std Dev |
|----------|-------|------|---------|
| A | 150 | 42.3 | 5.7 |
| B | 230 | 38.1 | 4.2 |
| C | 180 | 45.6 | 6.1 |
| D | 95 | 39.8 | 3.9 |

- Category C has the highest mean value
- Category B has the most samples
- Standard deviation is low across all categories

1. Run statistical significance tests
2. Generate visualization charts
3. Prepare final report

![chart](https://example.com/chart.png)

https://example.com/full-report

Check \`/data/reports/analysis.pdf\``

const LARGE = Array(10).fill(MEDIUM).join('\n\n---\n\n')

const STREAMING = `Partial response with unclosed code:

\`\`\`json
{
  "type": "progress",
  "title": "Processing",
  "progress": 45,
  "status": "running`

describe('hasTypelessCards', () => {
  bench('small', () => {
    hasTypelessCards(SMALL)
  })
  bench('medium', () => {
    hasTypelessCards(MEDIUM)
  })
  bench('large', () => {
    hasTypelessCards(LARGE)
  })
})

describe('parseTypelessContent (full parse)', () => {
  bench('small', () => {
    parseTypelessContent(SMALL)
  })
  bench('medium', () => {
    parseTypelessContent(MEDIUM)
  })
  bench('large', () => {
    parseTypelessContent(LARGE)
  })
})

describe('parseTypelessContentIncremental', () => {
  bench('small', () => {
    parseTypelessContentIncremental(SMALL, 'msg-1', 'conv-bench')
  })
  bench('medium', () => {
    parseTypelessContentIncremental(MEDIUM, 'msg-2', 'conv-bench')
  })
  bench('streaming (unclosed)', () => {
    parseTypelessContentIncremental(STREAMING, 'msg-3', 'conv-bench')
  })
})

describe('streaming simulation', () => {
  bench('medium content in 20-char chunks', () => {
    let buf = ''
    for (let c = 0; c < MEDIUM.length; c += 20) {
      buf += MEDIUM.slice(c, c + 20)
      parseTypelessContentIncremental(buf, 'msg-sim', 'conv-sim')
    }
  })
})
