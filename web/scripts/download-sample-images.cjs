/**
 * Download sample images from picsum.photos for preset questions
 * Run with: node scripts/download-sample-images.cjs
 */

const https = require('https')
const fs = require('fs')
const path = require('path')

const samplesDir = path.join(__dirname, '../public/samples')

// Ensure samples directory exists
if (!fs.existsSync(samplesDir)) {
  fs.mkdirSync(samplesDir, { recursive: true })
}

// Download a file from URL
function downloadFile(url, destPath) {
  return new Promise((resolve, reject) => {
    const file = fs.createWriteStream(destPath)

    const request = https.get(url, (response) => {
      // Handle redirects
      if (response.statusCode === 301 || response.statusCode === 302) {
        file.close()
        fs.unlinkSync(destPath)
        downloadFile(response.headers.location, destPath)
          .then(resolve)
          .catch(reject)
        return
      }

      if (response.statusCode !== 200) {
        file.close()
        fs.unlinkSync(destPath)
        reject(new Error(`Failed to download: ${response.statusCode}`))
        return
      }

      response.pipe(file)

      file.on('finish', () => {
        file.close()
        resolve()
      })
    })

    request.on('error', (err) => {
      file.close()
      fs.unlinkSync(destPath)
      reject(err)
    })
  })
}

// Sample images to download from picsum.photos
// Using specific image IDs for consistent results
const images = [
  { name: 'landscape.png', id: 10, width: 300, height: 200 },   // Nature landscape
  { name: 'room.png', id: 42, width: 300, height: 200 },        // Interior
  { name: 'cityscape.png', id: 274, width: 300, height: 200 },  // City view
  { name: 'chart.png', id: 180, width: 300, height: 200 },      // Abstract (for chart placeholder)
  { name: 'invoice.png', id: 24, width: 300, height: 200 },     // Document-like
]

async function main() {
  console.log('Downloading sample images from picsum.photos...')

  for (const img of images) {
    const url = `https://picsum.photos/id/${img.id}/${img.width}/${img.height}.jpg`
    const destPath = path.join(samplesDir, img.name.replace('.png', '.jpg'))

    try {
      console.log(`Downloading: ${img.name} from ${url}`)
      await downloadFile(url, destPath)
      const stats = fs.statSync(destPath)
      console.log(`  Saved: ${destPath} (${stats.size} bytes)`)
    } catch (err) {
      console.error(`  Error downloading ${img.name}:`, err.message)
    }
  }

  console.log('Done!')
}

main()
