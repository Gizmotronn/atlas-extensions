import SwiftUI

struct RootView: View {
    @EnvironmentObject private var authStore: AuthStore

    var body: some View {
        Group {
            if authStore.isSignedIn {
                TodayView()
            } else {
                SignInView()
            }
        }
        .tint(Theme.accent)
        .background(Theme.background.ignoresSafeArea())
    }
}
