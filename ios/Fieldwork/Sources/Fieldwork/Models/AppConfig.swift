import Foundation

/// Fieldwork talks to two backends: the shared Atlas/Star-Sailors
/// PocketBase (for Clerk sign-in) and the standalone Fieldwork service (for
/// scans). Both are overridable at build time via Info.plist/xcconfig so a
/// debug build can point at a local `fieldworkd` + local PocketBase.
enum AppConfig {
    static let pocketBaseURL = URL(string: "https://signal-k-starsailors.fly.dev")!
    static let fieldworkServiceURL = URL(string: "https://fieldwork.fly.dev")!
}
