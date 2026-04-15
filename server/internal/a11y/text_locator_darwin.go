//go:build darwin

package a11y

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

const darwinVisionTextLocatorScript = `ObjC.import('Foundation');
ObjC.import('Vision');
function run(argv) {
  var path = argv[0];
  if (!path) {
    throw new Error('missing image path');
  }
  var url = $.NSURL.fileURLWithPath(path);
  var request = $.VNRecognizeTextRequest.alloc.init;
  request.setRecognitionLevel($.VNRequestTextRecognitionLevelFast);
  request.setUsesLanguageCorrection(false);
  var handler = $.VNImageRequestHandler.alloc.initWithURLOptions(url, $({}));
  var error = Ref();
  if (!handler.performRequestsError($.NSArray.arrayWithObject(request), error)) {
    var reason = error[0] ? ObjC.unwrap(error[0].localizedDescription) : 'performRequests failed';
    throw new Error(reason);
  }
  var observations = request.results ? request.results.js : [];
  var out = [];
  for (var i = 0; i < observations.length; i++) {
    var observation = observations[i];
    var candidates = observation.topCandidates(1);
    if (!candidates || candidates.count === 0) {
      continue;
    }
    var candidate = candidates.objectAtIndex(0);
    var text = ObjC.unwrap(candidate.string);
    if (!text || !text.trim()) {
      continue;
    }
    var box = observation.boundingBox;
    out.push({
      text: text,
      bounds: {
        x: Number(box.origin.x),
        y: Number(1 - box.origin.y - box.size.height),
        width: Number(box.size.width),
        height: Number(box.size.height)
      },
      confidence: Number(candidate.confidence)
    });
  }
  return JSON.stringify(out);
}`

func LocateTextInImage(ctx context.Context, imagePath string) ([]TextLine, error) {
	imagePath = strings.TrimSpace(imagePath)
	if imagePath == "" {
		return nil, fmt.Errorf("locate text in image: image path is required")
	}
	output, err := darwinCLIFallback.exec(
		ctx,
		"osascript",
		"-l", "JavaScript",
		"-e", darwinVisionTextLocatorScript,
		"--",
		imagePath,
	)
	if err != nil {
		return nil, fmt.Errorf("vision text locate failed: %s: %w", strings.TrimSpace(output), err)
	}
	output = strings.TrimSpace(output)
	if output == "" || output == "null" {
		return nil, nil
	}
	var lines []TextLine
	if err := json.Unmarshal([]byte(output), &lines); err != nil {
		return nil, fmt.Errorf("decode vision text locate output: %w", err)
	}
	for idx := range lines {
		lines[idx].Bounds = clampNormalizedRect(lines[idx].Bounds)
	}
	return lines, nil
}

func clampNormalizedRect(rect NormalizedRect) NormalizedRect {
	rect.X = clampNormalizedValue(rect.X)
	rect.Y = clampNormalizedValue(rect.Y)
	rect.Width = clampNormalizedDimension(rect.Width, rect.X)
	rect.Height = clampNormalizedDimension(rect.Height, rect.Y)
	return rect
}

func clampNormalizedValue(value float64) float64 {
	switch {
	case value < 0:
		return 0
	case value > 1:
		return 1
	default:
		return value
	}
}

func clampNormalizedDimension(size float64, origin float64) float64 {
	switch {
	case size < 0:
		return 0
	case origin+size > 1:
		return 1 - origin
	default:
		return size
	}
}
