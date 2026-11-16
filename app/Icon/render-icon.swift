// Source of the app icon of Passierschein.app (issue 104, D69, T42).
//
// The program draws three drafts of the motif "stylised identity
// document" with Core Graphics and writes one PNG per size of the macOS
// icon set:
//
//   render-icon <draft 1|2|3> <output>.iconset
//
// Draft 1: identity card with a photo field and text lines.
// Draft 2: passport booklet with an abstract seal (a keyhole in a ring).
// Draft 3: permit sheet with a folded corner and an approval stamp.
//
// All drafts are original drawings: no logo, no trademark, no state
// emblem. The drawing uses the macOS icon grid: a canvas of 1024 x 1024
// units, a rounded square of 824 x 824 units with a drop shadow. Each size
// is drawn on its own pixel grid, not scaled down from 1024 px. Below
// 64 px a draft drops its fine details, so that it stays readable at
// 16 px. The output depends only on this file: no font, no random value,
// no date.
//
// scripts/app-icon.sh compiles and runs this program (task app:icon).

import CoreGraphics
import Foundation
import ImageIO
import UniformTypeIdentifiers

/// A colour from a hex value 0xRRGGBB in sRGB.
func rgb(_ hex: UInt32, _ alpha: CGFloat = 1) -> CGColor {
    let r = CGFloat((hex >> 16) & 0xFF) / 255
    let g = CGFloat((hex >> 8) & 0xFF) / 255
    let b = CGFloat(hex & 0xFF) / 255
    return CGColor(srgbRed: r, green: g, blue: b, alpha: alpha)
}

let sRGB = CGColorSpace(name: CGColorSpace.sRGB)!

/// The drawing context of one size. Coordinates are icon units (0...1024),
/// origin top left, y down.
struct Canvas {
    let ctx: CGContext
    /// The pixel size of the image.
    let pixels: Int
    /// True from 64 px: fine lines and small shapes are visible.
    var detail: Bool { pixels >= 64 }

    func fill(_ path: CGPath, _ color: CGColor) {
        ctx.addPath(path)
        ctx.setFillColor(color)
        ctx.fillPath()
    }

    func fillGradient(_ path: CGPath, top: CGColor, bottom: CGColor, from y0: CGFloat, to y1: CGFloat) {
        ctx.saveGState()
        ctx.addPath(path)
        ctx.clip()
        let gradient = CGGradient(colorsSpace: sRGB, colors: [top, bottom] as CFArray, locations: [0, 1])!
        ctx.drawLinearGradient(gradient, start: CGPoint(x: 0, y: y0), end: CGPoint(x: 0, y: y1), options: [])
        ctx.restoreGState()
    }

    func stroke(_ path: CGPath, _ color: CGColor, width: CGFloat) {
        ctx.addPath(path)
        ctx.setStrokeColor(color)
        ctx.setLineWidth(width)
        ctx.setLineCap(.round)
        ctx.setLineJoin(.round)
        ctx.strokePath()
    }

    /// A horizontal line with round ends from x0 to x1.
    func bar(_ x0: CGFloat, _ x1: CGFloat, y: CGFloat, width: CGFloat, _ color: CGColor) {
        let path = CGMutablePath()
        path.move(to: CGPoint(x: x0, y: y))
        path.addLine(to: CGPoint(x: x1, y: y))
        stroke(path, color, width: width)
    }

    /// Draw `body` with a soft shadow under it.
    func withShadow(offset: CGFloat, blur: CGFloat, alpha: CGFloat, _ body: () -> Void) {
        ctx.saveGState()
        // The shadow API works in device space, y up; scale by hand.
        let k = CGFloat(pixels) / 1024
        ctx.setShadow(offset: CGSize(width: 0, height: -offset * k), blur: blur * k, color: rgb(0x000000, alpha))
        ctx.beginTransparencyLayer(auxiliaryInfo: nil)
        body()
        ctx.endTransparencyLayer()
        ctx.restoreGState()
    }
}

