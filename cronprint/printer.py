import logging
import os
from typing import Optional, Dict

import cups

logger = logging.getLogger(__name__)


class PrintManager:
    """Manages CUPS printing operations."""

    def __init__(self, default_printer: Optional[str] = None):
        self.default_printer = default_printer
        self.conn = None
        self.cups_host = "localhost:631"

        try:
            cups.setServer(self.cups_host)
            self.conn = cups.Connection()
            logger.info(f"Connected to CUPS server at {self.cups_host}")
        except Exception as e:
            logger.error(f"Failed to connect to CUPS: {e}")

    def get_printers(self) -> dict:
        """Get available printers."""
        try:
            return self.conn.getPrinters()
        except Exception as e:
            logger.error(f"Failed to get printers: {e}")
            return {}

    def print_file(
        self,
        file_path: str,
        printer_name: Optional[str] = None,
        job_name: Optional[str] = None,
    ) -> bool:
        """Print a file to the specified printer."""
        if not os.path.exists(file_path):
            logger.error(f"File not found: {file_path}")
            return False

        printer = printer_name or self.default_printer
        if not printer:
            printers = self.get_printers()
            if printers:
                printer = list(printers.keys())[0]
                logger.info(f"Using first available printer: {printer}")
            else:
                logger.error("No printers available")
                return False

        job_title = job_name or os.path.basename(file_path)

        if not self.conn:
            logger.info(
                f"SIMULATION: Would print {file_path} to {printer} with title '{job_title}'"
            )
            return True

        try:
            job_id = self.conn.printFile(printer, file_path, job_title, {})
            logger.info(f"Print job {job_id} submitted to {printer}: {file_path}")
            return True
        except Exception as e:
            logger.error(f"Failed to print {file_path}: {e}")
            return False

    def is_printer_ready(self, printer_name: Optional[str] = None) -> bool:
        """Check if printer is ready to accept jobs."""
        if not self.conn:
            return True  # Simulator is always ready

        printer = printer_name or self.default_printer
        if not printer:
            return False

        try:
            printers = self.conn.getPrinters()
            if printer not in printers:
                return False

            printer_info = printers[printer]
            state = printer_info.get("printer-state", 0)
            # CUPS printer states: 3=idle, 4=processing, 5=stopped
            return state in [3, 4]
        except Exception as e:
            logger.error(f"Failed to check printer status: {e}")
            return False

    def add_printer(self, printer_config: Dict[str, str]) -> bool:
        """Add a printer to the CUPS server."""
        if not self.conn:
            logger.info(f"SIMULATION: Would add printer {printer_config['name']}")
            return True

        try:
            # Use driver if specified, otherwise let CUPS auto-detect (for IPP)
            ppdname = printer_config.get("driver") or None

            self.conn.addPrinter(
                name=printer_config["name"],
                device=printer_config["uri"],
                info=printer_config.get("description", ""),
                location=printer_config.get("location", ""),
                ppdname=ppdname,
            )
            logger.info(
                f"Added printer {printer_config['name']} with URI {printer_config['uri']}"
            )
            return True
        except Exception as e:
            logger.error(f"Failed to add printer {printer_config['name']}: {e}")
            return False
