package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Printer prints documents through the local CUPS daemon.
//
// The Epson ET-2810 advertises IPP Everywhere / PWG raster but does not
// actually render it — jobs are accepted and silently dropped. The reliable
// path is CUPS with the ESC/P-R driver (printer-driver-escpr), which talks
// to the printer over a raw AppSocket (port 9100) using Epson's native
// protocol. See cronprint-epson-notes in the Obsidian vault.
type Printer struct {
	// Name is the CUPS queue name (also used as lp -d target).
	Name string
	// URI is the device URI shown in lpstat; informational only.
	URI string
}

// NewPrinter returns a Printer for the named CUPS queue.
func NewPrinter(name string) *Printer {
	return &Printer{Name: name}
}

// Health returns the printer's current CUPS state ("idle", "printing",
// "stopped", etc.) and whether the queue exists. It is used by /healthz.
func (p *Printer) Health() (map[string]string, error) {
	out, err := exec.Command("lpstat", "-p", p.Name).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("lpstat -p %s: %w (%s)", p.Name, err, strings.TrimSpace(string(out)))
	}

	fields := strings.Fields(string(out))
	// Expected first line: "printer NAME is idle.  enable since ..."
	state := "unknown"
	if len(fields) >= 4 && fields[0] == "printer" && fields[2] == "is" {
		state = strings.TrimSuffix(fields[3], ".")
	}

	return map[string]string{
		"name":  p.Name,
		"state": state,
		"uri":   p.URI,
	}, nil
}

// PrintFile sends an existing file to the CUPS queue. CUPS handles format
// conversion (PDF/PostScript/plain text) to ESC/P-R via the escpr driver.
func (p *Printer) PrintFile(filePath, jobName string) error {
	if _, err := os.Stat(filePath); err != nil {
		return fmt.Errorf("print file: %w", err)
	}

	args := []string{"-d", p.Name}
	if jobName != "" {
		args = append(args, "-t", jobName)
	}
	args = append(args, filePath)

	out, err := exec.Command("lp", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("lp -d %s: %w (%s)", p.Name, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// PrintTestPage prints a minimal self-contained PDF via CUPS. The job name
// is passed through so scheduled "nozzle check" runs are identifiable in
// the queue.
func (p *Printer) PrintTestPage(jobName string) error {
	if jobName == "" {
		jobName = "cronprint-test-page"
	}
	tmp, err := os.CreateTemp("", "cronprint-*.pdf")
	if err != nil {
		return fmt.Errorf("create temp PDF: %w", err)
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.Write(generateTestPagePDF()); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp PDF: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp PDF: %w", err)
	}

	return p.PrintFile(tmp.Name(), jobName)
}

// generateTestPagePDF returns a tiny valid PDF with a printable page. Kept
// dependency-free so the binary stays static and the page is guaranteed to
// render through CUPS's pdftops filter.
//
// The page is A4 with a large "cronprint" label so a weekly nozzle-check
// print is visibly identifiable on the output tray. Offsets are computed
// so the xref table is always correct.
func generateTestPagePDF() []byte {
	var buf bytes.Buffer
	offsets := make([]int, 5)

	buf.WriteString("%PDF-1.4\n")

	offsets[0] = buf.Len()
	buf.WriteString("1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n")

	offsets[1] = buf.Len()
	buf.WriteString("2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n")

	offsets[2] = buf.Len()
	buf.WriteString("3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << /Font << /F1 5 0 R >> >> /Contents 4 0 R >>\nendobj\n")

	stream := `BT
/F1 48 Tf
72 720 Td
(cronprint) Tj
0 -70 Td
/F1 14 Tf
(weekly nozzle check) Tj
ET`
	offsets[3] = buf.Len()
	buf.WriteString(fmt.Sprintf("4 0 obj\n<< /Length %d >>\nstream\n%s\nendstream\nendobj\n", len(stream), stream))

	offsets[4] = buf.Len()
	buf.WriteString("5 0 obj\n<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>\nendobj\n")

	xrefOffset := buf.Len()
	buf.WriteString("xref\n")
	buf.WriteString(fmt.Sprintf("0 %d\n", len(offsets)+1))
	buf.WriteString("0000000000 65535 f \n")
	for _, off := range offsets {
		buf.WriteString(fmt.Sprintf("%010d 00000 n \n", off))
	}

	buf.WriteString("trailer\n")
	buf.WriteString(fmt.Sprintf("<< /Size %d /Root 1 0 R >>\n", len(offsets)+1))
	buf.WriteString("startxref\n")
	buf.WriteString(fmt.Sprintf("%d\n", xrefOffset))
	buf.WriteString("%%EOF\n")

	return buf.Bytes()
}