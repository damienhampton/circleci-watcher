package notify

import (
	"fmt"
	"os/exec"
	"runtime"
)

// Send fires a native desktop notification. Silently no-ops if the platform
// has no supported notification tool.
func Send(title, body string) {
	switch runtime.GOOS {
	case "darwin":
		script := fmt.Sprintf(`display notification %q with title %q`, body, title)
		_ = exec.Command("osascript", "-e", script).Run()
	case "linux":
		_ = exec.Command("notify-send", title, body).Run()
	case "windows":
		ps := fmt.Sprintf(
			`[Windows.UI.Notifications.ToastNotificationManager,Windows.UI.Notifications,ContentType=WindowsRuntime]|Out-Null;`+
				`$t=[Windows.UI.Notifications.ToastTemplateType]::ToastText02;`+
				`$x=[Windows.UI.Notifications.ToastNotificationManager]::GetTemplateContent($t);`+
				`$x.GetElementsByTagName('text')[0].AppendChild($x.CreateTextNode('%s'))|Out-Null;`+
				`$x.GetElementsByTagName('text')[1].AppendChild($x.CreateTextNode('%s'))|Out-Null;`+
				`[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier('circleci-watch').Show([Windows.UI.Notifications.ToastNotification]::new($x))`,
			title, body,
		)
		_ = exec.Command("powershell", "-Command", ps).Run()
	}
}