func roundedRect(_ x: CGFloat, _ y: CGFloat, _ w: CGFloat, _ h: CGFloat, _ r: CGFloat) -> CGPath {
    CGPath(roundedRect: CGRect(x: x, y: y, width: w, height: h), cornerWidth: r, cornerHeight: r, transform: nil)
}

func circle(_ cx: CGFloat, _ cy: CGFloat, _ r: CGFloat) -> CGPath {
    CGPath(ellipseIn: CGRect(x: cx - r, y: cy - r, width: 2 * r, height: 2 * r), transform: nil)
}

/// The rounded square of the macOS icon grid with its gradient and shadow.
func plate(_ c: Canvas, top: CGColor, bottom: CGColor) {
    let shape = roundedRect(100, 100, 824, 824, 185)
    c.withShadow(offset: 10, blur: 20, alpha: 0.30) {
        c.fill(shape, bottom)
    }
    c.fillGradient(shape, top: top, bottom: bottom, from: 100, to: 924)
}

// MARK: - Draft 1: identity card

func drawCard(_ c: Canvas) {
    plate(c, top: rgb(0x3B82F6), bottom: rgb(0x1D4ED8))

    // The card: ratio of an ID-1 card (85.6 x 54 mm), white.
    let x: CGFloat = 184, y: CGFloat = 300, w: CGFloat = 656, h: CGFloat = 424
    let card = roundedRect(x, y, w, h, 52)
    c.withShadow(offset: 14, blur: 28, alpha: 0.35) {
        c.fill(card, rgb(0xFFFFFF))
    }
    // The header band of the card.
    c.ctx.saveGState()
    c.ctx.addPath(card)
    c.ctx.clip()
    c.fill(CGPath(rect: CGRect(x: x, y: y, width: w, height: c.detail ? 92 : 110), transform: nil), rgb(0x1E3A8A))
    c.ctx.restoreGState()

    // The photo field with a neutral silhouette.
    let px: CGFloat = 236, py: CGFloat = 438, pw: CGFloat = 196, ph: CGFloat = 232
    let photo = roundedRect(px, py, pw, ph, 22)
    // Below 64 px the field is dark with a light figure: more contrast.
    let field = c.detail ? rgb(0xDBEAFE) : rgb(0x2563EB)
    let figure = c.detail ? rgb(0x2563EB) : rgb(0xDBEAFE)
    c.fill(photo, field)
    c.ctx.saveGState()
    c.ctx.addPath(photo)
    c.ctx.clip()
    c.fill(circle(px + pw / 2, py + 92, 46), figure)
    c.fill(CGPath(ellipseIn: CGRect(x: px + 18, y: py + 152, width: pw - 36, height: 170), transform: nil), figure)
    c.ctx.restoreGState()

    // Text lines: the name (dark), then the data (light).
    if c.detail {
        c.bar(484, 780, y: 476, width: 34, rgb(0x1E3A8A))
        c.bar(484, 740, y: 544, width: 26, rgb(0x94A3B8))
        c.bar(484, 760, y: 600, width: 26, rgb(0x94A3B8))
        c.bar(484, 680, y: 656, width: 26, rgb(0x94A3B8))
    } else {
        c.bar(484, 780, y: 500, width: 52, rgb(0x1E3A8A))
        c.bar(484, 720, y: 610, width: 52, rgb(0x94A3B8))
    }
}

// MARK: - Draft 2: passport booklet

