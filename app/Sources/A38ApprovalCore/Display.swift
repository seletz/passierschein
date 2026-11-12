// Helpers for the text that the views show.
import Foundation

public enum DisplayText {
    /// Makes a text of the request safe to show. A line break or a tab
    /// becomes a space. A control character or an invisible format
    /// character (for example a bidi override that turns text around)
    /// becomes U+FFFD, so that a claim cannot hide or fake text.
    public static func clean(_ text: String) -> String {
        var out = String.UnicodeScalarView()
        for scalar in text.unicodeScalars {
            switch scalar.properties.generalCategory {
            case .control:
                out.append(scalar == "\n" || scalar == "\r" || scalar == "\t" ? " " : "\u{FFFD}")
            case .format, .lineSeparator, .paragraphSeparator:
                out.append("\u{FFFD}")
            default:
                out.append(scalar)
            }
        }
        return String(out)
    }

    /// A time as local date and time, for example "2026-11-11 21:30".
    public static func time(_ date: Date, timeZone: TimeZone = .current) -> String {
        let formatter = DateFormatter()
        formatter.locale = Locale(identifier: "en_US_POSIX")
        formatter.timeZone = timeZone
        formatter.dateFormat = "yyyy-MM-dd HH:mm"
        return formatter.string(from: date)
    }

    /// A time with seconds, for the notice.
    public static func timeWithSeconds(_ date: Date, timeZone: TimeZone = .current) -> String {
        let formatter = DateFormatter()
        formatter.locale = Locale(identifier: "en_US_POSIX")
        formatter.timeZone = timeZone
        formatter.dateFormat = "yyyy-MM-dd HH:mm:ss"
        return formatter.string(from: date)
    }
}

/// The lifetimes that the human can select (D32). The set is fixed: the
/// proposal, half of the proposal and 15 minutes. Each value is at least
/// one minute and never more than the proposal; the proposal itself is
/// always in the set and is the first value.
public func lifetimeChoices(proposal: Int64) -> [Int64] {
    var values: [Int64] = [proposal]
    let half = (proposal / 2) / 60 * 60
    for candidate in [half, Int64(15 * 60)] where candidate >= 60 && candidate < proposal {
        values.append(candidate)
    }
    return Array(Set(values)).sorted(by: >)
}
