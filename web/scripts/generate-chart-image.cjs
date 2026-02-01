/**
 * Generate a simple bar chart PNG image
 * Run with: node scripts/generate-chart-image.cjs
 */

const fs = require('fs')
const path = require('path')
const zlib = require('zlib')

const samplesDir = path.join(__dirname, '../public/samples')

// Ensure samples directory exists
if (!fs.existsSync(samplesDir)) {
  fs.mkdirSync(samplesDir, { recursive: true })
}

// Create a simple PNG with a bar chart
function createChartPNG(width, height) {
  // PNG signature
  const signature = Buffer.from([0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A])

  // IHDR chunk
  const ihdrData = Buffer.alloc(13)
  ihdrData.writeUInt32BE(width, 0)
  ihdrData.writeUInt32BE(height, 4)
  ihdrData[8] = 8  // bit depth
  ihdrData[9] = 2  // color type (RGB)
  ihdrData[10] = 0 // compression
  ihdrData[11] = 0 // filter
  ihdrData[12] = 0 // interlace

  const ihdrChunk = createChunk('IHDR', ihdrData)

  // Create image data with a bar chart
  const rawData = Buffer.alloc((width * 3 + 1) * height)

  // Background color (white)
  const bgR = 255, bgG = 255, bgB = 255

  // Chart area
  const chartLeft = 50
  const chartRight = width - 30
  const chartTop = 30
  const chartBottom = height - 40
  const chartWidth = chartRight - chartLeft
  const chartHeight = chartBottom - chartTop

  // Bar data (sales data for 6 months)
  const barData = [65, 85, 45, 90, 70, 95] // percentages
  const barColors = [
    [66, 133, 244],   // Blue
    [52, 168, 83],    // Green
    [251, 188, 4],    // Yellow
    [234, 67, 53],    // Red
    [103, 58, 183],   // Purple
    [0, 172, 193],    // Cyan
  ]
  const barCount = barData.length
  const barWidth = Math.floor(chartWidth / barCount * 0.6)
  const barGap = Math.floor(chartWidth / barCount * 0.4)

  // Grid line color (light gray)
  const gridR = 230, gridG = 230, gridB = 230

  // Axis color (dark gray)
  const axisR = 100, axisG = 100, axisB = 100

  for (let y = 0; y < height; y++) {
    rawData[y * (width * 3 + 1)] = 0 // filter byte
    for (let x = 0; x < width; x++) {
      const offset = y * (width * 3 + 1) + 1 + x * 3

      let r = bgR, g = bgG, b = bgB

      // Draw horizontal grid lines
      if (x >= chartLeft && x <= chartRight) {
        for (let i = 0; i <= 4; i++) {
          const gridY = chartTop + Math.floor(chartHeight * i / 4)
          if (y === gridY) {
            r = gridR; g = gridG; b = gridB
          }
        }
      }

      // Draw Y axis
      if (x === chartLeft && y >= chartTop && y <= chartBottom) {
        r = axisR; g = axisG; b = axisB
      }

      // Draw X axis
      if (y === chartBottom && x >= chartLeft && x <= chartRight) {
        r = axisR; g = axisG; b = axisB
      }

      // Draw bars
      for (let i = 0; i < barCount; i++) {
        const barX = chartLeft + Math.floor(chartWidth * i / barCount) + Math.floor(barGap / 2)
        const barHeight = Math.floor(chartHeight * barData[i] / 100)
        const barTop = chartBottom - barHeight

        if (x >= barX && x < barX + barWidth && y >= barTop && y <= chartBottom) {
          r = barColors[i][0]
          g = barColors[i][1]
          b = barColors[i][2]
        }
      }

      rawData[offset] = r
      rawData[offset + 1] = g
      rawData[offset + 2] = b
    }
  }

  const compressedData = zlib.deflateSync(rawData)
  const idatChunk = createChunk('IDAT', compressedData)

  // IEND chunk
  const iendChunk = createChunk('IEND', Buffer.alloc(0))

  return Buffer.concat([signature, ihdrChunk, idatChunk, iendChunk])
}

function createChunk(type, data) {
  const length = Buffer.alloc(4)
  length.writeUInt32BE(data.length, 0)

  const typeBuffer = Buffer.from(type, 'ascii')
  const crcData = Buffer.concat([typeBuffer, data])

  const crc = Buffer.alloc(4)
  crc.writeUInt32BE(crc32(crcData), 0)

  return Buffer.concat([length, typeBuffer, data, crc])
}

// CRC32 implementation
function crc32(data) {
  let crc = 0xFFFFFFFF
  const table = []

  for (let i = 0; i < 256; i++) {
    let c = i
    for (let j = 0; j < 8; j++) {
      c = (c & 1) ? (0xEDB88320 ^ (c >>> 1)) : (c >>> 1)
    }
    table[i] = c
  }

  for (let i = 0; i < data.length; i++) {
    crc = table[(crc ^ data[i]) & 0xFF] ^ (crc >>> 8)
  }

  return (crc ^ 0xFFFFFFFF) >>> 0
}

// Generate chart image
console.log('Generating chart PNG image...')
const chartPng = createChartPNG(400, 250)
const chartPath = path.join(samplesDir, 'chart.png')
fs.writeFileSync(chartPath, chartPng)
console.log(`Created: chart.png (${chartPng.length} bytes)`)

console.log('Done!')
