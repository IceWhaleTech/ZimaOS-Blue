import Foundation
import AppKit
import PDFKit
import AVFoundation

func fail(_ message: String) -> Never {
    let payload: [String: Any] = ["error": message]
    if let data = try? JSONSerialization.data(withJSONObject: payload, options: []),
       let text = String(data: data, encoding: .utf8) {
        fputs(text + "\n", stderr)
    }
    exit(1)
}

func stringValue(_ value: Any?) -> String {
    if let s = value as? String { return s.trimmingCharacters(in: .whitespacesAndNewlines) }
    return ""
}

func intValue(_ value: Any?) -> Int {
    if let n = value as? Int { return n }
    if let n = value as? Double { return Int(n) }
    if let s = value as? String, let n = Int(s.trimmingCharacters(in: .whitespacesAndNewlines)) { return n }
    return 0
}

func dictValue(_ value: Any?) -> [String: Any] {
    return value as? [String: Any] ?? [:]
}

func arrayValue(_ value: Any?) -> [Any] {
    return value as? [Any] ?? []
}

func outputDict(path: String, previewKind: String? = nil, previewText: String? = nil) -> [String: Any] {
    let url = URL(fileURLWithPath: path)
    var payload: [String: Any] = [
        "path": path,
        "name": url.lastPathComponent,
    ]
    if let previewKind, !previewKind.isEmpty {
        payload["preview_kind"] = previewKind
    }
    if let previewText, !previewText.isEmpty {
        payload["preview_text"] = previewText
    }
    return payload
}

func writeImage(_ image: NSImage, to path: String, format: String) throws {
    guard let tiff = image.tiffRepresentation, let rep = NSBitmapImageRep(data: tiff) else {
        throw NSError(domain: "helper", code: 1, userInfo: [NSLocalizedDescriptionKey: "failed to encode image"])
    }
    let normalized = format.lowercased()
    let type: NSBitmapImageRep.FileType = (normalized == "jpg" || normalized == "jpeg") ? .jpeg : .png
    let props: [NSBitmapImageRep.PropertyKey: Any] = type == .jpeg ? [.compressionFactor: 0.92] : [:]
    guard let data = rep.representation(using: type, properties: props) else {
        throw NSError(domain: "helper", code: 2, userInfo: [NSLocalizedDescriptionKey: "failed to render image"])
    }
    try data.write(to: URL(fileURLWithPath: path))
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
        throw session.error ?? NSError(domain: "helper", code: 3, userInfo: [NSLocalizedDescriptionKey: "export failed"])
    case .cancelled:
        throw NSError(domain: "helper", code: 4, userInfo: [NSLocalizedDescriptionKey: "export cancelled"])
    default:
        throw NSError(domain: "helper", code: 5, userInfo: [NSLocalizedDescriptionKey: "unexpected export status"])
    }
}

func fileType(for format: String) -> AVFileType {
    switch format.lowercased() {
    case "mov": return .mov
    case "m4v": return .m4v
    default: return .mp4
    }
}

func extensionForVideo(format: String) -> String {
    let normalized = format.lowercased()
    if normalized == "mov" || normalized == "m4v" { return normalized }
    return "mp4"
}

func timeRange(from options: [String: Any], asset: AVAsset) -> CMTimeRange? {
    let video = dictValue(options["video"])
    let startMS = intValue(video["start_ms"])
    let endMS = intValue(video["end_ms"])
    if startMS <= 0 && endMS <= 0 { return nil }
    let start = CMTime(seconds: Double(max(startMS, 0)) / 1000.0, preferredTimescale: 600)
    let duration: CMTime
    if endMS > startMS && endMS > 0 {
        duration = CMTime(seconds: Double(endMS - max(startMS, 0)) / 1000.0, preferredTimescale: 600)
    } else {
        duration = CMTimeSubtract(asset.duration, start)
    }
    return CMTimeRange(start: start, duration: duration)
}

func renderDocumentPDF(source: String, outputDir: String) throws -> [[String: Any]] {
    let sourceURL = URL(fileURLWithPath: source)
    let options: [NSAttributedString.DocumentReadingOptionKey: Any] = [:]
    let attr = try NSAttributedString(url: sourceURL, options: options, documentAttributes: nil)
    let width: CGFloat = 612
    let containerWidth: CGFloat = width - 72
    let storage = NSTextStorage(attributedString: attr)
    let layout = NSLayoutManager()
    let container = NSTextContainer(size: NSSize(width: containerWidth, height: .greatestFiniteMagnitude))
    layout.addTextContainer(container)
    storage.addLayoutManager(layout)
    layout.ensureLayout(for: container)
    let used = layout.usedRect(for: container)
    let height = max(used.height + 72, 792)
    let textView = NSTextView(frame: NSRect(x: 0, y: 0, width: width, height: height))
    textView.textContainerInset = NSSize(width: 36, height: 36)
    textView.isVerticallyResizable = true
    textView.isHorizontallyResizable = false
    textView.textContainer?.containerSize = NSSize(width: containerWidth, height: .greatestFiniteMagnitude)
    textView.textStorage?.setAttributedString(attr)
    let data = textView.dataWithPDF(inside: textView.bounds)
    let outPath = URL(fileURLWithPath: outputDir).appendingPathComponent(sourceURL.deletingPathExtension().lastPathComponent + ".pdf").path
    try data.write(to: URL(fileURLWithPath: outPath))
    return [outputDict(path: outPath, previewKind: "pdf")]
}

