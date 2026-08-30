import Foundation

/// Exchanges a Clerk session JWT for a PocketBase user token, exactly the
/// flow Atlas's web client uses (POST {PB_URL}/auth/clerk-exchange, see
/// backend/clerk_exchange.go). Fieldwork authenticates against the same
/// shared PocketBase instance so scans can eventually link into a user's
/// Atlas journal/observations.
struct PocketBaseAuthClient {
    struct AuthResponse: Decodable {
        struct Record: Decodable {
            let id: String
            let email: String?
        }
        let token: String
        let record: Record
    }

    enum AuthError: Error {
        case invalidResponse
        case server(status: Int, message: String)
    }

    let baseURL: URL
    var session: URLSession = .shared

    func exchangeClerkSession(token clerkJWT: String) async throws -> AuthResponse {
        var request = URLRequest(url: baseURL.appendingPathComponent("auth/clerk-exchange"))
        request.httpMethod = "POST"
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        request.httpBody = try JSONEncoder().encode(["token": clerkJWT])

        let (data, response) = try await session.data(for: request)
        guard let http = response as? HTTPURLResponse else {
            throw AuthError.invalidResponse
        }
        guard (200..<300).contains(http.statusCode) else {
            throw AuthError.server(status: http.statusCode, message: String(data: data, encoding: .utf8) ?? "")
        }
        return try JSONDecoder().decode(AuthResponse.self, from: data)
    }
}
