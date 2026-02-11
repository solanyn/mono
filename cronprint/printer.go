package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/OpenPrinting/goipp"
)

type IPPPrinter struct {
	uri     string
	httpURI string
}

func NewIPPPrinter(uri string) *IPPPrinter {
	httpURI := uri
	if strings.HasPrefix(httpURI, "ipp://") {
		httpURI = "http://" + strings.TrimPrefix(httpURI, "ipp://")
	} else if strings.HasPrefix(httpURI, "ipps://") {
		httpURI = "https://" + strings.TrimPrefix(httpURI, "ipps://")
	}
	return &IPPPrinter{uri: uri, httpURI: httpURI}
}

func (p *IPPPrinter) sendRequest(msg *goipp.Message, body io.Reader) (*goipp.Message, error) {
	payload, err := msg.EncodeBytes()
	if err != nil {
		return nil, fmt.Errorf("encode IPP request: %w", err)
	}

	var reqBody io.Reader
	if body != nil {
		reqBody = io.MultiReader(bytes.NewReader(payload), body)
	} else {
		reqBody = bytes.NewReader(payload)
	}

	req, err := http.NewRequest(http.MethodPost, p.httpURI, reqBody)
	if err != nil {
		return nil, fmt.Errorf("create HTTP request: %w", err)
	}
	req.Header.Set("Content-Type", goipp.ContentType)
	req.Header.Set("Accept", goipp.ContentType)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("HTTP %s", resp.Status)
	}

	var rsp goipp.Message
	if err := rsp.Decode(resp.Body); err != nil {
		return nil, fmt.Errorf("decode IPP response: %w", err)
	}

	if goipp.Status(rsp.Code) != goipp.StatusOk {
		return &rsp, fmt.Errorf("IPP error: %s", goipp.Status(rsp.Code))
	}

	return &rsp, nil
}

func (p *IPPPrinter) newRequest(op goipp.Op) *goipp.Message {
	msg := goipp.NewRequest(goipp.DefaultVersion, op, 1)
	msg.Operation.Add(goipp.MakeAttribute("attributes-charset",
		goipp.TagCharset, goipp.String("utf-8")))
	msg.Operation.Add(goipp.MakeAttribute("attributes-natural-language",
		goipp.TagLanguage, goipp.String("en-US")))
	msg.Operation.Add(goipp.MakeAttribute("printer-uri",
		goipp.TagURI, goipp.String(p.uri)))
	return msg
}

func (p *IPPPrinter) GetPrinterAttributes() (map[string]string, error) {
	msg := p.newRequest(goipp.OpGetPrinterAttributes)
	msg.Operation.Add(goipp.MakeAttr("requested-attributes",
		goipp.TagKeyword, goipp.String("printer-state"),
		goipp.String("printer-state-reasons"),
		goipp.String("printer-make-and-model")))

	rsp, err := p.sendRequest(msg, nil)
	if err != nil {
		return nil, err
	}

	attrs := make(map[string]string)
	for _, group := range rsp.AttrGroups() {
		for _, attr := range group.Attrs {
			attrs[attr.Name] = attr.Values.String()
		}
	}
	return attrs, nil
}

func (p *IPPPrinter) PrintFile(filePath string, jobName string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	mimeType := "application/octet-stream"
	if strings.HasSuffix(filePath, ".pdf") {
		mimeType = "application/pdf"
	} else if strings.HasSuffix(filePath, ".ps") {
		mimeType = "application/postscript"
	}

	msg := p.newRequest(goipp.OpPrintJob)
	msg.Operation.Add(goipp.MakeAttribute("requesting-user-name",
		goipp.TagName, goipp.String("cronprint")))
	msg.Operation.Add(goipp.MakeAttribute("job-name",
		goipp.TagName, goipp.String(jobName)))
	msg.Operation.Add(goipp.MakeAttribute("document-format",
		goipp.TagMimeType, goipp.String(mimeType)))

	_, err = p.sendRequest(msg, f)
	return err
}

func (p *IPPPrinter) PrintTestPage(jobName string) error {
	testPDF := generateMinimalPDF()

	msg := p.newRequest(goipp.OpPrintJob)
	msg.Operation.Add(goipp.MakeAttribute("requesting-user-name",
		goipp.TagName, goipp.String("cronprint")))
	msg.Operation.Add(goipp.MakeAttribute("job-name",
		goipp.TagName, goipp.String(jobName)))
	msg.Operation.Add(goipp.MakeAttribute("document-format",
		goipp.TagMimeType, goipp.String("application/pdf")))

	_, err := p.sendRequest(msg, bytes.NewReader(testPDF))
	if err != nil {
		return err
	}
	return nil
}

func generateMinimalPDF() []byte {
	var buf bytes.Buffer
	offsets := make([]int, 5)

	buf.WriteString("%PDF-1.4\n")

	offsets[0] = buf.Len()
	buf.WriteString("1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n")

	offsets[1] = buf.Len()
	buf.WriteString("2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n")

	offsets[2] = buf.Len()
	buf.WriteString("3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents 4 0 R /Resources << /Font << /F1 5 0 R >> >> >>\nendobj\n")

	stream := "BT /F1 24 Tf 100 400 Td (cronprint test page) Tj ET"
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
