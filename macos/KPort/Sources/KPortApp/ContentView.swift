import SwiftUI

private let ink = Color(red: 0.055, green: 0.075, blue: 0.11)
private let panel = Color(red: 0.085, green: 0.11, blue: 0.15)
private let elevated = Color(red: 0.12, green: 0.15, blue: 0.20)
private let mint = Color(red: 0.36, green: 0.91, blue: 0.72)

struct ContentView: View {
    @EnvironmentObject private var model: PortViewModel

    var body: some View {
        HStack(spacing: 0) {
            sidebar
            VStack(spacing: 0) {
                topBar
                HStack(spacing: 0) {
                    processList
                    Rectangle().fill(.white.opacity(0.07)).frame(width: 1)
                    details
                }
            }
        }
        .background(ink)
        .foregroundStyle(.white)
        .confirmationDialog("Stop this listener?", isPresented: $model.showStopConfirmation, titleVisibility: .visible) {
            Button("Stop Process", role: .destructive) { model.executePlan() }
            Button("Cancel", role: .cancel) { model.pendingPlan = nil }
        } message: {
            Text(confirmationMessage)
        }
        .alert(item: $model.alert) { item in Alert(title: Text(item.title), message: Text(item.message), dismissButton: .default(Text("OK"))) }
    }

    private var sidebar: some View {
        VStack(alignment: .leading, spacing: 26) {
            HStack(spacing: 11) {
                ZStack { RoundedRectangle(cornerRadius: 12).fill(mint); Image(systemName: "bolt.fill").foregroundStyle(ink).font(.system(size: 16, weight: .black)) }.frame(width: 38, height: 38)
                VStack(alignment: .leading, spacing: 1) { Text("kport").font(.system(size: 19, weight: .bold)); Text("PORT MANAGER").font(.system(size: 9, weight: .bold, design: .rounded)).tracking(1.4).foregroundStyle(.white.opacity(0.42)) }
            }
            VStack(alignment: .leading, spacing: 8) {
                Text("WORKSPACE").font(.system(size: 10, weight: .bold)).tracking(1.4).foregroundStyle(.white.opacity(0.36)).padding(.leading, 11)
                Label("All Listeners", systemImage: "dot.radiowaves.left.and.right").font(.system(size: 13, weight: .semibold)).padding(.horizontal, 12).padding(.vertical, 10).frame(maxWidth: .infinity, alignment: .leading).background(mint.opacity(0.13), in: RoundedRectangle(cornerRadius: 9)).foregroundStyle(mint)
            }
            VStack(alignment: .leading, spacing: 10) {
                Text("OVERVIEW").font(.system(size: 10, weight: .bold)).tracking(1.4).foregroundStyle(.white.opacity(0.36)).padding(.leading, 11)
                metric("Listeners", value: "\(model.processes.count)", icon: "dot.radiowaves.left.and.right")
                metric("Processes", value: "\(model.ownerCount)", icon: "square.stack.3d.up")
                metric("Docker", value: model.processes.contains { $0.containerID != nil } ? "Connected" : "—", icon: "shippingbox")
            }
            Spacer()
            HStack(spacing: 8) { Circle().fill(mint).frame(width: 7, height: 7); Text(model.status).font(.system(size: 11)).foregroundStyle(.white.opacity(0.6)) }.padding(.leading, 10)
        }
        .padding(20)
        .frame(width: 230)
        .background(panel)
    }

    private func metric(_ title: String, value: String, icon: String) -> some View {
        HStack(spacing: 11) {
            Image(systemName: icon).font(.system(size: 13)).foregroundStyle(.white.opacity(0.48)).frame(width: 18)
            Text(title).font(.system(size: 12)).foregroundStyle(.white.opacity(0.65))
            Spacer()
            Text(value).font(.system(size: 12, weight: .semibold, design: .rounded)).foregroundStyle(.white.opacity(0.9))
        }.padding(.horizontal, 11).padding(.vertical, 9)
    }

