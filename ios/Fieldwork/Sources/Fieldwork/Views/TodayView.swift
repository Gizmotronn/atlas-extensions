import SwiftUI

/// The minimal home screen: today's Moon phase (reusing the same
/// identify.MoonPhase math the scan itself uses, via the last scan result
/// when available) plus the entry point into a scan. This intentionally
/// does not attempt Atlas's full Today hub (weather, watchlist, streaks) —
/// see the plan's scope note: this app is a vertical slice, not Atlas
/// parity.
struct TodayView: View {
    @EnvironmentObject private var authStore: AuthStore
    @State private var showScan = false

    var body: some View {
        NavigationStack {
            VStack(spacing: Theme.Space.s5) {
                VStack(alignment: .leading, spacing: Theme.Space.s2) {
                    Text("Tonight")
                        .font(.title2.weight(.semibold))
                        .foregroundStyle(Theme.text)
                    if let email = authStore.userEmail {
                        Text("Signed in as \(email)")
                            .font(.caption)
                            .foregroundStyle(Theme.textMuted)
                    }
                }
                .frame(maxWidth: .infinity, alignment: .leading)

                Button {
                    showScan = true
                } label: {
                    VStack(spacing: Theme.Space.s2) {
                        Image(systemName: "sparkles")
                            .font(.system(size: 32))
                        Text("Scan the sky")
                            .font(.headline)
                        Text("Point your phone up, or scan a photo you've already taken.")
                            .font(.caption)
                            .multilineTextAlignment(.center)
                    }
                    .foregroundStyle(Theme.accentText)
                    .frame(maxWidth: .infinity)
                    .padding(Theme.Space.s6)
                    .background(Theme.accent, in: RoundedRectangle(cornerRadius: Theme.Radius.lg))
                }

                Spacer()

                Button("Sign out", role: .destructive) {
                    authStore.signOut()
                }
                .font(.caption)
            }
            .padding(Theme.Space.s4)
            .background(Theme.background.ignoresSafeArea())
            .navigationTitle("Fieldwork")
            .sheet(isPresented: $showScan) {
                ScanCaptureView()
            }
        }
    }
}
