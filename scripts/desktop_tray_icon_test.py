import struct
import unittest
import xml.etree.ElementTree as ET
import zlib
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
ICONS = ROOT / "tauri-app/src-tauri/nsis/icons"


def rgba_pixels(path):
    data = path.read_bytes()
    offset, compressed = 8, b""
    while offset < len(data):
        size = struct.unpack_from(">I", data, offset)[0]
        kind, chunk = data[offset + 4:offset + 8], data[offset + 8:offset + 8 + size]
        offset += size + 12
        if kind == b"IHDR":
            width, height, depth, color, _, _, interlace = struct.unpack(">IIBBBBB", chunk)
            assert (depth, color, interlace) == (8, 6, 0)
        if kind == b"IDAT":
            compressed += chunk
    raw, previous, pixels = zlib.decompress(compressed), [0] * (width * 4), []
    for y in range(height):
        start = y * (width * 4 + 1)
        mode, row = raw[start], list(raw[start + 1:start + 1 + width * 4])
        for x in range(len(row)):
            left, above = (row[x - 4] if x >= 4 else 0), previous[x]
            corner = previous[x - 4] if x >= 4 else 0
            if mode == 4:
                estimate = left + above - corner
                predictor = min((left, above, corner), key=lambda value: abs(estimate - value))
            else:
                predictor = (0, left, above, (left + above) // 2)[mode]
            row[x] = (row[x] + predictor) & 255
        pixels.extend(tuple(row[x:x + 4]) for x in range(0, len(row), 4))
        previous = row
    return width, height, pixels


class DesktopTrayIconTests(unittest.TestCase):
    def test_macos_uses_a_dedicated_mask(self):
        source = (ROOT / "tauri-app/src-tauri/src/lib.rs").read_text()
        self.assertTrue('include_bytes!("../nsis/icons/tray-template.png")' in source)

    def test_mask_preserves_official_blue_geometry(self):
        path = ICONS / "tray-template.svg"
        self.assertTrue(path.exists(), "Missing reproducible Blue template artwork")
        template = ET.parse(path).getroot()
        logo = ET.parse(ROOT / "web/public/logo.svg").getroot()
        circles = lambda root: [
            {key: value for key, value in node.attrib.items() if key != "fill"}
            for node in root.iter("{http://www.w3.org/2000/svg}circle")
        ]
        self.assertEqual(circles(template), circles(logo))
        self.assertFalse(any(node.tag.endswith("rect") for node in template.iter()))
        self.assertFalse(any("url(#paint" in node.get("fill", "") for node in template.iter()))

    def test_retina_mask_has_transparent_padding_and_monochrome_detail(self):
        path = ICONS / "tray-template.png"
        self.assertTrue(path.exists(), "Missing Retina template PNG")
        width, height, pixels = rgba_pixels(path)
        self.assertEqual((width, height), (44, 44))
        self.assertEqual(pixels[0][3], 0)
        opaque = [pixel for pixel in pixels if pixel[3] > 0]
        self.assertGreater(len(opaque), len(pixels) * 0.15)
        self.assertLess(len(opaque), len(pixels) * 0.85)
        self.assertTrue(all(pixel[:3] == (0, 0, 0) for pixel in opaque))


if __name__ == "__main__":
    unittest.main()
