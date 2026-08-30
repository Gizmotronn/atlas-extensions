import Foundation

/// Same enum as PocketBase's `users.device_models` select field
/// (backend/migrations/33_atlas_device_models.go) and
/// fieldwork/go/advice/presets.json. Keep all three in sync.
public enum FieldworkDeviceModel: String, CaseIterable, Codable, Sendable, Identifiable {
    case nothingPhone1 = "nothing_phone_1"
    case nothingPhone2 = "nothing_phone_2"
    case nothingPhone2a = "nothing_phone_2a"
    case nothingPhone3 = "nothing_phone_3"
    case iphone15Pro = "iphone_15_pro"
    case iphone16Pro = "iphone_16_pro"
    case pixel8Pro = "pixel_8_pro"
    case pixel9Pro = "pixel_9_pro"
    case galaxyS24Ultra = "galaxy_s24_ultra"
    case galaxyS25Ultra = "galaxy_s25_ultra"
    case other

    public var id: String { rawValue }

    public var label: String {
        switch self {
        case .nothingPhone1: return "Nothing Phone (1)"
        case .nothingPhone2: return "Nothing Phone (2)"
        case .nothingPhone2a: return "Nothing Phone (2a)"
        case .nothingPhone3: return "Nothing Phone (3)"
        case .iphone15Pro: return "iPhone 15 Pro"
        case .iphone16Pro: return "iPhone 16 Pro"
        case .pixel8Pro: return "Google Pixel 8 Pro"
        case .pixel9Pro: return "Google Pixel 9 Pro"
        case .galaxyS24Ultra: return "Samsung Galaxy S24 Ultra"
        case .galaxyS25Ultra: return "Samsung Galaxy S25 Ultra"
        case .other: return "Other / not listed"
        }
    }
}
