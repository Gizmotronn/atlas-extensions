import XCTest
@testable import FieldworkKit

final class ModelsTests: XCTestCase {
    func testScanRequestEncodesInputModeAndOmitsNilFields() throws {
        let request = ScanRequest(
            latitude: 51.5,
            longitude: -0.12,
            inputMode: .live,
            heading: 42,
            pitch: 51.5,
            deviceModel: FieldworkDeviceModel.iphone16Pro.rawValue
        )

        let data = try JSONEncoder().encode(request)
        let json = try JSONSerialization.jsonObject(with: data) as? [String: Any]

        XCTAssertEqual(json?["inputMode"] as? String, "live")
        XCTAssertEqual(json?["deviceModel"] as? String, "iphone_16_pro")
        XCTAssertNil(json?["timestamp"])
    }

    func testScanResultDecodesFromContractShape() throws {
        let json = """
        {
          "scanId": "abc123",
          "objects": [
            {
              "key": "polaris",
              "name": "Polaris",
              "type": "star",
              "context": "The North Star.",
              "altitudeDeg": 51.4,
              "azimuthDeg": 359.8,
              "separationDeg": 2.1,
              "confidence": 0.86
            }
          ],
          "events": [],
          "moonPhase": 0.42,
          "advice": {
            "value": "iphone_16_pro",
            "label": "iPhone 16 Pro",
            "instructions": ["Lock focus on a bright star."]
          }
        }
        """.data(using: .utf8)!

        let decoder = JSONDecoder()
        decoder.dateDecodingStrategy = .iso8601
        let result = try decoder.decode(ScanResult.self, from: json)

        XCTAssertEqual(result.scanId, "abc123")
        XCTAssertEqual(result.objects.first?.key, "polaris")
        XCTAssertEqual(result.objects.first?.type, .star)
        XCTAssertEqual(result.advice.value, "iphone_16_pro")
    }

    func testFieldworkDeviceModelMatchesContractEnum() {
        let expected: Set<String> = [
            "nothing_phone_1", "nothing_phone_2", "nothing_phone_2a", "nothing_phone_3",
            "iphone_15_pro", "iphone_16_pro", "pixel_8_pro", "pixel_9_pro",
            "galaxy_s24_ultra", "galaxy_s25_ultra", "other",
        ]
        let actual = Set(FieldworkDeviceModel.allCases.map(\.rawValue))
        XCTAssertEqual(actual, expected)
    }
}
