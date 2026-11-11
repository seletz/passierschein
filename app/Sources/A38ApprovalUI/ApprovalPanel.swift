// The floating panel of the popup and of the notice (D45). The attributes
// are those of the accepted prototype (spike report "Fensterverhalten unter
// OmniWM"); do not change them without a new check under OmniWM.
import AppKit

final class ApprovalPanel: NSPanel {
    /// Esc. The panel must be the key window, so the human clicked into it
    /// before.
    var onCancel: () -> Void = {}

    init(width: CGFloat) {
        // No close button: Deny (or Close for a notice) is the only way to
        // close the panel by hand. ".nonactivatingPanel" keeps the app of
        // the human active.
        super.init(contentRect: NSRect(x: 0, y: 0, width: width, height: 400),
                   styleMask: [.titled, .fullSizeContentView, .nonactivatingPanel],
                   backing: .buffered, defer: false)
        titleVisibility = .hidden
        titlebarAppearsTransparent = true
        isFloatingPanel = true
        level = .floating
        hidesOnDeactivate = false
        collectionBehavior = [.canJoinAllSpaces, .fullScreenAuxiliary]
        isMovableByWindowBackground = true
        isReleasedWhenClosed = false
        // The panel becomes the key window only after a click.
        becomesKeyOnlyIfNeeded = true
        standardWindowButton(.miniaturizeButton)?.isHidden = true
        standardWindowButton(.zoomButton)?.isHidden = true
    }

    override var canBecomeKey: Bool { true }
    override var canBecomeMain: Bool { false }

    /// Esc denies the request or closes the notice.
    override func cancelOperation(_ sender: Any?) {
        onCancel()
    }

    /// Return, Enter and Space have no function in the panel (D31). The
    /// controls are not focusable; this is a second barrier.
    override func keyDown(with event: NSEvent) {
        if Self.blockedKeys.contains(event.keyCode) { return }
        super.keyDown(with: event)
    }

    override func performKeyEquivalent(with event: NSEvent) -> Bool {
        if Self.blockedKeys.contains(event.keyCode) { return true }
        return super.performKeyEquivalent(with: event)
    }

    /// Return (36), Enter on the keypad (76), Space (49).
    private static let blockedKeys: Set<UInt16> = [36, 76, 49]

    /// Puts the panel in the upper part of the screen with the mouse
    /// pointer.
    func place() {
        let mouse = NSEvent.mouseLocation
        let screen = NSScreen.screens.first { $0.frame.contains(mouse) } ?? NSScreen.main
        guard let visible = screen?.visibleFrame else { return }
        let size = frame.size
        let x = visible.midX - size.width / 2
        let y = visible.maxY - size.height - visible.height * 0.12
        setFrameOrigin(NSPoint(x: x, y: max(visible.minY, y)))
    }

    /// Writes the window attributes to the standard error (test switch
    /// --debug-window, for the manual check under OmniWM).
    func debugDescriptionLine() -> String {
        let info: [String: Any] = [
            "window_number": windowNumber,
            "level": level.rawValue,
            "ax_role": accessibilityRole()?.rawValue ?? "",
            "ax_subrole": accessibilitySubrole()?.rawValue ?? "",
            "is_key": isKeyWindow,
            "app_is_active": NSApp.isActive,
            "activation_policy": NSApp.activationPolicy().rawValue,
            "style_mask": styleMask.rawValue,
            "has_close_button": standardWindowButton(.closeButton) != nil,
            "frame": NSStringFromRect(frame),
            "is_visible": isVisible,
            "on_active_space": isOnActiveSpace,
        ]
        guard let data = try? JSONSerialization.data(withJSONObject: info, options: [.sortedKeys]),
              let text = String(data: data, encoding: .utf8) else { return "window-debug: {}" }
        return "window-debug: " + text
    }
}
