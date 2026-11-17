import Foundation
import SwiftUI

struct PortProcess: Codable, Identifiable, Hashable {
    var startID: String?
    var containerID: String?
    var containerName: String?
    var pid: Int
    var ppid: Int?
    var name: String
    var commandLine: String?
    var exe: String?
    var cwd: String?
    var user: String?
    var port: Int
    var protocolName: String
    var address: String
    var state: String?
    var memoryHuman: String?
    var childrenPIDs: [Int]?

    enum CodingKeys: String, CodingKey {
        case startID = "start_id", containerID = "container_id", containerName = "container_name"
        case pid, ppid, name, commandLine = "command_line", exe, cwd, user, port
        case protocolName = "protocol", address, state, memoryHuman = "memory_human", childrenPIDs = "children_pids"
    }
    var id: String { "\(containerID ?? "pid:\(pid):\(startID ?? "")")|\(port)|\(protocolName)|\(address)" }
    var ownerKey: String { "\(containerID ?? "pid:\(pid):\(startID ?? "")")" }
    var displayName: String { containerName.map { "\($0) · Container" } ?? name }
}

struct ActionPlan: Codable {
    var targets: [PortProcess]
    var impact: [PortProcess]
    var issues: [PlanIssue]?
}
struct PlanIssue: Codable, Identifiable {
    var process: PortProcess
    var code: String
    var message: String
    var id: String { "\(process.id):\(code)" }
}
struct ActionResult: Codable {
    var pid: Int
    var name: String
    var port: Int
    var killed: Bool
    var error: String?
    var errorCode: String?
    enum CodingKeys: String, CodingKey { case pid, name, port, killed, error, errorCode = "error_code" }
}

@MainActor
final class PortViewModel: ObservableObject {
    @Published var processes: [PortProcess] = []
    @Published var selectedID: String?
    @Published var search = ""
    @Published var status = "Ready"
    @Published var lastRefresh: Date?
    @Published var isLoading = false
    @Published var showStopConfirmation = false
    @Published var pendingPlan: ActionPlan?
    @Published var alert: AppAlert?
    @Published var filter = "All"

    private var timer: Timer?
    private let decoder = JSONDecoder()
    var selectedProcess: PortProcess? { visibleProcesses.first { $0.id == selectedID } }
    var visibleProcesses: [PortProcess] {
        processes.filter { p in
            let matchesFilter = filter == "All" || p.protocolName.uppercased() == filter.uppercased()
            let text = "\(p.port) \(p.name) \(p.containerName ?? "") \(p.commandLine ?? "") \(p.cwd ?? "") \(p.address)".lowercased()
            return matchesFilter && (search.isEmpty || text.contains(search.lowercased()))
        }
    }
    var ownerCount: Int { Set(processes.map(\.ownerKey)).count }

    init() {
        refresh()
        timer = Timer.scheduledTimer(withTimeInterval: 3, repeats: true) { [weak self] _ in
            Task { @MainActor in self?.refresh() }
        }
    }

    func refresh() {
        guard !isLoading else { return }
        isLoading = true
        Task {
            defer { isLoading = false }
            do {
                let data = try await runCLI(["--json", "--list"])
                processes = try decoder.decode([PortProcess].self, from: data).sorted {
                    $0.port == $1.port ? $0.name.localizedCaseInsensitiveCompare($1.name) == .orderedAscending : $0.port < $1.port
                }
                if selectedID == nil || !processes.contains(where: { $0.id == selectedID }) { selectedID = processes.first?.id }
                status = "Updated just now"
                lastRefresh = Date()
            } catch { status = "Couldn’t refresh"; alert = AppAlert(title: "Listener scan failed", message: error.localizedDescription) }
        }
    }

    func requestStop() {
        guard let selectedProcess else { return }
        Task {
            do {
                let data = try await runCLI(["--json", "--plan", String(selectedProcess.port)])
                let plan = try decoder.decode(ActionPlan.self, from: data)
                pendingPlan = plan
                showStopConfirmation = true
            } catch { alert = AppAlert(title: "Couldn’t prepare stop", message: error.localizedDescription) }
        }
    }

    func executePlan() {
        guard let pendingPlan else { return }
        Task {
            do {
                let data = try JSONEncoder().encode(pendingPlan)
                let url = FileManager.default.temporaryDirectory.appendingPathComponent("kport-plan-\(UUID().uuidString).json")
                try data.write(to: url, options: .atomic)
                defer { try? FileManager.default.removeItem(at: url) }
                let output = try await runCLI(["--json", "--execute-plan", url.path])
                let results = try decoder.decode([ActionResult].self, from: output)
                let failures = results.filter { !$0.killed }
                if failures.isEmpty { status = "Stopped \(results.count) listener(s)" }
                else { alert = AppAlert(title: "Some actions failed", message: failures.map { "Port \($0.port): \($0.error ?? $0.errorCode ?? "Unknown error")" }.joined(separator: "\n")) }
                self.pendingPlan = nil
                refresh()
            } catch { alert = AppAlert(title: "Couldn’t stop process", message: error.localizedDescription) }
        }
    }

    private func runCLI(_ arguments: [String]) async throws -> Data {
        try await Task.detached(priority: .userInitiated) {
            let executable = try Self.findCLI()
            let process = Process()
            process.executableURL = executable
            process.arguments = arguments
            var environment = ProcessInfo.processInfo.environment
            environment["TERM"] = "dumb"
            process.environment = environment
            let stdout = Pipe(), stderr = Pipe()
            process.standardOutput = stdout; process.standardError = stderr
            try process.run()
            let output = stdout.fileHandleForReading.readDataToEndOfFile()
            let errorOutput = stderr.fileHandleForReading.readDataToEndOfFile()
            process.waitUntilExit()
            guard process.terminationStatus == 0 else {
                let message = String(data: errorOutput, encoding: .utf8)?.trimmingCharacters(in: .whitespacesAndNewlines)
                throw CLIError.failed(message?.isEmpty == false ? message! : "kport exited with status \(process.terminationStatus)")
            }
            return output
        }.value
    }

    nonisolated private static func findCLI() throws -> URL {
        if let configured = ProcessInfo.processInfo.environment["KPORT_CLI_PATH"], FileManager.default.isExecutableFile(atPath: configured) { return URL(fileURLWithPath: configured) }
        if let bundled = Bundle.main.resourceURL?.appendingPathComponent("kport"), FileManager.default.isExecutableFile(atPath: bundled.path) { return bundled }
        let candidates = ["/opt/homebrew/bin/kport", "/usr/local/bin/kport", NSHomeDirectory() + "/.local/bin/kport"]
        if let path = candidates.first(where: { FileManager.default.isExecutableFile(atPath: $0) }) { return URL(fileURLWithPath: path) }
        throw CLIError.notFound
    }
}

struct AppAlert: Identifiable { let id = UUID(); var title: String; var message: String }
enum CLIError: LocalizedError {
    case notFound, failed(String)
    var errorDescription: String? {
        switch self { case .notFound: return "Couldn’t find kport. Install it or bundle it with the app."; case .failed(let message): return message }
    }
}