func mergePDF(sources: [String], outputDir: String) throws -> [[String: Any]] {
    let merged = PDFDocument()
    var index = 0
    for source in sources {
        let url = URL(fileURLWithPath: source)
        if url.pathExtension.lowercased() == "pdf", let doc = PDFDocument(url: url) {
            for pageIndex in 0..<doc.pageCount {
                if let page = doc.page(at: pageIndex) {
                    merged.insert(page, at: index)
                    index += 1
                }
            }
            continue
        }
        if let image = NSImage(contentsOf: url), let page = PDFPage(image: image) {
            merged.insert(page, at: index)
            index += 1
            continue
        }
        throw NSError(domain: "helper", code: 10, userInfo: [NSLocalizedDescriptionKey: "unsupported PDF merge source: \(source)"])
    }
    let outPath = URL(fileURLWithPath: outputDir).appendingPathComponent("merged.pdf").path
    if !merged.write(toFile: outPath) {
        throw NSError(domain: "helper", code: 11, userInfo: [NSLocalizedDescriptionKey: "failed to write merged PDF"])
    }
    return [outputDict(path: outPath, previewKind: "pdf")]
}

func splitPDF(source: String, pages: [Int], outputDir: String) throws -> [[String: Any]] {
    guard let doc = PDFDocument(url: URL(fileURLWithPath: source)) else {
        throw NSError(domain: "helper", code: 12, userInfo: [NSLocalizedDescriptionKey: "failed to open PDF"])
    }
    let selected = pages.isEmpty ? Array(1...doc.pageCount) : pages
    var outputs: [[String: Any]] = []
    for pageNum in selected where pageNum >= 1 && pageNum <= doc.pageCount {
        guard let page = doc.page(at: pageNum - 1) else { continue }
        let outDoc = PDFDocument()
        outDoc.insert(page, at: 0)
        let outPath = URL(fileURLWithPath: outputDir).appendingPathComponent("page-\(pageNum).pdf").path
        if outDoc.write(toFile: outPath) {
            outputs.append(outputDict(path: outPath, previewKind: "pdf"))
        }
    }
    return outputs
}

func pdfToImages(source: String, targetFormat: String, pages: [Int], outputDir: String) throws -> [[String: Any]] {
    guard let doc = PDFDocument(url: URL(fileURLWithPath: source)) else {
        throw NSError(domain: "helper", code: 13, userInfo: [NSLocalizedDescriptionKey: "failed to open PDF"])
    }
    let selected = pages.isEmpty ? Array(1...doc.pageCount) : pages
    var outputs: [[String: Any]] = []
    for pageNum in selected where pageNum >= 1 && pageNum <= doc.pageCount {
        guard let page = doc.page(at: pageNum - 1) else { continue }
        let image = page.thumbnail(of: NSSize(width: 1600, height: 1600), for: .mediaBox)
        let ext = (targetFormat.lowercased() == "jpg" || targetFormat.lowercased() == "jpeg") ? "jpg" : "png"
        let outPath = URL(fileURLWithPath: outputDir).appendingPathComponent("page-\(pageNum).\(ext)").path
        try writeImage(image, to: outPath, format: ext)
        outputs.append(outputDict(path: outPath, previewKind: "image"))
    }
    return outputs
}

func convertVideo(source: String, targetFormat: String, outputDir: String, options: [String: Any]) throws -> [[String: Any]] {
    let asset = AVURLAsset(url: URL(fileURLWithPath: source))
    guard let session = AVAssetExportSession(asset: asset, presetName: AVAssetExportPresetHighestQuality) else {
        throw NSError(domain: "helper", code: 20, userInfo: [NSLocalizedDescriptionKey: "failed to create export session"])
    }
    let ext = extensionForVideo(format: targetFormat)
    let outPath = URL(fileURLWithPath: outputDir).appendingPathComponent(URL(fileURLWithPath: source).deletingPathExtension().lastPathComponent + ".\(ext)").path
    try? FileManager.default.removeItem(atPath: outPath)
    session.outputURL = URL(fileURLWithPath: outPath)
    session.outputFileType = fileType(for: targetFormat)
    session.shouldOptimizeForNetworkUse = true
    if let range = timeRange(from: options, asset: asset) {
        session.timeRange = range
    }
    try runExport(session)
    return [outputDict(path: outPath, previewKind: "video")]
}