/// A keyhole: a ring with a hole in the shape of a lock.
func keyhole(_ c: Canvas, cx: CGFloat, cy: CGFloat, r: CGFloat, color: CGColor) {
    c.stroke(circle(cx, cy, r), color, width: c.detail ? 22 : 40)
    if c.detail {
        c.stroke(circle(cx, cy, r - 38), color, width: 8)
    }
    let hole = CGMutablePath()
    hole.addPath(circle(cx, cy - r * 0.18, r * 0.24))
    hole.move(to: CGPoint(x: cx - r * 0.12, y: cy - r * 0.1))
    hole.addLine(to: CGPoint(x: cx + r * 0.12, y: cy - r * 0.1))
    hole.addLine(to: CGPoint(x: cx + r * 0.2, y: cy + r * 0.42))
    hole.addLine(to: CGPoint(x: cx - r * 0.2, y: cy + r * 0.42))
    hole.closeSubpath()
    c.fill(hole, color)
}

func drawPassport(_ c: Canvas) {
    plate(c, top: rgb(0xFDF4E3), bottom: rgb(0xE9D5B5))

    // The booklet: portrait, the spine on the left.
    let x: CGFloat = 282, y: CGFloat = 180, w: CGFloat = 460, h: CGFloat = 664
    let cover = roundedRect(x, y, w, h, 44)
    c.withShadow(offset: 16, blur: 30, alpha: 0.35) {
        c.fill(cover, rgb(0x0F3D3E))
    }
    c.fillGradient(cover, top: rgb(0x145A5B), bottom: rgb(0x0B3132), from: y, to: y + h)
    // The spine.
    c.ctx.saveGState()
    c.ctx.addPath(cover)
    c.ctx.clip()
    c.fill(CGPath(rect: CGRect(x: x, y: y, width: 44, height: h), transform: nil), rgb(0x072627))
    c.ctx.restoreGState()

    let gold = rgb(0xE8B64C)
    // The seal: an abstract keyhole in a ring, no state emblem.
    keyhole(c, cx: x + 22 + (w - 22) / 2, cy: c.detail ? 470 : 480, r: c.detail ? 132 : 150, color: gold)
    if c.detail {
        // The title line and the line under it.
        c.bar(x + 120, x + w - 98, y: 690, width: 26, gold)
        c.bar(x + 160, x + w - 138, y: 744, width: 14, gold)
    }
}

// MARK: - Draft 3: permit with a stamp

func drawPermit(_ c: Canvas) {
    plate(c, top: rgb(0x34D399), bottom: rgb(0x047857))

    // The sheet with a folded corner top right.
    let x: CGFloat = 262, y: CGFloat = 170, w: CGFloat = 450, h: CGFloat = 610, fold: CGFloat = 120
    let sheet = CGMutablePath()
    sheet.move(to: CGPoint(x: x + 28, y: y))
    sheet.addLine(to: CGPoint(x: x + w - fold, y: y))
    sheet.addLine(to: CGPoint(x: x + w, y: y + fold))
    sheet.addLine(to: CGPoint(x: x + w, y: y + h - 28))
    sheet.addQuadCurve(to: CGPoint(x: x + w - 28, y: y + h), control: CGPoint(x: x + w, y: y + h))
    sheet.addLine(to: CGPoint(x: x + 28, y: y + h))
    sheet.addQuadCurve(to: CGPoint(x: x, y: y + h - 28), control: CGPoint(x: x, y: y + h))
    sheet.addLine(to: CGPoint(x: x, y: y + 28))
    sheet.addQuadCurve(to: CGPoint(x: x + 28, y: y), control: CGPoint(x: x, y: y))
    sheet.closeSubpath()
    c.withShadow(offset: 14, blur: 28, alpha: 0.35) {
        c.fill(sheet, rgb(0xFFFFFF))
    }
    let corner = CGMutablePath()
    corner.move(to: CGPoint(x: x + w - fold, y: y))
    corner.addLine(to: CGPoint(x: x + w - fold, y: y + fold - 14))
    corner.addQuadCurve(to: CGPoint(x: x + w - fold + 14, y: y + fold), control: CGPoint(x: x + w - fold, y: y + fold))
    corner.addLine(to: CGPoint(x: x + w, y: y + fold))
    corner.closeSubpath()
    c.fill(corner, rgb(0xCBD5E1))

    // Text lines.
    let ink = rgb(0x334155), light = rgb(0xA3B1C2)
    if c.detail {
        c.bar(x + 60, x + w - 170, y: y + 90, width: 30, ink)
        c.bar(x + 60, x + w - 60, y: y + 200, width: 22, light)
        c.bar(x + 60, x + w - 90, y: y + 260, width: 22, light)
        c.bar(x + 60, x + w - 60, y: y + 320, width: 22, light)
        c.bar(x + 60, x + w - 200, y: y + 380, width: 22, light)
    } else {
        c.bar(x + 70, x + w - 170, y: y + 110, width: 54, ink)
        c.bar(x + 70, x + w - 90, y: y + 240, width: 54, light)
    }

    // The stamp: a ring with a check mark, turned like a real stamp.
    let red = rgb(0xDC2626)
    let sx: CGFloat = 672, sy: CGFloat = 690, sr: CGFloat = c.detail ? 150 : 170
    c.ctx.saveGState()
    c.ctx.translateBy(x: sx, y: sy)
    c.ctx.rotate(by: -0.26)
    c.fill(circle(0, 0, sr), rgb(0xFFFFFF, 0.92))
    c.stroke(circle(0, 0, sr - 16), red, width: c.detail ? 22 : 40)
    if c.detail {
        c.stroke(circle(0, 0, sr - 46), red, width: 7)
    }
    let check = CGMutablePath()
    check.move(to: CGPoint(x: -sr * 0.42, y: sr * 0.02))
    check.addLine(to: CGPoint(x: -sr * 0.1, y: sr * 0.32))
    check.addLine(to: CGPoint(x: sr * 0.44, y: -sr * 0.3))
    c.stroke(check, red, width: c.detail ? 40 : 56)
    c.ctx.restoreGState()
}

