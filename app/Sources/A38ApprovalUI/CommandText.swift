// A text for a command (D65): monospaced, wrapped, never cut and never
// hyphenated. SwiftUI's Text hyphenates a long word ("auf-trag"); in a
// command a hyphen that is not in the argument changes what the human
// reads. An AppKit label with hyphenation off breaks a long word at a
// character and adds nothing.
import AppKit
import SwiftUI

struct CommandText: NSViewRepresentable {
    let text: String

    func makeNSView(context: Context) -> NSTextField {
        let field = NSTextField(wrappingLabelWithString: "")
        field.isSelectable = false
        field.refusesFirstResponder = true
        field.drawsBackground = false
        field.isBordered = false
        field.maximumNumberOfLines = 0
        field.cell?.wraps = true
        field.cell?.lineBreakMode = .byWordWrapping
        field.setContentCompressionResistancePriority(.defaultLow, for: .horizontal)
        return field
    }

    func updateNSView(_ field: NSTextField, context: Context) {
        let style = NSMutableParagraphStyle()
        style.lineBreakMode = .byWordWrapping
        style.hyphenationFactor = 0
        field.attributedStringValue = NSAttributedString(string: text, attributes: [
            .font: NSFont.monospacedSystemFont(ofSize: NSFont.systemFontSize, weight: .regular),
            .foregroundColor: NSColor.labelColor,
            .paragraphStyle: style,
        ])
    }

    func sizeThatFits(_ proposal: ProposedViewSize, nsView field: NSTextField, context: Context) -> CGSize? {
        let width = proposal.width ?? 400
        field.preferredMaxLayoutWidth = width
        let size = field.cell?.cellSize(forBounds: NSRect(x: 0, y: 0, width: width, height: .greatestFiniteMagnitude))
            ?? field.fittingSize
        return CGSize(width: width, height: ceil(size.height))
    }
}
