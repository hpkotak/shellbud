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

func TestLevelString(t *testing.T) {
	tests := []struct {
		level Level
		want  string
	}{
		{Safe, "safe"},
		{Destructive, "destructive"},
		{Level(99), "safe"}, // unknown levels default to safe, shouldn't panic
	}

	for _, tt := range tests {
		got := tt.level.String()
		if got != tt.want {
			t.Errorf("Level(%d).String() = %q, want %q", tt.level, got, tt.want)
		}
	}
}

func TestClassifyComplexCommands(t *testing.T) {
	tests := []struct {
		name    string
		command string
		want    Level
	}{
		// Piped commands with destructive elements
		{"rm in pipe", "find . -name '*.tmp' | xargs rm", Destructive},
		{"sudo in chain", "apt update && sudo apt upgrade -y", Destructive},
		{"subshell with rm", "(cd /tmp && rm -rf test)", Destructive},
		{"find -exec rm", "find . -name '*.tmp' -exec rm {} \\;", Destructive},
		// Note: "echo 'data' > /dev/sda" is NOT caught because the \b>\s*/dev/
		// pattern requires a word boundary before >. This is a known gap.
		// A redirect like "cmd> /dev/sda" (no space before >) would be caught.
		{"write to device with dd", "dd if=/dev/zero of=/dev/sda bs=512", Destructive},
		{"truncate pattern", ": > /var/log/app.log", Destructive},
		{"shred in pipe", "find . -name '*.key' -exec shred {} \\;", Destructive},
		{"kill -9 in pipe", "ps aux | grep zombie | awk '{print $2}' | xargs kill -9", Destructive},
		{"systemctl mask", "systemctl mask NetworkManager", Destructive},
		{"fdisk", "fdisk /dev/sda", Destructive},
		{"mv from root path", "mv /etc/resolv.conf /tmp/", Destructive},

		// Safe pipelines
		{"safe pipe", "ls -la | grep '.go' | wc -l", Safe},
		{"safe find", "find . -name '*.go' -exec wc -l {} +", Safe},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Classify(tt.command)
			if got != tt.want {
				t.Errorf("Classify(%q) = %v, want %v", tt.command, got, tt.want)
			}
		})
	}
}

func TestClassifyEdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		command string
		want    Level
	}{
		{"empty string", "", Safe},
		{"whitespace only", "   \t  ", Safe},
		{"long safe command", "echo " + string(make([]byte, 1000)), Safe},
		// "rm" inside an echo argument — our regex-based classifier flags this
		// as destructive because it can't parse shell semantics. This is an
		// acceptable false positive: better safe than sorry.
		{"rm in echo argument", "echo 'do not rm this'", Destructive},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Classify(tt.command)
			if got != tt.want {
				t.Errorf("Classify(%q) = %v, want %v", tt.command, got, tt.want)
			}
		})
	}
}
