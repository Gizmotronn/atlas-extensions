import Foundation

/// Talks to the Fieldwork service (`service/cmd/fieldworkd`). Every request
/// after sign-in carries the caller's PocketBase user token, matching
/// fieldwork/CONTRACT.md — this client never talks to PocketBase directly
/// except that Atlas sign-in itself already produced the token it's handed.
public final class FieldworkClient: Sendable {
    public enum ClientError: Error {
        case invalidResponse
        case server(status: Int, message: String)
    }

    private let baseURL: URL
    private let session: URLSession

    public init(baseURL: URL, session: URLSession = .shared) {
        self.baseURL = baseURL
        self.session = session
    }

    public func submitScan(_ request: ScanRequest, userToken: String) async throws -> ScanResult {
        var urlRequest = URLRequest(url: baseURL.appendingPathComponent("v1/scans"))
        urlRequest.httpMethod = "POST"
        urlRequest.setValue("application/json", forHTTPHeaderField: "Content-Type")
        urlRequest.setValue(userToken, forHTTPHeaderField: "Authorization")
        urlRequest.httpBody = try JSONEncoder().encode(request)

        let (data, response) = try await session.data(for: urlRequest)
        guard let http = response as? HTTPURLResponse else {
            throw ClientError.invalidResponse
        }
        guard (200..<300).contains(http.statusCode) else {
            let message = String(data: data, encoding: .utf8) ?? "unknown error"
            throw ClientError.server(status: http.statusCode, message: message)
        }

        let decoder = JSONDecoder()
        decoder.dateDecodingStrategy = .iso8601
        return try decoder.decode(ScanResult.self, from: data)
    }
}