func trimVideo(source: String, targetFormat: String, outputDir: String, options: [String: Any]) throws -> [[String: Any]] {
    return try convertVideo(source: source, targetFormat: targetFormat, outputDir: outputDir, options: options)
}

func extractAudio(source: String, outputDir: String) throws -> [[String: Any]] {
    let asset = AVURLAsset(url: URL(fileURLWithPath: source))
    guard let session = AVAssetExportSession(asset: asset, presetName: AVAssetExportPresetAppleM4A) else {
        throw NSError(domain: "helper", code: 21, userInfo: [NSLocalizedDescriptionKey: "failed to create audio export session"])
    }
    let outPath = URL(fileURLWithPath: outputDir).appendingPathComponent(URL(fileURLWithPath: source).deletingPathExtension().lastPathComponent + ".m4a").path
    try? FileManager.default.removeItem(atPath: outPath)
    session.outputURL = URL(fileURLWithPath: outPath)
    session.outputFileType = .m4a
    try runExport(session)
    return [outputDict(path: outPath, previewKind: "audio")]
}

func mergeVideo(sources: [String], targetFormat: String, outputDir: String) throws -> [[String: Any]] {
    let composition = AVMutableComposition()
    guard let videoTrack = composition.addMutableTrack(withMediaType: .video, preferredTrackID: kCMPersistentTrackID_Invalid) else {
        throw NSError(domain: "helper", code: 22, userInfo: [NSLocalizedDescriptionKey: "failed to create video composition"])
    }
    let audioTrack = composition.addMutableTrack(withMediaType: .audio, preferredTrackID: kCMPersistentTrackID_Invalid)
    var cursor = CMTime.zero
    for source in sources {
        let asset = AVURLAsset(url: URL(fileURLWithPath: source))
        if let srcVideo = asset.tracks(withMediaType: .video).first {
            try videoTrack.insertTimeRange(CMTimeRange(start: .zero, duration: asset.duration), of: srcVideo, at: cursor)
        }
        if let srcAudio = asset.tracks(withMediaType: .audio).first {
            try audioTrack?.insertTimeRange(CMTimeRange(start: .zero, duration: asset.duration), of: srcAudio, at: cursor)
        }
        cursor = CMTimeAdd(cursor, asset.duration)
    }
    guard let session = AVAssetExportSession(asset: composition, presetName: AVAssetExportPresetHighestQuality) else {
        throw NSError(domain: "helper", code: 23, userInfo: [NSLocalizedDescriptionKey: "failed to create merge session"])
    }
    let ext = extensionForVideo(format: targetFormat)
    let outPath = URL(fileURLWithPath: outputDir).appendingPathComponent("merged.\(ext)").path
    try? FileManager.default.removeItem(atPath: outPath)
    session.outputURL = URL(fileURLWithPath: outPath)
    session.outputFileType = fileType(for: targetFormat)
    session.shouldOptimizeForNetworkUse = true
    try runExport(session)
    return [outputDict(path: outPath, previewKind: "video")]
}

func splitVideo(source: String, targetFormat: String, outputDir: String, options: [String: Any]) throws -> [[String: Any]] {
    let video = dictValue(options["video"])
    let segments = arrayValue(video["segments"])
    let asset = AVURLAsset(url: URL(fileURLWithPath: source))
    var outputs: [[String: Any]] = []
    let ext = extensionForVideo(format: targetFormat)
    for (index, item) in segments.enumerated() {
        let segment = dictValue(item)
        let startMS = intValue(segment["start_ms"])
        let endMS = intValue(segment["end_ms"])
        guard endMS > startMS else { continue }
        guard let session = AVAssetExportSession(asset: asset, presetName: AVAssetExportPresetHighestQuality) else { continue }
        let outPath = URL(fileURLWithPath: outputDir).appendingPathComponent("segment-\(index + 1).\(ext)").path
        try? FileManager.default.removeItem(atPath: outPath)
        session.outputURL = URL(fileURLWithPath: outPath)
        session.outputFileType = fileType(for: targetFormat)
        session.timeRange = CMTimeRange(start: CMTime(seconds: Double(startMS) / 1000.0, preferredTimescale: 600), duration: CMTime(seconds: Double(endMS - startMS) / 1000.0, preferredTimescale: 600))
        try runExport(session)
        outputs.append(outputDict(path: outPath, previewKind: "video"))
    }
    return outputs
}

