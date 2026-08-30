#if os(iOS)
import CoreLocation
import CoreMotion
import Foundation

/// Wraps CoreLocation + CoreMotion into the (lat, lon, heading, pitch)
/// tuple a live scan needs. Kept inside FieldworkKit (not the app target)
/// so any future Fieldwork-consuming app gets the same sensor plumbing.
@MainActor
public final class ScanSensors: NSObject, ObservableObject, CLLocationManagerDelegate {
    @Published public private(set) var coordinate: CLLocationCoordinate2D?
    @Published public private(set) var headingDeg: Double?
    @Published public private(set) var pitchDeg: Double?
    @Published public private(set) var authorizationStatus: CLAuthorizationStatus

    private let locationManager = CLLocationManager()
    private let motionManager = CMMotionManager()

    public override init() {
        authorizationStatus = locationManager.authorizationStatus
        super.init()
        locationManager.delegate = self
        locationManager.desiredAccuracy = kCLLocationAccuracyBest
        locationManager.headingFilter = 1
    }

    public func requestPermission() {
        locationManager.requestWhenInUseAuthorization()
    }

    public func start() {
        locationManager.startUpdatingLocation()
        locationManager.startUpdatingHeading()

        guard motionManager.isDeviceMotionAvailable else { return }
        motionManager.deviceMotionUpdateInterval = 0.2
        motionManager.startDeviceMotionUpdates(to: .main) { [weak self] motion, _ in
            guard let self, let motion else { return }
            // Pitch above the horizon: attitude.pitch is 0 when the device
            // is flat, so a phone held vertically and tilted back to point
            // at the sky reads as roughly (90 - |pitch|) degrees of
            // elevation for this "point the top of the phone at the sky"
            // gesture.
            let elevation = 90 - abs(motion.attitude.pitch * 180 / .pi)
            self.pitchDeg = elevation
        }
    }

    public func stop() {
        locationManager.stopUpdatingLocation()
        locationManager.stopUpdatingHeading()
        motionManager.stopDeviceMotionUpdates()
    }

    public func locationManagerDidChangeAuthorization(_ manager: CLLocationManager) {
        authorizationStatus = manager.authorizationStatus
    }

    public func locationManager(_ manager: CLLocationManager, didUpdateLocations locations: [CLLocation]) {
        coordinate = locations.last?.coordinate
    }

    public func locationManager(_ manager: CLLocationManager, didUpdateHeading newHeading: CLHeading) {
        headingDeg = newHeading.trueHeading >= 0 ? newHeading.trueHeading : newHeading.magneticHeading
    }
}
#endif
