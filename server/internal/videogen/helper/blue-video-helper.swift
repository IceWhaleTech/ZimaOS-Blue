import Foundation
import AppKit
import AVFoundation
import CoreGraphics
import CoreVideo

struct FrameRect: Codable {
    let x: Double
    let y: Double
    let w: Double
    let h: Double
}

struct LayerSpec: Codable {
    let id: String?
    let imagePath: String
    let contentMode: String?
    let zIndex: Int?
    let startSec: Double?
    let endSec: Double?
    let frameStart: FrameRect
    let frameEnd: FrameRect
    let opacityStart: Double?
    let opacityEnd: Double?
}

struct Job: Codable {
    let width: Int
    let height: Int
    let fps: Int
    let durationSec: Int
    let layers: [LayerSpec]
    let audioPath: String?
    let outputPath: String
    let thumbnailPath: String
    let statusPath: String
    let thumbnailSec: Double?
}

struct ResultPayload: Codable {
    let outputPath: String
    let thumbnailPath: String
    let durationSec: Int

    enum CodingKeys: String, CodingKey {
        case outputPath = "output_path"
        case thumbnailPath = "thumbnail_path"
        case durationSec = "duration_sec"
    }
}

struct LoadedImage {
    let cgImage: CGImage
    let size: CGSize
}

func fail(_ message: String) -> Never {
    let payload = ["error": message]
    if let data = try? JSONSerialization.data(withJSONObject: payload, options: []),
       let text = String(data: data, encoding: .utf8) {
        fputs(text + "\n", stderr)
    } else {
        fputs(message + "\n", stderr)
    }
    exit(1)
}

func formatError(_ error: Error) -> String {
    let nsError = error as NSError
    var message = "\(nsError.domain)(\(nsError.code)): \(nsError.localizedDescription)"
    if let underlying = nsError.userInfo[NSUnderlyingErrorKey] as? NSError {
        message += " | underlying=\(underlying.domain)(\(underlying.code)): \(underlying.localizedDescription)"
    }
    return message
}

func writeProgress(path: String, stage: String, progress: Double, message: String) {
    guard !path.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty else { return }
    let formatter = ISO8601DateFormatter()
    let payload: [String: Any] = [
        "stage": stage,
        "progress": max(0, min(1, progress)),
        "message": message,
        "updated_at": formatter.string(from: Date()),
    ]
    guard let data = try? JSONSerialization.data(withJSONObject: payload, options: []) else { return }
    let url = URL(fileURLWithPath: path)
    let tmp = url.deletingLastPathComponent().appendingPathComponent(".\(UUID().uuidString).json")
    try? FileManager.default.createDirectory(at: url.deletingLastPathComponent(), withIntermediateDirectories: true)
    do {
        try data.write(to: tmp)
        _ = try? FileManager.default.replaceItemAt(url, withItemAt: tmp)
    } catch {
        try? data.write(to: url)
    }
}

func ensureParentDirectory(for path: String) throws {
    let url = URL(fileURLWithPath: path).deletingLastPathComponent()
    try FileManager.default.createDirectory(at: url, withIntermediateDirectories: true)
}

func loadImage(path: String) throws -> LoadedImage {
    let url = URL(fileURLWithPath: path)
    guard let image = NSImage(contentsOf: url) else {
        throw NSError(domain: "blue-video-helper", code: 1, userInfo: [NSLocalizedDescriptionKey: "failed to load image: \(path)"])
    }
    var proposed = CGRect(origin: .zero, size: image.size)
    guard let cgImage = image.cgImage(forProposedRect: &proposed, context: nil, hints: nil) else {
        throw NSError(domain: "blue-video-helper", code: 2, userInfo: [NSLocalizedDescriptionKey: "failed to decode image: \(path)"])
    }
    return LoadedImage(cgImage: cgImage, size: CGSize(width: cgImage.width, height: cgImage.height))
}

func lerp(_ a: Double, _ b: Double, _ t: Double) -> Double {
    return a + (b - a) * t
}

func layerWindow(_ layer: LayerSpec, duration: Double) -> (Double, Double) {
    let start = max(0, layer.startSec ?? 0)
    let end = max(start, layer.endSec ?? duration)
    return (start, end)
}

