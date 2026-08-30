import SwiftUI
import UIKit

/// Ported 1:1 from atlas-design/src/styles/tokens.css so Fieldwork matches
/// Atlas's look. Update both together if Atlas's palette changes.
enum Theme {
    // MARK: Colors (light/dark pairs match tokens.css's :root / [data-theme='dark'])

    static let background = Color(light: 0xFFFFFF, dark: 0x0B0C10)
    static let surface = Color(light: 0xF7F7F8, dark: 0x16171D)
    static let surfaceRaised = Color(light: 0xFFFFFF, dark: 0x1D1E26)
    static let border = Color(light: 0xE4E4E7, dark: 0x2C2D36)
    static let text = Color(light: 0x1A1A1F, dark: 0xF1F1F3)
    static let textMuted = Color(light: 0x6B6B76, dark: 0x9A9AA4)
    static let accent = Color(light: 0x7C3AED, dark: 0xA78BFA)
    static let accentText = Color(light: 0xFFFFFF, dark: 0x17121F)
    static let good = Color(light: 0x1A8F5E, dark: 0x3DDC97)
    static let warn = Color(light: 0xB8790A, dark: 0xF0B429)
    static let bad = Color(light: 0xC23B3B, dark: 0xF0685B)

    // MARK: Spacing (tokens.css --space-*)

    enum Space {
        static let s1: CGFloat = 4
        static let s2: CGFloat = 8
        static let s3: CGFloat = 12
        static let s4: CGFloat = 16
        static let s5: CGFloat = 24
        static let s6: CGFloat = 32
        static let s7: CGFloat = 48
    }

    // MARK: Radius (tokens.css --radius-*)

    enum Radius {
        static let sm: CGFloat = 8
        static let md: CGFloat = 12
        static let lg: CGFloat = 20
        static let pill: CGFloat = 999
    }
}

private extension Color {
    /// Builds a dynamic Color from two hex values, matching how tokens.css
    /// swaps its palette between :root and :root[data-theme='dark'].
    init(light: UInt32, dark: UInt32) {
        self.init(uiColor: UIColor { traits in
            traits.userInterfaceStyle == .dark ? UIColor(hex: dark) : UIColor(hex: light)
        })
    }
}

private extension UIColor {
    convenience init(hex: UInt32) {
        let r = CGFloat((hex >> 16) & 0xFF) / 255
        let g = CGFloat((hex >> 8) & 0xFF) / 255
        let b = CGFloat(hex & 0xFF) / 255
        self.init(red: r, green: g, blue: b, alpha: 1)
    }
}
