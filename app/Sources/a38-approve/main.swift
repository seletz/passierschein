// The approval app of a38 (F11). The daemon starts it with one message on
// the standard input. See docs/reference/90-freigabe-app.md.
import A38ApprovalUI

MainActor.assumeIsolated {
    AppMain.run()
}
