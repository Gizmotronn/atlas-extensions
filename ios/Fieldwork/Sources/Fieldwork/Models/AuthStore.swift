import Foundation

/// Holds the signed-in user's PocketBase token.
///
/// This scaffold does not yet embed the real Clerk iOS SDK — that's a
/// follow-up (add `ClerkSwift` via SPM, present its hosted sign-in UI, then
/// call `completeSignIn` with the resulting session JWT). For now,
/// `SignInView` accepts a Clerk session JWT directly so the rest of the
/// exchange -> scan flow can be built and tested end-to-end against a real
/// PocketBase instance.
@MainActor
final class AuthStore: ObservableObject {
    @Published private(set) var pbToken: String?
    @Published private(set) var userEmail: String?
    @Published var lastError: String?

    private let authClient = PocketBaseAuthClient(baseURL: AppConfig.pocketBaseURL)
    private let tokenStorageKey = "fieldwork.pbToken"
    private let emailStorageKey = "fieldwork.userEmail"

    init() {
        pbToken = UserDefaults.standard.string(forKey: tokenStorageKey)
        userEmail = UserDefaults.standard.string(forKey: emailStorageKey)
    }

    var isSignedIn: Bool { pbToken != nil }

    func completeSignIn(clerkSessionJWT: String) async {
        do {
            let auth = try await authClient.exchangeClerkSession(token: clerkSessionJWT)
            pbToken = auth.token
            userEmail = auth.record.email
            UserDefaults.standard.set(auth.token, forKey: tokenStorageKey)
            UserDefaults.standard.set(auth.record.email, forKey: emailStorageKey)
            lastError = nil
        } catch {
            lastError = "Sign-in failed: \(error.localizedDescription)"
        }
    }

    func signOut() {
        pbToken = nil
        userEmail = nil
        UserDefaults.standard.removeObject(forKey: tokenStorageKey)
        UserDefaults.standard.removeObject(forKey: emailStorageKey)
    }
}
