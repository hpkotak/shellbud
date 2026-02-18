import Foundation
import AFMBridgeCore

@main
struct AFMBridge {
    static func main() async {
        guard #available(macOS 26.0, *) else {
            IO.exitWithError("afm-bridge requires macOS 26.0 or later")
        }

        if CommandLine.arguments.contains("--check-availability") {
            let handler = Handler()
            handler.checkAvailability()
            return
        }

        let handler = Handler()
        await handler.handleRequest()
    }
}