func activeLocalProgress(layer: LayerSpec, timeSec: Double, duration: Double) -> Double? {
    let (start, end) = layerWindow(layer, duration: duration)
    if timeSec < start || timeSec > end {
        return nil
    }
    let span = max(0.0001, end - start)
    return max(0, min(1, (timeSec - start) / span))
}

func interpolatedRect(layer: LayerSpec, timeSec: Double, duration: Double) -> CGRect? {
    guard let t = activeLocalProgress(layer: layer, timeSec: timeSec, duration: duration) else { return nil }
    return CGRect(
        x: lerp(layer.frameStart.x, layer.frameEnd.x, t),
        y: lerp(layer.frameStart.y, layer.frameEnd.y, t),
        width: max(1, lerp(layer.frameStart.w, layer.frameEnd.w, t)),
        height: max(1, lerp(layer.frameStart.h, layer.frameEnd.h, t))
    )
}

func interpolatedOpacity(layer: LayerSpec, timeSec: Double, duration: Double) -> Double {
    let t = activeLocalProgress(layer: layer, timeSec: timeSec, duration: duration) ?? 0
    let start = layer.opacityStart ?? 1
    let end = layer.opacityEnd ?? start
    return max(0, min(1, lerp(start, end, t)))
}

func aspectFitRect(size: CGSize, in frame: CGRect) -> CGRect {
    guard size.width > 0, size.height > 0, frame.width > 0, frame.height > 0 else { return frame }
    let scale = min(frame.width / size.width, frame.height / size.height)
    let drawSize = CGSize(width: size.width * scale, height: size.height * scale)
    return CGRect(
        x: frame.origin.x + (frame.width - drawSize.width) / 2,
        y: frame.origin.y + (frame.height - drawSize.height) / 2,
        width: drawSize.width,
        height: drawSize.height
    )
}

func aspectFillRect(size: CGSize, in frame: CGRect) -> CGRect {
    guard size.width > 0, size.height > 0, frame.width > 0, frame.height > 0 else { return frame }
    let scale = max(frame.width / size.width, frame.height / size.height)
    let drawSize = CGSize(width: size.width * scale, height: size.height * scale)
    return CGRect(
        x: frame.origin.x + (frame.width - drawSize.width) / 2,
        y: frame.origin.y + (frame.height - drawSize.height) / 2,
        width: drawSize.width,
        height: drawSize.height
    )
}

func drawLayer(_ context: CGContext, layer: LayerSpec, image: LoadedImage, timeSec: Double, duration: Double) {
    guard let frame = interpolatedRect(layer: layer, timeSec: timeSec, duration: duration) else { return }
    let opacity = interpolatedOpacity(layer: layer, timeSec: timeSec, duration: duration)
    guard opacity > 0 else { return }

    let mode = (layer.contentMode ?? "fill").trimmingCharacters(in: .whitespacesAndNewlines).lowercased()
    let drawRect = mode == "fit" ? aspectFitRect(size: image.size, in: frame) : aspectFillRect(size: image.size, in: frame)

    context.saveGState()
    context.setAlpha(CGFloat(opacity))
    context.clip(to: frame)
    context.draw(image.cgImage, in: drawRect)
    context.restoreGState()
}

func drawFrame(in context: CGContext, job: Job, images: [String: LoadedImage], timeSec: Double) {
    context.setFillColor(NSColor.black.cgColor)
    context.fill(CGRect(x: 0, y: 0, width: job.width, height: job.height))
    context.translateBy(x: 0, y: CGFloat(job.height))
    context.scaleBy(x: 1, y: -1)

    let duration = Double(job.durationSec)
    let orderedLayers = job.layers.sorted { ($0.zIndex ?? 0) < ($1.zIndex ?? 0) }
    for layer in orderedLayers {
        guard let image = images[layer.imagePath] else { continue }
        drawLayer(context, layer: layer, image: image, timeSec: timeSec, duration: duration)
    }
}

