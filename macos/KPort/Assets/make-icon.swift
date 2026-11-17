import AppKit
import Foundation

let output = URL(fileURLWithPath: CommandLine.arguments[1])
let size = 1024
let image = NSImage(size: NSSize(width: size, height: size))
image.lockFocus()
let rect = NSRect(x: 0, y: 0, width: size, height: size)
let bg = NSBezierPath(roundedRect: rect.insetBy(dx: 18, dy: 18), xRadius: 228, yRadius: 228)
let gradient = NSGradient(colors: [NSColor(calibratedRed: 0.09, green: 0.15, blue: 0.24, alpha: 1), NSColor(calibratedRed: 0.04, green: 0.07, blue: 0.11, alpha: 1)])!
gradient.draw(in: bg, angle: 135)

let stroke = NSBezierPath()
stroke.lineWidth = 84
stroke.lineCapStyle = .round
stroke.lineJoinStyle = .round
stroke.move(to: NSPoint(x: 340, y: 236)); stroke.line(to: NSPoint(x: 340, y: 788))
stroke.move(to: NSPoint(x: 340, y: 512)); stroke.line(to: NSPoint(x: 548, y: 704))
stroke.move(to: NSPoint(x: 340, y: 512)); stroke.line(to: NSPoint(x: 548, y: 320))
NSColor(calibratedRed: 0.39, green: 0.90, blue: 0.72, alpha: 1).setStroke()
stroke.stroke()
for y: CGFloat in [354, 512, 670] {
    let line = NSBezierPath(); line.lineWidth = 54; line.lineCapStyle = .round
    line.move(to: NSPoint(x: 680, y: y)); line.line(to: NSPoint(x: 790, y: y))
    NSColor(calibratedWhite: 0.96, alpha: 0.95).setStroke(); line.stroke()
    let dot = NSBezierPath(ovalIn: NSRect(x: 816, y: y - 20, width: 40, height: 40))
    NSColor(calibratedRed: 0.39, green: 0.90, blue: 0.72, alpha: 1).setFill(); dot.fill()
}
image.unlockFocus()
guard let tiff = image.tiffRepresentation, let bitmap = NSBitmapImageRep(data: tiff), let png = bitmap.representation(using: .png, properties: [:]) else { fatalError("Could not render icon") }
try png.write(to: output)
