import FieldworkKit
import SwiftUI

struct ScanResultView: View {
    let result: ScanResult
    let onScanAgain: () -> Void

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: Theme.Space.s5) {
                if let top = result.objects.first {
                    VStack(alignment: .leading, spacing: Theme.Space.s2) {
                        Text(top.name)
                            .font(.title2.weight(.semibold))
                            .foregroundStyle(Theme.text)
                        Text(top.context)
                            .font(.body)
                            .foregroundStyle(Theme.textMuted)
                    }
                }

                if result.objects.count > 1 {
                    sectionHeader("Also nearby")
                    ForEach(result.objects.dropFirst()) { object in
                        VStack(alignment: .leading, spacing: Theme.Space.s1) {
                            Text(object.name).font(.subheadline.weight(.medium))
                            Text(object.context)
                                .font(.caption)
                                .foregroundStyle(Theme.textMuted)
                        }
                        .padding(Theme.Space.s3)
                        .frame(maxWidth: .infinity, alignment: .leading)
                        .background(Theme.surface, in: RoundedRectangle(cornerRadius: Theme.Radius.sm))
                    }
                }

                if !result.events.isEmpty {
                    sectionHeader("Happening now")
                    ForEach(result.events) { event in
                        VStack(alignment: .leading, spacing: Theme.Space.s1) {
                            Text(event.title).font(.subheadline.weight(.medium))
                            Text(event.description)
                                .font(.caption)
                                .foregroundStyle(Theme.textMuted)
                        }
                        .padding(Theme.Space.s3)
                        .frame(maxWidth: .infinity, alignment: .leading)
                        .background(Theme.accent.opacity(0.12), in: RoundedRectangle(cornerRadius: Theme.Radius.sm))
                    }
                }

                sectionHeader("Photography tips — \(result.advice.label)")
                VStack(alignment: .leading, spacing: Theme.Space.s2) {
                    ForEach(result.advice.instructions, id: \.self) { line in
                        Label(line, systemImage: "camera")
                            .font(.caption)
                            .foregroundStyle(Theme.text)
                    }
                }
                .padding(Theme.Space.s3)
                .frame(maxWidth: .infinity, alignment: .leading)
                .background(Theme.surface, in: RoundedRectangle(cornerRadius: Theme.Radius.md))

                Button("Scan again", action: onScanAgain)
                    .buttonStyle(.bordered)
                    .tint(Theme.accent)
                    .frame(maxWidth: .infinity)
            }
            .padding(Theme.Space.s4)
        }
    }

    private func sectionHeader(_ title: String) -> some View {
        Text(title)
            .font(.caption.weight(.semibold))
            .foregroundStyle(Theme.textMuted)
            .textCase(.uppercase)
    }
}
