import SwiftUI

@main
struct KPortApp: App {
    @StateObject private var model = PortViewModel()

    var body: some Scene {
        WindowGroup {
            ContentView()
                .environmentObject(model)
                .frame(minWidth: 920, minHeight: 620)
                .preferredColorScheme(.dark)
        }
        .windowStyle(.hiddenTitleBar)
        .commands {
            CommandGroup(replacing: .newItem) {}
            CommandMenu("Ports") {
                Button("Refresh") { model.refresh() }.keyboardShortcut("r", modifiers: [.command])
                Button("Stop Selected…") { model.requestStop() }.keyboardShortcut(.delete, modifiers: [.command])
                    .disabled(model.selectedProcess == nil)
            }
        }
    }
}