func extractFrames(source: String, targetFormat: String, outputDir: String, options: [String: Any]) throws -> [[String: Any]] {
    let asset = AVURLAsset(url: URL(fileURLWithPath: source))
    let generator = AVAssetImageGenerator(asset: asset)
    generator.appliesPreferredTrackTransform = true
    let video = dictValue(options["video"])
    let intervalMS = max(intValue(video["frame_interval_ms"]), 1000)
    let duration = CMTimeGetSeconds(asset.duration)
    let ext = (targetFormat.lowercased() == "jpg" || targetFormat.lowercased() == "jpeg") ? "jpg" : "png"
    var outputs: [[String: Any]] = []
    var index = 0
    var current = 0.0
    while current <= duration {
        let time = CMTime(seconds: current, preferredTimescale: 600)
        let cgImage = try generator.copyCGImage(at: time, actualTime: nil)
        let image = NSImage(cgImage: cgImage, size: .zero)
        let outPath = URL(fileURLWithPath: outputDir).appendingPathComponent(String(format: "frame-%04d.%@", index + 1, ext)).path
        try writeImage(image, to: outPath, format: ext)
        outputs.append(outputDict(path: outPath, previewKind: "image"))
        index += 1
        current += Double(intervalMS) / 1000.0
    }
    return outputs
}

guard CommandLine.arguments.count >= 2 else {
    fail("missing request path")
}
let requestPath = CommandLine.arguments[1]
let requestData: Data

do {
    requestData = try Data(contentsOf: URL(fileURLWithPath: requestPath))
} catch {
    fail("failed to read helper request: \(error.localizedDescription)")
}

guard let request = try? JSONSerialization.jsonObject(with: requestData, options: []), let payload = request as? [String: Any] else {
    fail("invalid helper request JSON")
}

let action = stringValue(payload["action"])
let sources = (payload["sources"] as? [String]) ?? arrayValue(payload["sources"]).map { stringValue($0) }.filter { !$0.isEmpty }
let targetFormat = stringValue(payload["target_format"])
let outputDir = stringValue(payload["output_dir"])
let options = dictValue(payload["options"])
if outputDir.isEmpty { fail("output_dir is required") }
try? FileManager.default.createDirectory(atPath: outputDir, withIntermediateDirectories: true)

let outputs: [[String: Any]]
do {
    switch action {
    case "image_to_pdf", "merge_pdf":
        outputs = try mergePDF(sources: sources, outputDir: outputDir)
    case "pdf_split":
        let pages = arrayValue(dictValue(options["document"])["pages"]).compactMap { value -> Int? in
            let number = intValue(value)
            return number > 0 ? number : nil
        }
        guard let source = sources.first else { fail("pdf_split requires one source") }
        outputs = try splitPDF(source: source, pages: pages, outputDir: outputDir)
    case "pdf_to_images":
        let pages = arrayValue(dictValue(options["document"])["pages"]).compactMap { value -> Int? in
            let number = intValue(value)
            return number > 0 ? number : nil
        }
        guard let source = sources.first else { fail("pdf_to_images requires one source") }
        outputs = try pdfToImages(source: source, targetFormat: targetFormat.isEmpty ? "png" : targetFormat, pages: pages, outputDir: outputDir)
    case "render_document_pdf":
        guard let source = sources.first else { fail("render_document_pdf requires one source") }
        outputs = try renderDocumentPDF(source: source, outputDir: outputDir)
    case "video_convert":
        guard let source = sources.first else { fail("video_convert requires one source") }
        outputs = try convertVideo(source: source, targetFormat: targetFormat.isEmpty ? "mp4" : targetFormat, outputDir: outputDir, options: options)
    case "video_trim":
        guard let source = sources.first else { fail("video_trim requires one source") }
        outputs = try trimVideo(source: source, targetFormat: targetFormat.isEmpty ? "mp4" : targetFormat, outputDir: outputDir, options: options)
    case "extract_audio":
        guard let source = sources.first else { fail("extract_audio requires one source") }
        outputs = try extractAudio(source: source, outputDir: outputDir)
    case "video_merge":
        outputs = try mergeVideo(sources: sources, targetFormat: targetFormat.isEmpty ? "mp4" : targetFormat, outputDir: outputDir)
    case "video_split":
        guard let source = sources.first else { fail("video_split requires one source") }
        outputs = try splitVideo(source: source, targetFormat: targetFormat.isEmpty ? "mp4" : targetFormat, outputDir: outputDir, options: options)
    case "extract_frames":
        guard let source = sources.first else { fail("extract_frames requires one source") }
        outputs = try extractFrames(source: source, targetFormat: targetFormat.isEmpty ? "png" : targetFormat, outputDir: outputDir, options: options)
    default:
        fail("unsupported helper action: \(action)")
    }
} catch {
    fail(error.localizedDescription)
}

let response: [String: Any] = ["outputs": outputs]
guard let responseData = try? JSONSerialization.data(withJSONObject: response, options: []), let text = String(data: responseData, encoding: .utf8) else {
    fail("failed to encode helper response")
}
print(text)
