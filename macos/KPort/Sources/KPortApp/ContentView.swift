import SwiftUI
import AppKit

struct ContentView: View {
    @EnvironmentObject private var model: PortViewModel

    var body: some View {
        NavigationSplitView {
            navigation
                .navigationTitle("kport")
                .navigationSplitViewColumnWidth(min: 180, ideal: 205, max: 250)
        } content: {
            portList
                .navigationTitle(model.filterTitle)
                .navigationSplitViewColumnWidth(min: 300, ideal: 390, max: 520)
        } detail: {
            inspector.navigationTitle("Details")
        }
        .navigationSplitViewStyle(.balanced)
        .searchable(text: $model.search, placement: .toolbar, prompt: "Search ports and processes")
        .toolbar {
            ToolbarItemGroup {
                Picker("Protocol", selection: $model.filter) {
                    Text("All").tag("All")
                    Text("TCP").tag("TCP")
                    Text("UDP").tag("UDP")
                }
                .pickerStyle(.segmented)
                .fixedSize()
                Button { model.refresh() } label: {
                    if model.isLoading { ProgressView().controlSize(.small) }
                    else { Label("Refresh", systemImage: "arrow.clockwise") }
                }
                .help("Refresh listeners")
                .disabled(model.isLoading)
            }
            ToolbarItem(placement: .status) {
                Text(model.status).foregroundStyle(.secondary)
            }
        }
        .confirmationDialog("Stop this process?", isPresented: $model.showStopConfirmation, titleVisibility: .visible) {
            Button("Stop Process", role: .destructive) { model.executePlan() }
            Button("Cancel", role: .cancel) { model.pendingPlan = nil }
        } message: {
            Text(confirmationMessage)
        }
        .alert(item: $model.alert) { item in
            Alert(title: Text(item.title), message: Text(item.message), dismissButton: .default(Text("OK")))
        }
    }

    private var navigation: some View {
        List(selection: $model.filter) {
            Section("Listeners") {
                Label { HStack { Text("All Ports"); Spacer(); Text("\(model.processes.count)").foregroundStyle(.secondary) } } icon: { Image(systemName: "network") }
                    .tag("All")
                Label("TCP", systemImage: "arrow.left.arrow.right").tag("TCP")
                Label("UDP", systemImage: "point.3.connected.trianglepath.dotted").tag("UDP")
            }
        }
        .listStyle(.sidebar)
    }

    private var portList: some View {
        List(selection: $model.selectedID) {
            if model.visibleProcesses.isEmpty {
                emptyState
            } else {
                ForEach(model.visibleProcesses) { process in
                    PortRow(process: process)
                        .tag(process.id as String?)
                        .contextMenu {
                            Button("Copy Address", systemImage: "doc.on.doc") { NSPasteboard.general.setString(process.address, forType: .string) }
                            Button("Copy Port", systemImage: "number") { NSPasteboard.general.setString(String(process.port), forType: .string) }
                        }
                }
            }
        }
        .listStyle(.inset)
        .overlay(alignment: .bottom) {
            if let date = model.lastRefresh {
                HStack { Image(systemName: "arrow.clockwise"); Text("Updated \(date, style: .relative) ago") }
                    .font(.caption2).foregroundStyle(.tertiary).padding(.bottom, 8)
                    .allowsHitTesting(false)
            }
        }
    }

    private var emptyState: some View {
        VStack(spacing: 8) {
            Image(systemName: model.search.isEmpty ? "network" : "magnifyingglass")
                .font(.system(size: 24)).foregroundStyle(.secondary)
            Text(model.search.isEmpty ? "No Listeners" : "No Results").font(.headline)
            Text(model.search.isEmpty ? "Services will appear here when they listen on a port." : "Try another search or protocol filter.")
                .font(.callout).foregroundStyle(.secondary).multilineTextAlignment(.center)
        }.frame(maxWidth: .infinity, minHeight: 220)
    }

    @ViewBuilder private var inspector: some View {
        if let process = model.selectedProcess {
            Form {
                Section {
                    LabeledContent("Process", value: process.displayName)
                    LabeledContent("PID", value: process.containerID == nil ? String(process.pid) : "Container")
                    LabeledContent("Port") {
                        HStack(spacing: 6) {
                            Text(String(process.port)).font(.system(.body, design: .monospaced))
                            Text(process.protocolName.uppercased()).font(.caption2.weight(.semibold)).foregroundStyle(.secondary)
                        }
                    }
                    LabeledContent("Address") { Text(process.address).font(.system(.body, design: .monospaced)).textSelection(.enabled) }
                }
                Section("Process Information") {
                    if let cwd = process.cwd { LabeledContent("Working Directory", value: cwd).textSelection(.enabled) }
                    if let user = process.user { LabeledContent("User", value: user) }
                    if let memory = process.memoryHuman { LabeledContent("Memory", value: memory) }
                    if let ppid = process.ppid { LabeledContent("Parent PID", value: String(ppid)) }
                    if let children = process.childrenPIDs, !children.isEmpty {
                        LabeledContent("Child PIDs", value: children.map(String.init).joined(separator: ", "))
                    }
                    if let container = process.containerName { LabeledContent("Container", value: container) }
                }
                Section("Command") {
                    Text(process.commandLine ?? process.exe ?? "Command unavailable")
                        .font(.system(.callout, design: .monospaced)).textSelection(.enabled)
                        .frame(maxWidth: .infinity, alignment: .leading)
                }
                Section {
                    Button(role: .destructive) { model.requestStop() } label: { Label("Stop Process…", systemImage: "stop.circle") }
                } footer: {
                    Text("Stopping a process closes every port it owns. Review the impact before confirming.")
                }
            }
            .formStyle(.grouped)
            .frame(maxWidth: 580, maxHeight: .infinity, alignment: .topLeading)
            .frame(maxWidth: .infinity, maxHeight: .infinity)
        } else {
            VStack(spacing: 8) {
                Image(systemName: "network").font(.system(size: 28)).foregroundStyle(.secondary)
                Text("Select a Port").font(.headline)
                Text("Choose a listener to inspect its process.").font(.callout).foregroundStyle(.secondary)
            }.frame(maxWidth: .infinity, maxHeight: .infinity)
        }
    }

    private var confirmationMessage: String {
        guard let plan = model.pendingPlan else { return "" }
        let ports = Set(plan.impact.map(\.port)).count
        let issueCount = plan.issues?.count ?? 0
        return "This process affects \(plan.impact.count) socket(s) on \(ports) port(s)." +
            (issueCount > 0 ? " \(issueCount) safety issue(s) will prevent the action." : "")
    }
}

private struct PortRow: View {
    let process: PortProcess

    var body: some View {
        HStack(spacing: 10) {
            Image(systemName: process.containerID == nil ? "network" : "shippingbox")
                .foregroundStyle(process.containerID == nil ? Color.secondary : Color.blue)
                .frame(width: 20)
            VStack(alignment: .leading, spacing: 3) {
                Text(process.displayName).lineLimit(1)
                Text("\(process.address) · PID \(process.pid)")
                    .font(.caption.monospaced()).foregroundStyle(.secondary).lineLimit(1)
            }
            Spacer(minLength: 8)
            Text(String(process.port)).font(.system(.body, design: .monospaced).weight(.semibold))
        }
        .padding(.vertical, 3)
        .contentShape(Rectangle())
    }
}
