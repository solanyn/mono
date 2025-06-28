# CUPS Container

A CUPS print server container for network printing.

## Configuration

### Printer Configuration

Mount your own printer configuration file to `/etc/cups/printers.yaml`:

```bash
docker run -v ./my-printers.yaml:/etc/cups/printers.yaml:ro cups:latest
```

**Example configuration format:**

```yaml
printers:
  - name: printer
    info: "Printer"
    device_uri: "ipp://printer.internal/ipp/print"
    make_model: "Generic IPP Everywhere Printer"
    default: true
```

### Fields

- `name`: Printer name (no spaces, use underscores)
- `info`: Human-readable printer description
- `device_uri`: Printer connection URI (socket, ipp, usb, etc.)
- `make_model`: Printer driver/model string
- `default`: Set as default printer (boolean)

## Usage

```bash
# Build
./tools/containers/build.sh cups

# Run with custom printer config
docker run -d \
  --name cups-server \
  -p 631:631 \
  -v ./printers.yaml:/etc/cups/printers.yaml:ro \
  cups:latest
```

Access the CUPS web interface at http://localhost:631