func renderPixelBuffer(_ pixelBuffer: CVPixelBuffer, job: Job, images: [String: LoadedImage], timeSec: Double) throws {
    CVPixelBufferLockBaseAddress(pixelBuffer, [])
    defer { CVPixelBufferUnlockBaseAddress(pixelBuffer, []) }
    guard let baseAddress = CVPixelBufferGetBaseAddress(pixelBuffer) else {
        throw NSError(domain: "blue-video-helper", code: 3, userInfo: [NSLocalizedDescriptionKey: "pixel buffer has no base address"])
    }
    let bytesPerRow = CVPixelBufferGetBytesPerRow(pixelBuffer)
    guard let context = CGContext(
        data: baseAddress,
        width: job.width,
        height: job.height,
        bitsPerComponent: 8,
        bytesPerRow: bytesPerRow,
        space: CGColorSpaceCreateDeviceRGB(),
        bitmapInfo: CGImageAlphaInfo.premultipliedFirst.rawValue | CGBitmapInfo.byteOrder32Little.rawValue
    ) else {
        throw NSError(domain: "blue-video-helper", code: 4, userInfo: [NSLocalizedDescriptionKey: "failed to create frame context"])
    }
    drawFrame(in: context, job: job, images: images, timeSec: timeSec)
}

func renderThumbnail(job: Job, images: [String: LoadedImage], timeSec: Double, outputPath: String) throws {
    let bytesPerRow = job.width * 4
    let byteCount = bytesPerRow * job.height
    let data = UnsafeMutableRawPointer.allocate(byteCount: byteCount, alignment: 16)
    defer { data.deallocate() }
    guard let context = CGContext(
        data: data,
        width: job.width,
        height: job.height,
        bitsPerComponent: 8,
        bytesPerRow: bytesPerRow,
        space: CGColorSpaceCreateDeviceRGB(),
        bitmapInfo: CGImageAlphaInfo.premultipliedFirst.rawValue | CGBitmapInfo.byteOrder32Little.rawValue
    ) else {
        throw NSError(domain: "blue-video-helper", code: 5, userInfo: [NSLocalizedDescriptionKey: "failed to create thumbnail context"])
    }
    drawFrame(in: context, job: job, images: images, timeSec: timeSec)
    guard let image = context.makeImage() else {
        throw NSError(domain: "blue-video-helper", code: 6, userInfo: [NSLocalizedDescriptionKey: "failed to finalize thumbnail image"])
    }
    let rep = NSBitmapImageRep(cgImage: image)
    guard let pngData = rep.representation(using: .png, properties: [:]) else {
        throw NSError(domain: "blue-video-helper", code: 7, userInfo: [NSLocalizedDescriptionKey: "failed to encode thumbnail"])
    }
    try ensureParentDirectory(for: outputPath)
    try pngData.write(to: URL(fileURLWithPath: outputPath))
}

func waitUntilReady(_ input: AVAssetWriterInput) {
    while !input.isReadyForMoreMediaData {
        Thread.sleep(forTimeInterval: 0.005)
    }
}

func runExport(_ session: AVAssetExportSession) throws {
    let semaphore = DispatchSemaphore(value: 0)
    session.exportAsynchronously {
        semaphore.signal()
    }
    semaphore.wait()
    switch session.status {
    case .completed:
        return
    case .failed:
        throw session.error ?? NSError(domain: "blue-video-helper", code: 8, userInfo: [NSLocalizedDescriptionKey: "export failed"])
    case .cancelled:
        throw NSError(domain: "blue-video-helper", code: 9, userInfo: [NSLocalizedDescriptionKey: "export cancelled"])
    default:
        throw NSError(domain: "blue-video-helper", code: 10, userInfo: [NSLocalizedDescriptionKey: "unexpected export status"])
    }
}

func exportVideo(videoURL: URL, outputURL: URL) throws {
    let asset = AVURLAsset(url: videoURL)
    guard let exporter = AVAssetExportSession(asset: asset, presetName: AVAssetExportPresetHighestQuality) else {
        throw NSError(domain: "blue-video-helper", code: 17, userInfo: [NSLocalizedDescriptionKey: "failed to create video export session"])
    }
    try ensureParentDirectory(for: outputURL.path)
    try? FileManager.default.removeItem(at: outputURL)
    exporter.outputURL = outputURL
    exporter.outputFileType = .mp4
    exporter.shouldOptimizeForNetworkUse = true
    try runExport(exporter)
}

