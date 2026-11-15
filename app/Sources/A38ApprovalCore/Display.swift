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

    /// One argument of a command as one word of a POSIX shell, without
    /// loss: a plain word stays as it is, another word gets single quotes,
    /// a word with a line break or a tab gets the form $'...' with "\\n"
    /// and "\\t". The human sees where each argument starts and ends (D65). The
    /// request has no other control character (Decoding).
    public static func shellWord(_ arg: String) -> String {
        if arg.isEmpty { return "''" }
        if arg.unicodeScalars.allSatisfy(isPlainShellCharacter) { return arg }
        if arg.contains("\n") || arg.contains("\t") {
            var out = "$'"
            for c in arg.unicodeScalars {
                switch c {
                case "\n": out += "\\n"
                case "\t": out += "\\t"
                case "\\": out += "\\\\"
                case "'": out += "\\'"
                default: out.unicodeScalars.append(c)
                }
            }
            return out + "'"
        }
        return "'" + arg.replacingOccurrences(of: "'", with: "'\\''") + "'"
    }

    private static func isPlainShellCharacter(_ c: Unicode.Scalar) -> Bool {
        ("a"..."z").contains(c) || ("A"..."Z").contains(c) || ("0"..."9").contains(c)
            || "_@%+=:,./-".unicodeScalars.contains(c)
    }

    /// The arguments of a command as shell words, separated by a space.
    public static func shellWords(_ args: [String]) -> String {
        args.map(shellWord).joined(separator: " ")
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

/// The lifetimes that the human can select (D32, D63). The set is fixed:
/// the proposal, half of the proposal (rounded down to minutes), 15 minutes
/// and 5 minutes. Each value other than the proposal is at least five
/// minutes and shorter than the proposal; the proposal is always the first
/// value. The daemon uses the same rule for a command
/// (approval.ExecLifetimeChoices).
public func lifetimeChoices(proposal: Int64) -> [Int64] {
    var values: [Int64] = [proposal]
    let half = (proposal / 2) / 60 * 60
    for candidate in [half, Int64(15 * 60), Int64(5 * 60)] where candidate >= 5 * 60 && candidate < proposal {
        values.append(candidate)
    }
    return Array(Set(values)).sorted(by: >)
}