    private var topBar: some View {
        HStack(spacing: 14) {
            VStack(alignment: .leading, spacing: 3) {
                Text("Port activity").font(.system(size: 20, weight: .bold))
                Text("Inspect and manage services running on your machine").font(.system(size: 12)).foregroundStyle(.white.opacity(0.48))
            }
            Spacer()
            HStack(spacing: 8) {
                Image(systemName: "magnifyingglass").foregroundStyle(.white.opacity(0.4))
                TextField("Search ports, processes, projects…", text: $model.search).textFieldStyle(.plain).font(.system(size: 12)).frame(width: 220)
                if !model.search.isEmpty { Button { model.search = "" } label: { Image(systemName: "xmark.circle.fill").foregroundStyle(.white.opacity(0.4)) }.buttonStyle(.plain) }
            }.padding(.horizontal, 11).padding(.vertical, 9).background(elevated, in: RoundedRectangle(cornerRadius: 9))
            Button { model.refresh() } label: { Image(systemName: "arrow.clockwise").font(.system(size: 13, weight: .semibold)).rotationEffect(.degrees(model.isLoading ? 180 : 0)).frame(width: 34, height: 34).background(elevated, in: RoundedRectangle(cornerRadius: 9)) }.buttonStyle(.plain).help("Refresh")
        }
        .padding(.horizontal, 24).padding(.vertical, 18)
        .background(ink)
    }

    private var processList: some View {
        VStack(spacing: 0) {
            HStack {
                Text("ACTIVE PORTS").font(.system(size: 10, weight: .bold)).tracking(1.2).foregroundStyle(.white.opacity(0.42))
                Spacer()
                Picker("Protocol", selection: $model.filter) { Text("All").tag("All"); Text("TCP").tag("TCP"); Text("UDP").tag("UDP") }.pickerStyle(.segmented).frame(width: 145)
            }.padding(.horizontal, 20).padding(.vertical, 13)
            Rectangle().fill(.white.opacity(0.07)).frame(height: 1)
            if model.visibleProcesses.isEmpty {
                emptyState(model.processes.isEmpty ? "No listeners found" : "No matching ports", detail: model.processes.isEmpty ? "Services will appear here as they start." : "Try a different search or protocol filter.", icon: "network.slash")
            } else {
                ScrollView {
                    LazyVStack(spacing: 5) {
                        ForEach(model.visibleProcesses) { process in
                            ProcessRow(process: process, selected: process.id == model.selectedID)
                                .contentShape(RoundedRectangle(cornerRadius: 9))
                                .onTapGesture { model.selectedID = process.id }
                        }
                    }.padding(10)
                }
            }
            HStack { Text("Auto refresh · 3 sec"); Spacer(); if let date = model.lastRefresh { Text(date, style: .time) } }.font(.system(size: 10)).foregroundStyle(.white.opacity(0.37)).padding(.horizontal, 18).padding(.vertical, 11).background(panel.opacity(0.6))
        }
        .frame(minWidth: 440, maxWidth: .infinity, maxHeight: .infinity)
        .background(ink)
    }

    private var details: some View {
        VStack(alignment: .leading, spacing: 19) {
            if let process = model.selectedProcess {
                HStack {
                    Text("DETAILS").font(.system(size: 10, weight: .bold)).tracking(1.3).foregroundStyle(.white.opacity(0.42))
                    Spacer()
                    Button { model.requestStop() } label: { Label("Stop", systemImage: "stop.fill").font(.system(size: 11, weight: .semibold)).padding(.horizontal, 12).padding(.vertical, 8).background(Color(red: 0.65, green: 0.24, blue: 0.27).opacity(0.24), in: RoundedRectangle(cornerRadius: 8)).foregroundStyle(Color(red: 1, green: 0.52, blue: 0.53)) }.buttonStyle(.plain)
                }
                VStack(alignment: .leading, spacing: 7) {
                    Text(process.displayName).font(.system(size: 19, weight: .bold)).lineLimit(2)
                    Text(process.containerID == nil ? "PID \(process.pid)" : "Container · \(process.containerID!.prefix(12))").font(.system(size: 11, design: .monospaced)).foregroundStyle(.white.opacity(0.43))
                }
                HStack(spacing: 8) {
                    badge("\(process.port)", color: mint)
                    badge(process.protocolName.uppercased(), color: .cyan)
                    badge(process.state ?? "LISTEN", color: .white.opacity(0.7))
                }
                VStack(alignment: .leading, spacing: 0) {
                    detailLine("Address", process.address, copy: process.address)
                    detailLine("Project", process.cwd ?? "Unavailable", copy: process.cwd)
                    detailLine("User", process.user ?? "—")
                    detailLine("Memory", process.memoryHuman ?? "—")
                    detailLine("Parent PID", process.ppid.map(String.init) ?? "—")
                    detailLine("Children", process.childrenPIDs.map { $0.map(String.init).joined(separator: ", ") }.flatMap { $0.isEmpty ? nil : $0 } ?? "—")
                }.padding(.horizontal, 13).background(panel, in: RoundedRectangle(cornerRadius: 11))
                VStack(alignment: .leading, spacing: 7) {
                    Text("COMMAND").font(.system(size: 9, weight: .bold)).tracking(1.1).foregroundStyle(.white.opacity(0.4))
                    Text(process.commandLine ?? process.exe ?? "Command unavailable").font(.system(size: 11, design: .monospaced)).foregroundStyle(.white.opacity(0.72)).textSelection(.enabled).lineLimit(5).frame(maxWidth: .infinity, alignment: .leading)
                }
                Spacer(minLength: 0)
                Text("Stopping a process closes every port it owns. kport checks the full impact before acting.").font(.system(size: 10)).foregroundStyle(.white.opacity(0.42)).fixedSize(horizontal: false, vertical: true)
            } else {
                emptyState("Select a listener", detail: "Choose a port to inspect its process details.", icon: "cursorarrow.click")
            }
        }
        .padding(20)
        .frame(width: 320, alignment: .topLeading)
        .frame(maxHeight: .infinity, alignment: .topLeading)
        .background(ink)
    }

