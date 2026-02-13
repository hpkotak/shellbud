package safety

import "testing"

func TestClassify(t *testing.T) {
	tests := []struct {
		command string
		want    Level
	}{
		// Safe commands
		{"ls -la", Safe},
		{"find . -name '*.log'", Safe},
		{"tar -czvf archive.tar.gz ./folder", Safe},
		{"grep -r 'TODO' .", Safe},
		{"du -sh * | sort -rh", Safe},
		{"cat /etc/hosts", Safe},
		{"echo hello", Safe},
		{"pwd", Safe},
		{"docker ps", Safe},
		{"git status", Safe},
		{"curl https://example.com", Safe},

		// Destructive commands
		{"rm file.txt", Destructive},
		{"rm -rf /tmp/test", Destructive},
		{"rm -fr /tmp/test", Destructive},
		{"sudo apt install vim", Destructive},
		{"sudo rm -rf /", Destructive},
		{"dd if=/dev/zero of=/dev/sda", Destructive},
		{"mkfs.ext4 /dev/sda1", Destructive},
		{"kill -9 1234", Destructive},
		{"killall nginx", Destructive},
		{"shutdown now", Destructive},
		{"reboot", Destructive},
		{"systemctl stop nginx", Destructive},
		{"systemctl disable nginx", Destructive},
		{"chmod 000 /etc/passwd", Destructive},
		{"mv /etc/hosts /tmp/", Destructive},
		{"chown -R root:root /home", Destructive},
		{"shred /tmp/secret.txt", Destructive},
		{"truncate -s 0 /var/log/syslog", Destructive},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			got := Classify(tt.command)
			if got != tt.want {
				t.Errorf("Classify(%q) = %v, want %v", tt.command, got, tt.want)
			}
		})
	}
}
