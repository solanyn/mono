#!/bin/sh

# Start CUPS in the background
/usr/sbin/cupsd -f &

# Wait for CUPS to be ready
while ! /usr/bin/lpstat -r; do
  echo "Waiting for CUPS scheduler..."
  sleep 1
done

# Add printers from printers.yaml
yq -r '.printers[] | .name + " " + .device_uri + " \"" + .make_model + "\""' /etc/cups/printers.yaml |
  while read -r name device_uri make_model; do
    echo "Adding printer: $name"
    echo "  URI: $device_uri"
    echo "  Model: $make_model"
    lpadmin -p "$name" -v "$device_uri" -m "$make_model"
    lpadmin -p "$name" -E
  done

# Set default printer
DEFAULT_PRINTER=$(yq -r '.printers[] | select(.default == true) | .name' /etc/cups/printers.yml)
if [ -n "$DEFAULT_PRINTER" ]; then
  echo "Setting default printer to: $DEFAULT_PRINTER"
  lpadmin -d "$DEFAULT_PRINTER"
fi

# Bring CUPS to the foreground
wait
