#!/bin/sh

export LD_LIBRARY_PATH="/app/lib:${LD_LIBRARY_PATH}"

if [ "${CALIBRE__CREATE_LIBRARY}" = "true" ] && [ ! -f "${CALIBRE__LIBRARY}/metadata.db" ]; then
  # Populate a blank library
  /app/bin/calibredb --library-path="${CALIBRE__LIBRARY}" list
fi

#shellcheck disable=SC2086
exec \
  /app/bin/calibre-server \
  --port=${CALIBRE__PORT} \
  ${CALIBRE__LIBRARY} \
  "$@"
