import FieldworkKit
import Foundation

@MainActor
final class ScanStore: ObservableObject {
    enum State: Equatable {
        case idle
        case scanning
        case result(ScanResult)
        case failed(String)

        static func == (lhs: State, rhs: State) -> Bool {
            switch (lhs, rhs) {
            case (.idle, .idle), (.scanning, .scanning): return true
            case let (.result(a), .result(b)): return a.scanId == b.scanId
            case let (.failed(a), .failed(b)): return a == b
            default: return false
            }
        }
    }

    @Published private(set) var state: State = .idle
    @Published var selectedDevice: FieldworkDeviceModel = .other

    private let client = FieldworkClient(baseURL: AppConfig.fieldworkServiceURL)

    func submitLiveScan(latitude: Double, longitude: Double, heading: Double, pitch: Double, userToken: String) async {
        state = .scanning
        let request = ScanRequest(
            latitude: latitude,
            longitude: longitude,
            timestamp: Date(),
            inputMode: .live,
            heading: heading,
            pitch: pitch,
            deviceModel: selectedDevice.rawValue
        )
        await submit(request, userToken: userToken)
    }

    func submitPhotoScan(latitude: Double, longitude: Double, capturedAt: Date, userToken: String) async {
        state = .scanning
        let request = ScanRequest(
            latitude: latitude,
            longitude: longitude,
            timestamp: capturedAt,
            inputMode: .photo,
            deviceModel: selectedDevice.rawValue
        )
        await submit(request, userToken: userToken)
    }

    private func submit(_ request: ScanRequest, userToken: String) async {
        do {
            let result = try await client.submitScan(request, userToken: userToken)
            state = .result(result)
        } catch {
            state = .failed(error.localizedDescription)
        }
    }

    func reset() {
        state = .idle
    }
}