    private func emptyState(_ title: String, detail: String, icon: String) -> some View {
        VStack(spacing: 10) {
            Image(systemName: icon).font(.system(size: 27)).foregroundStyle(mint.opacity(0.8))
            Text(title).font(.system(size: 15, weight: .semibold))
            Text(detail).font(.system(size: 11)).foregroundStyle(.white.opacity(0.45)).multilineTextAlignment(.center)
        }.frame(maxWidth: .infinity, maxHeight: .infinity)
    }

    private func detailLine(_ title: String, _ value: String, copy: String? = nil) -> some View {
        HStack(alignment: .top, spacing: 12) {
            Text(title).font(.system(size: 10)).foregroundStyle(.white.opacity(0.43)).frame(width: 72, alignment: .leading)
            Text(value).font(.system(size: 10, design: .monospaced)).foregroundStyle(.white.opacity(0.82)).textSelection(.enabled).lineLimit(3)
            Spacer(minLength: 0)
            if let copy { Button { NSPasteboard.general.clearContents(); NSPasteboard.general.setString(copy, forType: .string) } label: { Image(systemName: "doc.on.doc").font(.system(size: 10)).foregroundStyle(.white.opacity(0.37)) }.buttonStyle(.plain) }
        }.padding(.vertical, 10)
    }

    private func badge(_ text: String, color: Color) -> some View {
        Text(text).font(.system(size: 10, weight: .bold, design: .monospaced)).foregroundStyle(color).padding(.horizontal, 9).padding(.vertical, 6).background(color.opacity(0.12), in: Capsule())
    }

    private var confirmationMessage: String {
        guard let plan = model.pendingPlan else { return "" }
        let issues = plan.issues?.count ?? 0
        return "This process affects \(plan.impact.count) socket(s) across \(Set(plan.impact.map(\.port)).count) port(s).\(issues > 0 ? " \(issues) safety issue(s) will block the action." : "")"
    }
}

private struct ProcessRow: View {
    let process: PortProcess
    let selected: Bool
    var body: some View {
        HStack(spacing: 12) {
            RoundedRectangle(cornerRadius: 2).fill(process.containerID == nil ? mint : .cyan).frame(width: 3, height: 35)
            VStack(alignment: .leading, spacing: 5) {
                HStack(spacing: 8) { Text(process.displayName).font(.system(size: 12, weight: .semibold)).lineLimit(1); if process.containerID != nil { Image(systemName: "shippingbox.fill").font(.system(size: 9)).foregroundStyle(.cyan) } }
                Text("PID \(process.pid)  ·  \(process.address)").font(.system(size: 10, design: .monospaced)).foregroundStyle(.white.opacity(0.44)).lineLimit(1)
            }
            Spacer(minLength: 6)
            VStack(alignment: .trailing, spacing: 4) {
                Text(String(process.port)).font(.system(size: 15, weight: .bold, design: .rounded)).foregroundStyle(selected ? mint : .white)
                Text(process.protocolName.uppercased()).font(.system(size: 8, weight: .bold)).tracking(0.7).foregroundStyle(.white.opacity(0.38))
            }
        }
        .padding(.horizontal, 11).padding(.vertical, 9)
        .background(selected ? elevated : .clear, in: RoundedRectangle(cornerRadius: 9))
        .overlay(alignment: .leading) { if selected { RoundedRectangle(cornerRadius: 2).fill(mint).frame(width: 2).padding(.vertical, 8) } }
    }
}
