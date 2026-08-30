import Foundation

/// Mirrors fieldwork/CONTRACT.md — the wire contract with the Fieldwork
/// service. Keep this in sync with that document and with
/// fieldwork/go/identify, fieldwork/go/advice, fieldwork/go/events.

public enum FieldworkInputMode: String, Codable, Sendable {
    case live
    case photo
}

public struct ScanRequest: Codable, Sendable {
    public var latitude: Double
    public var longitude: Double
    public var timestamp: Date?
    public var inputMode: FieldworkInputMode
    public var heading: Double?
    public var pitch: Double?
    public var deviceModel: String?

    public init(
        latitude: Double,
        longitude: Double,
        timestamp: Date? = nil,
        inputMode: FieldworkInputMode,
        heading: Double? = nil,
        pitch: Double? = nil,
        deviceModel: String? = nil
    ) {
        self.latitude = latitude
        self.longitude = longitude
        self.timestamp = timestamp
        self.inputMode = inputMode
        self.heading = heading
        self.pitch = pitch
        self.deviceModel = deviceModel
    }

    enum CodingKeys: String, CodingKey {
        case latitude, longitude, timestamp, inputMode, heading, pitch, deviceModel
    }

    public func encode(to encoder: Encoder) throws {
        var container = encoder.container(keyedBy: CodingKeys.self)
        try container.encode(latitude, forKey: .latitude)
        try container.encode(longitude, forKey: .longitude)
        if let timestamp {
            let formatter = ISO8601DateFormatter()
            try container.encode(formatter.string(from: timestamp), forKey: .timestamp)
        }
        try container.encode(inputMode, forKey: .inputMode)
        try container.encodeIfPresent(heading, forKey: .heading)
        try container.encodeIfPresent(pitch, forKey: .pitch)
        try container.encodeIfPresent(deviceModel, forKey: .deviceModel)
    }
}

public enum FieldworkObjectType: String, Codable, Sendable {
    case star, planet, moon, deepSky = "deep_sky"
}

public struct IdentifiedObject: Codable, Identifiable, Sendable {
    public var key: String
    public var name: String
    public var type: FieldworkObjectType
    public var context: String
    public var altitudeDeg: Double
    public var azimuthDeg: Double
    public var separationDeg: Double?
    public var confidence: Double

    public var id: String { key }
}

public struct SkyEvent: Codable, Identifiable, Sendable {
    public var id: String
    public var kind: String
    public var target: String
    public var title: String
    public var description: String
    public var startsAt: Date
    public var endsAt: Date

    enum CodingKeys: String, CodingKey {
        case id, kind, target, title, description
        case startsAt = "starts_at"
        case endsAt = "ends_at"
    }
}

public struct DeviceAdvice: Codable, Sendable {
    public var value: String
    public var label: String
    public var instructions: [String]
}

public struct ScanResult: Codable, Sendable {
    public var scanId: String
    public var objects: [IdentifiedObject]
    public var events: [SkyEvent]
    public var moonPhase: Double
    public var advice: DeviceAdvice
}