// MARK: - Output

func render(draft: Int, pixels: Int) -> CGImage {
    let ctx = CGContext(
        data: nil, width: pixels, height: pixels, bitsPerComponent: 8, bytesPerRow: 0,
        space: sRGB, bitmapInfo: CGImageAlphaInfo.premultipliedLast.rawValue)!
    ctx.setShouldAntialias(true)
    ctx.interpolationQuality = .high
    // Icon units, origin top left.
    let k = CGFloat(pixels) / 1024
    ctx.translateBy(x: 0, y: CGFloat(pixels))
    ctx.scaleBy(x: k, y: -k)
    let canvas = Canvas(ctx: ctx, pixels: pixels)
    switch draft {
    case 1: drawCard(canvas)
    case 2: drawPassport(canvas)
    default: drawPermit(canvas)
    }
    return ctx.makeImage()!
}

func writePNG(_ image: CGImage, to url: URL) throws {
    guard let dest = CGImageDestinationCreateWithURL(url as CFURL, UTType.png.identifier as CFString, 1, nil) else {
        throw NSError(domain: "render-icon", code: 1, userInfo: [NSLocalizedDescriptionKey: "cannot create \(url.path)"])
    }
    CGImageDestinationAddImage(dest, image, nil)
    guard CGImageDestinationFinalize(dest) else {
        throw NSError(domain: "render-icon", code: 2, userInfo: [NSLocalizedDescriptionKey: "cannot write \(url.path)"])
    }
}

let args = CommandLine.arguments
guard args.count == 3, let draft = Int(args[1]), (1...3).contains(draft) else {
    FileHandle.standardError.write(Data("usage: render-icon <draft 1|2|3> <output>.iconset\n".utf8))
    exit(64)
}
let out = URL(fileURLWithPath: args[2], isDirectory: true)
try FileManager.default.createDirectory(at: out, withIntermediateDirectories: true)
for size in [16, 32, 128, 256, 512] {
    for scale in [1, 2] {
        let name = scale == 1 ? "icon_\(size)x\(size).png" : "icon_\(size)x\(size)@2x.png"
        try writePNG(render(draft: draft, pixels: size * scale), to: out.appendingPathComponent(name))
    }
}