func mergeAudio(videoURL: URL, audioURL: URL, outputURL: URL) throws {
    let composition = AVMutableComposition()
    let videoAsset = AVURLAsset(url: videoURL)
    let audioAsset = AVURLAsset(url: audioURL)

    guard let videoTrack = videoAsset.tracks(withMediaType: .video).first else {
        throw NSError(domain: "blue-video-helper", code: 11, userInfo: [NSLocalizedDescriptionKey: "video track missing"])
    }
    guard let compVideo = composition.addMutableTrack(withMediaType: .video, preferredTrackID: kCMPersistentTrackID_Invalid) else {
        throw NSError(domain: "blue-video-helper", code: 12, userInfo: [NSLocalizedDescriptionKey: "failed to create composition video track"])
    }
    try compVideo.insertTimeRange(CMTimeRange(start: .zero, duration: videoAsset.duration), of: videoTrack, at: .zero)
    compVideo.preferredTransform = videoTrack.preferredTransform

    if let audioTrack = audioAsset.tracks(withMediaType: .audio).first,
       let compAudio = composition.addMutableTrack(withMediaType: .audio, preferredTrackID: kCMPersistentTrackID_Invalid) {
        let duration = CMTimeMinimum(videoAsset.duration, audioAsset.duration)
        try compAudio.insertTimeRange(CMTimeRange(start: .zero, duration: duration), of: audioTrack, at: .zero)
    }

    guard let exporter = AVAssetExportSession(asset: composition, presetName: AVAssetExportPresetHighestQuality) else {
        throw NSError(domain: "blue-video-helper", code: 13, userInfo: [NSLocalizedDescriptionKey: "failed to create export session"])
    }
    try ensureParentDirectory(for: outputURL.path)
    try? FileManager.default.removeItem(at: outputURL)
    exporter.outputURL = outputURL
    exporter.outputFileType = .mp4
    exporter.shouldOptimizeForNetworkUse = true
    try runExport(exporter)
}

func renderVideo(job: Job, images: [String: LoadedImage]) throws {
    let finalOutputURL = URL(fileURLWithPath: job.outputPath)
    let temporaryOutputURL = finalOutputURL.deletingLastPathComponent().appendingPathComponent("video-\(UUID().uuidString).mov")

    try ensureParentDirectory(for: temporaryOutputURL.path)
    try? FileManager.default.removeItem(at: temporaryOutputURL)
    try? FileManager.default.removeItem(at: finalOutputURL)

    let writer = try AVAssetWriter(outputURL: temporaryOutputURL, fileType: .mov)
    let videoSettings: [String: Any] = [
        AVVideoCodecKey: AVVideoCodecType.proRes422LT,
        AVVideoWidthKey: job.width,
        AVVideoHeightKey: job.height,
    ]
    let videoInput = AVAssetWriterInput(mediaType: .video, outputSettings: videoSettings)
    videoInput.expectsMediaDataInRealTime = false
    let pixelBufferAttributes: [String: Any] = [
        kCVPixelBufferPixelFormatTypeKey as String: Int(kCVPixelFormatType_32BGRA),
        kCVPixelBufferWidthKey as String: job.width,
        kCVPixelBufferHeightKey as String: job.height,
        kCVPixelBufferCGImageCompatibilityKey as String: true,
        kCVPixelBufferCGBitmapContextCompatibilityKey as String: true,
    ]
    let adaptor = AVAssetWriterInputPixelBufferAdaptor(assetWriterInput: videoInput, sourcePixelBufferAttributes: pixelBufferAttributes)
    guard writer.canAdd(videoInput) else {
        throw NSError(domain: "blue-video-helper", code: 14, userInfo: [NSLocalizedDescriptionKey: "cannot add video input"])
    }
    writer.add(videoInput)
    guard writer.startWriting() else {
        throw writer.error ?? NSError(domain: "blue-video-helper", code: 15, userInfo: [NSLocalizedDescriptionKey: "failed to start writing"])
    }
    writer.startSession(atSourceTime: .zero)

    let totalFrames = max(1, job.durationSec * max(1, job.fps))
    for frameIndex in 0..<totalFrames {
        autoreleasepool {
            waitUntilReady(videoInput)
            guard let pool = adaptor.pixelBufferPool else {
                fail("pixel buffer pool unavailable")
            }
            var pixelBuffer: CVPixelBuffer?
            let status = CVPixelBufferPoolCreatePixelBuffer(nil, pool, &pixelBuffer)
            if status != kCVReturnSuccess || pixelBuffer == nil {
                fail("failed to allocate pixel buffer")
            }
            let timeSec = Double(frameIndex) / Double(max(1, job.fps))
            do {
                try renderPixelBuffer(pixelBuffer!, job: job, images: images, timeSec: timeSec)
            } catch {
                fail(formatError(error))
            }
            let presentationTime = CMTime(value: CMTimeValue(frameIndex), timescale: CMTimeScale(max(1, job.fps)))
            if !adaptor.append(pixelBuffer!, withPresentationTime: presentationTime) {
                if let error = writer.error {
                    fail(formatError(error))
                }
                fail("failed to append video frame")
            }
        }
        if frameIndex == 0 || frameIndex == totalFrames - 1 || frameIndex % max(1, job.fps / 2) == 0 {
            let progress = 0.10 + (Double(frameIndex + 1) / Double(totalFrames)) * 0.70
            writeProgress(path: job.statusPath, stage: "render_video", progress: progress, message: "Rendering native timeline video")
        }
    }

    videoInput.markAsFinished()
    let semaphore = DispatchSemaphore(value: 0)
    writer.finishWriting {
        semaphore.signal()
    }
    semaphore.wait()
    if writer.status != .completed {
        if let error = writer.error {
            throw NSError(domain: "blue-video-helper", code: 16, userInfo: [NSLocalizedDescriptionKey: formatError(error)])
        }
        throw NSError(domain: "blue-video-helper", code: 16, userInfo: [NSLocalizedDescriptionKey: "video writing failed"])
    }

    let thumbnailTime = min(Double(job.durationSec), max(0, job.thumbnailSec ?? Double(job.durationSec) * 0.5))
    try renderThumbnail(job: job, images: images, timeSec: thumbnailTime, outputPath: job.thumbnailPath)
    writeProgress(path: job.statusPath, stage: "thumbnail", progress: 0.86, message: "Rendered thumbnail")

    if let audioPath = job.audioPath, !audioPath.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty, FileManager.default.fileExists(atPath: audioPath) {
        do {
            try mergeAudio(videoURL: temporaryOutputURL, audioURL: URL(fileURLWithPath: audioPath), outputURL: finalOutputURL)
            try? FileManager.default.removeItem(at: temporaryOutputURL)
            writeProgress(path: job.statusPath, stage: "merge_audio", progress: 0.96, message: "Merged audio track")
        } catch {
            try exportVideo(videoURL: temporaryOutputURL, outputURL: finalOutputURL)
            try? FileManager.default.removeItem(at: temporaryOutputURL)
            writeProgress(path: job.statusPath, stage: "merge_audio", progress: 0.92, message: "Audio merge failed, kept silent video")
        }
        return
    }

    try exportVideo(videoURL: temporaryOutputURL, outputURL: finalOutputURL)
    try? FileManager.default.removeItem(at: temporaryOutputURL)
    writeProgress(path: job.statusPath, stage: "finalize", progress: 0.96, message: "Finalized MP4")
}

