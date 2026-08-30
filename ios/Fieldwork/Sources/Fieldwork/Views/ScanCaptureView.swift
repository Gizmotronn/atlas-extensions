import FieldworkKit
import SwiftUI

struct ScanCaptureView: View {
    @EnvironmentObject private var authStore: AuthStore
    @EnvironmentObject private var scanStore: ScanStore
    @Environment(\.dismiss) private var dismiss

    @StateObject private var sensors = ScanSensors()

    var body: some View {
        NavigationStack {
            content
                .navigationTitle("Scan")
                .toolbar {
                    ToolbarItem(placement: .cancellationAction) {
                        Button("Close") { dismiss() }
                    }
                }
                .onAppear {
                    sensors.requestPermission()
                    sensors.start()
                }
                .onDisappear {
                    sensors.stop()
                }
        }
    }

    @ViewBuilder
    private var content: some View {
        switch scanStore.state {
        case .idle:
            captureForm
        case .scanning:
            ProgressView("Identifying...")
                .frame(maxWidth: .infinity, maxHeight: .infinity)
        case .result(let result):
            ScanResultView(result: result) {
                scanStore.reset()
            }
        case .failed(let message):
            VStack(spacing: Theme.Space.s3) {
                Text("Scan failed")
                    .font(.headline)
                Text(message)
                    .font(.caption)
                    .foregroundStyle(Theme.textMuted)
                Button("Try again") { scanStore.reset() }
            }
            .padding(Theme.Space.s4)
        }
    }

    private var captureForm: some View {
        VStack(spacing: Theme.Space.s4) {
            VStack(alignment: .leading, spacing: Theme.Space.s2) {
                labeledRow(label: "Location", value: sensors.coordinate.map {
                    String(format: "%.3f, %.3f", $0.latitude, $0.longitude)
                } ?? "Waiting for GPS...")
                labeledRow(label: "Heading", value: sensors.headingDeg.map { String(format: "%.0f°", $0) } ?? "Waiting for compass...")
                labeledRow(label: "Pitch", value: sensors.pitchDeg.map { String(format: "%.0f°", $0) } ?? "Waiting for motion sensors...")
            }
            .padding(Theme.Space.s4)
            .background(Theme.surface, in: RoundedRectangle(cornerRadius: Theme.Radius.md))

            Picker("Device", selection: $scanStore.selectedDevice) {
                ForEach(FieldworkDeviceModel.allCases) { device in
                    Text(device.label).tag(device)
                }
            }
            .pickerStyle(.menu)

            Button {
                guard let coordinate = sensors.coordinate,
                      let heading = sensors.headingDeg,
                      let pitch = sensors.pitchDeg,
                      let token = authStore.pbToken else { return }
                Task {
                    await scanStore.submitLiveScan(
                        latitude: coordinate.latitude,
                        longitude: coordinate.longitude,
                        heading: heading,
                        pitch: pitch,
                        userToken: token
                    )
                }
            } label: {
                Text("Identify what I'm pointing at")
                    .frame(maxWidth: .infinity)
            }
            .buttonStyle(.borderedProminent)
            .tint(Theme.accent)
            .disabled(sensors.coordinate == nil || sensors.headingDeg == nil || sensors.pitchDeg == nil)

            Spacer()
        }
        .padding(Theme.Space.s4)
    }

    private func labeledRow(label: String, value: String) -> some View {
        HStack {
            Text(label)
                .foregroundStyle(Theme.textMuted)
            Spacer()
            Text(value)
                .foregroundStyle(Theme.text)
        }
        .font(.subheadline)
    }
}
