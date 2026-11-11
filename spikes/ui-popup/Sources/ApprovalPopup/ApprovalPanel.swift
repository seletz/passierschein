// Floating panel for the popup.
import AppKit
import SwiftUI

struct PanelOptions {
    /// Add a close button. Use this flag only to test the window manager.
    var closable = false
    /// Make the panel the key window when it opens.
    var takeFocus = false
}

final class ApprovalPanel: NSPanel {
    var onCancel: () -> Void = {}

    init(options: PanelOptions) {
        // The panel has no close button. The Deny button is the only way to close it.
        // ".nonactivatingPanel" keeps the app of the user active.
        var style: NSWindow.StyleMask = [.titled, .fullSizeContentView, .nonactivatingPanel]
        if options.closable { style.insert(.closable) }
        super.init(contentRect: NSRect(x: 0, y: 0, width: 540, height: 400),
                   styleMask: style, backing: .buffered, defer: false)
        title = "a38 approval"
        titleVisibility = .hidden
        titlebarAppearsTransparent = true
        isFloatingPanel = true
        level = .floating
        hidesOnDeactivate = false
        collectionBehavior = [.canJoinAllSpaces, .fullScreenAuxiliary]
        isMovableByWindowBackground = true
        isReleasedWhenClosed = false
        // The panel becomes the key window only after a click.
        becomesKeyOnlyIfNeeded = !options.takeFocus
        standardWindowButton(.miniaturizeButton)?.isHidden = true
        standardWindowButton(.zoomButton)?.isHidden = true
    }

    override var canBecomeKey: Bool { true }
    override var canBecomeMain: Bool { false }

    /// The Escape key denies the request. The panel must be the key window.
    override func cancelOperation(_ sender: Any?) {
        onCancel()
    }

    /// Puts the panel in the top part of the screen that contains the mouse pointer.
    func place() {
        let mouse = NSEvent.mouseLocation
        let screen = NSScreen.screens.first { $0.frame.contains(mouse) } ?? NSScreen.main
        guard let visible = screen?.visibleFrame else { return }
        let size = frame.size
        let x = visible.midX - size.width / 2
        let y = visible.maxY - size.height - visible.height * 0.12
        setFrameOrigin(NSPoint(x: x, y: max(visible.minY, y)))
    }
}

/// Writes the window attributes to stderr. This output is for the spike only.
func printWindowDebug(_ panel: NSPanel) {
    var info: [String: Any] = [
        "window_number": panel.windowNumber,
        "level": panel.level.rawValue,
        "ax_role": panel.accessibilityRole()?.rawValue ?? "",
        "ax_subrole": panel.accessibilitySubrole()?.rawValue ?? "",
        "is_key": panel.isKeyWindow,
        "app_is_active": NSApp.isActive,
        "activation_policy": NSApp.activationPolicy().rawValue,
        "style_mask": panel.styleMask.rawValue,
        "has_close_button": panel.standardWindowButton(.closeButton) != nil,
        "frame": NSStringFromRect(panel.frame),
        "is_visible": panel.isVisible,
        "on_active_space": panel.isOnActiveSpace,
    ]
    info["auth"] = [
        authAvailability(policy: .biometrics),
        authAvailability(policy: .biometricsOrCompanion),
        authAvailability(policy: .owner),
    ]
    if let data = try? JSONSerialization.data(withJSONObject: info, options: [.sortedKeys]),
       let text = String(data: data, encoding: .utf8) {
        FileHandle.standardError.write(Data(("window-debug: " + text + "\n").utf8))
    }
}
