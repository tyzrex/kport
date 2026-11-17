# kport for macOS

Native SwiftUI frontend for the existing Go process manager. The app refreshes
listener data every three seconds, supports search and TCP/UDP filtering, and
shows process details. Stop actions first request a JSON action plan and show
its affected socket and port count for confirmation; kport revalidates the plan
before stopping anything.

## Build

Install Xcode (including its macOS SDK), Go, then run from the repository root:

```sh
macos/KPort/Scripts/build-app.sh
open dist/KPort.app
```

The build places the matching macOS `kport` executable inside the app bundle.
For local development, set `KPORT_CLI_PATH` to an existing executable to use it
instead. The first build is ad hoc signed for local launch; release distribution
will need Developer ID signing and notarization.
