import SwiftUI

@main
struct FieldworkApp: App {
    @StateObject private var authStore = AuthStore()
    @StateObject private var scanStore = ScanStore()

    var body: some Scene {
        WindowGroup {
            RootView()
                .environmentObject(authStore)
                .environmentObject(scanStore)
        }
    }
}
