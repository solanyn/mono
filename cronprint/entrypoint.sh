#!/bin/sh
set -eu

# cronprint entrypoint: start CUPS, add the printer queue with the ESC/P-R
# PPD, then exec the scheduler. See cronprint-epson-notes in the Obsidian
# vault for why CUPS + escpr is required (the ET-2810 silently drops PWG
# raster jobs sent directly over IPP).

mkdir -p /var/run/cups /var/spool/cups /var/cache/cups /etc/cups
cupsd

# Wait for the CUPS control socket so lpadmin is safe to call.
i=0
while [ ! -S /run/cups/cups.sock ] && [ "$i" -lt 30 ]; do
    sleep 1
    i=$((i + 1))
done
if [ ! -S /run/cups/cups.sock ]; then
    echo "cupsd did not start in time" >&2
    exit 1
fi

# Re-add the queue on every boot: /etc/cups is ephemeral in the container.
# Discover the ESC/P-R PPD via the CUPS driver database (the escpr package
# generates PPDs at install time; there are no static files to reference).
ppd=$(lpinfo -m 2>/dev/null | grep -i escpr | grep -i "et-2810" | head -n1 | cut -d' ' -f1)
if [ -z "$ppd" ]; then
    ppd=$(lpinfo -m 2>/dev/null | grep -i escpr | head -n1 | cut -d' ' -f1)
fi
if [ -z "$ppd" ]; then
    echo "no ESC/P-R PPD found for the Epson printer (lpinfo -m)" >&2
    exit 1
fi

lpadmin -p "${CRONPRINT_PRINTER_NAME:-epson}" -E \
    -v "${CRONPRINT_PRINTER_URI:-socket://192.168.1.173:9100}" \
    -m "$ppd"

exec cronprint