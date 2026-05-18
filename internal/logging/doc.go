// Package logging provides colored, leveled terminal output for the SPIFFE/SPIRE
// demo CLI tools. It replaces the PowerShell logging.ps1 helpers with Go equivalents.
//
// Output levels:
//   - Step:   major phase markers (cyan, with arrow prefix)
//   - Info:   general information (white)
//   - Detail: debug-level output, only shown when verbose mode is active (gray)
//   - Ok:     success confirmations (green, with checkmark)
//   - Warn:   non-fatal warnings (yellow)
//   - Error:  fatal or significant errors (red)
//
// Colors use ANSI escape codes. On Windows, modern terminals (Windows Terminal,
// VS Code) support these natively. The package does not attempt to detect terminal
// capabilities - this is an educational tool, not a production library.
package logging
