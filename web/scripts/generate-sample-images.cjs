/**
 * Generate sample PNG images for preset questions
 * Run with: node scripts/generate-sample-images.js
 */

const fs = require('fs')
const path = require('path')

// Simple 1x1 pixel PNG in different colors (base64 encoded)
// These are minimal valid PNG files

const samplesDir = path.join(__dirname, '../public/samples')

// Ensure samples directory exists
if (!fs.existsSync(samplesDir)) {
  fs.mkdirSync(samplesDir, { recursive: true })
}

// Create a simple colored PNG using raw bytes
function createSimplePNG(width, height, r, g, b) {
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

  // IDAT chunk (image data)
  const zlib = require('zlib')
  const rawData = Buffer.alloc((width * 3 + 1) * height)
  for (let y = 0; y < height; y++) {
    rawData[y * (width * 3 + 1)] = 0 // filter byte
    for (let x = 0; x < width; x++) {
      const offset = y * (width * 3 + 1) + 1 + x * 3
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

// Generate sample images
const images = [
  { name: 'landscape.png', width: 400, height: 300, r: 135, g: 206, b: 235 },  // Sky blue
  { name: 'room.png', width: 400, height: 300, r: 245, g: 240, b: 232 },       // Beige
  { name: 'cityscape.png', width: 400, height: 300, r: 26, g: 26, b: 46 },     // Dark blue
  { name: 'chart.png', width: 400, height: 300, r: 248, g: 250, b: 252 },      // Light gray
  { name: 'invoice.png', width: 400, height: 300, r: 255, g: 255, b: 255 },    // White
]

console.log('Generating sample PNG images...')

for (const img of images) {
  const pngData = createSimplePNG(img.width, img.height, img.r, img.g, img.b)
  const filePath = path.join(samplesDir, img.name)
  fs.writeFileSync(filePath, pngData)
  console.log(`Created: ${img.name} (${pngData.length} bytes)`)
}

console.log('Done!')
