package scanner

import "regexp"

// Pattern is a single detection rule.
type Pattern struct {
	Name     string
	Category string
	Regex    *regexp.Regexp
	Severity Severity
}

// DefaultPatterns returns the built-in set of security detection patterns.
func DefaultPatterns() []Pattern {
	return []Pattern{
		// ── reverse_shell (Critical) ──────────────────────────────────
		{
			Name:     "bash reverse shell via /dev/tcp",
			Category: "reverse_shell",
			Regex:    regexp.MustCompile(`bash\s+-i\s+>&\s*/dev/tcp/`),
			Severity: SeverityCritical,
		},
		{
			Name:     "netcat exec shell",
			Category: "reverse_shell",
			Regex:    regexp.MustCompile(`\bnc\b.*\s-e\s+/bin/`),
			Severity: SeverityCritical,
		},
		{
			Name:     "python socket reverse shell",
			Category: "reverse_shell",
			Regex:    regexp.MustCompile(`python[23]?\s+-c\s+['"]import\s+socket`),
			Severity: SeverityCritical,
		},
		{
			Name:     "socat exec",
			Category: "reverse_shell",
			Regex:    regexp.MustCompile(`\bsocat\b.*\bexec\b`),
			Severity: SeverityCritical,
		},
		{
			Name:     "mkfifo netcat pipe",
			Category: "reverse_shell",
			Regex:    regexp.MustCompile(`\bmkfifo\b.*\bnc\b`),
			Severity: SeverityCritical,
		},

		// ── download_exec (Critical) ──────────────────────────────────
		{
			Name:     "curl pipe to shell",
			Category: "download_exec",
			Regex:    regexp.MustCompile(`curl\s.*\|\s*(ba)?sh`),
			Severity: SeverityCritical,
		},
		{
			Name:     "wget pipe to shell",
			Category: "download_exec",
			Regex:    regexp.MustCompile(`wget\s.*\|\s*(ba)?sh`),
			Severity: SeverityCritical,
		},
		{
			Name:     "curl pipe to python",
			Category: "download_exec",
			Regex:    regexp.MustCompile(`curl\s.*\|\s*python`),
			Severity: SeverityCritical,
		},
		{
			Name:     "python urllib download exec",
			Category: "download_exec",
			Regex:    regexp.MustCompile(`python[23]?\s+-c\s+['"]import\s+urllib`),
			Severity: SeverityCritical,
		},
		{
			Name:     "PowerShell download cradle",
			Category: "download_exec",
			Regex:    regexp.MustCompile(`(?i)\bIEX\b.*\bNew-Object\b.*WebClient`),
			Severity: SeverityCritical,
		},

		// ── credential_harvest (High) ─────────────────────────────────
		{
			Name:     "SSH key access",
			Category: "credential_harvest",
			Regex:    regexp.MustCompile(`cat\s+~/\.ssh/`),
			Severity: SeverityHigh,
		},
		{
			Name:     "shadow file access",
			Category: "credential_harvest",
			Regex:    regexp.MustCompile(`cat\s+/etc/shadow`),
			Severity: SeverityHigh,
		},
		{
			Name:     "AWS secret key reference",
			Category: "credential_harvest",
			Regex:    regexp.MustCompile(`\$AWS_SECRET_ACCESS_KEY`),
			Severity: SeverityHigh,
		},
		{
			Name:     "GitHub token reference",
			Category: "credential_harvest",
			Regex:    regexp.MustCompile(`\$GITHUB_TOKEN`),
			Severity: SeverityHigh,
		},
		{
			Name:     "macOS keychain access",
			Category: "credential_harvest",
			Regex:    regexp.MustCompile(`security\s+find-generic-password`),
			Severity: SeverityHigh,
		},

		// ── env_exfiltration (High) ───────────────────────────────────
		{
			Name:     "env piped to network tool",
			Category: "env_exfiltration",
			Regex:    regexp.MustCompile(`\benv\b\s*\|\s*(curl|wget|nc)\b`),
			Severity: SeverityHigh,
		},
		{
			Name:     "printenv piped to network tool",
			Category: "env_exfiltration",
			Regex:    regexp.MustCompile(`\bprintenv\b\s*\|\s*(curl|wget|nc)\b`),
			Severity: SeverityHigh,
		},

		// ── data_exfiltration (High) ──────────────────────────────────
		{
			Name:     "tar piped to network tool",
			Category: "data_exfiltration",
			Regex:    regexp.MustCompile(`tar\s.*\|\s*(curl|wget|nc)\b`),
			Severity: SeverityHigh,
		},
		{
			Name:     "curl POST file upload",
			Category: "data_exfiltration",
			Regex:    regexp.MustCompile(`\bcurl\b.*-[dX]\s*POST.*@`),
			Severity: SeverityHigh,
		},

		// ── obfuscation (High) ────────────────────────────────────────
		{
			Name:     "base64 decode pipe to shell",
			Category: "obfuscation",
			Regex:    regexp.MustCompile(`base64\s+(-d|--decode)\s*\|\s*(ba)?sh`),
			Severity: SeverityHigh,
		},
		{
			Name:     "echo base64 decode pipe to shell",
			Category: "obfuscation",
			Regex:    regexp.MustCompile(`echo\s.*\|\s*base64\s+(-d|--decode)\s*\|\s*(ba)?sh`),
			Severity: SeverityHigh,
		},

		// ── crypto_mining (Critical) ──────────────────────────────────
		{
			Name:     "XMRig miner",
			Category: "crypto_mining",
			Regex:    regexp.MustCompile(`(?i)\bxmrig\b`),
			Severity: SeverityCritical,
		},
		{
			Name:     "mining pool protocol",
			Category: "crypto_mining",
			Regex:    regexp.MustCompile(`stratum\+tcp://`),
			Severity: SeverityCritical,
		},
		{
			Name:     "CPU miner",
			Category: "crypto_mining",
			Regex:    regexp.MustCompile(`(?i)\bcpuminer\b`),
			Severity: SeverityCritical,
		},

		// ── shady_links (High) ────────────────────────────────────────
		{
			Name:     "suspicious TLD in URL",
			Category: "shady_links",
			Regex:    regexp.MustCompile(`https?://[^\s/]*\.(xyz|tk|top|pw|cc|icu|buzz|gq|cf|ml|ga|click|loan|work)\b`),
			Severity: SeverityHigh,
		},
		{
			Name:     "raw IP address URL",
			Category: "shady_links",
			Regex:    regexp.MustCompile(`https?://\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}[:/]`),
			Severity: SeverityHigh,
		},

		// ── agent_manipulation (Medium) ───────────────────────────────
		{
			Name:     "prompt injection: ignore instructions",
			Category: "agent_manipulation",
			Regex:    regexp.MustCompile(`(?i)ignore\s+(previous|all|prior)\s+(previous\s+)?instructions`),
			Severity: SeverityMedium,
		},
		{
			Name:     "hide actions from user",
			Category: "agent_manipulation",
			Regex:    regexp.MustCompile(`(?i)do\s+not\s+tell\s+the\s+user`),
			Severity: SeverityMedium,
		},
		{
			Name:     "bypass confirmation",
			Category: "agent_manipulation",
			Regex:    regexp.MustCompile(`(?i)(run|execute)\s+.*without\s+(asking|confirmation|telling)`),
			Severity: SeverityMedium,
		},
	}
}
