package notify

import (
	"fmt"
	"os/exec"
	"runtime"
)

// SendNotification dispatches a native desktop notification to the operating system.
func SendNotification(title string, message string) {
	switch runtime.GOOS {
	case "linux":
		if path, err := exec.LookPath("notify-send"); err == nil {
			_ = exec.Command(path, "-a", "killport", "-i", "process-stop", title, message).Run()
		}
	case "darwin":
		script := fmt.Sprintf(`display notification %q with title %q`, message, title)
		_ = exec.Command("osascript", "-e", script).Run()
	case "windows":
		psCmd := fmt.Sprintf(`[reflection.assembly]::loadwithpartialname('System.Windows.Forms'); [System.Windows.Forms.MessageBox]::Show(%q, %q)`, message, title)
		_ = exec.Command("powershell", "-NoProfile", "-Command", psCmd).Start()
	}
}