guard CommandLine.arguments.count >= 2 else {
    fail("usage: blue-video-helper <job.json>")
}

let requestPath = CommandLine.arguments[1]
let requestURL = URL(fileURLWithPath: requestPath)
guard let requestData = try? Data(contentsOf: requestURL) else {
    fail("failed to read helper job: \(requestPath)")
}

let decoder = JSONDecoder()
decoder.keyDecodingStrategy = .convertFromSnakeCase
let job: Job
do {
    job = try decoder.decode(Job.self, from: requestData)
} catch {
    fail("failed to decode helper job: \(error.localizedDescription)")
}

do {
    writeProgress(path: job.statusPath, stage: "loading", progress: 0.04, message: "Loading layer assets")
    var images: [String: LoadedImage] = [:]
    for layer in job.layers {
        if images[layer.imagePath] == nil {
            images[layer.imagePath] = try loadImage(path: layer.imagePath)
        }
    }
    writeProgress(path: job.statusPath, stage: "loaded", progress: 0.08, message: "Layer assets ready")
    try renderVideo(job: job, images: images)
    writeProgress(path: job.statusPath, stage: "completed", progress: 1.0, message: "Native timeline render completed")

    let payload = ResultPayload(outputPath: job.outputPath, thumbnailPath: job.thumbnailPath, durationSec: job.durationSec)
    let encoder = JSONEncoder()
    encoder.outputFormatting = [.withoutEscapingSlashes]
    let data = try encoder.encode(payload)
    if let text = String(data: data, encoding: .utf8) {
        print(text)
    } else {
        fail("failed to encode helper result")
    }
} catch {
    fail(formatError(error))
}
