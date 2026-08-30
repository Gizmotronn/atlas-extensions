import SwiftUI

struct SignInView: View {
    @EnvironmentObject private var authStore: AuthStore
    @State private var clerkSessionJWT = ""
    @State private var isSigningIn = false

    var body: some View {
        VStack(spacing: Theme.Space.s5) {
            Spacer()

            VStack(spacing: Theme.Space.s2) {
                Text("Fieldwork")
                    .font(.system(size: Theme.Space.s7, weight: .semibold, design: .serif))
                    .foregroundStyle(Theme.text)
                Text("Point your phone at the sky and see what you're looking at.")
                    .font(.subheadline)
                    .foregroundStyle(Theme.textMuted)
                    .multilineTextAlignment(.center)
            }
            .padding(.horizontal, Theme.Space.s5)

            VStack(alignment: .leading, spacing: Theme.Space.s2) {
                Text("Sign in with your Atlas account")
                    .font(.caption)
                    .foregroundStyle(Theme.textMuted)

                // TODO: replace with the real ClerkSwift SDK hosted sign-in
                // flow; this field is a development shortcut so the
                // exchange -> scan pipeline can be exercised end-to-end
                // before that SDK integration lands.
                SecureField("Clerk session token (dev only)", text: $clerkSessionJWT)
                    .textFieldStyle(.roundedBorder)

                if let error = authStore.lastError {
                    Text(error)
                        .font(.caption)
                        .foregroundStyle(Theme.bad)
                }

                Button {
                    Task {
                        isSigningIn = true
                        await authStore.completeSignIn(clerkSessionJWT: clerkSessionJWT)
                        isSigningIn = false
                    }
                } label: {
                    if isSigningIn {
                        ProgressView()
                    } else {
                        Text("Sign in")
                            .frame(maxWidth: .infinity)
                    }
                }
                .buttonStyle(.borderedProminent)
                .tint(Theme.accent)
                .disabled(clerkSessionJWT.isEmpty || isSigningIn)
            }
            .padding(Theme.Space.s4)
            .background(Theme.surface, in: RoundedRectangle(cornerRadius: Theme.Radius.lg))
            .padding(.horizontal, Theme.Space.s5)

            Spacer()
        }
    }
}
